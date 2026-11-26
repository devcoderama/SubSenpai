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
	"strings"
	"testing"
	"time"

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

// TestSyncTitleHTMLSearchLive menguji ujung-ke-ujung perbaikan 2026-11-26:
// judul donghua TANPA mapping disinkron lewat resolver Anichin asli —
// mapping harus dibuat lewat pencarian HTML (/?s=, bukan wp-json yang
// mati 403), episode harus tersimpan, dan sebagian harus ter-resolve.
func TestSyncTitleHTMLSearchLive(t *testing.T) {
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

	slug := fmt.Sprintf("uji-sync-html-%d", time.Now().UnixNano())
	var titleID string
	if err := pool.QueryRow(ctx, `
		insert into titles (slug, type, title, status)
		values ($1, 'donghua', 'Tales of Herding Gods', 'ongoing')
		returning id`, slug).Scan(&titleID); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		_, _ = pool.Exec(ctx, `delete from episode_sources where episode_id in (select id from episodes where title_id = $1::uuid)`, titleID)
		_, _ = pool.Exec(ctx, `delete from episodes where title_id = $1::uuid`, titleID)
		_, _ = pool.Exec(ctx, `delete from title_sources where title_id = $1::uuid`, titleID)
		_, _ = pool.Exec(ctx, `delete from titles where id = $1::uuid`, titleID)
	}
	defer cleanup()

	var src sources.Source
	if err := pool.QueryRow(ctx, `
		select id, name, content_type, lang, coalesce(base_url,''), resolver,
		       fetch_mode, subtitle_kind, priority
		from content_sources where resolver = 'anichin'`).Scan(
		&src.ID, &src.Name, &src.ContentType, &src.Lang, &src.BaseURL,
		&src.Resolver, &src.FetchMode, &src.SubtitleKind, &src.Priority); err != nil {
		t.Fatalf("baris content_sources anichin tidak ada: %v", err)
	}
	resolver, ok := sources.ByKey("anichin")
	if !ok {
		t.Fatal("resolver anichin tidak terdaftar")
	}
	done, err := syncTitle(ctx, pool, resolver, src, titleID, "Tales of Herding Gods", 3)
	if err != nil {
		t.Fatalf("syncTitle: %v", err)
	}
	if done < 1 {
		t.Fatalf("syncTitle menyelesaikan 0 episode (mapping/episode gagal?)")
	}
	var mappingURL string
	if err := pool.QueryRow(ctx, `select source_url from title_sources where title_id = $1::uuid`, titleID).Scan(&mappingURL); err != nil {
		t.Fatalf("mapping tidak dibuat: %v", err)
	}
	if !strings.Contains(strings.ToLower(mappingURL), "herding") {
		t.Fatalf("mapping mengarah ke judul yang salah: %s", mappingURL)
	}
	var epCount int
	if err := pool.QueryRow(ctx, `select count(*) from episodes where title_id = $1::uuid`, titleID).Scan(&epCount); err != nil {
		t.Fatal(err)
	}
	if epCount < 1 {
		t.Fatal("tidak ada episode tersimpan")
	}
	t.Logf("mapping %s, %d episode tersimpan, %d ter-resolve", mappingURL, epCount, done)
}

// TestSelectSyncTitlesLive menguji seleksi sync perbaikan 2026-11-26:
// judul dengan episode belum bersumber didahulukan; judul yang gagal
// sync <6 jam lalu dilewati; yang gagal >6 jam lalu ikut lagi.
func TestSelectSyncTitlesLive(t *testing.T) {
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

	suffix := fmt.Sprint(time.Now().UnixNano())
	mkTitle := func(slug string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			insert into titles (slug, type, title, status)
			values ($1, 'donghua', $2, 'ongoing') returning id`,
			slug, "Uji seleksi "+slug).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	slugA := "uji-sel-a-" + suffix
	slugB := "uji-sel-b-" + suffix
	slugC := "uji-sel-c-" + suffix
	slugD := "uji-sel-d-" + suffix
	idA := mkTitle(slugA)
	idB := mkTitle(slugB)
	idC := mkTitle(slugC)
	idD := mkTitle(slugD)
	_ = idA
	_ = idD
	cleanup := func() {
		for _, id := range []string{idA, idB, idC, idD} {
			_, _ = pool.Exec(ctx, `delete from episode_sources where episode_id in (select id from episodes where title_id = $1::uuid)`, id)
			_, _ = pool.Exec(ctx, `delete from episodes where title_id = $1::uuid`, id)
			_, _ = pool.Exec(ctx, `delete from titles where id = $1::uuid`, id)
		}
	}
	defer cleanup()

	// B: gagal baru saja → harus dilewati.
	if _, err := pool.Exec(ctx, `update titles set last_sync_at = now(), last_sync_error = 'uji gagal' where id = $1::uuid`, idB); err != nil {
		t.Fatal(err)
	}
	// D: gagal 7 jam lalu → cooldown habis, ikut lagi.
	if _, err := pool.Exec(ctx, `update titles set last_sync_at = now() - interval '7 hours', last_sync_error = 'uji gagal' where id = $1::uuid`, idD); err != nil {
		t.Fatal(err)
	}
	// C: 2 episode, 1 belum bersumber → harus didahulukan.
	var ep1, ep2 string
	for i, dst := range []*string{&ep1, &ep2} {
		if err := pool.QueryRow(ctx, `
			insert into episodes (title_id, episode_no, video_url)
			values ($1::uuid, $2, $3) returning id`,
			idC, i+1, fmt.Sprintf("https://uji.local/c%d", i+1)).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `
		insert into episode_sources (episode_id, url, label, lang)
		values ($1::uuid, 'https://cdn.uji.local/c1.mp4', 'HD', 'id')`, ep1); err != nil {
		t.Fatal(err)
	}

	got, err := selectSyncTitles(ctx, pool, "donghua", 500)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, it := range got {
		pos[it.id] = i
	}
	if _, ok := pos[idB]; ok {
		t.Fatal("judul yang gagal <6 jam lalu ikut terpilih (cooldown tidak jalan)")
	}
	for _, id := range []string{idA, idC, idD} {
		if _, ok := pos[id]; !ok {
			t.Fatalf("judul %s tidak terpilih", id)
		}
	}
	if pos[idC] > pos[idA] {
		t.Fatal("judul dengan episode belum bersumber tidak didahulukan atas judul eps=0")
	}
}

// TestLoadSourcesSkipsDownLive menguji perbaikan 2026-11-26: sumber
// dengan health='down' dilewati loadSources sampai pulih.
func TestLoadSourcesSkipsDownLive(t *testing.T) {
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

	srcID := "00000000-0000-0000-0000-000000000099"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `delete from content_sources where id = $1::uuid`, srcID)
	}
	cleanup()
	defer cleanup()
	if _, err := pool.Exec(ctx, `
		insert into content_sources (id, name, content_type, lang, base_url, resolver,
		                             fetch_mode, subtitle_kind, priority, is_active, health)
		values ($1::uuid, 'Uji Down', 'donghua', 'id', 'https://example.invalid',
		        'uji-down', 'server', 'hard', 99, true, 'down')`, srcID); err != nil {
		t.Fatal(err)
	}
	has := func() bool {
		srcs, err := loadSourcesMode(ctx, pool, "donghua", true)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range srcs {
			if s.Resolver == "uji-down" {
				return true
			}
		}
		return false
	}
	if has() {
		t.Fatal("sumber health=down ikut dimuat")
	}
	if _, err := pool.Exec(ctx, `update content_sources set health = 'ok' where id = $1::uuid`, srcID); err != nil {
		t.Fatal(err)
	}
	if !has() {
		t.Fatal("sumber health=ok tidak dimuat kembali")
	}
}
