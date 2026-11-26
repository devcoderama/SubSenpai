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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

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
