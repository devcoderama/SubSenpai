package jobs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/db"
)

// TestTVChannelsLive: migrasi 0030 men-seed 38 kanal kurasi (18 bagus
// + 20 tidak stabil), dan CheckTVChannels memperbarui status kanal
// uji yang menunjuk ke server lokal: yang hidup fail_count=0 +
// last_ok_at terisi; yang mati fail_count naik dan setelah 3x gagal
// beruntun is_active=false.
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestTVChannelsLive(t *testing.T) {
	if os.Getenv("SUBSENPAI_LIVE_TEST") != "1" {
		t.Skip("khusus uji hidup")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrasi: %v", err)
	}

	var total, bagus, tidakStabil int
	err = pool.QueryRow(ctx, `
		select count(*),
		       count(*) filter (where stability = 'bagus'),
		       count(*) filter (where stability = 'tidak stabil')
		from tv_channels`).Scan(&total, &bagus, &tidakStabil)
	if err != nil {
		t.Fatal(err)
	}
	if total != 38 || bagus != 18 || tidakStabil != 20 {
		t.Fatalf("seed: total=%d bagus=%d tidak stabil=%d, mau 38/18/20", total, bagus, tidakStabil)
	}

	// Kanal uji: satu hidup (server lokal), satu mati (port tertutup).
	// Sejak v2 probe wajib menemukan segmen — server hidup mengembalikan
	// playlist berisi #EXTINF, bukan sekadar #EXTM3U kosong.
	alive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-VERSION:3\n#EXTINF:6.0,\nseg0.ts\n"))
	}))
	defer alive.Close()
	var aliveID, deadID string
	err = pool.QueryRow(ctx, `
		insert into tv_channels (name, slug, stream_url, category, stability, source, sort_order)
		values ('Uji Hidup', 'uji-hidup', $1, 'Nasional', 'bagus', 'resmi', 900)
		returning id`, alive.URL).Scan(&aliveID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
		insert into tv_channels (name, slug, stream_url, category, stability, source, sort_order, fail_count)
		values ('Uji Mati', 'uji-mati', 'http://127.0.0.1:1/mati.m3u8', 'Nasional', 'tidak stabil', 'restream', 901, 2)
		returning id`).Scan(&deadID)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from tv_channels where id in ($1::uuid, $2::uuid)`, aliveID, deadID)

	if err := CheckTVChannels(ctx, pool); err != nil {
		t.Fatalf("CheckTVChannels: %v", err)
	}
	var fails int
	var lastOK *string
	var active bool
	err = pool.QueryRow(ctx, `select fail_count, last_ok_at::text, is_active from tv_channels where id = $1::uuid`,
		aliveID).Scan(&fails, &lastOK, &active)
	if err != nil {
		t.Fatal(err)
	}
	if fails != 0 || lastOK == nil || !active {
		t.Errorf("kanal hidup: fails=%d last_ok=%v active=%v", fails, lastOK, active)
	}
	err = pool.QueryRow(ctx, `select fail_count, is_active from tv_channels where id = $1::uuid`,
		deadID).Scan(&fails, &active)
	if err != nil {
		t.Fatal(err)
	}
	// fail_count awal 2 + 1 gagal = 3 → disembunyikan.
	if fails != 3 || active {
		t.Errorf("kanal mati: fails=%d active=%v, mau 3/false", fails, active)
	}
}
