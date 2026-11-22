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

// TestFiltersLive: faset /api/v1/filters + parameter year/country/
// network pada /api/v1/titles lewat Router() sungguhan.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestFiltersLive(t *testing.T) {
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
		slug, network, country string
		year                   int
		genres                 []string
	}{
		{"film-test-a", "Netflix", "US", 2024, []string{"Action"}},
		{"film-test-b", "HBO", "KR", 2024, []string{"Drama"}},
		{"film-test-c", "Netflix", "US", 2019, []string{"Action"}},
	}
	for _, s := range seed {
		if _, err := pool.Exec(ctx, `
			insert into titles (slug, type, title, synopsis, fetch_mode, genres, year, country, network)
			values ($1, 'film', $1, 'sinopsis uji', 'device', $2, $3, $4, $5)
			on conflict (slug) do nothing`, s.slug, s.genres, s.year, s.country, s.network); err != nil {
			t.Fatal(err)
		}
	}
	defer pool.Exec(ctx, `delete from titles where slug like 'film-test-%'`)

	cfg := config.Config{JWTSecret: "filter-test-secret", JWTTTLHours: 24, SessionTTLHours: 1}
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

	type facet struct {
		Name  string `json:"name"`
		Value string `json:"value"`
		Count int    `json:"count"`
	}
	var fOut struct {
		Genres    []facet `json:"genres"`
		Years     []facet `json:"years"`
		Countries []facet `json:"countries"`
		Networks  []facet `json:"networks"`
	}
	if err := json.Unmarshal(get("/api/v1/filters?type=film"), &fOut); err != nil {
		t.Fatal(err)
	}
	find := func(list []facet, value string) *facet {
		for i := range list {
			if list[i].Value == value {
				return &list[i]
			}
		}
		return nil
	}
	if y := find(fOut.Years, "2024"); y == nil || y.Count < 2 {
		t.Fatalf("faset tahun 2024 janggal: %+v", fOut.Years)
	}
	if len(fOut.Years) == 0 || fOut.Years[0].Value != "2024" {
		t.Fatalf("tahun harus urut menurun, teratas 2024: %+v", fOut.Years)
	}
	if c := find(fOut.Countries, "US"); c == nil || c.Count < 2 || c.Name != "Amerika Serikat" {
		t.Fatalf("faset negara US janggal: %+v", fOut.Countries)
	}
	if n := find(fOut.Networks, "Netflix"); n == nil || n.Count < 2 {
		t.Fatalf("faset network Netflix janggal: %+v", fOut.Networks)
	}
	if g := find(fOut.Genres, "Action"); g == nil || g.Count < 2 {
		t.Fatalf("faset genre Action janggal: %+v", fOut.Genres)
	}

	slugs := func(path string) map[string]bool {
		var out struct {
			Items []struct {
				Slug string `json:"slug"`
			} `json:"items"`
		}
		if err := json.Unmarshal(get(path), &out); err != nil {
			t.Fatal(err)
		}
		m := map[string]bool{}
		for _, it := range out.Items {
			m[it.Slug] = true
		}
		return m
	}
	byYear := slugs("/api/v1/titles?type=film&year=2024")
	if !byYear["film-test-a"] || !byYear["film-test-b"] || byYear["film-test-c"] {
		t.Fatalf("filter year=2024 salah: %v", byYear)
	}
	byCountry := slugs("/api/v1/titles?type=film&country=us")
	if !byCountry["film-test-a"] || !byCountry["film-test-c"] || byCountry["film-test-b"] {
		t.Fatalf("filter country=us (huruf kecil) salah: %v", byCountry)
	}
	byCombo := slugs("/api/v1/titles?type=film&year=2024&network=Netflix")
	if !byCombo["film-test-a"] || byCombo["film-test-b"] || byCombo["film-test-c"] {
		t.Fatalf("filter gabungan year+network salah: %v", byCombo)
	}
}
