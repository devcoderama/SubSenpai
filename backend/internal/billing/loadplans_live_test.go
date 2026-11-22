package billing

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Uji hidup sekali jalan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestLoadPlansLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	plans := LoadPlans(context.Background(), pool)
	if len(plans) != 5 {
		t.Fatalf("jumlah paket = %d, mau 5", len(plans))
	}
	p, ok := PlanByIDCtx(context.Background(), pool, "premium-1095")
	if !ok || p.Price != 180000 || p.Days != 1095 {
		t.Fatalf("premium-1095 salah: %+v ok=%v", p, ok)
	}
	if plans[0].ID != "premium-30" || plans[0].Price != 8000 {
		t.Fatalf("paket pertama salah: %+v", plans[0])
	}
}
