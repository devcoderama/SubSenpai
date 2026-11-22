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
	// Poster boleh kosong untuk Anichin: halaman serinya tidak membawa
	// og:image dan tidak ada featured media yang bisa dijangkau REST —
	// aplikasi menampilkan plakat judul sebagai gantinya. Cukup dicatat.
	if p := FetchPoster(ctx, found[0].URL); p == "" {
		t.Logf("poster untuk %s kosong (diterima — plakat judul di aplikasi)", found[0].URL)
	} else {
		t.Logf("poster contoh: %s", p)
	}
}
