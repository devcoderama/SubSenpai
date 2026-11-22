package jobs

import (
	"context"
	"log"
	"net/url"

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
	return loadSourcesMode(ctx, pool, contentType, true)
}

// loadDiscoverySources memuat sumber untuk penemuan katalog: semua
// sumber aktif ber-URL, termasuk sumber mode perangkat (komik F39)
// yang judulnya tetap didaftarkan ke katalog server.
func loadDiscoverySources(ctx context.Context, pool *pgxpool.Pool, contentType string) ([]sources.Source, error) {
	return loadSourcesMode(ctx, pool, contentType, false)
}

func loadSourcesMode(ctx context.Context, pool *pgxpool.Pool, contentType string, serverOnly bool) ([]sources.Source, error) {
	modeFilter := ""
	if serverOnly {
		modeFilter = " and fetch_mode = 'server'"
	}
	rows, err := pool.Query(ctx, `
		select id, name, content_type, lang, coalesce(base_url,''), resolver,
		       fetch_mode, subtitle_kind, priority
		from content_sources
		where is_active and base_url is not null
		  and content_type = $1`+modeFilter+`
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

// maxPosterPerRun membatasi pengambilan poster og:image (satu
// permintaan HTTP per judul) per putaran penemuan agar worker tidak
// membanjiri situs sumber. Poster dari umpan/peta sumber tidak dihitung
// (tidak menambah permintaan per judul).
const maxPosterPerRun = 40

// DiscoverTitles mengisi katalog dari indeks judul situs sumber (F40):
// untuk tiap sumber aktif yang resolvernya mendukung penemuan, judul
// yang belum ada dimasukkan ke titles. Sumber server: judul langsung
// dipetakan di title_sources (episode & aliran menyusul lewat
// SyncSources). Sumber perangkat (komik): judul masuk sebagai judul
// mode perangkat dengan template URL sumbernya (F39). Poster berurutan
// prioritasnya: dari umpan penemuan → peta poster sumber → og:image
// halaman seri (dibatasi per putaran), dan dilengkapi bertahap untuk
// judul lama yang belum berposter.
// PurgePosterless menghapus judul tanpa poster yang sudah melewati
// masa tenggang, agar putaran penemuan berikutnya memasukkannya
// kembali dari sumber dengan parser poster terkini (permintaan Rams
// 2026-11-22: data tanpa URL poster dihapus dulu supaya disinkronkan
// ulang dan mendapat URL poster). Pengaman agar tidak menggerus
// katalog: (1) hanya judul yang BISA ditemukan ulang — punya
// pemetaan title_sources atau mode perangkat; (2) masa tenggang 24
// jam sejak dibuat, memberi kesempatan pelengkapan poster biasa;
// (3) maksimal 100 judul per putaran. Episode/aliran/wishlist judul
// terhapus ikut hilang (cascade) dan terisi lagi oleh SyncSources —
// urutannya yang berepisode paling sedikit dulu, jadi judul hasil
// purge justru diprioritaskan.
func PurgePosterless(ctx context.Context, pool *pgxpool.Pool) error {
	tag, err := pool.Exec(ctx, `
		delete from titles t
		where t.poster_url is null
		  and t.created_at < now() - interval '24 hours'
		  and (t.fetch_mode = 'device' or exists (
		      select 1 from title_sources ts where ts.title_id = t.id))
		  and t.id in (
		      select id from titles
		      where poster_url is null
		        and created_at < now() - interval '24 hours'
		      order by created_at
		      limit 100)`)
	if err != nil {
		return err
	}
	if n := tag.RowsAffected(); n > 0 {
		log.Printf("purge poster: %d judul tanpa poster dihapus untuk ditemukan ulang", n)
	}
	return nil
}

// EnrichTitles memperkaya metadata judul yang masih kosong
// (sinopsis/judul alternatif/genre/tahun/season/network/studio/
// durasi/negara) dari halaman seri sumbernya, agar halaman detail
// aplikasi tampil lengkap ala Netflix. Hanya kolom yang masih
// kosong yang diisi (coalesce), maksimal 30 judul per putaran;
// judul yang sudah bersinopsis keluar dari antrean dengan
// sendirinya.
func EnrichTitles(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		select t.id, ts.source_url,
		       cs.id, cs.name, cs.content_type, cs.lang, coalesce(cs.base_url,''),
		       cs.resolver, cs.fetch_mode, cs.subtitle_kind, cs.priority
		from titles t
		join title_sources ts on ts.title_id = t.id
		join content_sources cs on cs.id = ts.source_id
		where t.synopsis is null and cs.is_active
		order by t.created_at
		limit 90`)
	if err != nil {
		return err
	}
	type job struct {
		titleID, url string
		src          sources.Source
	}
	jobs := []job{}
	seen := map[string]bool{}
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.titleID, &j.url, &j.src.ID, &j.src.Name,
			&j.src.ContentType, &j.src.Lang, &j.src.BaseURL, &j.src.Resolver,
			&j.src.FetchMode, &j.src.SubtitleKind, &j.src.Priority); err != nil {
			continue
		}
		if seen[j.titleID] {
			continue
		}
		seen[j.titleID] = true
		jobs = append(jobs, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	done := 0
	for _, j := range jobs {
		if done >= 30 {
			break
		}
		resolver, ok := sources.ByKey(j.src.Resolver)
		if !ok {
			continue
		}
		infoer, ok := resolver.(sources.SeriesInfoer)
		if !ok {
			continue
		}
		meta, err := infoer.SeriesInfo(ctx, j.src, j.url)
		if err != nil {
			// Dicatat (tidak ditelan diam-diam) agar kegagalan fetch
			// per sumber terlihat di log worker.
			log.Printf("enrich %s: %v", j.titleID, err)
			continue
		}
		if meta == nil {
			continue
		}
		if meta.Synopsis == "" && meta.Studio == "" && len(meta.Genres) == 0 {
			continue
		}
		genres := meta.Genres
		if genres == nil {
			genres = []string{}
		}
		var year, dur any
		if meta.Year > 0 {
			year = meta.Year
		}
		if meta.DurationMin > 0 {
			dur = meta.DurationMin
		}
		var season any
		if meta.Season != "" {
			season = meta.Season
		}
		if _, err := pool.Exec(ctx, `
			update titles set
				synopsis = coalesce(synopsis, $2),
				title_alt = coalesce(title_alt, $3),
				genres = case when coalesce(cardinality(genres), 0) = 0 then $4 else genres end,
				year = coalesce(year, $5),
				season = coalesce(season, $6),
				network = coalesce(network, $7),
				studio = coalesce(studio, $8),
				duration_min = coalesce(duration_min, $9),
				country = coalesce(country, $10),
				updated_at = now()
			where id = $1::uuid`,
			j.titleID, nullIfEmpty(meta.Synopsis), nullIfEmpty(meta.TitleAlt),
			genres, year, season, nullIfEmpty(meta.Network),
			nullIfEmpty(meta.Studio), dur, nullIfEmpty(meta.Country)); err != nil {
			log.Printf("enrich %s: %v", j.titleID, err)
			continue
		}
		done++
	}
	if done > 0 {
		log.Printf("enrich: %d judul diperkaya metadata serinya", done)
	}
	return nil
}

func DiscoverTitles(ctx context.Context, pool *pgxpool.Pool) error {
	posters := 0
	seen := map[string]bool{} // resolver+base yang sudah ditemukan putaran ini
	for _, ct := range []string{"anime", "donghua", "film", "manga", "manhua", "manhwa"} {
		srcs, err := loadDiscoverySources(ctx, pool, ct)
		if err != nil {
			return err
		}
		for _, src := range srcs {
			key := src.Resolver + "|" + src.BaseURL
			if seen[key] {
				continue // satu umpan bisa melayani beberapa jenis (komik)
			}
			seen[key] = true
			resolver, ok := sources.ByKey(src.Resolver)
			if !ok {
				continue
			}
			disc, ok := resolver.(sources.Discoverer)
			if !ok {
				continue
			}
			found, err := disc.Discover(ctx, src, 600)
			if err != nil {
				log.Printf("penemuan dari %s: %v", src.Name, err)
				continue
			}
			// Peta poster massal sumber ini (bila disediakan resolver).
			var pmap map[string]string
			if pp, ok := resolver.(sources.PosterProvider); ok {
				if m, err := pp.Posters(ctx, src); err == nil {
					pmap = m
				} else {
					log.Printf("peta poster %s: %v", src.Name, err)
				}
			}
			device := src.FetchMode == "device"
			baru := 0
			for _, d := range found {
				typ := d.Type
				if typ == "" {
					typ = ct
				}
				poster := d.Poster
				if poster == "" && pmap != nil {
					poster = pmap[sources.NormalizeTitleKey(d.Title)]
				}
				status := "ongoing"
				if d.Completed {
					status = "completed"
				}
				genres := d.Genres
				if genres == nil {
					genres = []string{}
				}
				var year any
				if d.Year > 0 {
					year = d.Year
				}
				var id string
				if device {
					err = pool.QueryRow(ctx, `
						insert into titles (slug, type, title, poster_url, year, genres, status,
						                    fetch_mode, source_domain, source_url_template)
						values ($1, $2, $3, $4, $5, $6, $7, 'device', $8, $9)
						on conflict (slug) do nothing
						returning id`,
						d.Slug, typ, d.Title, nullIfEmpty(poster), year, genres, status,
						hostOf(src.BaseURL), d.URL).Scan(&id)
				} else {
					err = pool.QueryRow(ctx, `
						insert into titles (slug, type, title, poster_url, year, genres, status)
						values ($1, $2, $3, $4, $5, $6, $7)
						on conflict (slug) do nothing
						returning id`,
						d.Slug, typ, d.Title, nullIfEmpty(poster), year, genres, status).Scan(&id)
				}
				if err != nil {
					// Sudah ada di katalog: sembuhkan kolom yang masih
					// kosong (poster/tahun/genre) dari hasil penemuan
					// terbaru — mis. poster anime yang dulu gagal
					// terbaca kini terbaca dari atribut src kartu.
					if poster != "" || year != nil || len(genres) > 0 {
						_, _ = pool.Exec(ctx, `
							update titles set
								poster_url = coalesce(poster_url, $2),
								year = coalesce(year, $3),
								genres = case when coalesce(cardinality(genres), 0) = 0 then $4 else genres end
							where slug = $1`,
							d.Slug, nullIfEmpty(poster), year, genres)
					}
					continue
				}
				baru++
				if device {
					continue // judul perangkat tidak memakai title_sources
				}
				if _, err := pool.Exec(ctx, `
					insert into title_sources (title_id, source_id, source_url)
					values ($1::uuid, $2::uuid, $3)
					on conflict (title_id, source_id) do nothing`,
					id, src.ID, d.URL); err != nil {
					log.Printf("petakan judul %s: %v", d.Title, err)
					continue
				}
				if poster == "" && posters < maxPosterPerRun {
					if p := sources.FetchPoster(ctx, d.URL); p != "" {
						_, _ = pool.Exec(ctx, `
							update titles set poster_url = $2, updated_at = now()
							where id = $1::uuid`, id, p)
						posters++
					}
				}
			}
			if !device {
				backfillPosters(ctx, pool, src, pmap, &posters)
			}
			log.Printf("penemuan %s: %d judul ditemukan, %d baru masuk katalog", src.Name, len(found), baru)
		}
	}
	return nil
}

// backfillPosters melengkapi poster judul terpetakan ke sumber ini yang
// belum berposter: dari peta poster sumber dulu (tanpa HTTP per judul),
// lalu og:image halaman seri (dibatasi anggaran putaran).
func backfillPosters(ctx context.Context, pool *pgxpool.Pool, src sources.Source, pmap map[string]string, posters *int) {
	rows, err := pool.Query(ctx, `
		select t.id, t.title, ts.source_url from titles t
		join title_sources ts
		  on ts.title_id = t.id and ts.source_id = $1::uuid
		where t.poster_url is null
		limit 200`, src.ID)
	if err != nil {
		return
	}
	type pend struct{ id, title, url string }
	list := []pend{}
	for rows.Next() {
		var p pend
		if err := rows.Scan(&p.id, &p.title, &p.url); err == nil {
			list = append(list, p)
		}
	}
	rows.Close()
	for _, p := range list {
		poster := ""
		if pmap != nil {
			poster = pmap[sources.NormalizeTitleKey(p.title)]
		}
		if poster == "" && *posters < maxPosterPerRun {
			poster = sources.FetchPoster(ctx, p.url)
			if poster != "" {
				*posters += 1
			}
		}
		if poster != "" {
			_, _ = pool.Exec(ctx, `
				update titles set poster_url = $2, updated_at = now()
				where id = $1::uuid`, p.id, poster)
		}
	}
}

// hostOf mengambil host dari URL basis sumber (untuk source_domain
// judul mode perangkat).
func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
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
		order by (select count(*) from episodes e where e.title_id = titles.id) asc,
		         created_at
		limit 50`)
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
