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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/discord"
	"subsenpai/backend/internal/jobs"
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

	// Server tangkap Discord: kait notifikasi (pengumuman admin &
	// kenaikan versi minimum) harus mengirim pesan ke channel yang
	// benar lewat klien Discord sungguhan (BaseURL dialihkan ke sini).
	var dmu sync.Mutex
	var dmsgs []string
	dsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		dmu.Lock()
		dmsgs = append(dmsgs, r.URL.Path+" "+string(b))
		dmu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer dsrv.Close()
	oldDiscordBase := discord.BaseURL
	discord.BaseURL = dsrv.URL
	defer func() { discord.BaseURL = oldDiscordBase }()
	waitDiscord := func(pathPart, substr string) bool {
		for i := 0; i < 30; i++ {
			dmu.Lock()
			for _, m := range dmsgs {
				if strings.Contains(m, pathPart) && strings.Contains(m, substr) {
					dmu.Unlock()
					return true
				}
			}
			dmu.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
		return false
	}

	cfg := config.Config{JWTSecret: "admin-test-secret", JWTTTLHours: 24, SessionTTLHours: 1,
		DiscordBotToken: "token-uji", DiscordChannelPengumuman: "111", DiscordChannelUpdateAPK: "222"}
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

	// Admin: buat 2 voucher hadiah -> user menukar satu (premium
	// bertambah), yang sudah ditukar tidak bisa dihapus, sisanya bisa.
	st, vc := call("POST", "/api/v1/admin/vouchers", adminJWT,
		map[string]any{"duration_days": 30, "quantity": 2})
	if st != 200 || len(vc["vouchers"].([]any)) != 2 {
		t.Fatalf("buat voucher: status=%d body=%v", st, vc)
	}
	vouchers := vc["vouchers"].([]any)
	code0 := vouchers[0].(map[string]any)["code"].(string)
	id0 := vouchers[0].(map[string]any)["id"].(string)
	id1 := vouchers[1].(map[string]any)["id"].(string)
	if len(code0) < 3 || code0[:3] != "SS-" {
		t.Fatalf("format kode voucher salah: %q", code0)
	}
	var before time.Time
	_ = pool.QueryRow(ctx, `select coalesce(premium_until, now()) from profiles where id = $1::uuid`, userID).Scan(&before)
	st, redeem := call("POST", "/api/v1/vouchers/redeem", userJWT, map[string]any{"code": code0})
	if st != 200 {
		t.Fatalf("tukar voucher oleh user: status=%d body=%v", st, redeem)
	}
	var after time.Time
	if err := pool.QueryRow(ctx, `select premium_until from profiles where id = $1::uuid`, userID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after.Before(before.Add(29 * 24 * time.Hour)) {
		t.Fatalf("premium harus bertambah ~30 hari: sebelum=%v sesudah=%v", before, after)
	}
	if st, _ := call("DELETE", "/api/v1/admin/vouchers/"+id0, adminJWT, nil); st != 400 {
		t.Fatalf("hapus voucher yang sudah ditukar harus 400, dapat %d", st)
	}
	if st, _ := call("DELETE", "/api/v1/admin/vouchers/"+id1, adminJWT, nil); st != 200 {
		t.Fatalf("hapus voucher aktif: status=%d", st)
	}
	if st, _ := call("POST", "/api/v1/admin/vouchers", userJWT,
		map[string]any{"duration_days": 30}); st != 403 {
		t.Fatalf("buat voucher oleh user harus 403, dapat %d", st)
	}
	// Voucher bertanda tangan (assigned) untuk user uji.
	st, vc2 := call("POST", "/api/v1/admin/vouchers", adminJWT,
		map[string]any{"duration_days": 7, "assigned_email": "user-test@local"})
	if st != 200 || vc2["vouchers"].([]any)[0].(map[string]any)["assigned_to_email"] != "user-test@local" {
		t.Fatalf("buat voucher assigned: status=%d body=%v", st, vc2)
	}

	// Kelola judul: seed satu judul -> tampil di katalog -> admin
	// menyembunyikan -> hilang dari katalog & detail 404 -> tampilkan
	// lagi -> kembali.
	_, _ = pool.Exec(ctx, `delete from titles where slug = 'judul-uji-admin'`)
	var titleID string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title, poster_url, year)
		values ('judul-uji-admin', 'anime', 'Judul Uji Admin', 'https://img/poster.jpg', 2025)
		returning id`).Scan(&titleID); err != nil {
		t.Fatal(err)
	}
	if st, lst := call("GET", "/api/v1/titles?type=anime", userJWT, nil); st != 200 ||
		!strings.Contains(fmt.Sprint(lst), "Judul Uji Admin") {
		t.Fatalf("judul harus tampil di katalog: status=%d", st)
	}
	st, admTitles := call("GET", "/api/v1/admin/titles?q=Uji", adminJWT, nil)
	if st != 200 || len(admTitles["titles"].([]any)) != 1 {
		t.Fatalf("admin titles: status=%d body=%v", st, admTitles)
	}
	if st, _ := call("PATCH", "/api/v1/admin/titles/"+titleID, adminJWT,
		map[string]any{"is_hidden": true}); st != 200 {
		t.Fatalf("sembunyikan judul: status=%d", st)
	}
	if st, lst := call("GET", "/api/v1/titles?type=anime", userJWT, nil); st != 200 ||
		strings.Contains(fmt.Sprint(lst), "Judul Uji Admin") {
		t.Fatalf("judul tersembunyi harus hilang dari katalog: status=%d", st)
	}
	if st, _ := call("GET", "/api/v1/titles/"+titleID, userJWT, nil); st != 404 {
		t.Fatalf("detail judul tersembunyi harus 404, dapat %d", st)
	}
	if st, _ := call("PATCH", "/api/v1/admin/titles/"+titleID, adminJWT,
		map[string]any{"is_hidden": false}); st != 200 {
		t.Fatalf("tampilkan judul: status=%d", st)
	}
	if st, _ := call("GET", "/api/v1/titles/"+titleID, userJWT, nil); st != 200 {
		t.Fatalf("detail judul tampil harus 200, dapat %d", st)
	}
	if st, _ := call("PATCH", "/api/v1/admin/titles/"+titleID, userJWT,
		map[string]any{"is_hidden": true}); st != 403 {
		t.Fatalf("sembunyikan judul oleh user harus 403, dapat %d", st)
	}

	// Kelengkapan katalog + status worker: filter komplit /
	// belum / judul saja, hitungan, label kekurangan, dan
	// endpoint pantauan worker.
	for _, slug := range []string{"uji-kelengkapan-komplit", "uji-kelengkapan-sebagian", "uji-kelengkapan-saja"} {
		_, _ = pool.Exec(ctx, `delete from titles where slug = $1`, slug)
	}
	var komplitID string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title, poster_url, synopsis, year, genres, status)
		values ('uji-kelengkapan-komplit', 'anime', 'Uji Kelengkapan Komplit',
		        'https://cdn.invalid/k.jpg', 'Sinopsis lengkap.', 2024, '{Action}', 'ongoing')
		returning id`).Scan(&komplitID); err != nil {
		t.Fatalf("seed judul komplit: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into episodes (title_id, episode_no, video_url)
		values ($1::uuid, 1, 'https://cdn.invalid/v.mp4')`, komplitID); err != nil {
		t.Fatalf("seed episode komplit: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into titles (slug, type, title, poster_url, synopsis, year, genres, status)
		values ('uji-kelengkapan-sebagian', 'anime', 'Uji Kelengkapan Sebagian',
		        'https://cdn.invalid/s.jpg', 'Sinopsis ada.', 2023, '{Drama}', 'ongoing'),
		       ('uji-kelengkapan-saja', 'anime', 'Uji Kelengkapan Saja', null, null, null, '{}', 'ongoing')`); err != nil {
		t.Fatalf("seed judul sebagian/saja: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `delete from titles where slug like 'uji-kelengkapan-%'`)
	}()
	slugAda := func(body map[string]any, slug string) bool {
		for _, it := range body["titles"].([]any) {
			if it.(map[string]any)["slug"] == slug {
				return true
			}
		}
		return false
	}
	st, fKomplit := call("GET", "/api/v1/admin/titles?q=Kelengkapan&filter=komplit", adminJWT, nil)
	if st != 200 || !slugAda(fKomplit, "uji-kelengkapan-komplit") ||
		slugAda(fKomplit, "uji-kelengkapan-sebagian") || slugAda(fKomplit, "uji-kelengkapan-saja") {
		t.Fatalf("filter komplit: status=%d body=%v", st, fKomplit)
	}
	st, fKosong := call("GET", "/api/v1/admin/titles?q=Kelengkapan&filter=kosong", adminJWT, nil)
	if st != 200 || !slugAda(fKosong, "uji-kelengkapan-saja") ||
		slugAda(fKosong, "uji-kelengkapan-komplit") || slugAda(fKosong, "uji-kelengkapan-sebagian") {
		t.Fatalf("filter judul saja: status=%d body=%v", st, fKosong)
	}
	st, fSemua := call("GET", "/api/v1/admin/titles?q=Kelengkapan", adminJWT, nil)
	cnt := fSemua["counts"].(map[string]any)
	if st != 200 || cnt["komplit"].(float64) < 1 || cnt["kosong"].(float64) < 1 || cnt["belum"].(float64) < 2 {
		t.Fatalf("hitungan kelengkapan: status=%d counts=%v", st, cnt)
	}
	for _, it := range fSemua["titles"].([]any) {
		m := it.(map[string]any)
		if m["slug"] == "uji-kelengkapan-sebagian" &&
			!strings.Contains(fmt.Sprint(m["missing"]), "episode kosong") {
			t.Fatalf("label kekurangan sebagian salah: %v", m["missing"])
		}
	}
	if _, err := pool.Exec(ctx, `
		insert into app_settings (key, value)
		values ('worker_status', '{"phase":"Uji Fase","done":2,"total":5,"current":"Judul Uji","updated_at":"2026-11-23T00:00:00Z"}')
		on conflict (key) do update set value = excluded.value`); err != nil {
		t.Fatalf("seed status worker: %v", err)
	}
	st, wk := call("GET", "/api/v1/admin/worker", adminJWT, nil)
	if st != 200 {
		t.Fatalf("admin worker: status=%d", st)
	}
	if s, _ := wk["status"].(map[string]any); s == nil || s["phase"] != "Uji Fase" {
		t.Fatalf("status worker terbaca salah: %v", wk["status"])
	}
	if qn, _ := wk["queue"].(map[string]any); qn == nil || qn["total"].(float64) < 3 {
		t.Fatalf("antrean worker: %v", wk["queue"])
	}
	if st, _ := call("GET", "/api/v1/admin/worker", userJWT, nil); st != 403 {
		t.Fatalf("admin worker oleh user harus 403, dapat %d", st)
	}

	// Crash: user mengunggah laporan -> admin melihat -> menghapus.
	st, _ = call("POST", "/api/v1/crashes", userJWT, map[string]any{
		"app_version": "0.22.0", "version_code": 49,
		"android_version": "Android 14 (SDK 34)", "device": "POCO X6",
		"message": "java.lang.IllegalStateException: uji",
		"stack_trace": "java.lang.IllegalStateException: uji\n\tat com.subsenpai.MainActivity.onCreate",
	})
	if st != 200 {
		t.Fatalf("unggah crash: status=%d", st)
	}
	st, crashes := call("GET", "/api/v1/admin/crashes", adminJWT, nil)
	if st != 200 || len(crashes["crashes"].([]any)) != 1 {
		t.Fatalf("daftar crash admin: status=%d body=%v", st, crashes)
	}
	crashItem := crashes["crashes"].([]any)[0].(map[string]any)
	if _, ok := crashItem["user_name"]; !ok {
		t.Fatalf("laporan crash harus membawa user_name: %v", crashItem)
	}
	crashID := crashItem["id"].(string)
	// Penanda perbaikan: tandai sudah -> filter sudah memuatnya,
	// filter belum kosong; user biasa tidak boleh menandai.
	if st, _ := call("PATCH", "/api/v1/admin/crashes/"+crashID, adminJWT,
		map[string]any{"is_fixed": true}); st != 200 {
		t.Fatalf("tandai crash diperbaiki: status=%d", st)
	}
	st, crashesFixed := call("GET", "/api/v1/admin/crashes?fixed=sudah", adminJWT, nil)
	if st != 200 || len(crashesFixed["crashes"].([]any)) != 1 ||
		crashesFixed["counts"].(map[string]any)["sudah"].(float64) != 1 {
		t.Fatalf("daftar crash sudah diperbaiki: status=%d body=%v", st, crashesFixed)
	}
	st, crashesBelum := call("GET", "/api/v1/admin/crashes?fixed=belum", adminJWT, nil)
	if st != 200 || len(crashesBelum["crashes"].([]any)) != 0 {
		t.Fatalf("daftar crash belum diperbaiki harus kosong: status=%d body=%v", st, crashesBelum)
	}
	if st, _ := call("PATCH", "/api/v1/admin/crashes/"+crashID, userJWT,
		map[string]any{"is_fixed": false}); st != 403 {
		t.Fatalf("tandai crash oleh user harus 403, dapat %d", st)
	}
	if st, _ := call("DELETE", "/api/v1/admin/crashes/"+crashID, adminJWT, nil); st != 200 {
		t.Fatalf("hapus crash: status=%d", st)
	}

	// Pengaturan: admin menyalakan mode perawatan -> app-config
	// publik mencerminkannya -> matikan lagi.
	st, _ = call("PATCH", "/api/v1/admin/settings", adminJWT,
		map[string]any{"maintenance_mode": true, "announcement": "Halo uji"})
	if st != 200 {
		t.Fatalf("ubah settings: status=%d", st)
	}
	st, appCfg := call("GET", "/api/v1/app-config", "", nil)
	if st != 200 || appCfg["maintenance_mode"] != true || appCfg["announcement"] != "Halo uji" {
		t.Fatalf("app-config: status=%d body=%v", st, appCfg)
	}
	if st, _ := call("PATCH", "/api/v1/admin/settings", adminJWT,
		map[string]any{"maintenance_mode": false, "announcement": ""}); st != 200 {
		t.Fatalf("kembalikan settings: status=%d", st)
	}
	// Versi minimum berbasis NAMA versi (permintaan Rams: admin
	// mengetik "0.22.2", bukan kode versi): tersimpan, tampil di
	// app-config, dan format liar ditolak.
	if st, _ := call("PATCH", "/api/v1/admin/settings", adminJWT,
		map[string]any{"min_version_name": "0.22.2"}); st != 200 {
		t.Fatalf("simpan min_version_name: status=%d", st)
	}
	st, appCfg = call("GET", "/api/v1/app-config", "", nil)
	if st != 200 || appCfg["min_version_name"] != "0.22.2" {
		t.Fatalf("app-config min_version_name: status=%d body=%v", st, appCfg)
	}
	if st, _ := call("PATCH", "/api/v1/admin/settings", adminJWT,
		map[string]any{"min_version_name": "nol-koma"}); st != 400 {
		t.Fatalf("min_version_name liar harus 400, dapat %d", st)
	}
	if st, _ := call("PATCH", "/api/v1/admin/settings", adminJWT,
		map[string]any{"min_version_name": ""}); st != 200 {
		t.Fatalf("kosongkan min_version_name: status=%d", st)
	}

	// Notifikasi siaran: admin mengirim -> pengguna menerimanya di
	// daftar notifikasi (kind broadcast, belum dibaca) -> admin
	// melihat jumlah penerima -> hapus siaran -> salinan ikut hilang.
	st, sent := call("POST", "/api/v1/admin/notifications", adminJWT,
		map[string]any{"title": "Halo Uji", "body": "Isi pengumuman uji", "audience": "all"})
	if st != 200 {
		t.Fatalf("kirim siaran: status=%d body=%v", st, sent)
	}
	bid, _ := sent["id"].(string)
	if bid == "" || sent["recipients"].(float64) < 1 {
		t.Fatalf("hasil siaran janggal: %v", sent)
	}
	// Siaran yang sama juga terposting ke Discord #pengumuman.
	if !waitDiscord("/channels/111/messages", "Halo Uji") {
		t.Fatal("pesan Discord pengumuman tidak terkirim")
	}
	if st, _ := call("POST", "/api/v1/admin/notifications", userJWT,
		map[string]any{"title": "x", "body": "y"}); st != 403 {
		t.Fatalf("user mengirim siaran harus 403, dapat %d", st)
	}
	st, mine := call("GET", "/api/v1/feed/notifications", userJWT, nil)
	items, _ := mine["items"].([]any)
	found := false
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["kind"] == "broadcast" && m["title"] == "Halo Uji" {
			found = true
		}
	}
	if st != 200 || !found || mine["unread"].(float64) < 1 {
		t.Fatalf("notifikasi user: status=%d found=%v body=%v", st, found, mine)
	}
	// Pemisahan jenis: ?kind=social TIDAK memuat siaran, ?kind=
	// broadcast hanya siaran; tandai-dibaca per jenis tidak
	// menyentuh jenis lain.
	st, soc := call("GET", "/api/v1/feed/notifications?kind=social", userJWT, nil)
	for _, it := range soc["items"].([]any) {
		if it.(map[string]any)["kind"] == "broadcast" {
			t.Fatalf("kind=social memuat siaran: %v", soc)
		}
	}
	st, bro := call("GET", "/api/v1/feed/notifications?kind=broadcast", userJWT, nil)
	bItems, _ := bro["items"].([]any)
	if st != 200 || len(bItems) < 1 || bro["unread"].(float64) < 1 {
		t.Fatalf("kind=broadcast: status=%d body=%v", st, bro)
	}
	if st, _ := call("POST", "/api/v1/feed/notifications/read", userJWT,
		map[string]any{"kind": "social"}); st != 200 {
		t.Fatalf("read kind=social: status=%d", st)
	}
	if _, after := call("GET", "/api/v1/feed/notifications?kind=broadcast", userJWT, nil); after["unread"].(float64) < 1 {
		t.Fatalf("read kind=social ikut menandai siaran: %v", after)
	}
	if st, _ := call("POST", "/api/v1/feed/notifications/read", userJWT,
		map[string]any{"kind": "broadcast"}); st != 200 {
		t.Fatalf("read kind=broadcast: status=%d", st)
	}
	if _, after := call("GET", "/api/v1/feed/notifications?kind=broadcast", userJWT, nil); after["unread"].(float64) != 0 {
		t.Fatalf("read kind=broadcast tidak menandai: %v", after)
	}
	st, bl := call("GET", "/api/v1/admin/notifications", adminJWT, nil)
	bs, _ := bl["broadcasts"].([]any)
	if st != 200 || len(bs) < 1 {
		t.Fatalf("daftar siaran admin: status=%d body=%v", st, bl)
	}
	if st, _ := call("DELETE", "/api/v1/admin/notifications/"+bid, adminJWT, nil); st != 200 {
		t.Fatalf("hapus siaran: status=%d", st)
	}
	st, mine = call("GET", "/api/v1/feed/notifications", userJWT, nil)
	items, _ = mine["items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		if m["kind"] == "broadcast" && m["title"] == "Halo Uji" {
			t.Fatalf("siaran terhapus masih tampil di user: %v", m)
		}
	}

	// Log admin & info sistem bisa dibaca admin. (Tabel
	// schema_migrations dirawat migrator Go di produksi; di uji
	// lokal migrasi diterapkan via psql, jadi buat + isi di sini.)
	_, _ = pool.Exec(ctx, `create table if not exists schema_migrations (version text primary key)`)
	_, _ = pool.Exec(ctx, `insert into schema_migrations (version) values ('0017_admin_tools.sql') on conflict do nothing`)
	st, logs := call("GET", "/api/v1/admin/logs", adminJWT, nil)
	if st != 200 || len(logs["logs"].([]any)) == 0 {
		t.Fatalf("log admin: status=%d", st)
	}
	st, sys := call("GET", "/api/v1/admin/system", adminJWT, nil)
	if st != 200 || sys["latest_migration"] == "" || sys["counts"] == nil {
		t.Fatalf("info sistem: status=%d body=%v", st, sys)
	}
	if st, _ := call("GET", "/api/v1/admin/system", userJWT, nil); st != 403 {
		t.Fatalf("info sistem oleh user harus 403, dapat %d", st)
	}

	// Riwayat tonton: lapor progres -> muncul di daftar riwayat
	// (regresi keluhan Rams 2026-11-23: sudah menonton tapi Riwayat
	// kosong — sisi server harus terbukti sehat).
	if _, err := pool.Exec(ctx, `
		insert into viewing_profiles (account_id, name) values ($1::uuid, 'Utama')`, userID); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(ctx, `delete from titles where slug = 'judul-riwayat-uji'`)
	var histEpisodeID string
	if err := pool.QueryRow(ctx, `
		with t as (
			insert into titles (slug, type, title)
			values ('judul-riwayat-uji', 'film', 'Film Riwayat Uji') returning id
		)
		insert into episodes (title_id, episode_no, video_url)
		select id, 1, 'https://cdn.example/v.mp4' from t returning id`).Scan(&histEpisodeID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from titles where slug = 'judul-riwayat-uji'`)
	st, _ = call("POST", "/api/v1/history/watch", userJWT, map[string]any{
		"episode_id": histEpisodeID, "position_sec": 120,
	})
	if st != 200 {
		t.Fatalf("lapor riwayat tonton: status=%d", st)
	}
	st, hist := call("GET", "/api/v1/history/watch?limit=10", userJWT, nil)
	if st != 200 || len(hist["items"].([]any)) != 1 ||
		hist["items"].([]any)[0].(map[string]any)["title"] != "Film Riwayat Uji" {
		t.Fatalf("daftar riwayat: status=%d body=%v", st, hist)
	}

	// Notifikasi rilis favorit: user memfavoritkan judul -> sebaran
	// jobs.NotifyRelease -> tampil HANYA di ?kind=release, membawa
	// ref_title_id, dan tidak bocor ke ?kind=social.
	_, _ = pool.Exec(ctx, `delete from titles where slug = 'judul-rilis-uji'`)
	var relTitleID string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title)
		values ('judul-rilis-uji', 'anime', 'Anime Rilis Uji') returning id`).Scan(&relTitleID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from titles where slug = 'judul-rilis-uji'`)
	var vpID string
	if err := pool.QueryRow(ctx, `
		select id from viewing_profiles where account_id = $1::uuid limit 1`, userID).Scan(&vpID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		insert into watchlist (user_id, viewing_profile_id, title_id, notify)
		values ($1::uuid, $2::uuid, $3::uuid, true)`, userID, vpID, relTitleID); err != nil {
		t.Fatal(err)
	}
	if err := jobs.NotifyRelease(ctx, pool, relTitleID, "Anime Rilis Uji", []int{7}); err != nil {
		t.Fatalf("NotifyRelease: %v", err)
	}
	st, rel := call("GET", "/api/v1/feed/notifications?kind=release", userJWT, nil)
	relItems, _ := rel["items"].([]any)
	if st != 200 || len(relItems) != 1 {
		t.Fatalf("kind=release: status=%d body=%v", st, rel)
	}
	relItem, _ := relItems[0].(map[string]any)
	if relItem["kind"] != "release" || relItem["ref_title_id"] != relTitleID ||
		relItem["title"] != "Episode baru: Anime Rilis Uji" {
		t.Fatalf("isi notifikasi rilis salah: %v", relItem)
	}
	if rel["unread"].(float64) < 1 {
		t.Fatalf("unread rilis: %v", rel)
	}
	st, soc2 := call("GET", "/api/v1/feed/notifications?kind=social", userJWT, nil)
	if st != 200 {
		t.Fatalf("kind=social (cek rilis): status=%d", st)
	}
	for _, it := range soc2["items"].([]any) {
		if it.(map[string]any)["kind"] == "release" {
			t.Fatalf("notifikasi rilis bocor ke kind=social: %v", it)
		}
	}
	if st, _ := call("POST", "/api/v1/feed/notifications/read", userJWT,
		map[string]any{"kind": "release"}); st != 200 {
		t.Fatalf("read kind=release: status=%d", st)
	}
	if _, after := call("GET", "/api/v1/feed/notifications?kind=release", userJWT, nil); after["unread"].(float64) != 0 {
		t.Fatalf("read kind=release tidak menandai: %v", after)
	}

	// Telusur langsung + impor judul: impor dari URL sumber aktif
	// membuat judul + pemetaan (idempoten), URL asing ditolak,
	// live-browse memvalidasi type dan melayani tanpa genre.
	defer pool.Exec(ctx, `delete from titles where slug in ('judul-impor-uji', 'film-impor-uji-999001')`)
	st, imp := call("POST", "/api/v1/titles/import", userJWT, map[string]any{
		"type": "anime", "url": "https://kuronime.sbs/anime/judul-impor-uji/",
		"title": "Judul Impor Uji", "poster_url": "https://img.example/p.jpg",
		"genre": "Action",
	})
	if st != 200 || imp["created"] != true || imp["id"] == nil || imp["id"] == "" {
		t.Fatalf("impor anime: status=%d body=%v", st, imp)
	}
	impID := imp["id"].(string)
	st, imp2 := call("POST", "/api/v1/titles/import", userJWT, map[string]any{
		"type": "anime", "url": "https://kuronime.sbs/anime/judul-impor-uji/",
		"title": "Judul Impor Uji",
	})
	if st != 200 || imp2["created"] != false || imp2["id"] != impID {
		t.Fatalf("impor ulang harus idempoten: status=%d body=%v", st, imp2)
	}
	var mapCount int
	if err := pool.QueryRow(ctx, `
		select count(*) from title_sources where title_id = $1::uuid`, impID).Scan(&mapCount); err != nil || mapCount != 1 {
		t.Fatalf("pemetaan impor: count=%d err=%v", mapCount, err)
	}
	if st, _ := call("POST", "/api/v1/titles/import", userJWT, map[string]any{
		"type": "anime", "url": "https://evil.example.com/anime/x/", "title": "X",
	}); st != 400 {
		t.Fatalf("impor URL asing: status=%d, want 400", st)
	}
	st, impF := call("POST", "/api/v1/titles/import", userJWT, map[string]any{
		"type": "film", "url": "https://hashflix.top/movie/999001", "title": "Film Impor Uji",
		"genres": []string{"Action"},
	})
	if st != 200 || impF["slug"] != "film-impor-uji-999001" || impF["created"] != true {
		t.Fatalf("impor film: status=%d body=%v", st, impF)
	}
	if st, _ := call("GET", "/api/v1/live-browse?type=manga&genre=Action", userJWT, nil); st != 400 {
		t.Fatalf("live-browse type liar: status=%d, want 400", st)
	}
	st, lb := call("GET", "/api/v1/live-browse?type=anime", userJWT, nil)
	if st != 200 || len(lb["items"].([]any)) != 0 {
		t.Fatalf("live-browse tanpa genre: status=%d body=%v", st, lb)
	}

	// Pencarian live (permintaan Rams 2026-11-23): kata yang belum
	// ada di katalog memicu scraping halaman pencarian sumber;
	// hasilnya DISIMPAN ke katalog (poster ikut), dan pencarian
	// kedua dilayani dari database tanpa menghubungi sumber lagi.
	cariHits := 0
	var cariSrv *httptest.Server
	cariSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cariHits++
		_, _ = w.Write([]byte(`<article class="bs"><div class="bsx">
			<a href="` + cariSrv.URL + `/anime/zztop-cari-uji/" title="Zztop Cari Uji">
			<div class="limit"><img src="https://i1.wp.com/img.example/zztop.webp"></div>
			<div class="tt"><h2>Zztop Cari Uji</h2></div></a></div></article>`))
	}))
	defer cariSrv.Close()
	if _, err := pool.Exec(ctx, `
		insert into content_sources (name, content_type, lang, base_url, resolver, fetch_mode, subtitle_kind, priority, is_active)
		values ('Sumber Cari Uji', 'anime', 'zz', $1, 'kuronime', 'server', 'hard', 1, true)`, cariSrv.URL); err != nil {
		t.Fatalf("seed sumber cari uji: %v", err)
	}
	defer pool.Exec(ctx, `delete from content_sources where name = 'Sumber Cari Uji'`)
	defer pool.Exec(ctx, `delete from titles where slug = 'zztop-cari-uji'`)
	// Sumber anime lain dinonaktifkan selama seksi ini (id-nya
	// dicatat agar dipulihkan persis seperti semula).
	offRows, err := pool.Query(ctx, `
		update content_sources set is_active = false
		where content_type = 'anime' and is_active and base_url <> $1
		returning id`, cariSrv.URL)
	if err != nil {
		t.Fatalf("nonaktifkan sumber anime lain: %v", err)
	}
	var offIDs []string
	for offRows.Next() {
		var id string
		if err := offRows.Scan(&id); err == nil {
			offIDs = append(offIDs, id)
		}
	}
	offRows.Close()
	defer pool.Exec(ctx, `update content_sources set is_active = true where id = any($1::uuid[])`, offIDs)
	st, cari := call("GET", "/api/v1/titles?type=anime&q=zztop", userJWT, nil)
	cariItems, _ := cari["items"].([]any)
	if st != 200 || len(cariItems) != 1 {
		t.Fatalf("pencarian live pertama: status=%d body=%v", st, cari)
	}
	cariFirst := cariItems[0].(map[string]any)
	if cariFirst["title"] != "Zztop Cari Uji" || cariFirst["poster_url"] == nil || cariFirst["poster_url"] == "" {
		t.Fatalf("hasil pencarian live harus berjudul benar + berposter: %v", cariFirst)
	}
	if cariHits < 1 {
		t.Fatalf("sumber pencarian tidak pernah dihubungi")
	}
	hitsBefore := cariHits
	st, cari2 := call("GET", "/api/v1/titles?type=anime&q=zztop", userJWT, nil)
	if st != 200 || len(cari2["items"].([]any)) != 1 || cariHits != hitsBefore {
		t.Fatalf("pencarian kedua harus dari DB: status=%d hits %d->%d body=%v",
			st, hitsBefore, cariHits, cari2)
	}

	// --- Unlock episode lewat iklan (Monetag) ---
	// Judul anime uji dengan 2 episode: episode 2 terkunci untuk
	// user gratis; alur start -> complete (token HMAC + waktu tonton
	// minimum yang ditegakkan server) membukanya, untuk user login
	// maupun tamu per perangkat (X-Device-Id). User uji utama sudah
	// premium di titik ini (seksi transaksi), jadi dipakai user
	// gratis khusus.
	var freeID string
	if err := pool.QueryRow(ctx, `
		insert into profiles (email, google_sub, display_name, role)
		values ('free-iklan@local', 'free-iklan-sub', 'Free Iklan', 'user')
		returning id`).Scan(&freeID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from profiles where id = $1::uuid`, freeID)
	freeJWT, err := auth.IssueJWT(cfg.JWTSecret, freeID, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pool.Exec(ctx, `delete from titles where slug = 'judul-iklan-uji'`)
	var iklanTitleID, iklanEp2 string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title) values ('judul-iklan-uji', 'anime', 'Anime Iklan Uji')
		returning id`).Scan(&iklanTitleID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from titles where id = $1`, iklanTitleID)
	pool.Exec(ctx, `insert into episodes (title_id, episode_no, video_url) values ($1, 1, 'https://v.example/1.mp4')`, iklanTitleID)
	if err := pool.QueryRow(ctx, `
		insert into episodes (title_id, episode_no, video_url) values ($1, 2, 'https://v.example/2.mp4')
		returning id`, iklanTitleID).Scan(&iklanEp2); err != nil {
		t.Fatal(err)
	}
	epLocked := func(detail map[string]any) bool {
		for _, e := range detail["episodes"].([]any) {
			em := e.(map[string]any)
			if em["id"] == iklanEp2 {
				return em["locked"].(bool)
			}
		}
		t.Fatalf("episode 2 tidak ada di detail: %v", detail)
		return true
	}
	st, det := call("GET", "/api/v1/titles/"+iklanTitleID, freeJWT, nil)
	if st != 200 || !epLocked(det) {
		t.Fatalf("episode 2 harus terkunci untuk user gratis: status=%d", st)
	}
	oldWait := adUnlockWait
	defer func() { adUnlockWait = oldWait }()
	adUnlockWait = time.Hour
	st, start := call("POST", "/api/v1/episodes/"+iklanEp2+"/ad-unlock/start", freeJWT, nil)
	token, _ := start["token"].(string)
	if st != 200 || token == "" {
		t.Fatalf("ad-unlock start: status=%d body=%v", st, start)
	}
	if st, _ := call("POST", "/api/v1/episodes/"+iklanEp2+"/ad-unlock/complete", freeJWT,
		map[string]any{"token": token}); st != 400 {
		t.Fatalf("complete sebelum waktunya harus 400, dapat %d", st)
	}
	adUnlockWait = 0
	if st, done := call("POST", "/api/v1/episodes/"+iklanEp2+"/ad-unlock/complete", freeJWT,
		map[string]any{"token": token}); st != 200 || done["unlocked"] != true {
		t.Fatalf("ad-unlock complete: status=%d body=%v", st, done)
	}
	st, det = call("GET", "/api/v1/titles/"+iklanTitleID, freeJWT, nil)
	if st != 200 || epLocked(det) {
		t.Fatalf("episode 2 harus terbuka setelah unlock iklan: status=%d", st)
	}
	if st, _ := call("POST", "/api/v1/episodes/"+iklanEp2+"/ad-unlock/complete", freeJWT,
		map[string]any{"token": "token-ngawur"}); st != 400 {
		t.Fatalf("token ngawur harus 400, dapat %d", st)
	}
	if st, _ := call("POST", "/api/v1/episodes/00000000-0000-0000-0000-000000000000/ad-unlock/start",
		freeJWT, nil); st != 404 {
		t.Fatalf("start episode tak dikenal harus 404, dapat %d", st)
	}
	// Tamu: unlock terikat perangkat (X-Device-Id).
	callDev := func(method, path, device string, body any) (int, map[string]any) {
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
		req.Header.Set("X-Device-Id", device)
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
			t.Fatal(err)
		}
		var out map[string]any
		if err := json.Unmarshal(plain, &out); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, out
	}
	st, gstart := callDev("POST", "/api/v1/episodes/"+iklanEp2+"/ad-unlock/start", "dev-iklan-uji-1", nil)
	gtoken, _ := gstart["token"].(string)
	if st != 200 || gtoken == "" {
		t.Fatalf("ad-unlock start tamu: status=%d body=%v", st, gstart)
	}
	if st, _ := callDev("POST", "/api/v1/episodes/"+iklanEp2+"/ad-unlock/complete", "dev-iklan-uji-1",
		map[string]any{"token": gtoken}); st != 200 {
		t.Fatalf("ad-unlock complete tamu: status=%d", st)
	}
	st, gdet := callDev("GET", "/api/v1/titles/"+iklanTitleID, "dev-iklan-uji-1", nil)
	if st != 200 || epLocked(gdet) {
		t.Fatalf("episode 2 harus terbuka untuk perangkat tamu yang unlock: status=%d", st)
	}
	st, gdet2 := callDev("GET", "/api/v1/titles/"+iklanTitleID, "dev-iklan-lain", nil)
	if st != 200 || !epLocked(gdet2) {
		t.Fatalf("episode 2 harus tetap terkunci untuk perangkat lain: status=%d", st)
	}

	// Laporan user: buat (user + tamu) -> dedupe -> admin
	// menindak (perbaiki link, sembunyikan episode, ubah status,
	// sembunyikan judul, hapus).
	_, _ = pool.Exec(ctx, `delete from titles where slug = 'uji-laporan'`)
	var lapTitleID, lapEp1, lapEp2 string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title, poster_url, synopsis, year, genres, status)
		values ('uji-laporan', 'anime', 'Uji Laporan', 'https://cdn.invalid/l.jpg',
		        'Sinopsis laporan.', 2024, '{Action}', 'ongoing') returning id`).Scan(&lapTitleID); err != nil {
		t.Fatalf("seed judul laporan: %v", err)
	}
	defer func() { _, _ = pool.Exec(ctx, `delete from titles where id = $1::uuid`, lapTitleID) }()
	if err := pool.QueryRow(ctx, `
		insert into episodes (title_id, episode_no, video_url)
		values ($1::uuid, 1, 'https://cdn.invalid/e1.mp4') returning id`, lapTitleID).Scan(&lapEp1); err != nil {
		t.Fatalf("seed ep1 laporan: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		insert into episodes (title_id, episode_no, video_url)
		values ($1::uuid, 2, 'https://cdn.invalid/e2.mp4') returning id`, lapTitleID).Scan(&lapEp2); err != nil {
		t.Fatalf("seed ep2 laporan: %v", err)
	}
	st, rep1 := call("POST", "/api/v1/reports", userJWT, map[string]any{
		"episode_id": lapEp1, "reason": "semua_server_gagal", "note": "semua server mati"})
	if st != 201 || rep1["id"] == nil {
		t.Fatalf("buat laporan user: status=%d body=%v", st, rep1)
	}
	rep1ID := rep1["id"].(string)
	st, repDup := call("POST", "/api/v1/reports", userJWT, map[string]any{
		"episode_id": lapEp1, "reason": "link_rusak"})
	if st != 200 || repDup["id"] != rep1ID {
		t.Fatalf("dedupe laporan: status=%d body=%v", st, repDup)
	}
	st, rep2 := callDev("POST", "/api/v1/reports", "dev-laporan-uji", map[string]any{
		"episode_id": lapEp2, "reason": "link_rusak"})
	if st != 201 || rep2["id"] == nil {
		t.Fatalf("buat laporan tamu: status=%d body=%v", st, rep2)
	}
	rep2ID := rep2["id"].(string)
	if st, _ := call("POST", "/api/v1/reports", userJWT, map[string]any{"episode_id": "bukan-uuid"}); st != 404 {
		t.Fatalf("laporan episode tidak ada harus 404, dapat %d", st)
	}
	repAda := func(body map[string]any, id string) bool {
		for _, it := range body["reports"].([]any) {
			if it.(map[string]any)["id"] == id {
				return true
			}
		}
		return false
	}
	st, lstRep := call("GET", "/api/v1/admin/reports?status=baru", adminJWT, nil)
	if st != 200 || !repAda(lstRep, rep1ID) || !repAda(lstRep, rep2ID) {
		t.Fatalf("daftar laporan admin: status=%d body=%v", st, lstRep)
	}
	if st, _ := call("GET", "/api/v1/admin/reports", userJWT, nil); st != 403 {
		t.Fatalf("daftar laporan oleh user harus 403, dapat %d", st)
	}
	// Perbaiki link: video_url lama dibuang, status jadi diproses.
	if st, _ := call("POST", "/api/v1/admin/reports/"+rep1ID+"/fix-links", adminJWT, nil); st != 200 {
		t.Fatalf("fix-links: status=%d", st)
	}
	var ep1URL string
	_ = pool.QueryRow(ctx, `select video_url from episodes where id = $1::uuid`, lapEp1).Scan(&ep1URL)
	if ep1URL != "" {
		t.Fatalf("fix-links harus membuang video_url lama, masih %q", ep1URL)
	}
	st, lstProses := call("GET", "/api/v1/admin/reports?status=diproses", adminJWT, nil)
	if st != 200 || !repAda(lstProses, rep1ID) {
		t.Fatalf("laporan harus berstatus diproses: status=%d", st)
	}
	// Sembunyikan episode dari laporan tamu: hilang dari detail
	// dan putar mengembalikan 404.
	if st, _ := call("POST", "/api/v1/admin/reports/"+rep2ID+"/hide-episode", adminJWT, nil); st != 200 {
		t.Fatalf("hide-episode: status=%d", st)
	}
	st, detLap := call("GET", "/api/v1/titles/"+lapTitleID, userJWT, nil)
	if st != 200 || strings.Contains(fmt.Sprint(detLap["episodes"]), lapEp2) {
		t.Fatalf("episode tersembunyi harus hilang dari detail: status=%d", st)
	}
	if st, _ := call("GET", "/api/v1/episodes/"+lapEp2+"/playback", userJWT, nil); st != 404 {
		t.Fatalf("putar episode tersembunyi harus 404, dapat %d", st)
	}
	// Ubah status manual + sembunyikan judul + hapus laporan.
	if st, _ := call("PATCH", "/api/v1/admin/reports/"+rep1ID, adminJWT,
		map[string]any{"status": "diabaikan"}); st != 200 {
		t.Fatalf("patch status laporan: status=%d", st)
	}
	if st, _ := call("POST", "/api/v1/admin/reports/"+rep1ID+"/hide-title", adminJWT, nil); st != 200 {
		t.Fatalf("hide-title: status=%d", st)
	}
	if st, _ := call("GET", "/api/v1/titles/"+lapTitleID, userJWT, nil); st != 404 {
		t.Fatalf("detail judul tersembunyi harus 404, dapat %d", st)
	}
	if st, _ := call("DELETE", "/api/v1/admin/reports/"+rep2ID, adminJWT, nil); st != 200 {
		t.Fatalf("hapus laporan: status=%d", st)
	}
	st, lstAkhir := call("GET", "/api/v1/admin/reports", adminJWT, nil)
	if st != 200 || repAda(lstAkhir, rep2ID) {
		t.Fatalf("laporan terhapus masih tampil: status=%d", st)
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
