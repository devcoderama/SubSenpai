package httpapi

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestGenresLive: endpoint /api/v1/genres + filter genre pada
// /api/v1/titles lewat Router() sungguhan, dengan judul-judul
// semaian bergenre di DB uji lokal.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestGenresLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()

	seed := []struct {
		slug, typ string
		genres    []string
	}{
		{"genre-test-a", "anime", []string{"Action", "Fantasy"}},
		{"genre-test-b", "anime", []string{"Action", "Comedy"}},
		{"genre-test-c", "donghua", []string{"Action"}},
	}
	for _, s := range seed {
		if _, err := pool.Exec(ctx, `
			insert into titles (slug, type, title, synopsis, fetch_mode, genres)
			values ($1, $2, $3, 'sinopsis uji', 'device', $4)
			on conflict (slug) do nothing`, s.slug, s.typ, s.slug, s.genres); err != nil {
			t.Fatal(err)
		}
	}
	defer pool.Exec(ctx, `delete from titles where slug like 'genre-test-%'`)

	cfg := config.Config{JWTSecret: "genre-test-secret", JWTTTLHours: 24, SessionTTLHours: 1}
	a := New(pool, middleware.NewSessions(time.Hour), cfg)
	srv := httptest.NewServer(a.Router())
	defer srv.Close()

	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hsBody, _ := json.Marshal(map[string]string{
		"client_public_key": base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
	})
	hsRes, err := http.Post(srv.URL+"/api/v1/session", "application/json", bytes.NewReader(hsBody))
	if err != nil {
		t.Fatal(err)
	}
	var hs struct {
		SessionID       string `json:"session_id"`
		ServerPublicKey string `json:"server_public_key"`
	}
	if err := json.NewDecoder(hsRes.Body).Decode(&hs); err != nil {
		t.Fatal(err)
	}
	hsRes.Body.Close()
	serverPubBytes, err := base64.RawURLEncoding.DecodeString(hs.ServerPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	serverPub, err := ecdh.X25519().NewPublicKey(serverPubBytes)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := priv.ECDH(serverPub)
	if err != nil {
		t.Fatal(err)
	}
	key, err := hkdf.Key(sha256.New, shared, nil, "subsenpai-xdata-v1", 32)
	if err != nil {
		t.Fatal(err)
	}

	get := func(path string) []byte {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		req.Header.Set("X-Session-Id", hs.SessionID)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d: %s", path, res.StatusCode, raw)
		}
		var env struct {
			XData string `json:"xdata"`
			XTime int64  `json:"xtime"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("%s: envelope rusak: %v", path, err)
		}
		plain, err := middleware.Open(key, env.XData, env.XTime)
		if err != nil {
			t.Fatalf("%s: dekripsi gagal: %v", path, err)
		}
		return plain
	}

	// Daftar genre anime: Action harus teratas (2 judul semaian,
	// bisa lebih bila DB uji berisi data lain) dan Comedy/Fantasy ada.
	var genresOut struct {
		Genres []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"genres"`
	}
	if err := json.Unmarshal(get("/api/v1/genres?type=anime"), &genresOut); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, g := range genresOut.Genres {
		counts[g.Name] = g.Count
	}
	if counts["Action"] < 2 || counts["Comedy"] < 1 || counts["Fantasy"] < 1 {
		t.Fatalf("genre anime janggal: %v", counts)
	}
	if len(genresOut.Genres) == 0 || genresOut.Genres[0].Name != "Action" {
		t.Fatalf("genre teratas harus Action: %+v", genresOut.Genres)
	}

	// Filter genre pada daftar judul.
	var titlesOut struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	if err := json.Unmarshal(get("/api/v1/titles?type=anime&genre=Fantasy"), &titlesOut); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range titlesOut.Items {
		if it.Slug == "genre-test-a" {
			found = true
		}
		if it.Slug == "genre-test-b" {
			t.Fatal("genre-test-b (Comedy) tidak boleh muncul di filter Fantasy")
		}
	}
	if !found {
		t.Fatal("genre-test-a harus muncul di filter Fantasy")
	}
}
