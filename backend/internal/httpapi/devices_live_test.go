package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/jobs"
	"subsenpai/backend/internal/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDevicesLive: batas login perangkat (permintaan Rams 27 Nov 2026).
// Premium maks 2 perangkat, gratis maks 1; login baru yang melebihi
// batas MENCABUT perangkat yang paling lama tidak aktif (bukan menolak
// login seperti perilaku lama 409 "batas_perangkat"). Perangkat yang
// dicabut: endpoint Require → 401 "perangkat_dicabut:<nama>", endpoint
// Optional → turun jadi tamu. Sweeper TrimDevices merapikan akun yang
// premiumnya habis (2 → 1) tiap jam.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL (DB uji lokal
// yang sudah dimigrasikan).
func TestDevicesLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()

	mkUser := func(email, sub string, premium bool) string {
		var id string
		premiumUntil := "null"
		if premium {
			premiumUntil = "now() + interval '3 days'"
		}
		q := `insert into profiles (email, google_sub, friend_code, is_premium, premium_until)
			values ($1, $2, $3, $4, ` + premiumUntil + `) returning id`
		if err := pool.QueryRow(ctx, q, email, sub, "DEV-"+sub, premium).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	freeID := mkUser("dev-free@local", "dev-free-sub", false)
	premID := mkUser("dev-prem@local", "dev-prem-sub", true)
	defer func() {
		for _, id := range []string{freeID, premID} {
			_, _ = pool.Exec(ctx, `delete from user_devices where user_id = $1::uuid`, id)
			_, _ = pool.Exec(ctx, `delete from profiles where id = $1::uuid`, id)
		}
	}()

	cfg := config.Config{JWTSecret: "devices-test-secret", JWTTTLHours: 24, SessionTTLHours: 1}
	a := New(pool, middleware.NewSessions(time.Hour), cfg)

	reg := func(userID, dev, name string) (*deviceOut, bool) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/google", nil)
		rec := httptest.NewRecorder()
		kicked, ok := a.registerDevice(rec, req, userID, dev, name)
		if !ok {
			t.Fatalf("registerDevice %s/%s ditolak: %d %s", userID, dev, rec.Code, rec.Body.String())
		}
		return kicked, ok
	}
	active := func(userID string) []string {
		rows, err := pool.Query(ctx, `
			select device_id from user_devices
			where user_id = $1::uuid and revoked_at is null order by device_id`, userID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err == nil {
				out = append(out, d)
			}
		}
		return out
	}
	eq := func(got, want []string) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	// GRATIS (batas 1): A masuk; B masuk → A ditendang; A masuk lagi
	// → B ditendang; A masuk lagi (perangkat dikenal) → tak ada yang
	// ditendang.
	if k, _ := reg(freeID, "free-A", "HP Gratis"); k != nil {
		t.Fatalf("gratis A: ditendang %v, mau nil", k.DeviceID)
	}
	if k, _ := reg(freeID, "free-B", "Tablet Gratis"); k == nil || k.DeviceID != "free-A" {
		t.Fatalf("gratis B: ditendang %v, mau free-A", k)
	}
	if !eq(active(freeID), []string{"free-B"}) {
		t.Fatalf("gratis aktif = %v, mau [free-B]", active(freeID))
	}
	if k, _ := reg(freeID, "free-A", "HP Gratis"); k == nil || k.DeviceID != "free-B" {
		t.Fatalf("gratis A lagi: ditendang %v, mau free-B", k)
	}
	if k, _ := reg(freeID, "free-A", "HP Gratis"); k != nil {
		t.Fatalf("gratis A dikenal: ditendang %v, mau nil", k.DeviceID)
	}
	if !eq(active(freeID), []string{"free-A"}) {
		t.Fatalf("gratis aktif = %v, mau [free-A]", active(freeID))
	}

	// PREMIUM (batas 2): A, B masuk; C masuk → A (tertua) ditendang;
	// B masuk lagi (dikenal) → tak ada yang ditendang.
	reg(premID, "prem-A", "HP Premium")
	reg(premID, "prem-B", "Laptop Premium")
	if k, _ := reg(premID, "prem-C", "Tablet Premium"); k == nil || k.DeviceID != "prem-A" {
		t.Fatalf("premium C: ditendang %v, mau prem-A", k)
	}
	if !eq(active(premID), []string{"prem-B", "prem-C"}) {
		t.Fatalf("premium aktif = %v, mau [prem-B prem-C]", active(premID))
	}
	if k, _ := reg(premID, "prem-B", "Laptop Premium"); k != nil {
		t.Fatalf("premium B dikenal: ditendang %v, mau nil", k.DeviceID)
	}

	// Middleware: perangkat dicabut → Require 401 "perangkat_dicabut:",
	// perangkat aktif → 200, tanpa header X-Device-Id → 200 (klien
	// lama tidak diusir). Optional + perangkat dicabut → 200 sebagai
	// tamu (profil nil di context).
	authn := &auth.Authenticator{Pool: pool, JWTSecret: cfg.JWTSecret}
	jwtFree, err := auth.IssueJWT(cfg.JWTSecret, freeID, "user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	call := func(mw func(http.Handler) http.Handler, deviceID string) (int, string, bool) {
		var sawProfile bool
		h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sawProfile = auth.FromContext(r.Context()) != nil
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+jwtFree)
		if deviceID != "" {
			req.Header.Set("X-Device-Id", deviceID)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var body struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&body)
		return rec.Code, body.Error, sawProfile
	}
	// free-B dicabut pada langkah gratis di atas.
	if code, msg, _ := call(authn.Require, "free-B"); code != http.StatusUnauthorized || !strings.HasPrefix(msg, "perangkat_dicabut:") {
		t.Fatalf("Require perangkat dicabut: %d %q, mau 401 perangkat_dicabut:", code, msg)
	}
	if code, _, _ := call(authn.Require, "free-A"); code != http.StatusOK {
		t.Fatalf("Require perangkat aktif: %d, mau 200", code)
	}
	if code, _, _ := call(authn.Require, ""); code != http.StatusOK {
		t.Fatalf("Require tanpa header perangkat: %d, mau 200", code)
	}
	if code, _, saw := call(authn.Optional, "free-B"); code != http.StatusOK || saw {
		t.Fatalf("Optional perangkat dicabut: %d sawProfile=%v, mau 200 + tamu", code, saw)
	}
	if code, _, saw := call(authn.Optional, "free-A"); code != http.StatusOK || !saw {
		t.Fatalf("Optional perangkat aktif: %d sawProfile=%v, mau 200 + profil", code, saw)
	}

	// Sweeper: tanam perangkat aktif ekstra (seolah premiumnya baru
	// habis / akun lama yang kelebihan) → trim menyisakan yang
	// terbaru saja: gratis 1 (free-A, baru login), premium 2
	// (prem-B & prem-C — B/C didaftarkan terakhir, lebih baru dari
	// seed X/Y/Z yang berumur 1–3 jam).
	seed := func(userID, dev string, seen time.Time) {
		_, err := pool.Exec(ctx, `
			insert into user_devices (user_id, device_id, device_name, last_seen_at)
			values ($1::uuid, $2, $3, $4)
			on conflict (user_id, device_id) do update
			set last_seen_at = excluded.last_seen_at, revoked_at = null`,
			userID, dev, dev, seen)
		if err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	seed(freeID, "free-X", now.Add(-2*time.Hour))
	seed(freeID, "free-Y", now.Add(-time.Hour))
	seed(premID, "prem-X", now.Add(-3*time.Hour))
	seed(premID, "prem-Y", now.Add(-2*time.Hour))
	seed(premID, "prem-Z", now.Add(-time.Hour))
	n, err := jobs.TrimDevices(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("TrimDevices mencabut %d, mau >= 2", n)
	}
	// free: A (baru saja login, terakhir) + X + Y → trim menyisakan 1 (free-A).
	if got := active(freeID); !eq(got, []string{"free-A"}) {
		t.Fatalf("gratis setelah trim = %v, mau [free-A]", got)
	}
	if got := active(premID); !eq(got, []string{"prem-B", "prem-C"}) {
		t.Fatalf("premium setelah trim = %v, mau [prem-B prem-C]", got)
	}
}
