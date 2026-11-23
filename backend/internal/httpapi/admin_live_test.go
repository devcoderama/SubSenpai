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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/config"
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
	crashID := crashes["crashes"].([]any)[0].(map[string]any)["id"].(string)
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
