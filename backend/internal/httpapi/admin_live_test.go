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

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/middleware"
)

// TestAdminLive: endpoint dashboard admin (stats/users/role/
// transactions/plans) lewat Router sungguhan + envelope ECDH.
// Admin lolos semua; user biasa ditolak 403; CORS preflight dari
// origin Netlify dijawab, origin asing tidak.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestAdminLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()

	var adminID, userID string
	if err := pool.QueryRow(ctx, `
		insert into profiles (email, google_sub, display_name, role)
		values ('admin-test@local', 'admin-test-sub', 'Admin Uji', 'admin')
		returning id`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from profiles where id = $1::uuid`, adminID)
	if err := pool.QueryRow(ctx, `
		insert into profiles (email, google_sub, display_name, role)
		values ('user-test@local', 'user-test-sub', 'User Uji', 'user')
		returning id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from profiles where id = $1::uuid`, userID)
	if _, err := pool.Exec(ctx, `
		insert into payment_sessions (user_id, purpose, provider, amount, unique_code, status, plan, expires_at)
		values ($1::uuid, 'subscription', 'tokopay', 8000, 25, 'paid', 'premium-30', now() + interval '15 minutes')`,
		userID); err != nil {
		t.Fatal(err)
	}
	var pendingSessionID string
	if err := pool.QueryRow(ctx, `
		insert into payment_sessions (user_id, purpose, provider, amount, unique_code, status, plan, expires_at)
		values ($1::uuid, 'subscription', 'tokopay', 8000, 30, 'pending', 'premium-30', now() + interval '15 minutes')
		returning id`, userID).Scan(&pendingSessionID); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{JWTSecret: "admin-test-secret", JWTTTLHours: 24, SessionTTLHours: 1}
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
	serverPubBytes, _ := base64.RawURLEncoding.DecodeString(hs.ServerPublicKey)
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

	call := func(method, path, jwt string, body any) (int, map[string]any) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, srv.URL+path, rd)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Session-Id", hs.SessionID)
		if jwt != "" {
			req.Header.Set("Authorization", "Bearer "+jwt)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var env struct {
			XData string `json:"xdata"`
			XTime int64  `json:"xtime"`
		}
		if err := json.Unmarshal(raw, &env); err != nil || env.XData == "" {
			t.Fatalf("%s %s: respons bukan envelope: %s", method, path, raw)
		}
		plain, err := middleware.Open(key, env.XData, env.XTime)
		if err != nil {
			t.Fatalf("%s %s: gagal dekripsi: %v", method, path, err)
		}
		var out map[string]any
		if err := json.Unmarshal(plain, &out); err != nil {
			t.Fatalf("%s %s: JSON rusak: %s", method, path, plain)
		}
		return res.StatusCode, out
	}

	adminJWT, err := auth.IssueJWT(cfg.JWTSecret, adminID, "admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	userJWT, err := auth.IssueJWT(cfg.JWTSecret, userID, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Admin: stats terisi.
	st, stats := call("GET", "/api/v1/admin/stats", adminJWT, nil)
	if st != 200 || stats["users"].(float64) < 2 || stats["paid_transactions"].(float64) < 1 {
		t.Fatalf("stats admin: status=%d body=%v", st, stats)
	}
	// User biasa: ditolak.
	if st, _ := call("GET", "/api/v1/admin/stats", userJWT, nil); st != 403 {
		t.Fatalf("stats user biasa harus 403, dapat %d", st)
	}
	// Admin: daftar pengguna memuat dua akun uji.
	st, users := call("GET", "/api/v1/admin/users?q=test@local", adminJWT, nil)
	if st != 200 || len(users["users"].([]any)) != 2 {
		t.Fatalf("users admin: status=%d body=%v", st, users)
	}
	// Admin: angkat user menjadi admin (jalur "invite admin").
	st, upd := call("PATCH", "/api/v1/admin/users/"+userID, adminJWT, map[string]any{"role": "admin"})
	if st != 200 || upd["user"].(map[string]any)["role"] != "admin" {
		t.Fatalf("promote user: status=%d body=%v", st, upd)
	}
	// Admin tidak bisa menurunkan role sendiri.
	if st, _ := call("PATCH", "/api/v1/admin/users/"+adminID, adminJWT, map[string]any{"role": "user"}); st != 400 {
		t.Fatalf("self-demote harus 400, dapat %d", st)
	}
	// Admin: transaksi memuat sesi uji.
	st, trx := call("GET", "/api/v1/admin/transactions", adminJWT, nil)
	if st != 200 || len(trx["transactions"].([]any)) < 1 {
		t.Fatalf("transactions admin: status=%d body=%v", st, trx)
	}
	// Admin: paket terbaca + harga bisa diubah lalu dikembalikan.
	st, plans := call("GET", "/api/v1/admin/plans", adminJWT, nil)
	if st != 200 || len(plans["plans"].([]any)) < 1 {
		t.Fatalf("plans admin: status=%d body=%v", st, plans)
	}
	first := plans["plans"].([]any)[0].(map[string]any)
	planID := first["id"].(string)
	oldPrice := int(first["price"].(float64))
	st, updPlan := call("PATCH", "/api/v1/admin/plans/"+planID, adminJWT, map[string]any{"price": oldPrice + 1})
	if st != 200 || int(updPlan["plan"].(map[string]any)["price"].(float64)) != oldPrice+1 {
		t.Fatalf("update plan: status=%d body=%v", st, updPlan)
	}
	call("PATCH", "/api/v1/admin/plans/"+planID, adminJWT, map[string]any{"price": oldPrice})

	// Admin: edit transaksi pending -> paid harus mengaktifkan
	// premium pengguna (jalur ActivateSession yang sama dengan
	// callback gateway).
	st, updTrx := call("PATCH", "/api/v1/admin/transactions/"+pendingSessionID, adminJWT,
		map[string]any{"status": "paid"})
	if st != 200 || updTrx["transaction"].(map[string]any)["status"] != "paid" {
		t.Fatalf("edit transaksi ke paid: status=%d body=%v", st, updTrx)
	}
	var premiumUntil time.Time
	if err := pool.QueryRow(ctx, `
		select premium_until from profiles where id = $1::uuid`, userID).Scan(&premiumUntil); err != nil {
		t.Fatal(err)
	}
	if premiumUntil.Before(time.Now().Add(20 * 24 * time.Hour)) {
		t.Fatalf("premium harus diperpanjang ~30 hari, dapat %v", premiumUntil)
	}
	// Admin: koreksi jumlah.
	st, updTrx = call("PATCH", "/api/v1/admin/transactions/"+pendingSessionID, adminJWT,
		map[string]any{"amount": 9000})
	if st != 200 || int(updTrx["transaction"].(map[string]any)["amount"].(float64)) != 9000 {
		t.Fatalf("edit jumlah transaksi: status=%d body=%v", st, updTrx)
	}
	// Turunkan lagi user uji menjadi user biasa (di atas ia sempat
	// diangkat admin untuk uji promote) — autentikator membaca role
	// dari database per request.
	if st, _ := call("PATCH", "/api/v1/admin/users/"+userID, adminJWT,
		map[string]any{"role": "user"}); st != 200 {
		t.Fatalf("demote user uji: status=%d", st)
	}
	// User biasa tidak boleh edit/hapus transaksi.
	if st, _ := call("PATCH", "/api/v1/admin/transactions/"+pendingSessionID, userJWT,
		map[string]any{"status": "expired"}); st != 403 {
		t.Fatalf("edit transaksi oleh user harus 403, dapat %d", st)
	}
	if st, _ := call("DELETE", "/api/v1/admin/transactions/"+pendingSessionID, userJWT, nil); st != 403 {
		t.Fatalf("hapus transaksi oleh user harus 403, dapat %d", st)
	}
	// Admin: hapus transaksi -> hilang dari daftar & database.
	if st, _ := call("DELETE", "/api/v1/admin/transactions/"+pendingSessionID, adminJWT, nil); st != 200 {
		t.Fatalf("hapus transaksi: status=%d", st)
	}
	var sisa int
	if err := pool.QueryRow(ctx, `
		select count(*) from payment_sessions where id = $1::uuid`, pendingSessionID).Scan(&sisa); err != nil {
		t.Fatal(err)
	}
	if sisa != 0 {
		t.Fatalf("transaksi harus terhapus dari database, sisa=%d", sisa)
	}

	// CORS: preflight dari origin Netlify dijawab dengan echo origin;
	// origin asing tidak mendapat izin.
	for _, tc := range []struct {
		origin  string
		allowed bool
	}{
		{"https://subsenpai-admin.netlify.app", true},
		{"https://evil.example.com", false},
	} {
		req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/v1/admin/stats", nil)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("Access-Control-Request-Method", "GET")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		got := res.Header.Get("Access-Control-Allow-Origin")
		res.Body.Close()
		if tc.allowed && got != tc.origin {
			t.Fatalf("CORS %s: harus diizinkan, ACAO=%q", tc.origin, got)
		}
		if !tc.allowed && got != "" {
			t.Fatalf("CORS %s: harus ditolak, ACAO=%q", tc.origin, got)
		}
	}
}
