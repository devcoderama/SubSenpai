package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/middleware"
)

// TestSharePageLive: halaman bagikan /share/{id} dirender server
// dengan tag Open Graph (poster via img-proxy, sinopsis sebagai
// deskripsi) agar pratinjau tautan di WhatsApp/medsos benar.
// Judul tersembunyi & id ngawur → 404.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestSharePageLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()

	cfg := config.Config{JWTSecret: "share-test-secret", JWTTTLHours: 24, SessionTTLHours: 1}
	a := New(pool, middleware.NewSessions(time.Hour), cfg)
	srv := httptest.NewServer(a.Router())
	defer srv.Close()

	var id string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title, poster_url, year, genres, synopsis, status)
		values ('judul-bagikan-uji', 'film', 'Film Bagikan Uji',
		        'https://cdn.example.com/poster.jpg', 2024,
		        array['Action','Drama'], 'Sinopsis film bagikan uji untuk pratinjau.', 'ongoing')
		returning id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from titles where id = $1::uuid`, id)

	get := func(path string) (int, string) {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}

	code, body := get("/share/" + id)
	if code != 200 {
		t.Fatalf("share page: status %d", code)
	}
	for _, want := range []string{
		`property="og:title" content="Film Bagikan Uji — SubSenpai"`,
		`property="og:image" content="`,
		"/api/v1/img-proxy?u=https%3A%2F%2Fcdn.example.com%2Fposter.jpg",
		"Sinopsis film bagikan uji untuk pratinjau.",
		"https://www.subsenpai.web.id/judul/" + id,
		"intent://title/" + id,
		"subsenpai://title/" + id,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("share page tidak memuat %q", want)
		}
	}

	// Halaman bagikan episode: tautan /nonton/{episodeId} dari
	// notifikasi jadwal harus membuka pemutar episode itu.
	var epID string
	if err := pool.QueryRow(ctx, `
		insert into episodes (title_id, episode_no, video_url)
		values ($1::uuid, 3, 'https://cdn.example.com/v.mp4') returning id`, id).Scan(&epID); err != nil {
		t.Fatal(err)
	}
	code, body = get("/share-episode/" + epID)
	if code != 200 {
		t.Fatalf("share episode: status %d", code)
	}
	for _, want := range []string{
		"Film Bagikan Uji — Episode 3",
		"https://www.subsenpai.web.id/nonton/" + epID,
		"intent://episode/" + epID,
		"subsenpai://episode/" + epID,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("share episode tidak memuat %q", want)
		}
	}

	// Judul tersembunyi tidak boleh tampil.
	if _, err := pool.Exec(ctx, `update titles set is_hidden = true where id = $1::uuid`, id); err != nil {
		t.Fatal(err)
	}
	if code, _ := get("/share/" + id); code != 404 {
		t.Fatalf("judul tersembunyi: status %d, want 404", code)
	}

	// Id ngawur → 404.
	if code, _ := get("/share/bukan-id-sama-sekali"); code != 404 {
		t.Fatalf("id ngawur: status %d, want 404", code)
	}
}
