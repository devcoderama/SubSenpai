package jobs

import (
	"context"
	"os"
	"sync"
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

// ReportLaneProgress dari 2 jalur bersamaan: status gabungan
// menjumlahkan kedua jalur, rincian per jalur terbaca di Lanes,
// dan fase berurutan sesudahnya membersihkan jalur.
func TestLaneProgressLive(t *testing.T) {
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

	var wg sync.WaitGroup
	for _, ln := range []struct {
		typ   string
		total int
	}{{"anime", 5}, {"donghua", 3}} {
		wg.Add(1)
		go func(typ string, total int) {
			defer wg.Done()
			for i := 1; i <= total; i++ {
				ReportLaneProgress(ctx, pool, "Mengisi episode", typ, i, total, "Judul "+typ)
			}
		}(ln.typ, ln.total)
	}
	wg.Wait()

	st := ReadWorkerStatus(ctx, pool)
	if st == nil {
		t.Fatal("status nil setelah ReportLaneProgress")
	}
	// Panggilan terakhir tiap jalur menyelesaikan agregat (8/8)
	// sehingga tulisannya lolos pembatas 2 detik.
	if st.Phase != "Mengisi episode" || st.Done != 8 || st.Total != 8 {
		t.Fatalf("agregat = %+v", st)
	}
	if len(st.Lanes) != 2 || st.Lanes["anime"].Done != 5 || st.Lanes["anime"].Total != 5 ||
		st.Lanes["donghua"].Done != 3 || st.Lanes["donghua"].Total != 3 {
		t.Fatalf("lanes = %+v", st.Lanes)
	}

	ReportProgress(ctx, pool, "Selesai", 0, 0, "")
	st = ReadWorkerStatus(ctx, pool)
	if st == nil || len(st.Lanes) != 0 {
		t.Fatalf("lanes tidak dibersihkan fase berurutan: %+v", st)
	}
}
