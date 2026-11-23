package jobs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/discord"
	"subsenpai/backend/internal/telegram"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.24.11", "0.24.10", 1},
		{"0.24.10", "0.24.11", -1},
		{"0.24.11", "0.24.11", 0},
		{"1.0", "0.99.99", 1},
		{"0.24", "0.24.0", 0},
		{"0.24.9", "0.24.11", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, mau %d", c.a, c.b, got, c.want)
		}
	}
}

// Server GitHub tiruan: rilis pertama draf (dilewati), kedua tanpa
// APK (dilewati), ketiga yang dipakai.
func fakeGitHub(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[
			{"tag_name":"v9.9.10","draft":true,"assets":[{"name":"SubSenpai-v9.9.10.apk"}]},
			{"tag_name":"v9.9.9","draft":false,"assets":[{"name":"catatan.txt"}]},
			{"tag_name":"v9.9.8","draft":false,"assets":[{"name":"SubSenpai-v9.9.8.apk"}]}
		]`)
	}))
}

func TestLatestReleaseVersion(t *testing.T) {
	srv := fakeGitHub(t)
	defer srv.Close()
	old := GitHubAPIBase
	GitHubAPIBase = srv.URL
	defer func() { GitHubAPIBase = old }()
	v, err := latestReleaseVersion(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != "9.9.8" {
		t.Fatalf("versi = %q, mau 9.9.8", v)
	}
}

// Alur penuh ber-DB: posting sekali untuk versi baru, penanda
// tersimpan, panggilan kedua tidak memposting lagi.
func TestPostUpdateToDiscordLive(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL kosong")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `delete from app_settings where key = any($1)`,
		[]string{discordUpdateNotifiedKey, telegramUpdateNotifiedKey}); err != nil {
		t.Fatal(err)
	}

	var posts atomic.Int32
	var lastBody string
	dc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		posts.Add(1)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"1"}`)
	}))
	defer dc.Close()
	oldDC := discord.BaseURL
	discord.BaseURL = dc.URL
	defer func() { discord.BaseURL = oldDC }()

	var tgPosts atomic.Int32
	tg := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		tgPosts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true,"result":{}}`)
	}))
	defer tg.Close()
	oldTG := telegram.BaseURL
	telegram.BaseURL = tg.URL
	defer func() { telegram.BaseURL = oldTG }()

	gh := fakeGitHub(t)
	defer gh.Close()
	oldGH := GitHubAPIBase
	GitHubAPIBase = gh.URL
	defer func() { GitHubAPIBase = oldGH }()

	cfg := config.Config{DiscordBotToken: "token-uji", DiscordChannelUpdateAPK: "upd",
		TelegramBotToken: "token-tg", TelegramGroupID: "-100123", TelegramTopicUpdateAPK: "20731"}
	PostUpdateToDiscordIfNew(ctx, pool, cfg)
	if posts.Load() != 1 {
		t.Fatalf("posting pertama: %d, mau 1", posts.Load())
	}
	if tgPosts.Load() != 1 {
		t.Fatalf("posting telegram pertama: %d, mau 1", tgPosts.Load())
	}
	if !strings.Contains(lastBody, "v9.9.8") ||
		!strings.Contains(lastBody, "https://www.subsenpai.web.id") {
		t.Fatalf("isi pesan tak sesuai: %s", lastBody)
	}
	var marker string
	if err := pool.QueryRow(ctx,
		`select value from app_settings where key = $1`, discordUpdateNotifiedKey).Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if marker != "9.9.8" {
		t.Fatalf("penanda = %q, mau 9.9.8", marker)
	}
	PostUpdateToDiscordIfNew(ctx, pool, cfg)
	if posts.Load() != 1 {
		t.Fatalf("posting kedua harus no-op, total %d", posts.Load())
	}
	if tgPosts.Load() != 1 {
		t.Fatalf("posting telegram kedua harus no-op, total %d", tgPosts.Load())
	}
	// Regresi 2026-11-24: penanda Telegram tertinggal dari
	// Discord (kanal dikonfigurasi belakangan) — Telegram harus
	// tetap menerima versi ini, Discord tidak boleh dobel.
	if _, err := pool.Exec(ctx, `delete from app_settings where key = $1`,
		telegramUpdateNotifiedKey); err != nil {
		t.Fatal(err)
	}
	PostUpdateToDiscordIfNew(ctx, pool, cfg)
	if tgPosts.Load() != 2 {
		t.Fatalf("telegram tertinggal harus tetap memposting, total %d", tgPosts.Load())
	}
	if posts.Load() != 1 {
		t.Fatalf("discord tidak boleh dobel, total %d", posts.Load())
	}
	// Token kosong → no-op total.
	PostUpdateToDiscordIfNew(ctx, pool, config.Config{})
	if posts.Load() != 1 {
		t.Fatalf("tanpa token harus no-op, total %d", posts.Load())
	}
}
