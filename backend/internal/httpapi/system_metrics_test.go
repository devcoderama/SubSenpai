package httpapi

import "testing"

func TestParseMeminfo(t *testing.T) {
	total, avail := parseMeminfo("MemTotal:       16384000 kB\nMemFree:         1024000 kB\nMemAvailable:    8192000 kB\nBuffers:          100000 kB\n")
	if total != 16384000*1024 || avail != 8192000*1024 {
		t.Fatalf("total=%d avail=%d", total, avail)
	}
}

func TestParseLoadavg(t *testing.T) {
	load := parseLoadavg("0.52 0.31 0.18 2/345 12345\n")
	if len(load) != 3 || load[0] != 0.52 || load[2] != 0.18 {
		t.Fatalf("load=%v", load)
	}
}

func TestParseCPUStatDanPercent(t *testing.T) {
	idle, total := parseCPUStat("cpu  100 0 100 800 50 0 10 0 0 0\ncpu0 1 2 3 4 5 6 7 8 9 10\n")
	if idle != 850 || total != 1060 {
		t.Fatalf("idle=%d total=%d", idle, total)
	}
	// Dari (850,1060) ke (900,1260): sibuk 150 dari 200 → 75%.
	if p := cpuPercent(850, 1060, 900, 1260); p != 75 {
		t.Fatalf("cpuPercent=%v, mau 75", p)
	}
	if p := cpuPercent(0, 0, 0, 0); p != 0 {
		t.Fatalf("cpuPercent nol=%v", p)
	}
}

func TestParseNetDev(t *testing.T) {
	content := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1000 10 0 0 0 0 0 0 1000 10 0 0 0 0 0 0
  eth0: 5000 50 0 0 0 0 0 0 3000 30 0 0 0 0 0 0
  eth1: 2000 20 0 0 0 0 0 0 1000 10 0 0 0 0 0 0
`
	rx, tx := parseNetDev(content)
	if rx != 7000 || tx != 4000 {
		t.Fatalf("rx=%d tx=%d, mau 7000/4000 (lo dikecualikan)", rx, tx)
	}
}

func TestParseStatmRSS(t *testing.T) {
	if got := parseStatmRSS("12345 2500 900 5 0 8000 0", 4096); got != 2500*4096 {
		t.Fatalf("rss=%d", got)
	}
}
