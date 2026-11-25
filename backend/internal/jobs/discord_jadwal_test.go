package jobs

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"subsenpai/backend/internal/sources"
	"subsenpai/backend/internal/telegram"
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

// TestKirimJadwalTelegram: jadwal Telegram terkirim sebagai
// header + 1 pesan per judul + footer, semuanya ke topik jadwal;
// hanya pesan judul & footer yang pratinjaunya menyala (header
// dikirim dengan Send biasa).
func TestKirimJadwalTelegram(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var p map[string]any
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Errorf("payload bukan JSON: %v", err)
		}
		bodies = append(bodies, p)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer srv.Close()
	old := telegram.BaseURL
	telegram.BaseURL = srv.URL
	defer func() { telegram.BaseURL = old }()

	blocks := []string{
		"• **Judul A** — Ep 1\n▶️ Nonton Ep 1: https://www.subsenpai.web.id/nonton/ep-a",
		"• **Judul B** — Ep 2\n▶️ Nonton Ep 2: https://www.subsenpai.web.id/nonton/ep-b",
	}
	headerID, err := kirimJadwalTelegram(context.Background(), telegram.New("token-uji"),
		"-100123", "20730", "🗓️ **Jadwal Rilis — Selasa, 24 November 2026**",
		blocks, "Selamat menonton di SubSenpai! 🍿 https://www.subsenpai.web.id")
	if err != nil {
		t.Fatalf("kirimJadwalTelegram: %v", err)
	}
	if headerID != 1 {
		t.Fatalf("id header harus 1 (stub), dapat %d", headerID)
	}
	if len(bodies) != 4 {
		t.Fatalf("harus 4 pesan (header + 2 judul + footer), dapat %d", len(bodies))
	}
	wants := []struct {
		muat    string
		preview bool
	}{
		{"Jadwal Rilis — Selasa, 24 November 2026", false},
		{"Judul A", true},
		{"Judul B", true},
		{"Selamat menonton", true},
	}
	for i, w := range wants {
		p := bodies[i]
		if p["message_thread_id"] != float64(20730) {
			t.Errorf("pesan %d: thread id janggal: %v", i, p)
		}
		if !strings.Contains(p["text"].(string), w.muat) {
			t.Errorf("pesan %d: teks tidak memuat %q: %v", i, w.muat, p["text"])
		}
		if got := p["disable_web_page_preview"] == false; got != w.preview {
			t.Errorf("pesan %d: pratinjau menyala = %v, mau %v", i, got, w.preview)
		}
	}
}

// Grup jadwal kedua (permintaan Rams 2026-11-25): helper yang sama
// mengirim ke grup + topik berbeda dengan isi pesan yang sama —
// chat_id grup baru dan thread topik barunya harus tepat.
func TestKirimJadwalTelegramGrup2(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var p map[string]any
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Errorf("payload bukan JSON: %v", err)
		}
		bodies = append(bodies, p)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer srv.Close()
	old := telegram.BaseURL
	telegram.BaseURL = srv.URL
	defer func() { telegram.BaseURL = old }()

	blocks := []string{"• **Judul A** — Ep 1\n▶️ Nonton Ep 1: https://www.subsenpai.web.id/nonton/ep-a"}
	headerID, err := kirimJadwalTelegram(context.Background(), telegram.New("token-uji"),
		"-1002084839985", "65332", "🗓️ **Jadwal Rilis — Kamis, 26 November 2026**",
		blocks, "Selamat menonton di SubSenpai! 🍿 https://www.subsenpai.web.id")
	if err != nil {
		t.Fatalf("kirimJadwalTelegram grup 2: %v", err)
	}
	if headerID != 1 {
		t.Fatalf("id header grup 2 harus 1 (stub), dapat %d", headerID)
	}
	if len(bodies) != 3 {
		t.Fatalf("harus 3 pesan (header + 1 judul + footer), dapat %d", len(bodies))
	}
	for i, p := range bodies {
		if p["chat_id"] != "-1002084839985" {
			t.Errorf("pesan %d: chat_id janggal: %v", i, p["chat_id"])
		}
		if p["message_thread_id"] != float64(65332) {
			t.Errorf("pesan %d: thread id janggal: %v", i, p["message_thread_id"])
		}
	}
}
