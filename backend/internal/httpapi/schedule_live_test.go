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

// TestScheduleLive: endpoint jadwal lewat Router() sungguhan.
// Donghua diambil hidup dari Anichin (bisa dari VM ini); film dari
// DB lokal; anime dari VM ini DITOLAK Cloudflare Kuronime sehingga
// jalur error 502 yang teruji — parser anime diuji via fixture di
// paket sources dan diverifikasi lewat API produksi pasca-deploy.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestScheduleLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	cfg := config.Config{JWTSecret: "sched-test-secret", JWTTTLHours: 24, SessionTTLHours: 1}
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

	get := func(path string) (int, []byte) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		req.Header.Set("X-Session-Id", hs.SessionID)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		if res.StatusCode != http.StatusOK {
			return res.StatusCode, raw
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
		return res.StatusCode, plain
	}

	// Donghua: grup hari + item dari Anichin asli.
	code, body := get("/api/v1/schedule?type=donghua")
	if code != http.StatusOK {
		t.Fatalf("donghua status %d: %s", code, body)
	}
	var donghua struct {
		Groups []struct {
			Day   string `json:"day"`
			Items []struct {
				Title   string `json:"title"`
				Time    string `json:"time"`
				Episode int    `json:"episode"`
			} `json:"items"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(body, &donghua); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, g := range donghua.Groups {
		total += len(g.Items)
	}
	if len(donghua.Groups) < 5 || total < 20 {
		t.Fatalf("donghua: %d grup %d item — terlalu sedikit", len(donghua.Groups), total)
	}
	first := donghua.Groups[0].Items[0]
	if first.Title == "" || first.Time == "" {
		t.Fatalf("item donghua pertama janggal: %+v", first)
	}

	// Film: satu grup "Rilis Terbaru" (isi mengikuti DB uji).
	code, body = get("/api/v1/schedule?type=film")
	if code != http.StatusOK {
		t.Fatalf("film status %d: %s", code, body)
	}
	var film struct {
		Groups []struct {
			Day string `json:"day"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(body, &film); err != nil {
		t.Fatal(err)
	}
	if len(film.Groups) != 1 || film.Groups[0].Day != "Rilis Terbaru" {
		t.Fatalf("grup film salah: %+v", film.Groups)
	}

	// Anime dari VM ini: Kuronime menolak (403) → 502 yang sopan.
	code, _ = get("/api/v1/schedule?type=anime")
	if code != http.StatusBadGateway {
		t.Fatalf("anime dari VM harusnya 502 (Cloudflare), dapat %d", code)
	}

	// Tipe tidak dikenal → 400.
	code, _ = get("/api/v1/schedule?type=manga")
	if code != http.StatusBadRequest {
		t.Fatalf("tipe manga harusnya 400, dapat %d", code)
	}
}
