package sources

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestDiscoverAnichinLive adalah uji asap terhadap situs asli — hanya
// jalan bila SUBSENPAI_LIVE_TEST=1 (tidak jalan di CI/pengujian biasa).
func TestDiscoverAnichinLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	r, ok := ByKey("anichin")
	if !ok {
		t.Fatal("resolver anichin tidak terdaftar")
	}
	disc, ok := r.(Discoverer)
	if !ok {
		t.Fatal("anichin tidak mendukung Discover")
	}
	src := Source{Name: "Anichin", ContentType: "donghua", Lang: "id", BaseURL: "https://anichin.my", Resolver: "anichin"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	found, err := disc.Discover(ctx, src, 600)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	t.Logf("total ditemukan: %d", len(found))
	for i, d := range found {
		if i >= 8 {
			break
		}
		t.Logf("- %s | %s | %s", d.Title, d.Slug, d.URL)
	}
	if len(found) < 50 {
		t.Fatalf("terlalu sedikit judul ditemukan: %d", len(found))
	}
	// Peta poster dari mirror anichin.moe harus berisi seri-seri aktif.
	pp, ok := r.(PosterProvider)
	if !ok {
		t.Fatal("anichin tidak mendukung PosterProvider")
	}
	pmap, err := pp.Posters(ctx, src)
	if err != nil {
		t.Fatalf("Posters: %v", err)
	}
	t.Logf("peta poster: %d entri", len(pmap))
	if len(pmap) < 10 {
		t.Fatalf("peta poster terlalu sedikit: %d", len(pmap))
	}
	for k, v := range pmap {
		t.Logf("poster %q -> %s", k, v)
		break
	}
}

// TestDiscoverManhuarmtlLive menguji penemuan komik dari API publik
// manhuarmtl (hanya jalan bila SUBSENPAI_LIVE_TEST=1).
func TestDiscoverManhuarmtlLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	r, ok := ByKey("manhuarmtl")
	if !ok {
		t.Fatal("resolver manhuarmtl tidak terdaftar")
	}
	disc, ok := r.(Discoverer)
	if !ok {
		t.Fatal("manhuarmtl tidak mendukung Discover")
	}
	src := Source{Name: "ManhuaRMTL", ContentType: "manga", Lang: "en", BaseURL: "https://manhuarmtl.com", Resolver: "manhuarmtl", FetchMode: "device"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	found, err := disc.Discover(ctx, src, 48)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	t.Logf("total ditemukan: %d", len(found))
	withPoster, withType := 0, 0
	for _, d := range found {
		if d.Poster != "" {
			withPoster++
		}
		if d.Type != "" {
			withType++
		}
	}
	t.Logf("berposter: %d, bertype: %d", withPoster, withType)
	if len(found) < 20 || withPoster < 10 || withType < 20 {
		t.Fatalf("hasil penemuan komik terlalu tipis: %d judul", len(found))
	}
}
