package jobs

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ReportProgress -> ReadWorkerStatus pulang-pergi lewat
// app_settings.
func TestWorkerStatusRoundTrip(t *testing.T) {
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
	defer pool.Exec(ctx, `delete from app_settings where key = $1`, workerStatusKey)

	ReportProgress(ctx, pool, "Melengkapi metadata", 3, 3, "Judul Uji")
	st := ReadWorkerStatus(ctx, pool)
	if st == nil {
		t.Fatal("status nil setelah ReportProgress")
	}
	if st.Phase != "Melengkapi metadata" || st.Done != 3 || st.Total != 3 || st.Current != "Judul Uji" {
		t.Fatalf("status = %+v", st)
	}
}
