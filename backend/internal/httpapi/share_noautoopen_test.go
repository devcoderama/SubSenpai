package httpapi

import (
	"html/template"
	"strings"
	"testing"
)

// TestSharePageNoAutoOpen: halaman bagikan TIDAK BOLEH menembak
// intent:// otomatis saat dibuka (regresi 2026-11-25: tembakan
// otomatis 0,6 detik tidak membuka aplikasi dari Chrome,
// melainkan mengikuti browser_fallback_url ke #download sehingga
// preview-nya tidak sempat dibaca). Aplikasi hanya dibuka saat
// pengguna mengetuk: tombol merah, bar bawah, atau poster.
// Tes ini tidak butuh database: cukup render templatnya.
func TestSharePageNoAutoOpen(t *testing.T) {
	data := sharePageData{
		Title:      "Film Uji — Episode 3",
		MetaLine:   "Episode 3 · 2026 · Anime",
		GenresLine: "Action • Drama",
		Synopsis:   "Sinopsis uji.",
		IntentURL:  template.URL("intent://episode/00000000-0000-0000-0000-000000000000#Intent;scheme=subsenpai;package=com.subsenpai;S.browser_fallback_url=https%3A%2F%2Fwww.subsenpai.web.id%2F%23download;end"),
		SchemeURL:  template.JS(`"subsenpai://episode/00000000-0000-0000-0000-000000000000"`),
		PosterURL:  "https://cdn.example.com/poster.jpg",
		Found:      true,
	}
	var b strings.Builder
	if err := shareTpl.Execute(&b, data); err != nil {
		t.Fatal(err)
	}
	body := b.String()
	if strings.Contains(body, "go(intent); }, 600") ||
		strings.Contains(body, "go(scheme); }, 1700") {
		t.Errorf("halaman masih menembak buka-aplikasi otomatis:\n%s", body)
	}
	// Cadangan 500 mdtk di DALAM penangan ketuk harus tetap ada.
	if !strings.Contains(body, "go(scheme); }, 500") {
		t.Errorf("penangan ketuk kehilangan cadangan skema 500 mdtk")
	}
	for _, want := range []string{
		`bindOpen("openBtn")`,
		`bindOpen("openBtn2")`,
		`bindOpen("playBtn")`,
		`id="playBtn"`,
		`class="stickybar"`,
		"intent://episode/",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("halaman tidak memuat %q", want)
		}
	}
}
