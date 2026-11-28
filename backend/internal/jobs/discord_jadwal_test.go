package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/db"
	"subsenpai/backend/internal/discord"
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

// TestParseJadwalJam: hanya "HH:MM" yang dijadwalkan per slot;
// "Sudah Rilis"/"??"/kosong/tak valid masuk batch pagi.
func TestParseJadwalJam(t *testing.T) {
	cases := []struct {
		in     string
		hh, mm int
		ok     bool
	}{
		{"05:00", 5, 0, true},
		{"5:00", 5, 0, true},
		{"16:26", 16, 26, true},
		{"23:59", 23, 59, true},
		{"Sudah Rilis", 0, 0, false},
		{"??", 0, 0, false},
		{"", 0, 0, false},
		{"24:00", 0, 0, false},
		{"12:60", 0, 0, false},
		{"jam 05:00", 0, 0, false},
	}
	for _, c := range cases {
		hh, mm, ok := parseJadwalJam(c.in)
		if hh != c.hh || mm != c.mm || ok != c.ok {
			t.Errorf("parseJadwalJam(%q) = %d:%d,%v; mau %d:%d,%v",
				c.in, hh, mm, ok, c.hh, c.mm, c.ok)
		}
	}
}

// TestSusunJadwalHariIni: urutan kirim = tanpa jam dulu, lalu
// jam naik; yang sudah tiba waktunya saja yang due.
func TestSusunJadwalHariIni(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600)
	now := time.Date(2026, 11, 28, 5, 30, 0, 0, loc) // Sabtu 05:30 WIB
	enrich := func(string) (jadwalMeta, bool) { return jadwalMeta{}, false }
	anime := []sources.ScheduleGroup{{
		Day: "Sabtu",
		Items: []sources.ScheduleItem{
			{Title: "Anime Pagi", Slug: "anime-pagi", Time: "05:00", Episode: 1},
			{Title: "Anime Sore", Slug: "anime-sore", Time: "16:00", Episode: 2},
			{Title: "Anime Rilis", Slug: "anime-rilis", Time: "Sudah Rilis", Episode: 3},
		},
	}, {
		Day:   "Jumat",
		Items: []sources.ScheduleItem{{Title: "Kemarin", Slug: "kemarin", Time: "05:00", Episode: 1}},
	}}
	donghua := []sources.ScheduleGroup{{
		Day: "Sabtu",
		Items: []sources.ScheduleItem{
			{Title: "Donghua Acak", Slug: "donghua-acak", Time: "??", Episode: 1},
			{Title: "Donghua Siang", Slug: "donghua-siang", Time: "12:00", Episode: 4},
		},
	}}
	items := susunJadwalHariIni("Sabtu", now, loc, enrich, anime, donghua)
	if len(items) != 5 {
		t.Fatalf("harus 5 item hari ini, dapat %d", len(items))
	}
	// Urutan: tanpa jam (rilis, acak — urut slug), lalu 05:00, 12:00, 16:00.
	wantOrder := []string{"anime-rilis", "donghua-acak", "anime-pagi", "donghua-siang", "anime-sore"}
	for i, w := range wantOrder {
		if items[i].slug != w {
			t.Errorf("urutan [%d] = %q, mau %q", i, items[i].slug, w)
		}
	}
	// Tiap pesan membawa label seksinya sendiri (pesan berdiri sendiri).
	if !strings.HasPrefix(items[0].pesan, "**🎌 Anime**") {
		t.Errorf("pesan anime harus diawali label seksi: %.40q", items[0].pesan)
	}
	if !strings.HasPrefix(items[1].pesan, "**🐉 Donghua**") {
		t.Errorf("pesan donghua harus diawali label seksi: %.40q", items[1].pesan)
	}
	due := jadwalDue(items, now)
	if len(due) != 3 {
		t.Fatalf("jam 05:30 harus 3 due (tanpa jam + 05:00), dapat %d", len(due))
	}
	for _, d := range due {
		if d.slug == "anime-sore" || d.slug == "donghua-siang" {
			t.Errorf("judul jam %s belum boleh due jam 05:30", d.slug)
		}
	}
	// Jam 16:30: semua due.
	late := jadwalDue(items, now.Add(11*time.Hour))
	if len(late) != 5 {
		t.Errorf("jam 16:30 harus 5 due, dapat %d", len(late))
	}
}

// TestRetryAfterDariError: angka retry_after dibaca dari badan
// error 429 Telegram (int) maupun Discord (float).
func TestRetryAfterDariError(t *testing.T) {
	cases := []struct {
		err  error
		want time.Duration
	}{
		{fmt.Errorf(`telegram: status 429: {"ok":false,"error_code":429,"description":"Too Many Requests: retry after 42","parameters":{"retry_after":42}}`), 42 * time.Second},
		{fmt.Errorf(`discord: status 429: {"message": "You are being rate limited.", "retry_after": 1.5}`), 1500 * time.Millisecond},
		{fmt.Errorf("telegram: status 500: boom"), 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := retryAfterDariError(c.err); got != c.want {
			t.Errorf("retryAfterDariError(%v) = %v, mau %v", c.err, got, c.want)
		}
	}
}

// TestKirimJadwalJudulTelegram: satu judul = SATU request
// SendPreview (pratinjau menyala) ke topik jadwal yang tepat.
func TestKirimJadwalJudulTelegram(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var p map[string]any
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Errorf("payload bukan JSON: %v", err)
		}
		bodies = append(bodies, p)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
	}))
	defer srv.Close()
	old := telegram.BaseURL
	telegram.BaseURL = srv.URL
	defer func() { telegram.BaseURL = old }()

	err := kirimJadwalJudulTelegram(context.Background(), telegram.New("token-uji"),
		"-100123", "20730", "**🎌 Anime**\n\n• **Judul A** — Ep 1 • 05:00")
	if err != nil {
		t.Fatalf("kirimJadwalJudulTelegram: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("harus 1 pesan, dapat %d", len(bodies))
	}
	p := bodies[0]
	if p["message_thread_id"] != float64(20730) {
		t.Errorf("thread id janggal: %v", p["message_thread_id"])
	}
	if p["disable_web_page_preview"] != false {
		t.Errorf("pratinjau harus menyala (disable_web_page_preview=false)")
	}
	if !strings.Contains(p["text"].(string), "Judul A") {
		t.Errorf("teks tidak memuat judul: %v", p["text"])
	}
}

// TestKirimJadwalJudulDiscord: satu judul = SATU request ke channel
// yang tepat dengan header otorisasi bot.
func TestKirimJadwalJudulDiscord(t *testing.T) {
	var bodies []map[string]string
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		var p map[string]string
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Errorf("payload bukan JSON: %v", err)
		}
		bodies = append(bodies, p)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()
	old := discord.BaseURL
	discord.BaseURL = srv.URL
	defer func() { discord.BaseURL = old }()

	err := kirimJadwalJudulDiscord(context.Background(), discord.New("token-uji"),
		"chan-1", "**🎌 Anime**\n\n• **Judul A** — Ep 1 • 05:00")
	if err != nil {
		t.Fatalf("kirimJadwalJudulDiscord: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("harus 1 pesan, dapat %d", len(bodies))
	}
	if auth != "Bot token-uji" {
		t.Errorf("Authorization janggal: %q", auth)
	}
	if !strings.Contains(bodies[0]["content"], "Judul A") {
		t.Errorf("content tidak memuat judul: %v", bodies[0])
	}
}

// TestJadwalNotifiedLive: penanda per judul di tabel jadwal_notified
// (migrasi 0027) — belum ditandai -> tandai -> sudah ditandai;
// tandai ulang aman (idempotent, tanpa error).
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestJadwalNotifiedLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrasi: %v", err)
	}
	kanal, hari, slug := "telegram", "2000-01-01", "uji-jadwal-dobel"
	defer pool.Exec(ctx, `delete from jadwal_notified where channel = $1 and day = $2 and slug = $3`,
		kanal, hari, slug)
	if jadwalSudahTerkirim(ctx, pool, kanal, hari, slug) {
		t.Fatal("sebelum ditandai harus belum terkirim")
	}
	jadwalTandaiTerkirim(ctx, pool, kanal, hari, slug)
	if !jadwalSudahTerkirim(ctx, pool, kanal, hari, slug) {
		t.Fatal("sesudah ditandai harus sudah terkirim")
	}
	jadwalTandaiTerkirim(ctx, pool, kanal, hari, slug) // ulang: aman
	if !jadwalSudahTerkirim(ctx, pool, kanal, hari, slug) {
		t.Fatal("tandai ulang tidak boleh menghapus penanda")
	}
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from jadwal_notified
		where channel = $1 and day = $2 and slug = $3`, kanal, hari, slug).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("harus tepat 1 baris penanda, dapat %d", n)
	}
}
