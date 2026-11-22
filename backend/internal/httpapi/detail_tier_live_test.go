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

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDetailTierLive: regresi bug 2026-11-23 — rute detail judul
// tidak memakai auth opsional sehingga FromContext selalu nil,
// semua penonton dianggap tamu, dan episode pengguna PREMIUM ikut
// terkunci. Uji ini menembak Router() sungguhan (handshake ECDH +
// JWT) dan memastikan tier premium terbaca di detail judul.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL (DB uji lokal
// yang sudah dimigrasikan).
func TestDetailTierLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()

	// Data uji: pengguna premium (trial), satu anime, dua episode.
	var userID, titleID string
	if err := pool.QueryRow(ctx, `
		insert into profiles (email, google_sub, is_premium, premium_until, trial_used_at)
		values ('tier-test@local', 'tier-test-sub', true, now() + interval '3 days', now())
		returning id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from profiles where id = $1::uuid`, userID)
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title, synopsis, fetch_mode)
		values ('tier-test-title', 'anime', 'Tier Test', 'sinopsis uji', 'device')
		returning id`).Scan(&titleID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from titles where id = $1::uuid`, titleID)
	for _, no := range []int{1, 2} {
		if _, err := pool.Exec(ctx, `
			insert into episodes (title_id, episode_no, video_url)
			values ($1::uuid, $2, 'https://example.invalid/v.mp4')`, titleID, no); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.Config{JWTSecret: "tier-test-secret", JWTTTLHours: 24, SessionTTLHours: 1}
	a := New(pool, middleware.NewSessions(time.Hour), cfg)
	srv := httptest.NewServer(a.Router())
	defer srv.Close()

	// Handshake ECDH seperti klien sungguhan.
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

	getDetail := func(jwt string) (string, []bool) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/titles/"+titleID, nil)
		req.Header.Set("X-Session-Id", hs.SessionID)
		if jwt != "" {
			req.Header.Set("Authorization", "Bearer "+jwt)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("detail status %d: %s", res.StatusCode, b)
		}
		var env struct {
			XData string `json:"xdata"`
			XTime int64  `json:"xtime"`
		}
		if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
			t.Fatal(err)
		}
		plain, err := middleware.Open(key, env.XData, env.XTime)
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Access struct {
				Tier string `json:"tier"`
			} `json:"access"`
			Episodes []struct {
				Locked bool `json:"locked"`
			} `json:"episodes"`
		}
		if err := json.Unmarshal(plain, &out); err != nil {
			t.Fatal(err)
		}
		locked := make([]bool, len(out.Episodes))
		for i, e := range out.Episodes {
			locked[i] = e.Locked
		}
		return out.Access.Tier, locked
	}

	jwt, err := auth.IssueJWT(cfg.JWTSecret, userID, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Premium: tier terbaca premium, tidak ada episode terkunci.
	tier, locked := getDetail(jwt)
	if tier != "premium" {
		t.Fatalf("tier pengguna premium = %q, mau \"premium\"", tier)
	}
	if len(locked) != 2 || locked[0] || locked[1] {
		t.Fatalf("locked premium = %v, mau [false false]", locked)
	}

	// Tamu (kontrol): episode pertama terbuka, lanjutan terkunci.
	tier, locked = getDetail("")
	if tier != "guest" {
		t.Fatalf("tier tamu = %q, mau \"guest\"", tier)
	}
	if len(locked) != 2 || locked[0] || !locked[1] {
		t.Fatalf("locked tamu = %v, mau [false true]", locked)
	}
}
