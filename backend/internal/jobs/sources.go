package jobs

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/sources"
)

// CheckSources memeriksa kesehatan semua sumber aktif di registri (F40/O9)
// dan mencatat hasilnya ke content_sources.
func CheckSources(ctx context.Context, pool *pgxpool.Pool) error {
	rows, err := pool.Query(ctx, `
		select id, resolver, base_url from content_sources where is_active`)
	if err != nil {
		return err
	}
	type item struct{ id, base string }
	items := []item{}
	for rows.Next() {
		var it item
		var resolver string
		var base *string
		if err := rows.Scan(&it.id, &resolver, &base); err == nil && base != nil {
			it.base = sources.EffectiveBaseURL(resolver, *base)
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
		  -- Sumber yang kesehatannya 'down' dilewati sampai pulih
		  -- (diagnosis 2026-11-26: HiAnime down tetap dicoba tiap judul,
		  -- tiap percobaan membakar timeout 30 dtk; 'unknown' tetap dicoba).
		  and coalesce(health, 'unknown') <> 'down'
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
			sources.ApplyBaseOverride(&s)
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

// maxDevicePosterPerRun adalah anggaran PERCOBAAN HTTP og:image
// tersendiri untuk backfill poster judul mode perangkat (mis.
// Kuronime) per putaran. Dihitung per percobaan, bukan per keberhasilan:
// Fetch bisa menggantung sampai timeout 30 dtk, jadi 40 percobaan
// adalah batas atas yang aman agar satu putaran worker tidak
// menggantung. Kecocokan dari peta poster tidak dihitung (tanpa HTTP).
const maxDevicePosterPerRun = 40

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

// enrichPerRun membatasi judul yang berhasil diperkaya per putaran
// worker (30 -> 60 pada paket percepatan 2026-11-24, kini paralel).
// enrichPerRun: maksimal judul yang diperkaya per jenis per putaran
// (30 × 3 jenis = 90 total; dulu 60 untuk semua jenis sebelum
// jalur dipecah 2026-11-26).
const enrichPerRun = 30

// fetchParallel membatasi fetch HTTP bersamaan di fase enrich dan
// resolve episode (naik 5 → 8, paket percepatan 2026-11-26): tiap
// fetch menunggu jaringan situs sumber, jadi menaikkan jumlah
// bersamaan menaikkan throughput sampai situs sumber mulai
// membatasi — 429/403 di log worker = tanda turunkan lagi.
const fetchParallel = 8

// EnrichTitles memperkaya metadata judul yang masih kosong
// (sinopsis/judul alternatif/genre/tahun/season/network/studio/
// durasi/negara) dari halaman seri sumbernya, agar halaman detail
// aplikasi tampil lengkap ala Netflix. Hanya kolom yang masih
// kosong yang diisi (coalesce). Ketiga jenis berjalan sebagai 3
// jalur BERSAMAAN (situs sumbernya berbeda per jenis); judul yang
// sudah bersinopsis keluar dari antrean dengan sendirinya. Judul
// yang sinopsisnya boilerplate iklan unduhan (data lama dari
// halaman salah alamat) ikut dipilih agar sembuh: applySeriesMeta
// menimpa sinopsis boilerplate dengan yang sah.
func EnrichTitles(ctx context.Context, pool *pgxpool.Pool) error {
	type job struct {
		titleID, name, url string
		src                sources.Source
	}
	loadJobs := func(typ string) ([]job, error) {
		rows, err := pool.Query(ctx, `
			select t.id, t.title, ts.source_url,
			       cs.id, cs.name, cs.content_type, cs.lang, coalesce(cs.base_url,''),
			       cs.resolver, cs.fetch_mode, cs.subtitle_kind, cs.priority
			from titles t
			join title_sources ts on ts.title_id = t.id
			join content_sources cs on cs.id = ts.source_id
			where t.type = $2 and (t.synopsis is null or lower(t.synopsis) like any ($1)) and cs.is_active
			  and coalesce(cs.health, 'unknown') <> 'down'
			  -- Cooldown kegagalan enrich (migrasi 0024, diagnosis
			  -- 2026-11-27): judul yang SeriesInfo-nya gagal <24 jam
			  -- lalu dilewati, supaya 60 judul tertua yang selalu
			  -- gagal tidak memonopoli antrean tiap siklus.
			  and not (t.last_enrich_at > now() - interval '24 hours' and t.last_enrich_error is not null)
			order by case
			  -- Judul yang episodenya sudah lengkap didahulukan: satu
			  -- sinopsis lagi mereka jadi komplit (Q1 2026-11-27: 81
			  -- donghua dalam keadaan ini).
			  when exists (select 1 from episodes e where e.title_id = t.id)
			       and not exists (
			         select 1 from episodes e
			         where e.title_id = t.id
			           and not exists (select 1 from episode_sources es
			                           where es.episode_id = e.id))
			  then 0
			  when not exists (select 1 from episodes e where e.title_id = t.id) then 1
			  else 2 end,
			  t.created_at
			limit 60`, boilerplatePatterns(), typ)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		jobs := []job{}
		seen := map[string]bool{}
		for rows.Next() {
			var j job
			if err := rows.Scan(&j.titleID, &j.name, &j.url, &j.src.ID, &j.src.Name,
				&j.src.ContentType, &j.src.Lang, &j.src.BaseURL, &j.src.Resolver,
				&j.src.FetchMode, &j.src.SubtitleKind, &j.src.Priority); err != nil {
				continue
			}
			if seen[j.titleID] {
				continue
			}
			seen[j.titleID] = true
			sources.ApplyBaseOverride(&j.src)
			jobs = append(jobs, j)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return jobs, nil
	}
	// recordEnrich mencatat tiap percobaan enrich (migrasi 0024):
	// gagal -> error dicatat (dilewati 24 jam); berhasil -> penanda
	// gagal dihapus. Tanpa ini, judul yang SeriesInfo-nya selalu
	// gagal diputar ulang tiap siklus tanpa henti.
	recordEnrich := func(titleID string, err error) {
		if err != nil {
			_, _ = pool.Exec(ctx, `update titles set last_enrich_at = now(), last_enrich_error = $2 where id = $1::uuid`, titleID, err.Error())
		} else {
			_, _ = pool.Exec(ctx, `update titles set last_enrich_at = now(), last_enrich_error = null where id = $1::uuid`, titleID)
		}
	}
	enrichOne := func(j job) bool {
		resolver, ok := sources.ByKey(j.src.Resolver)
		if !ok {
			log.Printf("enrich %s: resolver %q sumber %s tidak ditemukan, dilewati", j.titleID, j.src.Resolver, j.src.Name)
			return false
		}
		infoer, ok := resolver.(sources.SeriesInfoer)
		if !ok {
			log.Printf("enrich %s: resolver %q sumber %s tidak mendukung SeriesInfoer, dilewati", j.titleID, j.src.Resolver, j.src.Name)
			return false
		}
		meta, err := infoer.SeriesInfo(ctx, j.src, j.url)
		if err != nil {
			// Dicatat (tidak ditelan diam-diam) agar kegagalan fetch
			// per sumber terlihat di log worker.
			log.Printf("enrich %s: %v", j.titleID, err)
			recordEnrich(j.titleID, err)
			return false
		}
		if meta == nil {
			log.Printf("enrich %s dari %s (%s): SeriesInfo mengembalikan meta nil", j.titleID, j.src.Name, j.url)
			recordEnrich(j.titleID, fmt.Errorf("SeriesInfo mengembalikan meta nil"))
			return false
		}
		if meta.Synopsis == "" && meta.Studio == "" && len(meta.Genres) == 0 {
			log.Printf("enrich %s dari %s (%s): meta ditolak penjaga kosong: %s", j.titleID, j.src.Name, j.url, metaSummary(meta))
			recordEnrich(j.titleID, fmt.Errorf("meta ditolak penjaga kosong"))
			return false
		}
		if err := applySeriesMeta(ctx, pool, j.titleID, meta); err != nil {
			log.Printf("enrich %s: %v", j.titleID, err)
			recordEnrich(j.titleID, err)
			return false
		}
		recordEnrich(j.titleID, nil)
		return true
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	totalDone := 0
	var firstErr error
	for _, typ := range []string{"anime", "donghua", "film", "series"} {
		wg.Add(1)
		go func(typ string) {
			defer wg.Done()
			jobs, err := loadJobs(typ)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			// Kerjakan paralel, maksimal fetchParallel fetch
			// bersamaan: cara lama satu per satu membuat fase ini
			// berjalan belasan menit sendiri karena tiap judul
			// menunggu 1 fetch HTTP.
			sem := make(chan struct{}, fetchParallel)
			var jwg sync.WaitGroup
			var jmu sync.Mutex
			done := 0
			attempt := 0
			for _, j := range jobs {
				jmu.Lock()
				penuh := done >= enrichPerRun
				jmu.Unlock()
				if penuh {
					break
				}
				attempt++
				ReportLaneProgress(ctx, pool, "Melengkapi metadata", typ, attempt, min(len(jobs), enrichPerRun), j.name)
				jwg.Add(1)
				go func(j job) {
					defer jwg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					if enrichOne(j) {
						jmu.Lock()
						done++
						jmu.Unlock()
					}
				}(j)
			}
			jwg.Wait()
			mu.Lock()
			totalDone += done
			mu.Unlock()
		}(typ)
	}
	wg.Wait()
	if totalDone > 0 {
		log.Printf("enrich: %d judul diperkaya metadata serinya", totalDone)
	}
	return firstErr
}

func DiscoverTitles(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := discoverTitles(ctx, pool, "")
	return err
}

// DiscoverType menjalankan penemuan katalog untuk SATU jenis konten
// saja — titik masuk jalur on-demand API (katalog per jenis masih
// kosong → diisi saat itu juga, tanpa menunggu siklus worker). Inti
// dan perilakunya sama persis dengan bagian per-jenis DiscoverTitles
// (sumber dimuat lewat loadDiscoverySources yang sama, termasuk
// override base URL env). Mengembalikan jumlah judul baru yang
// masuk katalog.
func DiscoverType(ctx context.Context, pool *pgxpool.Pool, contentType string) (int, error) {
	return discoverTitles(ctx, pool, contentType)
}

// discoverTitles adalah inti penemuan katalog: onlyType kosong =
// semua jenis (worker), terisi = hanya jenis itu (on-demand API).
// Mengembalikan jumlah judul baru yang masuk katalog.
func discoverTitles(ctx context.Context, pool *pgxpool.Pool, onlyType string) (int, error) {
	total := 0
	posters := 0
	devicePosters := 0        // percobaan HTTP backfill poster judul perangkat
	seen := map[string]bool{} // resolver+base yang sudah ditemukan putaran ini
	for _, ct := range []string{"anime", "donghua", "film", "series", "manga", "manhua", "manhwa"} {
		if onlyType != "" && ct != onlyType {
			continue
		}
		srcs, err := loadDiscoverySources(ctx, pool, ct)
		if err != nil {
			return 0, err
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
					// Judul perangkat tidak memakai title_sources, tapi
					// tetap dapat percobaan poster og:image halaman
					// serinya (poster dari peta sumber sudah dicoba
					// sebelum insert di atas). Tanpa ini judul anime
					// Kuronime tidak pernah berposter: penemuan WP
					// hanya memberi nama+slug+URL.
					if poster == "" && posters < maxPosterPerRun {
						if p := sources.FetchPoster(ctx, d.URL); p != "" {
							_, _ = pool.Exec(ctx, `
								update titles set poster_url = $2, updated_at = now()
								where id = $1::uuid`, id, p)
							posters++
						}
					}
					continue
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
			} else if ct == "anime" || ct == "donghua" {
				// Judul perangkat video (Kuronime) tidak punya
				// title_sources, jadi backfill server di atas tidak
				// pernah menyentuhnya — lengkapi lewat template URL
				// sumbernya. Jenis komik tidak diubah perilakunya.
				backfillDevicePosters(ctx, pool, src, pmap, &devicePosters)
			}
			total += baru
			log.Printf("penemuan %s: %d judul ditemukan, %d baru masuk katalog", src.Name, len(found), baru)
		}
	}
	return total, nil
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

// backfillDevicePosters melengkapi poster judul mode perangkat lama
// yang belum berposter (mis. 600 judul anime Kuronime hasil penemuan
// sebelum poster perangkat didukung). Judul perangkat tidak punya
// baris title_sources, jadi kaitannya ke sumber adalah source_domain
// (= host base sumber) dan URL halamannya ada di source_url_template.
// Urutan percobaan: peta poster sumber dulu (tanpa HTTP, berdasarkan
// nama ternormalisasi), lalu og:image halaman seri — dibatasi
// anggaran percobaan HTTP maxDevicePosterPerRun per putaran agar
// putaran worker tidak menggantung pada fetch yang timeout.
func backfillDevicePosters(ctx context.Context, pool *pgxpool.Pool, src sources.Source, pmap map[string]string, attempts *int) {
	rows, err := pool.Query(ctx, `
		select id, title, source_url_template from titles
		where fetch_mode = 'device' and poster_url is null
		  and source_url_template is not null and source_url_template <> ''
		  and source_domain = $1
		order by created_at
		limit 200`, hostOf(src.BaseURL))
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
	filled := 0
	for _, p := range list {
		poster := ""
		if pmap != nil {
			poster = pmap[sources.NormalizeTitleKey(p.title)]
		}
		if poster == "" && *attempts < maxDevicePosterPerRun {
			*attempts++
			poster = sources.FetchPoster(ctx, p.url)
		}
		if poster != "" {
			_, _ = pool.Exec(ctx, `
				update titles set poster_url = $2, updated_at = now()
				where id = $1::uuid`, p.id, poster)
			filled++
		}
	}
	if filled > 0 {
		log.Printf("backfill poster perangkat %s: %d judul dilengkapi", src.Name, filled)
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
// agar satu putaran worker tidak berjalan tanpa batas. 20 -> 100 pada
// paket percepatan worker 2026-11-24 (permintaan Rams: antrean 592
// judul belum komplit terlalu lambat bersihnya pada batas lama).
const maxResolvePerRun = 100

// syncTitlesPerType / maxResolvePerType: jatah per jenis di tiap
// siklus sinkronisasi (paket percepatan 2026-11-26). Sebelumnya
// 100 judul + 100 resolve dipakai bersama semua jenis, dan
// pemilihan "paling sedikit episode dulu" membuat judul pendek
// (film/anime pendek) menghabiskan anggaran di awal siklus —
// Donghua yang episodenya puluhan tidak pernah kebagian, komplitnya
// mentok di 403 sementara antrean belum-komplit menumpuk. Dengan
// jatah per jenis (50 judul + 50 resolve tiap jenis — naik dari 34
// saat jalur dipecah 2026-11-26), tiap jenis dijamin mendapat
// giliran tiap siklus.
const syncTitlesPerType = 50
const maxResolvePerType = 50

// SyncSources menjalankan resolver untuk judul-judul katalog: memetakan
// judul ke halaman sumbernya (pencarian pertama yang cocok), mengambil
// syncTitleItem adalah satu judul yang dipilih untuk disinkron.
type syncTitleItem struct{ id, typ, name string }

// selectSyncTitles memilih judul yang akan disinkron pada satu siklus,
// dibagi 3 kelompok dengan slot terjamin (diagnosis 2026-11-26 19:11
// WIB: 8 judul donghua eps=0 teratas punya mapping + metadata lengkap
// tapi tidak pernah dicoba -- antrean tunggal "belum bersumber dulu"
// membuat mereka kelaparan di belakang, dan jatah resolve yang habis
// membuat loop melewati mereka total):
//
//	G1 belum bersumber (60% slot): episode ada tapi belum semua
//	   bersumber -- paling mungkin jadi komplit;
//	G2 tanpa episode (30% slot): belum punya episode sama sekali;
//	G3 ongoing lengkap (sisa slot): semua episode sudah bersumber --
//	   diperiksa bergiliran (last_sync_at terlama dulu) untuk mendeteksi
//	   episode baru.
//
// Judul yang gagal sync kurang dari 6 jam lalu dilewati di semua
// kelompok -- cooldown kegagalan (migrasi 0023).
func selectSyncTitles(ctx context.Context, pool *pgxpool.Pool, typ string, limit int) ([]syncTitleItem, error) {
	// Cooldown kegagalan sync (migrasi 0023).
	cooldown := `and not (last_sync_at > now() - interval '6 hours' and last_sync_error is not null)`
	run := func(where, order string, lim int) ([]syncTitleItem, error) {
		rows, err := pool.Query(ctx, `
			select id, type, title from titles
			where type = $1 `+where+` `+cooldown+`
			order by `+order+`
			limit $2`, typ, lim)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []syncTitleItem{}
		for rows.Next() {
			var t syncTitleItem
			if err := rows.Scan(&t.id, &t.typ, &t.name); err == nil {
				out = append(out, t)
			}
		}
		return out, rows.Err()
	}
	g1, err := run(`
		  and exists (
		      select 1 from episodes e
		      where e.title_id = titles.id
		        and not exists (select 1 from episode_sources es
		                        where es.episode_id = e.id))`,
		`(select count(*) from episodes e where e.title_id = titles.id) asc,
		         created_at`,
		limit*3/5)
	if err != nil {
		return nil, err
	}
	g2, err := run(`
		  and not exists (select 1 from episodes e where e.title_id = titles.id)`,
		`created_at`,
		limit*3/10)
	if err != nil {
		return nil, err
	}
	g3, err := run(`
		  and titles.status = 'ongoing'
		  and exists (select 1 from episodes e where e.title_id = titles.id)
		  and not exists (
		      select 1 from episodes e
		      where e.title_id = titles.id
		        and not exists (select 1 from episode_sources es
		                        where es.episode_id = e.id))`,
		`last_sync_at asc nulls first, created_at`,
		limit-len(g1)-len(g2))
	if err != nil {
		return nil, err
	}
	return append(append(g1, g2...), g3...), nil
}

// daftar episode, dan menyelesaikan aliran + trek subtitle episode yang
// belum punya sumber putar. Keempat jenis (anime/donghua/film/series)
// berjalan sebagai jalur BERSAMAAN — situs sumbernya berbeda per jenis,
// jadi tidak saling berebut; kegagalan satu jalur (mis. satu situs
// down) hanya menghentikan jalur itu, jalur lain tetap jalan.
// Kegagalan per judul/sumber dicatat, tidak menghentikan putaran.
func SyncSources(ctx context.Context, pool *pgxpool.Pool) error {
	types := []string{"anime", "donghua", "film", "series"}
	lists := make([][]syncTitleItem, len(types))
	for i, typ := range types {
		list, err := selectSyncTitles(ctx, pool, typ, syncTitlesPerType)
		if err != nil {
			return err
		}
		lists[i] = list
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	resolvedByType := map[string]int{}
	var firstErr error
	for i, typ := range types {
		wg.Add(1)
		go func(typ string, titles []syncTitleItem) {
			defer wg.Done()
			resolved := 0
			for j, t := range titles {
				ReportLaneProgress(ctx, pool, "Mengisi episode", typ, j+1, len(titles), t.name)
				// Jatah resolve boleh habis -- judul tetap dipanggil
				// dengan anggaran 0: langkah pemetaan + daftar episode
				// tetap jalan (hanya resolve aliran yang 0). Tanpa
				// ini, judul tanpa episode di belakang antrean tidak
				// pernah dicoba (diagnosis 2026-11-26 19:11 WIB).
				srcs, err := loadSources(ctx, pool, typ)
				if err != nil {
					log.Printf("sinkron %s: %v", typ, err)
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				var titleErr error
				for _, src := range srcs {
					resolver, ok := sources.ByKey(src.Resolver)
					if !ok {
						continue // resolver belum diimplementasikan (menyusul per situs)
					}
					budget := maxResolvePerType - resolved
					if budget < 0 {
						budget = 0
					}
					done, err := syncTitle(ctx, pool, resolver, src, t.id, t.name, budget)
					if err != nil {
						log.Printf("sinkron %s dari %s: %v", t.name, src.Name, err)
						if titleErr == nil {
							titleErr = err
						}
					}
					resolved += done
					if resolved >= maxResolvePerType {
						break
					}
				}
				// Catat waktu percobaan sync judul ini (migrasi 0023):
				// dipakai round-robin kelompok ongoing (G3) dan cooldown
				// kegagalan -- gagal di semua sumber -> error dicatat
				// (dilewati 6 jam); berhasil/bersih -> penanda gagal
				// dihapus.
				if titleErr != nil {
					_, _ = pool.Exec(ctx, `update titles set last_sync_at = now(), last_sync_error = $2 where id = $1::uuid`, t.id, titleErr.Error())
				} else {
					_, _ = pool.Exec(ctx, `update titles set last_sync_at = now(), last_sync_error = null where id = $1::uuid`, t.id)
				}
			}
			mu.Lock()
			resolvedByType[typ] = resolved
			mu.Unlock()
		}(typ, lists[i])
	}
	wg.Wait()
	total := 0
	for _, n := range resolvedByType {
		total += n
	}
	log.Printf("sinkron sumber selesai: %d episode diselesaikan %v", total, resolvedByType)
	return firstErr
}

// syncTitle menyinkronkan SATU judul dari satu sumber: memetakan judul,
// mengambil daftar episode, lalu menyelesaikan aliran episode yang
// belum punya sumber (maksimal budget resolve). Mengembalikan jumlah
// episode yang berhasil diselesaikan.
func syncTitle(ctx context.Context, pool *pgxpool.Pool, resolver sources.Resolver,
	src sources.Source, titleID, titleName string, budget int) (int, error) {
	// 1) Pemetaan judul → halaman sumber (buat lewat pencarian bila belum ada).
	var titleURL string
	err := pool.QueryRow(ctx, `
		select source_url from title_sources
		where title_id = $1::uuid and source_id = $2::uuid`, titleID, src.ID).Scan(&titleURL)
	if err != nil {
		// Pencarian HTML didahulukan bila resolver menyediakannya
		// (TitleSearcher, mis. Anichin/Kuronime lewat /?s=): wp-json
		// Anichin mati 403 dari VPS sejak 2026-11-26, jadi jalur lama
		// tidak bisa membuat mapping baru. Pencarian dibatasi 10 dtk
		// (bukan timeout HTTP 30 dtk) supaya satu sumber yang
		// menggantung tidak membakar satu siklus.
		sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if hs, ok := resolver.(sources.TitleSearcher); ok {
			items, serr := hs.SearchTitles(sctx, src, titleName)
			if serr != nil {
				return 0, serr
			}
			if len(items) == 0 {
				return 0, nil // tidak ditemukan di sumber ini — bukan kegagalan
			}
			titleURL = items[0].URL
			if _, err := pool.Exec(ctx, `
				insert into title_sources (title_id, source_id, source_url, source_ref)
				values ($1::uuid, $2::uuid, $3, $4)
				on conflict (title_id, source_id) do nothing`,
				titleID, src.ID, titleURL, nil); err != nil {
				return 0, err
			}
			log.Printf("pemetaan baru: %s → %s (%s)", titleName, src.Name, titleURL)
		} else {
			cands, err := resolver.Search(sctx, src, titleName)
			if err != nil {
				return 0, err
			}
			if len(cands) == 0 {
				return 0, nil // tidak ditemukan di sumber ini — bukan kegagalan
			}
			titleURL = cands[0].URL
			if _, err := pool.Exec(ctx, `
				insert into title_sources (title_id, source_id, source_url, source_ref)
				values ($1::uuid, $2::uuid, $3, $4)
				on conflict (title_id, source_id) do nothing`,
				titleID, src.ID, titleURL, nullIfEmpty(cands[0].Ref)); err != nil {
				return 0, err
			}
			log.Printf("pemetaan baru: %s → %s (%s)", titleName, src.Name, titleURL)
		}
	}

	// 2) Daftar episode dari sumber → baris episodes katalog.
	eps, err := resolver.Episodes(ctx, src, titleURL)
	if err != nil {
		return 0, err
	}
	// Satu query agregat menggantikan 2 query per episode (cari baris
	// episode + hitung sumbernya): judul 100 episode tadinya butuh
	// 200+ query hanya untuk menyimpulkan "tidak ada yang perlu
	// dikerjakan". Peta (season_no, episode_no) → (id, sudah punya
	// sumber?) — kunci ganda karena S1E1 dan S2E1 berbagi episode_no.
	type epKey struct {
		season, no int
	}
	type epState struct {
		id         string
		hasSources bool
	}
	existing := map[epKey]epState{}
	erows, err := pool.Query(ctx, `
		select e.season_no, e.episode_no, e.id,
		       exists (select 1 from episode_sources es where es.episode_id = e.id)
		from episodes e where e.title_id = $1::uuid`, titleID)
	if err != nil {
		return 0, err
	}
	for erows.Next() {
		var k epKey
		var st epState
		if err := erows.Scan(&k.season, &k.no, &st.id, &st.hasSources); err == nil {
			existing[k] = st
		}
	}
	erows.Close()
	if err := erows.Err(); err != nil {
		return 0, err
	}
	// Episode yang sudah ada SEBELUM sinkron ini: notifikasi rilis
	// hanya untuk judul yang memang sudah punya episode (bukan
	// pengisian awal katalog yang bisa ratusan episode sekaligus).
	hadEps := len(existing)
	var newEps []int
	type resolveJob struct {
		id string
		ep sources.EpisodeRef
	}
	var jobList []resolveJob
	for _, ep := range eps {
		// Resolver tanpa season (anime/donghua/film) memakai 1 —
		// sama dengan nilai default migrasi untuk episode lama.
		season := ep.Season
		if season < 1 {
			season = 1
		}
		k := epKey{season: season, no: ep.Number}
		st, ok := existing[k]
		if !ok {
			var epID string
			if err := pool.QueryRow(ctx, `
				insert into episodes (title_id, season_no, episode_no, title, video_url)
				values ($1::uuid, $2, $3, $4, $5) returning id`,
				titleID, season, ep.Number, nullIfEmpty(ep.Title), ep.URL).Scan(&epID); err != nil {
				return 0, err
			}
			newEps = append(newEps, ep.Number)
			st = epState{id: epID}
			existing[k] = st
		}
		// 3) Kumpulkan episode yang belum punya sumber untuk
		// diselesaikan alirannya (dibatasi anggaran panggilan ini).
		if st.hasSources || len(jobList) >= budget {
			continue
		}
		jobList = append(jobList, resolveJob{st.id, ep})
	}
	// Selesaikan aliran kandidat secara PARALEL (maks fetchParallel
	// bersamaan). Cara lama satu per satu membuat pengisian 20
	// episode bisa memakan semenit lebih — halaman detail yang
	// menunggu terasa sangat lambat (keluhan Rams 2026-11-23).
	// Kandidat berurutan menaik sehingga episode awal masuk antrean
	// pekerja lebih dulu.
	done := 0
	if len(jobList) > 0 {
		sem := make(chan struct{}, fetchParallel)
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, jb := range jobList {
			wg.Add(1)
			go func(jb resolveJob) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				res, err := resolver.Resolve(ctx, src, jb.ep)
				if err != nil {
					log.Printf("resolve %s ep %d dari %s: %v", titleName, jb.ep.Number, src.Name, err)
					return
				}
				if err := storeResolved(ctx, pool, jb.id, res); err != nil {
					log.Printf("simpan hasil resolve %s ep %d: %v", titleName, jb.ep.Number, err)
					return
				}
				mu.Lock()
				done++
				mu.Unlock()
			}(jb)
		}
		wg.Wait()
	}
	// Notifikasi rilis ke pemfavorit judul (Daftar Saya) — hanya
	// bila judul sudah punya episode sebelumnya (bukan backfill).
	if len(newEps) > 0 && hadEps > 0 {
		if err := NotifyRelease(ctx, pool, titleID, titleName, newEps); err != nil {
			log.Printf("notifikasi rilis %s: %v", titleName, err)
		}
	}
	return done, nil
}

// storeResolved menyimpan hasil Resolve satu episode: aliran ke
// episode_sources, trek subtitle ke subtitles, dan video_url
// utama mengikuti aliran pertama. Dipakai sinkronisasi worker
// dan penyelesaian on-demand per episode.
func storeResolved(ctx context.Context, pool *pgxpool.Pool, epID string, res sources.Resolved) error {
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
	return nil
}

// ResolveOneEpisode menyelesaikan aliran satu episode untuk
// jalur on-demand playback: daftar episode diambil segar dari
// halaman seri sumber (URL halaman episode TIDAK dibaca dari
// episodes.video_url — kolom itu ditimpa aliran utama sesudah
// penyelesaian pertama), nomor yang cocok diselesaikan, lalu
// hasilnya disimpan lewat storeResolved. Error bila tidak ada
// sumber terpeta yang berhasil.
func ResolveOneEpisode(ctx context.Context, pool *pgxpool.Pool, episodeID string) error {
	var titleID, typ string
	var epSeason, epNo int
	if err := pool.QueryRow(ctx, `
		select e.title_id, e.season_no, e.episode_no, t.type
		from episodes e join titles t on t.id = e.title_id
		where e.id = $1::uuid`, episodeID).Scan(&titleID, &epSeason, &epNo, &typ); err != nil {
		return err
	}
	srcs, err := loadSources(ctx, pool, typ)
	if err != nil {
		return err
	}
	var lastErr error
	for _, src := range srcs {
		resolver, ok := sources.ByKey(src.Resolver)
		if !ok {
			continue
		}
		var seriesURL string
		if err := pool.QueryRow(ctx, `
			select source_url from title_sources
			where title_id = $1::uuid and source_id = $2::uuid`,
			titleID, src.ID).Scan(&seriesURL); err != nil {
			continue // judul tidak dipetakan ke sumber ini
		}
		eps, err := resolver.Episodes(ctx, src, seriesURL)
		if err != nil {
			lastErr = err
			continue
		}
		for _, ep := range eps {
			// Cocokkan season + nomor (serial TV: S1E1 dan S2E1
			// berbagi episode_no; resolver tanpa season memakai
			// Season 0 yang dinormalkan ke 1 saat sinkron).
			epSeason2 := ep.Season
			if epSeason2 < 1 {
				epSeason2 = 1
			}
			if epSeason2 != epSeason || ep.Number != epNo {
				continue
			}
			res, err := resolver.Resolve(ctx, src, ep)
			if err != nil {
				lastErr = err
				continue
			}
			if err := storeResolved(ctx, pool, episodeID, res); err != nil {
				return err
			}
			return nil
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("episode S%dE%d tidak ditemukan di sumber mana pun", epSeason, epNo)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
