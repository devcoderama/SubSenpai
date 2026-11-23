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
// bandwidth dari dashboard). Semua dibaca dari /proc + syscall
// Linux — tanpa dependensi luar. CPU% dan laju jaringan dihitung
// dari selisih dua sampel berurutan yang disimpan di paket ini;
// panggilan pertama sesudah proses start mengambil dua sampel
// berurutan singkat (200 mdtk) agar langsung punya angka.

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
	NetRxBytes      uint64    `json:"net_rx_bytes"`
	NetTxBytes      uint64    `json:"net_tx_bytes"`
	NetRxRate       uint64    `json:"net_rx_rate"` // byte/detik sejak sampel sebelumnya
	NetTxRate       uint64    `json:"net_tx_rate"`
	ProcessMemBytes uint64    `json:"process_mem_bytes"`
	Goroutines      int       `json:"goroutines"`
}

var metricSample = struct {
	sync.Mutex
	idle, total uint64
	rx, tx      uint64
	at          time.Time
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

func collectServerMetrics() serverMetrics {
	m := serverMetrics{OS: runtime.GOOS + "/" + runtime.GOARCH, NumCPU: runtime.NumCPU()}
	m.Hostname, _ = os.Hostname()
	if f := strings.Fields(readProc("/proc/uptime")); len(f) > 0 {
		if v, err := strconv.ParseFloat(f[0], 64); err == nil {
			m.UptimeSeconds = int64(v)
		}
	}
	m.Load = parseLoadavg(readProc("/proc/loadavg"))
	if total, avail := parseMeminfo(readProc("/proc/meminfo")); total > 0 {
		m.MemTotalBytes = total
		m.MemUsedBytes = total - avail
		m.MemPercent = float64(m.MemUsedBytes) / float64(total) * 100
	}
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err == nil && fs.Blocks > 0 {
		m.DiskTotalBytes = fs.Blocks * uint64(fs.Bsize)
		free := fs.Bfree * uint64(fs.Bsize)
		m.DiskUsedBytes = m.DiskTotalBytes - free
		m.DiskPercent = float64(m.DiskUsedBytes) / float64(m.DiskTotalBytes) * 100
	}

	// CPU% + laju jaringan dari selisih sampel.
	idle, total := parseCPUStat(readProc("/proc/stat"))
	rx, tx := parseNetDev(readProc("/proc/net/dev"))
	metricSample.Lock()
	if metricSample.total == 0 {
		// Sampel pertama: simpan, tunggu sebentar, ambil lagi.
		metricSample.idle, metricSample.total = idle, total
		metricSample.rx, metricSample.tx = rx, tx
		metricSample.at = time.Now()
		metricSample.Unlock()
		time.Sleep(200 * time.Millisecond)
		idle, total = parseCPUStat(readProc("/proc/stat"))
		rx, tx = parseNetDev(readProc("/proc/net/dev"))
		metricSample.Lock()
	}
	if metricSample.total > 0 {
		m.CPUPercent = cpuPercent(metricSample.idle, metricSample.total, idle, total)
	}
	if dt := time.Since(metricSample.at).Seconds(); dt > 0 && metricSample.at.Year() > 1 {
		if rx >= metricSample.rx {
			m.NetRxRate = uint64(float64(rx-metricSample.rx) / dt)
		}
		if tx >= metricSample.tx {
			m.NetTxRate = uint64(float64(tx-metricSample.tx) / dt)
		}
	}
	metricSample.idle, metricSample.total = idle, total
	metricSample.rx, metricSample.tx = rx, tx
	metricSample.at = time.Now()
	metricSample.Unlock()
	m.NetRxBytes, m.NetTxBytes = rx, tx

	m.ProcessMemBytes = parseStatmRSS(readProc("/proc/self/statm"), os.Getpagesize())
	m.Goroutines = runtime.NumGoroutine()
	return m
}
