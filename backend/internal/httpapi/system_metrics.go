package httpapi

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Metrik sumber daya server untuk halaman Sistem dashboard admin
// (permintaan Rams 2026-11-23 malam: pantau CPU, RAM, disk, dan
// bandwidth dari dashboard; tampilannya kemudian diminta ala
// konsol monitoring — gauge + grafik riwayat). Semua dibaca dari
// /proc + syscall Linux, tanpa dependensi luar. Sebuah sampler
// latar mengambil sampel tiap 5 detik dan menyimpan 30 menit
// riwayat di memori (untuk sparkline); persen CPU dan laju
// (jaringan + disk) dihitung dari selisih dua sampel berurutan.

type serverMetrics struct {
	Hostname        string    `json:"hostname"`
	OS              string    `json:"os"`
	NumCPU          int       `json:"num_cpu"`
	UptimeSeconds   int64     `json:"uptime_seconds"`
	Load            []float64 `json:"load"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemTotalBytes   uint64    `json:"mem_total_bytes"`
	MemUsedBytes    uint64    `json:"mem_used_bytes"`
	MemPercent      float64   `json:"mem_percent"`
	DiskTotalBytes  uint64    `json:"disk_total_bytes"`
	DiskUsedBytes   uint64    `json:"disk_used_bytes"`
	DiskPercent     float64   `json:"disk_percent"`
	DiskReadRate    uint64    `json:"disk_read_rate"`  // byte/detik
	DiskWriteRate   uint64    `json:"disk_write_rate"` // byte/detik
	NetRxBytes      uint64    `json:"net_rx_bytes"`
	NetTxBytes      uint64    `json:"net_tx_bytes"`
	NetRxRate       uint64    `json:"net_rx_rate"` // byte/detik
	NetTxRate       uint64    `json:"net_tx_rate"`
	ProcessMemBytes uint64    `json:"process_mem_bytes"`
	Goroutines      int       `json:"goroutines"`
}

// metricPoint adalah satu titik riwayat untuk grafik sparkline.
type metricPoint struct {
	T             int64   `json:"t"` // unix detik
	CPU           float64 `json:"cpu"`
	Mem           float64 `json:"mem"`
	Load1         float64 `json:"load1"`
	RxRate        uint64  `json:"rx_rate"`
	TxRate        uint64  `json:"tx_rate"`
	DiskReadRate  uint64  `json:"disk_read_rate"`
	DiskWriteRate uint64  `json:"disk_write_rate"`
}

const historyCap = 360 // 360 titik x 5 detik = 30 menit

var metricsState = struct {
	sync.Mutex
	started             bool
	hasPrev             bool
	idle, total         uint64
	rx, tx              uint64
	diskRead, diskWrite uint64
	at                  time.Time
	latest              serverMetrics
	history             []metricPoint
}{}

func readProc(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

// parseMeminfo mengembalikan (total, tersedia) dalam byte.
func parseMeminfo(content string) (uint64, uint64) {
	var total, avail uint64
	for _, line := range strings.Split(content, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		kb, _ := strconv.ParseUint(f[1], 10, 64)
		switch f[0] {
		case "MemTotal:":
			total = kb * 1024
		case "MemAvailable:":
			avail = kb * 1024
		}
	}
	return total, avail
}

func parseLoadavg(content string) []float64 {
	f := strings.Fields(content)
	out := make([]float64, 0, 3)
	for i := 0; i < 3 && i < len(f); i++ {
		v, _ := strconv.ParseFloat(f[i], 64)
		out = append(out, v)
	}
	return out
}

// parseCPUStat membaca baris "cpu" agregat /proc/stat →
// (idle, total) dalam jiffies. idle mencakup iowait.
func parseCPUStat(content string) (uint64, uint64) {
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		f := strings.Fields(line)[1:]
		var total, idle uint64
		for i, s := range f {
			v, _ := strconv.ParseUint(s, 10, 64)
			total += v
			if i == 3 || i == 4 { // idle + iowait
				idle += v
			}
		}
		return idle, total
	}
	return 0, 0
}

// cpuPercent menghitung persen sibuk CPU dari dua sampel.
func cpuPercent(prevIdle, prevTotal, idle, total uint64) float64 {
	dt := total - prevTotal
	if dt == 0 {
		return 0
	}
	return float64(dt-(idle-prevIdle)) / float64(dt) * 100
}

// parseNetDev menjumlahkan byte rx/tx semua antarmuka kecuali
// loopback dari /proc/net/dev.
func parseNetDev(content string) (uint64, uint64) {
	var rx, tx uint64
	for _, line := range strings.Split(content, "\n") {
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		if strings.TrimSpace(line[:i]) == "lo" {
			continue
		}
		f := strings.Fields(line[i+1:])
		if len(f) < 9 {
			continue
		}
		r, _ := strconv.ParseUint(f[0], 10, 64)
		t, _ := strconv.ParseUint(f[8], 10, 64)
		rx += r
		tx += t
	}
	return rx, tx
}

// wholeDisk menyaring nama perangkat blok utuh (bukan partisi,
// bukan loop/ram/dm) agar sektor disk tidak terhitung ganda.
func wholeDisk(name string) bool {
	if strings.HasPrefix(name, "loop") || strings.HasPrefix(name, "ram") ||
		strings.HasPrefix(name, "dm-") {
		return false
	}
	if strings.HasPrefix(name, "nvme") || strings.HasPrefix(name, "mmcblk") {
		return !strings.Contains(name, "p")
	}
	// sd/vd/xvd/hd: disk utuh berakhiran huruf (sda), partisi angka (sda1).
	if len(name) == 0 {
		return false
	}
	c := name[len(name)-1]
	return c < '0' || c > '9'
}

// parseDiskstats menjumlahkan byte baca/tulis disk fisik utuh
// dari /proc/diskstats (sektor x 512).
func parseDiskstats(content string) (uint64, uint64) {
	var read, written uint64
	for _, line := range strings.Split(content, "\n") {
		f := strings.Fields(line)
		if len(f) < 10 || !wholeDisk(f[2]) {
			continue
		}
		// f[3]=reads completed, f[5]=sectors read,
		// f[7]=writes completed, f[9]=sectors written.
		sr, _ := strconv.ParseUint(f[5], 10, 64)
		sw, _ := strconv.ParseUint(f[9], 10, 64)
		read += sr * 512
		written += sw * 512
	}
	return read, written
}

// parseStatmRSS membaca RSS proses dari /proc/self/statm
// (field ke-2, dalam halaman).
func parseStatmRSS(content string, pageSize int) uint64 {
	f := strings.Fields(content)
	if len(f) < 2 {
		return 0
	}
	pages, _ := strconv.ParseUint(f[1], 10, 64)
	return pages * uint64(pageSize)
}

// sampleMetrics mengambil satu sampel; mengisi metricsState.latest
// dan menambahkan titik riwayat. Panggilan pertama tanpa sampel
// sebelumnya mengambil dua bacaan berurutan singkat agar persen
// CPU dan laju langsung punya angka.
func sampleMetrics() {
	idle, total := parseCPUStat(readProc("/proc/stat"))
	rx, tx := parseNetDev(readProc("/proc/net/dev"))
	dRead, dWrite := parseDiskstats(readProc("/proc/diskstats"))

	metricsState.Lock()
	needSecond := !metricsState.hasPrev
	metricsState.Unlock()
	if needSecond {
		metricsState.Lock()
		metricsState.idle, metricsState.total = idle, total
		metricsState.rx, metricsState.tx = rx, tx
		metricsState.diskRead, metricsState.diskWrite = dRead, dWrite
		metricsState.at = time.Now()
		metricsState.hasPrev = true
		metricsState.Unlock()
		time.Sleep(200 * time.Millisecond)
		idle, total = parseCPUStat(readProc("/proc/stat"))
		rx, tx = parseNetDev(readProc("/proc/net/dev"))
		dRead, dWrite = parseDiskstats(readProc("/proc/diskstats"))
	}

	m := serverMetrics{OS: runtime.GOOS + "/" + runtime.GOARCH, NumCPU: runtime.NumCPU()}
	m.Hostname, _ = os.Hostname()
	if f := strings.Fields(readProc("/proc/uptime")); len(f) > 0 {
		if v, err := strconv.ParseFloat(f[0], 64); err == nil {
			m.UptimeSeconds = int64(v)
		}
	}
	m.Load = parseLoadavg(readProc("/proc/loadavg"))
	if t, avail := parseMeminfo(readProc("/proc/meminfo")); t > 0 {
		m.MemTotalBytes = t
		m.MemUsedBytes = t - avail
		m.MemPercent = float64(m.MemUsedBytes) / float64(t) * 100
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err == nil && fs.Blocks > 0 {
		m.DiskTotalBytes = fs.Blocks * uint64(fs.Bsize)
		m.DiskUsedBytes = m.DiskTotalBytes - fs.Bfree*uint64(fs.Bsize)
		m.DiskPercent = float64(m.DiskUsedBytes) / float64(m.DiskTotalBytes) * 100
	}
	m.NetRxBytes, m.NetTxBytes = rx, tx
	m.ProcessMemBytes = parseStatmRSS(readProc("/proc/self/statm"), os.Getpagesize())
	m.Goroutines = runtime.NumGoroutine()

	metricsState.Lock()
	defer metricsState.Unlock()
	if dt := time.Since(metricsState.at).Seconds(); dt > 0 {
		m.CPUPercent = cpuPercent(metricsState.idle, metricsState.total, idle, total)
		if rx >= metricsState.rx {
			m.NetRxRate = uint64(float64(rx-metricsState.rx) / dt)
		}
		if tx >= metricsState.tx {
			m.NetTxRate = uint64(float64(tx-metricsState.tx) / dt)
		}
		if dRead >= metricsState.diskRead {
			m.DiskReadRate = uint64(float64(dRead-metricsState.diskRead) / dt)
		}
		if dWrite >= metricsState.diskWrite {
			m.DiskWriteRate = uint64(float64(dWrite-metricsState.diskWrite) / dt)
		}
	}
	metricsState.idle, metricsState.total = idle, total
	metricsState.rx, metricsState.tx = rx, tx
	metricsState.diskRead, metricsState.diskWrite = dRead, dWrite
	metricsState.at = time.Now()
	metricsState.latest = m

	load1 := 0.0
	if len(m.Load) > 0 {
		load1 = m.Load[0]
	}
	metricsState.history = append(metricsState.history, metricPoint{
		T: time.Now().Unix(), CPU: m.CPUPercent, Mem: m.MemPercent,
		Load1: load1, RxRate: m.NetRxRate, TxRate: m.NetTxRate,
		DiskReadRate: m.DiskReadRate, DiskWriteRate: m.DiskWriteRate,
	})
	if len(metricsState.history) > historyCap {
		metricsState.history = metricsState.history[len(metricsState.history)-historyCap:]
	}
}

// ensureSampler menyalakan sampler latar (sekali saja) — dimulai
// malas saat halaman Sistem pertama kali dibuka.
func ensureSampler() {
	metricsState.Lock()
	if metricsState.started {
		metricsState.Unlock()
		return
	}
	metricsState.started = true
	metricsState.Unlock()
	go func() {
		for {
			sampleMetrics()
			time.Sleep(5 * time.Second)
		}
	}()
}

// collectServerMetrics mengembalikan sampel terbaru + riwayat
// untuk grafik. Sampel pertama diambil sinkron agar respons
// pertama tidak kosong.
func collectServerMetrics() (serverMetrics, []metricPoint) {
	ensureSampler()
	metricsState.Lock()
	fresh := metricsState.at.IsZero()
	metricsState.Unlock()
	if fresh {
		sampleMetrics()
	}
	metricsState.Lock()
	defer metricsState.Unlock()
	hist := make([]metricPoint, len(metricsState.history))
	copy(hist, metricsState.history)
	return metricsState.latest, hist
}
