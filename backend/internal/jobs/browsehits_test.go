package jobs

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/sources"
)

// StoreBrowseHits ber-DB: item berposter tersimpan + terpetakan,
// yang tanpa poster dilewati, panggilan ulang idempoten.
func TestStoreBrowseHitsLive(t *testing.T) {
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

	var srcID string
	if err := pool.QueryRow(ctx, `
		insert into content_sources (name, content_type, lang, base_url, resolver, fetch_mode, subtitle_kind, priority, is_active)
		values ('Sumber Jelajah Uji', 'anime', 'zz', 'https://uji.invalid', 'kuronime', 'server', 'hard', 99, true)
		returning id`).Scan(&srcID); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `delete from content_sources where id = $1::uuid`, srcID)
	defer pool.Exec(ctx, `delete from titles where slug in ('jelajah-uji-satu', 'jelajah-uji-dua')`)

	src := sources.Source{ID: srcID}
	items := []sources.DiscoveredTitle{
		{Title: "Jelajah Uji Satu", Slug: "jelajah-uji-satu", Poster: "https://cdn.invalid/p1.jpg", Year: 2024, Genres: []string{"Action"}, URL: "https://uji.invalid/anime/jelajah-uji-satu/"},
		{Title: "Jelajah Uji Dua", Slug: "jelajah-uji-dua", Poster: "", URL: "https://uji.invalid/anime/jelajah-uji-dua/"},
	}
	if n := StoreBrowseHits(ctx, pool, "anime", src, items); n != 1 {
		t.Fatalf("tersimpan = %d, mau 1 (yang berposter saja)", n)
	}
	var cnt, mapped int
	if err := pool.QueryRow(ctx,
		`select count(*) from titles where slug = 'jelajah-uji-satu'`).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("judul tersimpan: count=%d err=%v", cnt, err)
	}
	if err := pool.QueryRow(ctx, `
		select count(*) from title_sources ts
		join titles t on t.id = ts.title_id
		where t.slug = 'jelajah-uji-satu' and ts.source_id = $1::uuid`, srcID).Scan(&mapped); err != nil || mapped != 1 {
		t.Fatalf("pemetaan sumber: count=%d err=%v", mapped, err)
	}
	if err := pool.QueryRow(ctx,
		`select count(*) from titles where slug = 'jelajah-uji-dua'`).Scan(&cnt); err != nil || cnt != 0 {
		t.Fatalf("judul tanpa poster harus dilewati: count=%d err=%v", cnt, err)
	}
	// Idempoten: simpan ulang tidak menggandakan judul/pemetaan.
	_ = StoreBrowseHits(ctx, pool, "anime", src, items)
	if err := pool.QueryRow(ctx, `
		select count(*) from title_sources ts
		join titles t on t.id = ts.title_id
		where t.slug = 'jelajah-uji-satu'`).Scan(&mapped); err != nil || mapped != 1 {
		t.Fatalf("setelah simpan ulang: pemetaan=%d err=%v", mapped, err)
	}
}
