package jobs

// Uji audit & perbaikan katalog terhadap database uji lokal
// (DATABASE_URL). Perbaikan satu judul diganti tiruan agar uji
// tidak menyentuh situs sumber: satu judul "diperbaiki" beneran di
// database, satu dibiarkan — hasil audit + penanda percobaan yang
// diperiksa.

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepairCatalogLive(t *testing.T) {
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

	slugs := []string{"uji-lengkap", "uji-bolong", "uji-setengah", "uji-jeda"}
	cleanup := func() {
		for _, s := range slugs {
			_, _ = pool.Exec(ctx, `delete from titles where slug = $1`, s)
		}
	}
	cleanup()
	defer cleanup()

	seed := func(slug, typ string, poster, synopsis any, year any, genres []string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			insert into titles (slug, type, title, poster_url, synopsis, year, genres)
			values ($1, $2, $3, $4, $5, $6, $7) returning id`,
			slug, typ, "Judul "+slug, poster, synopsis, year, genres).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	addEp := func(titleID string, no int, videoURL string, withSource bool) {
		var epID string
		if err := pool.QueryRow(ctx, `
			insert into episodes (title_id, episode_no, video_url)
			values ($1::uuid, $2, $3) returning id`, titleID, no, videoURL).Scan(&epID); err != nil {
			t.Fatal(err)
		}
		if withSource {
			if _, err := pool.Exec(ctx, `
				insert into episode_sources (episode_id, url, label, lang)
				values ($1::uuid, 'https://cdn.example/v.mp4', 'HD', 'id')`, epID); err != nil {
				t.Fatal(err)
			}
		}
	}

	lengkapID := seed("uji-lengkap", "anime", "https://img.example/p.jpg", "Sinopsis lengkap.", 2020, []string{"Action"})
	addEp(lengkapID, 1, "https://cdn.example/v.mp4", true)

	bolongID := seed("uji-bolong", "anime", "https://img.example/p.jpg", nil, nil, []string{})
	// bolong: tanpa sinopsis (2) + tahun (1) + genre (1) + episode (5) = 9

	setengahID := seed("uji-setengah", "donghua", "https://img.example/p.jpg", "Ada sinopsis.", 2021, []string{"Drama"})
	addEp(setengahID, 1, "https://cdn.example/v.mp4", true)
	addEp(setengahID, 2, "", false)
	// setengah: 1 episode tanpa sumber = skor 1

	jedaID := seed("uji-jeda", "film", "https://img.example/p.jpg", nil, nil, []string{})
	if _, err := pool.Exec(ctx, `update titles set last_repair_at = now() where id = $1::uuid`, jedaID); err != nil {
		t.Fatal(err)
	}

	cands, err := auditCatalog(ctx, pool, 50)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range cands {
		got = append(got, c.id)
		if c.id == lengkapID || c.id == jedaID {
			t.Fatalf("judul tak seharusnya jadi kandidat: %v", c)
		}
	}
	if len(got) < 2 || got[0] != bolongID {
		t.Fatalf("urutan kandidat salah: %v", got)
	}
	b, err := auditTitle(ctx, pool, bolongID)
	if err != nil || b.score != 9 {
		t.Fatalf("audit bolong: skor=%d err=%v (%v)", b.score, err, b.missing)
	}
	s, err := auditTitle(ctx, pool, setengahID)
	if err != nil || s.score != 1 || s.unsourced != 1 {
		t.Fatalf("audit setengah: skor=%d unsourced=%d err=%v", s.score, s.unsourced, err)
	}

	// Tiruan perbaikan: bolong diisi sampai lengkap; setengah
	// dibiarkan (sumber ceritanya rusak).
	old := repairOneTitle
	repairOneTitle = func(ctx context.Context, pool *pgxpool.Pool, c repairCandidate) {
		if c.id != bolongID {
			return
		}
		_, _ = pool.Exec(ctx, `
			update titles set synopsis = 'Sudah diisi.', year = 2022, genres = '{Action}'
			where id = $1::uuid`, c.id)
		_, _ = pool.Exec(ctx, `
			insert into episodes (title_id, episode_no, video_url)
			values ($1::uuid, 1, 'https://cdn.example/v.mp4')`, c.id)
	}
	defer func() { repairOneTitle = old }()

	if err := RepairCatalog(ctx, pool); err != nil {
		t.Fatal(err)
	}
	after, err := auditTitle(ctx, pool, bolongID)
	if err != nil || after.score != 0 {
		t.Fatalf("sesudah perbaikan bolong: skor=%d err=%v", after.score, err)
	}
	var attempts int
	var lastAt *string
	if err := pool.QueryRow(ctx, `
		select repair_attempts, last_repair_at::text from titles where id = $1::uuid`,
		bolongID).Scan(&attempts, &lastAt); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 || lastAt == nil {
		t.Fatalf("penanda bolong: attempts=%d last=%v", attempts, lastAt)
	}
	if err := pool.QueryRow(ctx, `
		select repair_attempts from titles where id = $1::uuid`, setengahID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("penanda setengah: attempts=%d, want 1", attempts)
	}
}
