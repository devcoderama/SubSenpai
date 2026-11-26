package jobs

// Uji ExpireStaleSessions terhadap database uji lokal (DATABASE_URL):
// sesi Tokopay basi yang belum lunas jadi 'expired', yang ternyata
// sudah lunas justru DIAKTIFKAN (jaring pengaman callback terlewat),
// yang gagal dicek dilewati, GoPay basi langsung kedaluwarsa, sesi
// segar dan provider tanpa klien cek status tidak disentuh. Server
// Tokopay ditiru dengan httptest — tidak menyentuh gateway asli.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/billing"
	"subsenpai/backend/internal/payments"
)

// fakeTokopayCheck meniru GET /v1/check-order: TRX_LUNAS dibalas
// Success, TRX_BELUM dibalas Unpaid, trx lain dibalas error; bila
// dicari lewat ref_id (sesi tanpa matched_ref) selalu Unpaid.
func fakeTokopayCheck() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		trx := r.URL.Query().Get("trx_id")
		switch {
		case trx == "TRX_LUNAS":
			fmt.Fprint(w, `{"status":1,"data":{"status":"Success"}}`)
		case trx == "TRX_BELUM":
			fmt.Fprint(w, `{"status":1,"data":{"status":"Unpaid"}}`)
		case trx != "":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"status":0,"error_msg":"tidak ditemukan"}`)
		default:
			fmt.Fprint(w, `{"status":1,"data":{"status":"Unpaid"}}`)
		}
	}))
}

func TestExpireStaleSessionsLive(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL kosong")
	}
	srv := fakeTokopayCheck()
	defer srv.Close()
	client := payments.NewTokopayClient(srv.URL, "merchant-uji", "secret-uji")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var userID string
	if err := pool.QueryRow(ctx, `
		insert into profiles (email) values ('uji-expire@local') returning id`,
	).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_, _ = pool.Exec(ctx, `delete from subscriptions where user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `delete from payment_sessions where user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `delete from profiles where id = $1::uuid`, userID)
	}
	defer cleanup()

	seed := func(provider, trx string, stale bool, amount int) string {
		age := "now()"
		if stale {
			age = "now() - interval '25 hours'"
		}
		var trxArg any
		if trx != "" {
			trxArg = trx
		}
		var id string
		if err := pool.QueryRow(ctx, `
			insert into payment_sessions (user_id, purpose, provider, amount, unique_code, plan, expires_at, matched_ref, created_at)
			values ($1::uuid, 'subscription', $2, $3, $4, 'premium-30', now() + interval '15 minutes', $5, `+age+`)
			returning id`,
			userID, provider, amount, amount%999+1, trxArg).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	statusOf := func(id string) string {
		var s string
		if err := pool.QueryRow(ctx, `select status from payment_sessions where id = $1::uuid`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	belumID := seed("tokopay", "TRX_BELUM", true, 8001)
	lunasID := seed("tokopay", "TRX_LUNAS", true, 8002)
	gagalID := seed("tokopay", "TRX_GAGAL", true, 8003)
	gopayID := seed("gopay", "", true, 8004)
	tanpaRefID := seed("tokopay", "", true, 8005)
	segarID := seed("tokopay", "TRX_BELUM", false, 8006)
	midtransID := seed("midtrans", "", true, 8007)

	ExpireStaleSessions(ctx, pool, client)

	if got := statusOf(belumID); got != "expired" {
		t.Errorf("tokopay belum lunas basi = %q, mau expired", got)
	}
	if got := statusOf(lunasID); got != "paid" {
		t.Errorf("tokopay sudah lunas basi = %q, mau paid", got)
	}
	var premium bool
	if err := pool.QueryRow(ctx, `select is_premium from profiles where id = $1::uuid`, userID).Scan(&premium); err != nil {
		t.Fatal(err)
	}
	if !premium {
		t.Errorf("profil sesi lunas basi belum premium")
	}
	if got := statusOf(gagalID); got != "pending" {
		t.Errorf("tokopay gagal dicek = %q, mau pending", got)
	}
	if got := statusOf(gopayID); got != "expired" {
		t.Errorf("gopay basi = %q, mau expired", got)
	}
	if got := statusOf(tanpaRefID); got != "expired" {
		t.Errorf("tokopay tanpa matched_ref basi = %q, mau expired", got)
	}
	if got := statusOf(segarID); got != "pending" {
		t.Errorf("tokopay segar = %q, mau pending", got)
	}
	if got := statusOf(midtransID); got != "pending" {
		t.Errorf("midtrans basi = %q, mau pending", got)
	}

	// Tanpa klien Tokopay: sesi Tokopay dilewati, GoPay tetap expired.
	belum2ID := seed("tokopay", "TRX_BELUM", true, 8011)
	gopay2ID := seed("gopay", "", true, 8012)
	ExpireStaleSessions(ctx, pool, nil)
	if got := statusOf(belum2ID); got != "pending" {
		t.Errorf("tokopay tanpa klien = %q, mau pending", got)
	}
	if got := statusOf(gopay2ID); got != "expired" {
		t.Errorf("gopay tanpa klien = %q, mau expired", got)
	}
}

// seedTestProfile + seedTestSession adalah pembantu untuk uji
// rekonsiliasi/aktivasi: satu profil uji + satu sesi subscription
// premium-30 (stale = dibuat 25 jam lalu, segar = sekarang).
func seedTestProfile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx, `
		insert into profiles (email) values ($1) returning id`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func seedTestSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, provider, trx string, stale bool, amount int) string {
	t.Helper()
	age := "now()"
	if stale {
		age = "now() - interval '25 hours'"
	}
	var trxArg any
	if trx != "" {
		trxArg = trx
	}
	var id string
	if err := pool.QueryRow(ctx, `
		insert into payment_sessions (user_id, purpose, provider, amount, unique_code, plan, expires_at, matched_ref, created_at)
		values ($1::uuid, 'subscription', $2, $3, $4, 'premium-30', now() + interval '15 minutes', $5, `+age+`)
		returning id`,
		userID, provider, amount, amount%999+1, trxArg).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func sessionStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(ctx, `select status from payment_sessions where id = $1::uuid`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestReconcileTokopayPaymentsLive(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL kosong")
	}
	srv := fakeTokopayCheck()
	defer srv.Close()
	client := payments.NewTokopayClient(srv.URL, "merchant-uji", "secret-uji")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	userID := seedTestProfile(t, ctx, pool, fmt.Sprintf("uji-rekonsiliasi-%d@local", time.Now().UnixNano()))
	cleanup := func() {
		_, _ = pool.Exec(ctx, `delete from subscriptions where user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `delete from payment_sessions where user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `delete from profiles where id = $1::uuid`, userID)
	}
	defer cleanup()

	segarLunasID := seedTestSession(t, ctx, pool, userID, "tokopay", "TRX_LUNAS", false, 8101)
	segarBelumID := seedTestSession(t, ctx, pool, userID, "tokopay", "TRX_BELUM", false, 8102)
	basiLunasID := seedTestSession(t, ctx, pool, userID, "tokopay", "TRX_LUNAS", true, 8103)

	ReconcileTokopayPayments(ctx, pool, client)

	if got := sessionStatus(t, ctx, pool, segarLunasID); got != "paid" {
		t.Errorf("tokopay segar lunas = %q, mau paid", got)
	}
	var premium bool
	if err := pool.QueryRow(ctx, `select is_premium from profiles where id = $1::uuid`, userID).Scan(&premium); err != nil {
		t.Fatal(err)
	}
	if !premium {
		t.Errorf("profil sesi lunas segar belum premium")
	}
	if got := sessionStatus(t, ctx, pool, segarBelumID); got != "pending" {
		t.Errorf("tokopay segar belum lunas = %q, mau pending", got)
	}
	if got := sessionStatus(t, ctx, pool, basiLunasID); got != "pending" {
		t.Errorf("tokopay basi lunas sesudah rekonsiliasi = %q, mau pending (urusan ExpireStaleSessions)", got)
	}
	// Jendela bersambung: sesi basi yang lunas diaktifkan ExpireStaleSessions.
	ExpireStaleSessions(ctx, pool, client)
	if got := sessionStatus(t, ctx, pool, basiLunasID); got != "paid" {
		t.Errorf("tokopay basi lunas sesudah expire = %q, mau paid", got)
	}

	// Tanpa klien: tidak ada yang berubah.
	segarLunas2ID := seedTestSession(t, ctx, pool, userID, "tokopay", "TRX_LUNAS", false, 8104)
	ReconcileTokopayPayments(ctx, pool, nil)
	if got := sessionStatus(t, ctx, pool, segarLunas2ID); got != "pending" {
		t.Errorf("tokopay tanpa klien = %q, mau pending", got)
	}
}

// TestActivateSessionConcurrentLive membuktikan anti-dobel: 8 pemanggil
// ActivateSession bersamaan pada SATU sesi hanya menghasilkan SATU
// perpanjangan premium (1 baris subscriptions, ~30 hari — bukan 8x).
func TestActivateSessionConcurrentLive(t *testing.T) {
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

	userID := seedTestProfile(t, ctx, pool, fmt.Sprintf("uji-konkurensi-%d@local", time.Now().UnixNano()))
	cleanup := func() {
		_, _ = pool.Exec(ctx, `delete from subscriptions where user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `delete from payment_sessions where user_id = $1::uuid`, userID)
		_, _ = pool.Exec(ctx, `delete from profiles where id = $1::uuid`, userID)
	}
	defer cleanup()

	sessID := seedTestSession(t, ctx, pool, userID, "tokopay", "TRX_KONKUREN", false, 8201)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- billing.ActivateSession(ctx, pool, sessID, "TRX_KONKUREN")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ActivateSession konkuren: %v", err)
		}
	}
	if got := sessionStatus(t, ctx, pool, sessID); got != "paid" {
		t.Fatalf("sesi konkuren = %q, mau paid", got)
	}
	var n int
	if err := pool.QueryRow(ctx, `select count(*) from subscriptions where user_id = $1::uuid`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("baris subscriptions = %d, mau 1 (aktivasi dobel!)", n)
	}
	var until time.Time
	if err := pool.QueryRow(ctx, `select premium_until from profiles where id = $1::uuid`, userID).Scan(&until); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if until.Before(now.Add(29*24*time.Hour)) || until.After(now.Add(31*24*time.Hour)) {
		t.Errorf("premium_until = %v, mau ~30 hari dari sekarang (dobel = ~240 hari)", until)
	}
}
