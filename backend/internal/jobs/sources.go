package jobs

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/sources"
)

// CheckSources memeriksa kesehatan semua sumber aktif di registri (F40/O9)
// dan mencatat hasilnya ke content_sources.
func CheckSources(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		select id, base_url from content_sources where is_active`)
	if err != nil {
		return err
	}
	type item struct{ id, base string }
	items := []item{}
	for rows.Next() {
		var it item
		var base *string
		if err := rows.Scan(&it.id, &base); err == nil && base != nil {
			it.base = *base
			items = append(items, it)
		}
	}
	rows.Close()
	for _, it := range items {
		health := "down"
		if sources.Health(ctx, it.base) {
			health = "ok"
		}
		if _, err := pool.Exec(ctx, `
			update content_sources
			set health = $2, last_checked_at = now(), updated_at = now()
			where id = $1::uuid`, it.id, health); err != nil {
			log.Printf("cek sumber %s: %v", it.id, err)
		}
	}
	log.Printf("cek kesehatan sumber: %d sumber diperiksa", len(items))
	return nil
}

func loadSources(ctx context.Context, pool *pgxpool.Pool, contentType string) ([]sources.Source, error) {
	rows, err := pool.Query(ctx, `
		select id, name, content_type, lang, coalesce(base_url,''), resolver,
		       fetch_mode, subtitle_kind, priority
		from content_sources
		where is_active and fetch_mode = 'server' and base_url is not null
		  and content_type = $1
		order by priority, name`, contentType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []sources.Source{}
	for rows.Next() {
		var s sources.Source
		if err := rows.Scan(&s.ID, &s.Name, &s.ContentType, &s.Lang, &s.BaseURL,
			&s.Resolver, &s.FetchMode, &s.SubtitleKind, &s.Priority); err == nil {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

// maxResolvePerRun membatasi episode yang diselesaikan per putaran sinkron
// agar satu putaran worker tidak berjalan tanpa batas.
const maxResolvePerRun = 20

// SyncSources menjalankan resolver untuk judul-judul katalog: memetakan
// judul ke halaman sumbernya (pencarian pertama yang cocok), mengambil
// daftar episode, dan menyelesaikan aliran + trek subtitle episode yang
// belum punya sumber putar. Kegagalan per judul/sumber dicatat, tidak
// menghentikan putaran.
func SyncSources(ctx context.Context, pool *pgxpool.Pool) error {
	trows, err := pool.Query(ctx, `
		select id, type, title from titles
		where type in ('anime','donghua','film')
		order by created_at limit 50`)
	if err != nil {
		return err
	}
	type title struct{ id, typ, name string }
	titles := []title{}
	for trows.Next() {
		var t title
		if err := trows.Scan(&t.id, &t.typ, &t.name); err == nil {
			titles = append(titles, t)
		}
	}
	trows.Close()

	resolved := 0
	for _, t := range titles {
		srcs, err := loadSources(ctx, pool, t.typ)
		if err != nil {
			return err
		}
		for _, src := range srcs {
			resolver, ok := sources.ByKey(src.Resolver)
			if !ok {
				continue // resolver belum diimplementasikan (menyusul per situs)
			}
			if err := syncTitle(ctx, pool, resolver, src, t.id, t.name, &resolved); err != nil {
				log.Printf("sinkron %s dari %s: %v", t.name, src.Name, err)
			}
			if resolved >= maxResolvePerRun {
				log.Printf("sinkron sumber: batas %d episode tercapai", maxResolvePerRun)
				return nil
			}
		}
	}
	log.Printf("sinkron sumber selesai: %d episode diselesaikan", resolved)
	return nil
}

func syncTitle(ctx context.Context, pool *pgxpool.Pool, resolver sources.Resolver,
	src sources.Source, titleID, titleName string, resolved *int) error {
	// 1) Pemetaan judul → halaman sumber (buat lewat pencarian bila belum ada).
	var titleURL string
	err := pool.QueryRow(ctx, `
		select source_url from title_sources
		where title_id = $1::uuid and source_id = $2::uuid`, titleID, src.ID).Scan(&titleURL)
	if err != nil {
		cands, err := resolver.Search(ctx, src, titleName)
		if err != nil {
			return err
		}
		if len(cands) == 0 {
			return nil // tidak ditemukan di sumber ini — bukan kegagalan
		}
		titleURL = cands[0].URL
		if _, err := pool.Exec(ctx, `
			insert into title_sources (title_id, source_id, source_url, source_ref)
			values ($1::uuid, $2::uuid, $3, $4)
			on conflict (title_id, source_id) do nothing`,
			titleID, src.ID, titleURL, nullIfEmpty(cands[0].Ref)); err != nil {
			return err
		}
		log.Printf("pemetaan baru: %s → %s (%s)", titleName, src.Name, titleURL)
	}

	// 2) Daftar episode dari sumber → baris episodes katalog.
	eps, err := resolver.Episodes(ctx, src, titleURL)
	if err != nil {
		return err
	}
	for _, ep := range eps {
		var epID string
		err := pool.QueryRow(ctx, `
			select id from episodes where title_id = $1::uuid and episode_no = $2`,
			titleID, ep.Number).Scan(&epID)
		if err != nil {
			if err := pool.QueryRow(ctx, `
				insert into episodes (title_id, episode_no, title, video_url)
				values ($1::uuid, $2, $3, $4) returning id`,
				titleID, ep.Number, nullIfEmpty(ep.Title), ep.URL).Scan(&epID); err != nil {
				return err
			}
		}
		// 3) Selesaikan aliran + trek untuk episode yang belum punya sumber.
		var have int
		if err := pool.QueryRow(ctx, `
			select count(*) from episode_sources where episode_id = $1::uuid`,
			epID).Scan(&have); err != nil {
			return err
		}
		if have > 0 || *resolved >= maxResolvePerRun {
			continue
		}
		res, err := resolver.Resolve(ctx, src, ep)
		if err != nil {
			log.Printf("resolve %s ep %d dari %s: %v", titleName, ep.Number, src.Name, err)
			continue
		}
		for _, st := range res.Streams {
			if _, err := pool.Exec(ctx, `
				insert into episode_sources (episode_id, url, label, lang)
				values ($1::uuid, $2, $3, $4)`,
				epID, st.URL, nullIfEmpty(st.Label), nullIfEmpty(st.Lang)); err != nil {
				return err
			}
		}
		for _, tr := range res.Tracks {
			if _, err := pool.Exec(ctx, `
				insert into subtitles (episode_id, lang, source, format, file_url, status)
				values ($1::uuid, $2, 'sumber', $3, $4, 'done')
				on conflict (episode_id, lang)
				do update set file_url = excluded.file_url, status = 'done',
				              source = excluded.source, format = excluded.format`,
				epID, tr.Lang, tr.Format, tr.URL); err != nil {
				return err
			}
		}
		if len(res.Streams) > 0 {
			// video_url utama mengikuti aliran pertama yang sehat.
			if _, err := pool.Exec(ctx, `
				update episodes set video_url = $2 where id = $1::uuid`,
				epID, res.Streams[0].URL); err != nil {
				return err
			}
		}
		*resolved++
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
