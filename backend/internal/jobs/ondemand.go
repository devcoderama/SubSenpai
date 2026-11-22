package jobs

// Jalur on-demand (permintaan Rams 2026-11-22): fungsi per-judul yang
// dipakai sisi API agar user yang membuka judul yang belum dimuat
// worker tidak menunggu siklus worker — backend menarik langsung dari
// sumber saat itu juga, menyimpan ke DB, dan mengembalikan hasil
// lengkap. Worker tetap memakai fungsi inti yang sama (EnrichTitles
// memakai applySeriesMeta; SyncSources dan SyncOneTitle sama-sama
// memakai syncTitle), jadi tidak ada duplikasi logika resolver.

import (
	"context"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/sources"
)

// titleMapping adalah satu pemetaan judul → sumber server aktif.
type titleMapping struct {
	url string
	src sources.Source
}

func loadTitleMappings(ctx context.Context, pool *pgxpool.Pool, titleID string) ([]titleMapping, error) {
	rows, err := pool.Query(ctx, `
		select ts.source_url,
		       cs.id, cs.name, cs.content_type, cs.lang, coalesce(cs.base_url,''),
		       cs.resolver, cs.fetch_mode, cs.subtitle_kind, cs.priority
		from title_sources ts
		join content_sources cs on cs.id = ts.source_id
		where ts.title_id = $1::uuid and cs.is_active and cs.fetch_mode = 'server'
		order by cs.priority, cs.name`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []titleMapping{}
	for rows.Next() {
		var m titleMapping
		if err := rows.Scan(&m.url, &m.src.ID, &m.src.Name, &m.src.ContentType,
			&m.src.Lang, &m.src.BaseURL, &m.src.Resolver, &m.src.FetchMode,
			&m.src.SubtitleKind, &m.src.Priority); err == nil {
			out = append(out, m)
		}
	}
	return out, rows.Err()
}

// applySeriesMeta menulis metadata seri ke judul, hanya mengisi kolom
// yang masih kosong (coalesce) — dipakai EnrichTitles (worker) dan
// EnrichOneTitle (on-demand) agar perilakunya identik.
func applySeriesMeta(ctx context.Context, pool *pgxpool.Pool, titleID string, meta *sources.SeriesMeta) error {
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
	_, err := pool.Exec(ctx, `
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
		titleID, nullIfEmpty(meta.Synopsis), nullIfEmpty(meta.TitleAlt),
		genres, year, season, nullIfEmpty(meta.Network),
		nullIfEmpty(meta.Studio), dur, nullIfEmpty(meta.Country))
	return err
}

// EnrichOneTitle memperkaya metadata SATU judul dari sumber server
// terpetakan pertamanya yang resolvernya mendukung SeriesInfoer.
// Mengembalikan true bila metadata berhasil ditulis. Kegagalan satu
// sumber dicoba lanjut ke sumber berikutnya; error terakhir
// dikembalikan bila tidak ada sumber yang berhasil.
func EnrichOneTitle(ctx context.Context, pool *pgxpool.Pool, titleID string) (bool, error) {
	mappings, err := loadTitleMappings(ctx, pool, titleID)
	if err != nil {
		return false, err
	}
	var lastErr error
	for _, m := range mappings {
		resolver, ok := sources.ByKey(m.src.Resolver)
		if !ok {
			continue
		}
		infoer, ok := resolver.(sources.SeriesInfoer)
		if !ok {
			continue
		}
		meta, err := infoer.SeriesInfo(ctx, m.src, m.url)
		if err != nil {
			log.Printf("enrich on-demand %s dari %s: %v", titleID, m.src.Name, err)
			lastErr = err
			continue
		}
		if meta == nil {
			continue
		}
		if meta.Synopsis == "" && meta.Studio == "" && len(meta.Genres) == 0 {
			continue // halaman seri tidak memberi apa-apa — coba sumber lain
		}
		if err := applySeriesMeta(ctx, pool, titleID, meta); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, lastErr
}

// SyncOneTitle menyinkronkan episode SATU judul dari semua sumber
// server aktif untuk jenisnya, memakai inti syncTitle yang sama
// dengan SyncSources worker (termasuk batas resolusi aliran per
// judul yang sama, maxResolvePerRun). Error per sumber dicatat dan
// dilanjutkan ke sumber berikutnya; error terakhir dikembalikan
// hanya bila tidak ada aliran yang berhasil diselesaikan sama
// sekali dan setidaknya satu sumber gagal.
func SyncOneTitle(ctx context.Context, pool *pgxpool.Pool, titleID, titleName, titleType string) error {
	srcs, err := loadSources(ctx, pool, titleType)
	if err != nil {
		return err
	}
	resolved := 0
	var lastErr error
	for _, src := range srcs {
		resolver, ok := sources.ByKey(src.Resolver)
		if !ok {
			continue
		}
		if err := syncTitle(ctx, pool, resolver, src, titleID, titleName, &resolved); err != nil {
			log.Printf("sinkron on-demand %s dari %s: %v", titleName, src.Name, err)
			lastErr = err
		}
		if resolved >= maxResolvePerRun {
			break
		}
	}
	if resolved == 0 {
		return lastErr
	}
	return nil
}

var slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify membuat slug katalog dari judul bebas: huruf kecil, selain
// a-z0-9 menjadi satu tanda hubung, tanpa hubung di tepi.
func slugify(s string) string {
	s = slugNonAlnumRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	return strings.Trim(s, "-")
}

// slugFromCandidate menurunkan slug katalog untuk satu kandidat hasil
// pencarian: segmen terakhir URL kandidat (pola situs sumber memang
// memakai slug seri sebagai segmen itu — mis. .../series/<slug>/ dan
// /watch/<slug>-<id>); bila URL tidak memberi segmen yang layak,
// slug dibuat dari judulnya.
func slugFromCandidate(c sources.Candidate) string {
	if c.URL != "" {
		if u, err := url.Parse(c.URL); err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) > 0 {
				if seg := slugify(parts[len(parts)-1]); seg != "" {
					return seg
				}
			}
		}
	}
	return slugify(c.Title)
}

// LiveSearchStore menjalankan pencarian live ke semua sumber server
// aktif untuk satu jenis konten dan menyimpan kandidat yang belum
// ada ke katalog: insert titles (slug unik, on conflict do nothing)
// + pemetaan title_sources, meniru pola DiscoverTitles/SyncSources.
// Mengembalikan jumlah kandidat yang tersimpan/terpetakan. Kegagalan
// satu sumber dicatat dan tidak menghentikan sumber lain.
func LiveSearchStore(ctx context.Context, pool *pgxpool.Pool, contentType, query string) (int, error) {
	srcs, err := loadSources(ctx, pool, contentType)
	if err != nil {
		return 0, err
	}
	stored := 0
	for _, src := range srcs {
		resolver, ok := sources.ByKey(src.Resolver)
		if !ok {
			continue
		}
		cands, err := resolver.Search(ctx, src, query)
		if err != nil {
			log.Printf("pencarian live %s di %s: %v", query, src.Name, err)
			continue
		}
		for _, c := range cands {
			if c.Title == "" || c.URL == "" {
				continue
			}
			slug := slugFromCandidate(c)
			if slug == "" {
				continue
			}
			var id, typ string
			err := pool.QueryRow(ctx, `
				insert into titles (slug, type, title, genres, status)
				values ($1, $2, $3, $4, 'ongoing')
				on conflict (slug) do nothing
				returning id, type`,
				slug, contentType, c.Title, []string{}).Scan(&id, &typ)
			if err != nil {
				// Sudah ada di katalog dengan slug sama: petakan saja
				// bila jenisnya sama.
				if err := pool.QueryRow(ctx, `
					select id, type from titles where slug = $1`, slug).Scan(&id, &typ); err != nil {
					continue
				}
				if typ != contentType {
					continue
				}
			}
			if _, err := pool.Exec(ctx, `
				insert into title_sources (title_id, source_id, source_url, source_ref)
				values ($1::uuid, $2::uuid, $3, $4)
				on conflict (title_id, source_id) do nothing`,
				id, src.ID, c.URL, nullIfEmpty(c.Ref)); err != nil {
				log.Printf("petakan hasil live %s: %v", c.Title, err)
				continue
			}
			stored++
		}
	}
	return stored, nil
}
