package jobs

import (
	"strings"
	"testing"

	"subsenpai/backend/internal/sources"
)

func TestJadwalBlockKaya(t *testing.T) {
	it := sources.ScheduleItem{Title: "Uji Anime", Slug: "uji-anime", Time: "19:30", Episode: 5}
	m := &jadwalMeta{
		TitleAlt: "Test Anime Alt", Genres: []string{"Action", "Fantasy"},
		Studio: "Studio Uji", Season: "fall", Year: 2026,
		Synopsis: "Sinopsis singkat judul uji.", EpisodeID: "ep-123", LatestNo: 5,
	}
	blk := jadwalBlock(it, m)
	for _, want := range []string{
		"**Uji Anime** — Ep 5 • 19:30",
		"Judul lain: Test Anime Alt",
		"Genre: Action, Fantasy",
		"Studio: Studio Uji • Musim: Fall 2026",
		"Sinopsis singkat judul uji.",
		"▶️ Nonton Ep 5: https://www.subsenpai.web.id/nonton/ep-123",
	} {
		if !strings.Contains(blk, want) {
			t.Errorf("blok tidak memuat %q:\n%s", want, blk)
		}
	}
	// Tanpa metadata katalog: baris ringkas saja.
	ringkas := jadwalBlock(it, nil)
	if strings.Contains(ringkas, "Nonton") || strings.Contains(ringkas, "Genre") {
		t.Errorf("blok ringkas tidak boleh memuat metadata: %q", ringkas)
	}
	// Metadata tanpa episode tersedia: tanpa tautan nonton.
	m2 := &jadwalMeta{Genres: []string{"Drama"}}
	blk2 := jadwalBlock(it, m2)
	if strings.Contains(blk2, "Nonton") {
		t.Errorf("tanpa episode tidak boleh ada tautan: %q", blk2)
	}
	if !strings.Contains(blk2, "Genre: Drama") {
		t.Errorf("genre harus tetap tampil: %q", blk2)
	}
}

func TestPackJadwal(t *testing.T) {
	blocks := []string{
		strings.Repeat("a", 600), strings.Repeat("b", 600),
		strings.Repeat("c", 600), strings.Repeat("d", 100),
	}
	msgs := packJadwal("HEADER", blocks, "FOOTER", 1700)
	if len(msgs) < 2 {
		t.Fatalf("harus terpecah jadi >=2 pesan, dapat %d", len(msgs))
	}
	for i, m := range msgs {
		if len([]rune(m)) > 1700 {
			t.Errorf("pesan %d melebihi batas: %d", i, len([]rune(m)))
		}
	}
	if !strings.HasPrefix(msgs[0], "HEADER") {
		t.Errorf("pesan pertama harus diawali header: %.20q", msgs[0])
	}
	if !strings.HasSuffix(msgs[len(msgs)-1], "FOOTER") {
		t.Errorf("pesan terakhir harus diakhiri footer")
	}
	// Tanpa blok: header + footer saja dalam satu pesan.
	satu := packJadwal("HEADER", nil, "FOOTER", 1700)
	if len(satu) != 1 || !strings.Contains(satu[0], "HEADER") || !strings.Contains(satu[0], "FOOTER") {
		t.Errorf("kasus kosong janggal: %v", satu)
	}
}
