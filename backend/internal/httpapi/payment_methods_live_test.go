package httpapi

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Uji hidup sekali jalan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL
// (database uji lokal yang sudah dimigrasikan sampai 0015).
func TestPaymentMethodsLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	a := &API{pool: pool}
	ctx := context.Background()

	methods := a.loadPaymentMethods(ctx, true)
	if len(methods) != 23 {
		t.Fatalf("metode aktif = %d, mau 23", len(methods))
	}
	if methods[0].Code != "BRIVA" || methods[0].IconURL == "" {
		t.Fatalf("metode pertama salah: %+v", methods[0])
	}
	if !a.tokopayChannelActive(ctx, "QRIS") {
		t.Fatal("QRIS harus aktif")
	}
	if a.tokopayChannelActive(ctx, "OVO") {
		t.Fatal("OVO tidak boleh dikenal (Tokopay tidak punya OVO)")
	}
	// Nonaktifkan DANA → validasi checkout harus menolaknya.
	if _, err := pool.Exec(ctx, `update payment_methods set is_active = false where code = 'DANA'`); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `update payment_methods set is_active = true where code = 'DANA'`)
	if a.tokopayChannelActive(ctx, "DANA") {
		t.Fatal("DANA nonaktif harus ditolak")
	}
	if got := len(a.loadPaymentMethods(ctx, true)); got != 22 {
		t.Fatalf("metode aktif setelah nonaktif = %d, mau 22", got)
	}
	if got := len(a.loadPaymentMethods(ctx, false)); got != 23 {
		t.Fatalf("semua metode = %d, mau 23", got)
	}
}
