package jobs

// Uji syncTitle terhadap database uji lokal (DATABASE_URL) dengan
// resolver tiruan — tidak menyentuh situs sumber. Memastikan
// penulisan ulang percepatan 2026-11-26 benar: episode yang sudah
// bersumber tidak di-resolve ulang, episode baru dibuat barisnya,
// anggaran resolve dihormati, dan pemanggilan kedua tidak
// mengerjakan apa-apa lagi.

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/sources"
)

type stubResolver struct{}

func (stubResolver) Key() string { return "uji" }

func (stubResolver) Search(ctx context.Context, src sources.Source, query string) ([]sources.Candidate, error) {
	return nil, fmt.Errorf("search tidak boleh dipanggil di uji ini")
}

func (stubResolver) Episodes(ctx context.Context, src sources.Source, titleURL string) ([]sources.EpisodeRef, error) {
	return []sources.EpisodeRef{
		{Number: 1, Title: "Episode 1", URL: "https://uji.local/ep1"},
		{Number: 2, Title: "Episode 2", URL: "https://uji.local/ep2"},
		{Number: 3, Title: "Episode 3", URL: "https://uji.local/ep3"},
		{Number: 4, Title: "Episode 4", URL: "https://uji.local/ep4"},
		{Number: 5, Title: "Episode 5", URL: "https://uji.local/ep5"},
	}, nil
}

func (stubResolver) Resolve(ctx context.Context, src sources.Source, ep sources.EpisodeRef) (sources.Resolved, error) {
	return sources.Resolved{Streams: []sources.Stream{
		{URL: fmt.Sprintf("https://cdn.uji.local/ep%d.mp4", ep.Number), Label: "HD", Lang: "id"},
	}}, nil
}

func TestSyncTitleLive(t *testing.T) {
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

	const slug = "uji-sync-title"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `delete from titles where slug = $1`, slug)
		_, _ = pool.Exec(ctx, `delete from content_sources where resolver = 'uji'`)
	}
	cleanup()
	defer cleanup()

	var titleID string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title, poster_url, synopsis, year, genres)
		values ($1, 'donghua', 'Judul uji-sync', 'https://img.example/p.jpg', 'Sinopsis.', 2024, '{Action}')
		returning id`, slug).Scan(&titleID); err != nil {
		t.Fatal(err)
	}
	srcID := "00000000-0000-0000-0000-000000000001"
	if _, err := pool.Exec(ctx, `
		insert into content_sources (id, name, content_type, lang, base_url, resolver, fetch_mode, subtitle_kind, priority, is_active)
		values ($1::uuid, 'Uji', 'donghua', 'id', 'https://uji.local', 'uji', 'server', 'hard', 10, true)
		on conflict (resolver, content_type, lang) do nothing`, srcID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		insert into title_sources (title_id, source_id, source_url, source_ref)
		values ($1::uuid, $2::uuid, 'https://uji.local/series/x', 'x')`, titleID, srcID); err != nil {
		t.Fatal(err)
	}
	addEp := func(no int, withSource bool) string {
		var epID string
		if err := pool.QueryRow(ctx, `
			insert into episodes (title_id, episode_no, video_url)
			values ($1::uuid, $2, $3) returning id`, titleID, no, fmt.Sprintf("https://uji.local/ep%d", no)).Scan(&epID); err != nil {
			t.Fatal(err)
		}
		if withSource {
			if _, err := pool.Exec(ctx, `
				insert into episode_sources (episode_id, url, label, lang)
				values ($1::uuid, 'https://cdn.uji.local/lama.mp4', 'HD', 'id')`, epID); err != nil {
				t.Fatal(err)
			}
		}
		return epID
	}
	countSources := func(epID string) int {
		var n int
		if err := pool.QueryRow(ctx, `select count(*) from episode_sources where episode_id = $1::uuid`, epID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	epIDByNo := func(no int) (string, bool) {
		var id string
		if err := pool.QueryRow(ctx, `select id from episodes where title_id = $1::uuid and episode_no = $2`, titleID, no).Scan(&id); err != nil {
			return "", false
		}
		return id, true
	}

	// Keadaan awal: ep 1 sudah bersumber, ep 2 belum, ep 3-5 belum
	// ada barisnya. Kandidat resolve: ep 2, 3, 4, 5.
	ep1ID := addEp(1, true)
	ep2ID := addEp(2, false)
	src := sources.Source{ID: srcID, Name: "uji", Resolver: "uji"}

	// Anggaran 1 → tepat 1 episode (ep 2, yang pertama belum
	// bersumber); ep 3-5 belum tersentuh.
	done, err := syncTitle(ctx, pool, stubResolver{}, src, titleID, "Judul uji-sync", 1)
	if err != nil {
		t.Fatal(err)
	}
	if done != 1 {
		t.Fatalf("budget 1: done=%d, mau 1", done)
	}
	if n := countSources(ep2ID); n != 1 {
		t.Errorf("ep2 sumber=%d, mau 1", n)
	}
	// Baris ep 3-5 boleh sudah dibuat (pembuatan baris tidak
	// dihitung anggaran), tapi tidak satu pun boleh punya sumber.
	var nUnsourced int
	if err := pool.QueryRow(ctx, `
		select count(*) from episode_sources es
		join episodes e on e.id = es.episode_id
		where e.title_id = $1::uuid and e.episode_no in (3,4,5)`, titleID).Scan(&nUnsourced); err != nil {
		t.Fatal(err)
	}
	if nUnsourced != 0 {
		t.Errorf("ep3-5 sudah punya %d sumber padahal anggaran habis", nUnsourced)
	}

	// Anggaran 10 → sisa 3 kandidat (ep 3, 4, 5) selesai; ep 1
	// tetap 1 sumber (tidak di-resolve ulang).
	done, err = syncTitle(ctx, pool, stubResolver{}, src, titleID, "Judul uji-sync", 10)
	if err != nil {
		t.Fatal(err)
	}
	if done != 3 {
		t.Fatalf("budget 10: done=%d, mau 3 (ep 3,4,5)", done)
	}
	if n := countSources(ep1ID); n != 1 {
		t.Errorf("ep1 sumber=%d, mau 1 (tidak di-resolve ulang)", n)
	}
	for _, no := range []int{3, 4, 5} {
		id, ok := epIDByNo(no)
		if !ok {
			t.Fatalf("ep%d belum dibuat", no)
		}
		if n := countSources(id); n != 1 {
			t.Errorf("ep%d sumber=%d, mau 1", no, n)
		}
	}

	// Panggilan ketiga: semua sudah bersumber → tidak ada kerja.
	done, err = syncTitle(ctx, pool, stubResolver{}, src, titleID, "Judul uji-sync", 10)
	if err != nil {
		t.Fatal(err)
	}
	if done != 0 {
		t.Fatalf("syncTitle ketiga: done=%d, mau 0", done)
	}
}
