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
	src := Source{Name: "Anichin", ContentType: "donghua", Lang: "id", BaseURL: "https://anichin.moe", Resolver: "anichin"}
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

func TestDiscoverHashflixLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("live test nonaktif")
	}
	r, ok := ByKey("hashflix")
	if !ok {
		t.Fatal("resolver hashflix tidak terdaftar")
	}
	disc, ok := r.(Discoverer)
	if !ok {
		t.Fatal("hashflix tidak mendukung Discover")
	}
	src := Source{Name: "Hashflix", ContentType: "film", Lang: "multi", BaseURL: "https://hashflix.top", Resolver: "hashflix", FetchMode: "server"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	items, err := disc.Discover(ctx, src, 60)
	if err != nil {
		t.Fatalf("discover hashflix: %v", err)
	}
	posters := 0
	for _, it := range items {
		if it.Poster != "" {
			posters++
		}
	}
	t.Logf("hashflix: %d film, %d berposter; contoh: %+v", len(items), posters, items[0])
	if len(items) < 20 || posters < 15 {
		t.Fatalf("hasil terlalu sedikit: %d film, %d poster", len(items), posters)
	}
}

// TestEpisodesAnichinMoeLive memverifikasi cadangan eplister HTML:
// daftar episode "Tales of Herding Gods" dari anichin.moe.
func TestEpisodesAnichinMoeLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	r, ok := ByKey("anichin")
	if !ok {
		t.Fatal("resolver anichin tidak terdaftar")
	}
	src := Source{Name: "Anichin", ContentType: "donghua", Lang: "id", BaseURL: "https://anichin.moe", Resolver: "anichin", FetchMode: "server"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	eps, err := r.Episodes(ctx, src, "https://anichin.moe/tales-of-herding-god/")
	if err != nil {
		t.Fatalf("Episodes: %v", err)
	}
	t.Logf("episode Tales of Herding Gods: %d (pertama %d, terakhir %d)", len(eps), eps[0].Number, eps[len(eps)-1].Number)
	if len(eps) < 50 {
		t.Fatalf("episode terlalu sedikit: %d", len(eps))
	}
	res, err := r.Resolve(ctx, src, eps[len(eps)-1])
	if err != nil {
		t.Fatalf("Resolve ep terakhir: %v", err)
	}
	t.Logf("aliran ep %d: %d server; pertama: %s (%s)", eps[len(eps)-1].Number, len(res.Streams), res.Streams[0].Label, res.Streams[0].URL)
	if len(res.Streams) < 3 {
		t.Fatalf("server terlalu sedikit: %d", len(res.Streams))
	}
}

// TestSeriesInfoAnichinLive memverifikasi parser metadata seri
// (blok info Animestream) pada "Tales of Herding Gods".
func TestSeriesInfoAnichinLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	r, ok := ByKey("anichin")
	if !ok {
		t.Fatal("resolver anichin tidak terdaftar")
	}
	infoer, ok := r.(SeriesInfoer)
	if !ok {
		t.Fatal("anichin tidak mendukung SeriesInfo")
	}
	src := Source{Name: "Anichin", ContentType: "donghua", Lang: "id", BaseURL: "https://anichin.moe", Resolver: "anichin", FetchMode: "server"}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	meta, err := infoer.SeriesInfo(ctx, src, "https://anichin.moe/tales-of-herding-god/")
	if err != nil {
		t.Fatalf("SeriesInfo: %v", err)
	}
	t.Logf("meta: %+v | sinopsis %.80q", *meta, meta.Synopsis)
	if meta.Network != "Bilibili" || meta.Studio != "Sparkly Key Animation Studio" ||
		meta.Country != "China" || meta.DurationMin != 20 || meta.Year != 2024 ||
		meta.Season != "fall" || len(meta.Genres) != 3 || meta.Synopsis == "" {
		t.Fatalf("metadata tidak sesuai halaman sumber: %+v", *meta)
	}
}
