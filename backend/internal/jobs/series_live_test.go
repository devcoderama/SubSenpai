package jobs

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/db"
)

// TestSeriesLive: alur serial TV ujung-ke-ujung lawan API Hashflix
// asli + DB uji lokal: migrasi 0028 (tipe series + season_no +
// baris sumber Hashflix series) → DiscoverType("series") →
// LiveSearchStore Breaking Bad → EnrichOneTitle + SyncOneTitle
// (62 episode, 5 season) → episode_sources berisi URL embed TV.
// Sinkron ulang harus idempotent (tetap 62 episode).
//
// Jalan dengan: SUBSENPAI_LIVE_TEST=1 + DATABASE_URL.
func TestSeriesLive(t *testing.T) {
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

	var nSrc int
	if err := pool.QueryRow(ctx, `select count(*) from content_sources where resolver = 'hashflix' and content_type = 'series'`).Scan(&nSrc); err != nil || nSrc != 1 {
		t.Fatalf("baris sumber Hashflix series mau 1, dapat %d (err=%v)", nSrc, err)
	}

	n, err := DiscoverType(ctx, pool, "series")
	if err != nil {
		t.Fatalf("DiscoverType series: %v", err)
	}
	var nTitles int
	if err := pool.QueryRow(ctx, `select count(*) from titles where type = 'series'`).Scan(&nTitles); err != nil {
		t.Fatal(err)
	}
	t.Logf("discover series: %d judul baru, total series di katalog %d", n, nTitles)
	if nTitles == 0 {
		t.Fatal("katalog series kosong sesudah discover")
	}

	if _, err := LiveSearchStore(ctx, pool, "series", "breaking bad"); err != nil {
		t.Fatalf("LiveSearchStore series: %v", err)
	}
	var bbID, bbTitle string
	if err := pool.QueryRow(ctx, `select id, title from titles where slug = 'breaking-bad-1396'`).Scan(&bbID, &bbTitle); err != nil {
		t.Fatalf("Breaking Bad tidak masuk katalog: %v", err)
	}
	if _, err := EnrichOneTitle(ctx, pool, bbID); err != nil {
		t.Fatalf("EnrichOneTitle: %v", err)
	}
	var year int
	var synopsis string
	if err := pool.QueryRow(ctx, `select year, coalesce(synopsis,'') from titles where id = $1::uuid`, bbID).Scan(&year, &synopsis); err != nil {
		t.Fatal(err)
	}
	if year != 2008 || synopsis == "" {
		t.Fatalf("metadata BB salah: tahun=%d synopsis=%dB", year, len(synopsis))
	}
	if err := SyncOneTitle(ctx, pool, bbID, bbTitle, "series"); err != nil {
		t.Fatalf("SyncOneTitle: %v", err)
	}
	rows, err := pool.Query(ctx, `select season_no, count(*) from episodes where title_id = $1::uuid group by season_no order by season_no`, bbID)
	if err != nil {
		t.Fatal(err)
	}
	seasons := map[int]int{}
	total := 0
	for rows.Next() {
		var s, c int
		if err := rows.Scan(&s, &c); err == nil {
			seasons[s] = c
			total += c
		}
	}
	rows.Close()
	t.Logf("episode BB: total %d, per season %v", total, seasons)
	if total != 62 || seasons[1] != 7 || seasons[2] != 13 || seasons[3] != 13 || seasons[4] != 13 || seasons[5] != 16 {
		t.Fatalf("episode per season salah: %v", seasons)
	}
	var tvURL string
	if err := pool.QueryRow(ctx, `
		select es.url from episodes e join episode_sources es on es.episode_id = e.id
		where e.title_id = $1::uuid and e.season_no = 1 and e.episode_no = 1
		order by es.created_at limit 1`, bbID).Scan(&tvURL); err != nil {
		t.Fatalf("sumber S1E1 tidak ada: %v", err)
	}
	if tvURL != "https://vidstuck.cc/embed/tv/1396/1/1" {
		t.Fatalf("sumber S1E1 = %q, mau embed tv vidstuck", tvURL)
	}
	// Sinkron ulang: idempotent.
	if err := SyncOneTitle(ctx, pool, bbID, bbTitle, "series"); err != nil {
		t.Fatalf("SyncOneTitle ulang: %v", err)
	}
	var total2 int
	if err := pool.QueryRow(ctx, `select count(*) from episodes where title_id = $1::uuid`, bbID).Scan(&total2); err != nil || total2 != 62 {
		t.Fatalf("sesudah sinkron ulang mau 62 episode, dapat %d (err=%v)", total2, err)
	}
}
