package httpapi

import (
	"testing"
	"time"
)

// Uji logika murni jalur on-demand: kunci per kunci, cooldown
// percobaan gagal, dan keputusan aspek yang perlu diisi.

func TestFillGuardSingleFlight(t *testing.T) {
	g := newFillGuard(time.Minute)
	if !g.tryStart("a") {
		t.Fatal("klaim pertama harus berhasil")
	}
	if g.tryStart("a") {
		t.Fatal("klaim kedua untuk kunci sama harus ditolak selagi berjalan")
	}
	if !g.tryStart("b") {
		t.Fatal("kunci lain tidak boleh terpengaruh")
	}
	g.finish("a", false)
	if !g.tryStart("a") {
		t.Fatal("sesudah finish sukses, klaim baru harus boleh")
	}
}

func TestFillGuardCooldown(t *testing.T) {
	now := time.Date(2026, 11, 22, 14, 0, 0, 0, time.UTC)
	g := newFillGuard(10 * time.Minute)
	g.now = func() time.Time { return now }

	if !g.tryStart("x") {
		t.Fatal("klaim pertama harus berhasil")
	}
	g.finish("x", true) // gagal → cooldown
	if g.tryStart("x") {
		t.Fatal("selama cooldown klaim harus ditolak")
	}
	now = now.Add(9 * time.Minute)
	if g.tryStart("x") {
		t.Fatal("cooldown 10 menit belum lewat pada menit ke-9")
	}
	now = now.Add(2 * time.Minute)
	if !g.tryStart("x") {
		t.Fatal("sesudah cooldown lewat klaim harus boleh lagi")
	}
	g.finish("x", false)
	if !g.tryStart("x") {
		t.Fatal("finish sukses tidak boleh memasang cooldown")
	}
}

func TestNeedsOnDemand(t *testing.T) {
	cases := []struct {
		name              string
		fetchMode, typ    string
		hasSynopsis       bool
		episodeCount      int
		wantMeta, wantEps bool
	}{
		{"server donghua kosong semua", "server", "donghua", false, 0, true, true},
		{"server anime sinopsis ada episode kosong", "server", "anime", true, 0, false, true},
		{"server film lengkap", "server", "film", true, 12, false, false},
		{"device anime tidak disentuh", "device", "anime", false, 0, false, false},
		{"server komik tidak disentuh", "server", "manhwa", false, 0, false, false},
		{"mode kosong tidak disentuh", "", "donghua", false, 0, false, false},
	}
	for _, c := range cases {
		m, e := needsOnDemand(c.fetchMode, c.typ, c.hasSynopsis, c.episodeCount)
		if m != c.wantMeta || e != c.wantEps {
			t.Errorf("%s: needsOnDemand = (%v,%v), mau (%v,%v)", c.name, m, e, c.wantMeta, c.wantEps)
		}
	}
}
