package sources

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestSearchTitlesAnichinLive menguji pencarian HTML Anichin (/?s=) —
// jalur yang dipakai syncTitle untuk membuat mapping baru sejak
// perbaikan 2026-11-26 (wp-json mati 403 dari VPS). Base .my sengaja
// dipakai untuk membuktikan moeBase menormalisasi ke .moe.
func TestSearchTitlesAnichinLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	r, ok := ByKey("anichin")
	if !ok {
		t.Fatal("resolver anichin tidak terdaftar")
	}
	hs, ok := r.(TitleSearcher)
	if !ok {
		t.Fatal("anichin tidak mendukung SearchTitles")
	}
	src := Source{Name: "Anichin", ContentType: "donghua", Lang: "id", BaseURL: "https://anichin.my", Resolver: "anichin"}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	items, err := hs.SearchTitles(ctx, src, "Tales of Herding Gods")
	if err != nil {
		t.Fatalf("SearchTitles: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("SearchTitles mengembalikan 0 hasil")
	}
	for _, it := range items {
		if strings.Contains(it.URL, "anichin.my") {
			t.Fatalf("hasil masih memakai .my: %s", it.URL)
		}
	}
	if !strings.Contains(items[0].URL, "anichin.moe") {
		t.Fatalf("hasil pertama bukan .moe: %s", items[0].URL)
	}
	t.Logf("hasil pertama: %s → %s", items[0].Title, items[0].URL)
}

// TestEpisodesAnichinMissingLive memastikan Episodes Anichin untuk seri
// yang tidak ada TIDAK lagi jatuh ke wp-json (dulu: HTTP 403 yang
// menyesatkan) — sejak perbaikan 2026-11-26 fallback WP dicabut.
func TestEpisodesAnichinMissingLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("uji langsung dilewati (SUBSENPAI_LIVE_TEST != 1)")
	}
	r, ok := ByKey("anichin")
	if !ok {
		t.Fatal("resolver anichin tidak terdaftar")
	}
	src := Source{Name: "Anichin", ContentType: "donghua", Lang: "id", BaseURL: "https://anichin.moe", Resolver: "anichin"}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := r.Episodes(ctx, src, "https://anichin.moe/seri-tidak-ada-xyz123/")
	if err == nil {
		t.Fatal("Episodes seri fiktif seharusnya error")
	}
	if strings.Contains(err.Error(), "wp-json") {
		t.Fatalf("error masih menyebut wp-json (fallback belum dicabut?): %v", err)
	}
	t.Logf("error: %v", err)
}
