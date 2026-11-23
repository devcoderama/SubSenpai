package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var titleTypes = map[string]bool{
	"anime": true, "donghua": true, "film": true,
	"manga": true, "manhua": true, "manhwa": true,
}

type titleCard struct {
	ID       string   `json:"id"`
	Slug     string   `json:"slug"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	TitleAlt *string  `json:"title_alt,omitempty"`
	Poster   *string  `json:"poster_url,omitempty"`
	Year     *int     `json:"year,omitempty"`
	Status   string   `json:"status"`
	Genres   []string `json:"genres"`
	Rating   *float64 `json:"rating,omitempty"`
	IsAdult  bool     `json:"is_adult"`
}

const titleCardCols = `id, slug, type, title, title_alt, poster_url, year, status, genres, rating::float8, is_adult`

func scanTitleCard(row pgx.Row) (titleCard, error) {
	var c titleCard
	err := row.Scan(&c.ID, &c.Slug, &c.Type, &c.Title, &c.TitleAlt, &c.Poster,
		&c.Year, &c.Status, &c.Genres, &c.Rating, &c.IsAdult)
	if c.Genres == nil {
		c.Genres = []string{}
	}
	return c, err
}

func (a *API) queryCards(ctx context.Context, query string, args ...any) ([]titleCard, error) {
	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []titleCard{}
	for rows.Next() {
		c, err := scanTitleCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// handleHome: endpoint beranda gabungan (tren + terbaru) — satu panggilan.
func (a *API) handleHome(w http.ResponseWriter, r *http.Request) {
	a.serveCached(w, "home", func() (any, error) {
		ctx := r.Context()
		trending, err := a.queryCards(ctx, `
			select `+titleCardCols+` from titles t
			join trending_cache tc on tc.title_id = t.id
			where tc."window" = 'day' and not t.is_adult and not t.is_hidden
			and t.type not in ('manga', 'manhua', 'manhwa')
			order by tc.score desc limit 10`)
		if err != nil {
			return nil, err
		}
		if len(trending) == 0 {
			// Cache tren belum terisi worker — cadangan: rating tertinggi.
			trending, err = a.queryCards(ctx, `
				select `+titleCardCols+` from titles
				where not is_adult and not is_hidden
				and type not in ('manga', 'manhua', 'manhwa')
				order by rating desc nulls last, created_at desc limit 10`)
			if err != nil {
				return nil, err
			}
		}
		terbaru, err := a.queryCards(ctx, `
			select `+titleCardCols+` from titles
			where not is_adult and not is_hidden
			and type not in ('manga', 'manhua', 'manhwa')
			order by created_at desc, id desc limit 10`)
		if err != nil {
			return nil, err
		}
		return map[string]any{"trending": trending, "terbaru": terbaru}, nil
	})
}

type cursor struct {
	C time.Time `json:"c"`
	I string    `json:"i"`
}

func encodeCursor(c cursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (cursor, error) {
	var c cursor
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// titleFilter: saringan katalog dari parameter URL /titles — genre,
// tahun rilis, negara (region), dan network/perusahaan. Nilai kosong
// berarti tidak menyaring. Filter lanjutan film meniru "Advance
// Filter" hashflix (permintaan Rams 2026-11-23); faset pilihannya
// disajikan /api/v1/filters dari isi katalog sendiri.
type titleFilter struct {
	Genre   string
	Year    int
	Country string
	Network string
}

func (f titleFilter) empty() bool {
	return f.Genre == "" && f.Year == 0 && f.Country == "" && f.Network == ""
}

// titlesPage menjalankan query katalog satu halaman dan mengembalikan
// payload respons + jumlah item di halaman ini (sebelum dipotong
// limit, untuk keputusan pencarian live).
func (a *API) titlesPage(ctx context.Context, typ string, flt titleFilter, search string, cur *cursor, limit int) (map[string]any, int, error) {
	args := []any{}
	// Komik dicabut dari katalog (keputusan 2026-11-22): jenis
	// manga/manhua/manhwa tidak pernah ditampilkan, apa pun
	// parameter jenis yang diminta klien.
	where := "where not t.is_adult and not t.is_hidden and t.type not in ('manga', 'manhua', 'manhwa')"
	if typ != "" {
		args = append(args, typ)
		where += " and t.type = $" + strconv.Itoa(len(args))
	}
	if flt.Genre != "" {
		args = append(args, []string{flt.Genre})
		where += " and t.genres @> $" + strconv.Itoa(len(args))
	}
	if flt.Year > 0 {
		args = append(args, flt.Year)
		where += " and t.year = $" + strconv.Itoa(len(args))
	}
	if flt.Country != "" {
		args = append(args, flt.Country)
		where += " and lower(t.country) = lower($" + strconv.Itoa(len(args)) + ")"
	}
	if flt.Network != "" {
		args = append(args, flt.Network)
		where += " and t.network = $" + strconv.Itoa(len(args))
	}
	if search != "" {
		args = append(args, search)
		where += " and t.search_tsv @@ plainto_tsquery('simple', $" + strconv.Itoa(len(args)) + ")"
	}
	if cur != nil {
		args = append(args, cur.C, cur.I)
		where += " and (t.created_at, t.id) < ($" + strconv.Itoa(len(args)-1) + ", $" + strconv.Itoa(len(args)) + "::uuid)"
	}
	args = append(args, limit+1)
	sql := "select " + titleCardCols + ", t.created_at from titles t " + where +
		" order by t.created_at desc, t.id desc limit $" + strconv.Itoa(len(args))
	rows, err := a.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []titleCard{}
	createds := []time.Time{}
	for rows.Next() {
		var c titleCard
		var created time.Time
		if err := rows.Scan(&c.ID, &c.Slug, &c.Type, &c.Title, &c.TitleAlt, &c.Poster,
			&c.Year, &c.Status, &c.Genres, &c.Rating, &c.IsAdult, &created); err != nil {
			return nil, 0, err
		}
		if c.Genres == nil {
			c.Genres = []string{}
		}
		items = append(items, c)
		createds = append(createds, created)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeCursor(cursor{C: createds[limit-1], I: last.ID})
		items = items[:limit]
	}
	return map[string]any{"items": items, "next_cursor": next}, len(items), nil
}

// handleTitles: telusur katalog dengan pagination kursor (bukan OFFSET).
func (a *API) handleTitles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	if typ != "" && !titleTypes[typ] {
		writeErr(w, http.StatusBadRequest, "type tidak dikenal")
		return
	}
	limit := 20
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 50 {
		limit = v
	}
	var cur *cursor
	if s := q.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "cursor tidak valid")
			return
		}
		cur = &c
	}
	search := q.Get("q")
	year, _ := strconv.Atoi(q.Get("year"))
	flt := titleFilter{
		Genre:   q.Get("genre"),
		Year:    year,
		Country: q.Get("country"),
		Network: q.Get("network"),
	}
	if search != "" && cur == nil {
		// Halaman pertama pencarian TIDAK lewat cache: hasil kosong
		// harus bisa langsung jatuh ke pencarian live on-demand, dan
		// hasilnya harus langsung terlihat (bukan menunggu TTL cache).
		payload, n, err := a.titlesPage(r.Context(), typ, flt, search, nil, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		if n == 0 {
			lctx, cancel := context.WithTimeout(r.Context(), onDemandTimeout)
			a.liveSearch(lctx, typ, search)
			cancel()
			if p2, _, err := a.titlesPage(r.Context(), typ, flt, search, nil, limit); err == nil {
				payload = p2
			}
		}
		writeJSON(w, http.StatusOK, payload)
		return
	}
	if cur == nil && search == "" && flt.empty() && shouldDiscoverEmpty(typ, "", 0) {
		// Halaman pertama per tipe video server tanpa filter TIDAK
		// lewat cache: katalog tipe yang masih kosong harus langsung
		// memicu discovery on-demand (bukan menunggu siklus worker)
		// dan hasilnya — termasuk yang diselesaikan goroutine
		// latar — harus langsung terlihat, bukan tertahan TTL
		// cache. Pola yang sama seperti pencarian di atas.
		payload, n, err := a.titlesPage(r.Context(), typ, titleFilter{}, "", nil, limit)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		if shouldDiscoverEmpty(typ, "", n) && a.discoverOnDemand(r, typ) {
			if p2, _, err := a.titlesPage(r.Context(), typ, titleFilter{}, "", nil, limit); err == nil {
				payload = p2
			}
		}
		writeJSON(w, http.StatusOK, payload)
		return
	}
	key := "titles?" + r.URL.RawQuery
	a.serveCached(w, key, func() (any, error) {
		payload, _, err := a.titlesPage(r.Context(), typ, flt, search, cur, limit)
		return payload, err
	})
}

// handleGenres: daftar genre yang BENAR-BENAR ada di katalog
// (dihitung dari titles.genres) beserta jumlah judulnya — bahan
// chip filter genre di halaman telusur aplikasi (permintaan Rams
// 2026-11-23, meniru halaman /genres/ Kuronime). Filter-nya sendiri
// sudah lama didukung handleTitles lewat parameter genre.
func (a *API) handleGenres(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	if typ != "" && !titleTypes[typ] {
		writeErr(w, http.StatusBadRequest, "type tidak dikenal")
		return
	}
	a.serveCached(w, "genres?"+r.URL.RawQuery, func() (any, error) {
		where := "where not t.is_adult and not t.is_hidden and t.type not in ('manga', 'manhua', 'manhwa')"
		args := []any{}
		if typ != "" {
			args = append(args, typ)
			where += " and t.type = $1"
		}
		rows, err := a.pool.Query(r.Context(), `
			select g, count(*) as c from titles t, unnest(t.genres) g
			`+where+` group by g order by c desc, g asc`, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		type genreOut struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		genres := []genreOut{}
		for rows.Next() {
			var g genreOut
			if err := rows.Scan(&g.Name, &g.Count); err == nil {
				genres = append(genres, g)
			}
		}
		return map[string]any{"genres": genres}, rows.Err()
	})
}

// namaNegaraISO: nama tampilan untuk kode negara ISO 3166 alpha-2
// yang disimpan hashflix di titles.country film. Nilai lain (nama
// negara utuh dari Anichin/Kuronime, mis. "China") tampil apa adanya.
var namaNegaraISO = map[string]string{
	"US": "Amerika Serikat", "GB": "Inggris", "KR": "Korea Selatan",
	"JP": "Jepang", "CN": "China", "TW": "Taiwan", "HK": "Hong Kong",
	"ID": "Indonesia", "IN": "India", "TH": "Thailand", "FR": "Prancis",
	"DE": "Jerman", "ES": "Spanyol", "IT": "Italia", "CA": "Kanada",
	"AU": "Australia", "PH": "Filipina", "VN": "Vietnam", "MY": "Malaysia",
	"SG": "Singapura", "RU": "Rusia", "TR": "Turki", "MX": "Meksiko",
	"BR": "Brasil", "AR": "Argentina", "NL": "Belanda", "SE": "Swedia",
	"NO": "Norwegia", "DK": "Denmark", "FI": "Finlandia", "PL": "Polandia",
	"CZ": "Ceko", "HU": "Hungaria", "UA": "Ukraina", "EG": "Mesir",
	"SA": "Arab Saudi", "AE": "Uni Emirat Arab", "IR": "Iran",
	"PK": "Pakistan", "BD": "Bangladesh", "MM": "Myanmar", "KH": "Kamboja",
	"NZ": "Selandia Baru", "ZA": "Afrika Selatan", "CL": "Chile",
	"CO": "Kolombia", "PE": "Peru", "BE": "Belgia", "CH": "Swiss",
	"AT": "Austria", "GR": "Yunani", "PT": "Portugal", "IE": "Irlandia",
	"IL": "Israel", "IQ": "Irak", "LB": "Lebanon", "QA": "Qatar",
	"NP": "Nepal", "MN": "Mongolia", "KZ": "Kazakhstan", "UZ": "Uzbekistan",
	"HR": "Kroasia", "RS": "Serbia", "BG": "Bulgaria", "RO": "Rumania",
	"IS": "Islandia", "LK": "Sri Lanka", "NG": "Nigeria", "KE": "Kenya",
}

type facetOut struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Count int    `json:"count"`
}

// handleFilters: faset filter lanjutan untuk satu tipe — genre,
// tahun rilis, negara (region), network/perusahaan — dihitung dari
// katalog sendiri supaya tidak ada pilihan yang hasilnya kosong.
// Bahan panel "Filter" ala Advance Filter hashflix di aplikasi.
func (a *API) handleFilters(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	if typ == "" || !titleTypes[typ] {
		writeErr(w, http.StatusBadRequest, "type wajib dikenal")
		return
	}
	a.serveCached(w, "filters?"+r.URL.RawQuery, func() (any, error) {
		ctx := r.Context()
		base := "from titles t where not t.is_adult and not t.is_hidden and t.type = $1"
		facet := func(query string, display func(string) string) ([]facetOut, error) {
			rows, err := a.pool.Query(ctx, query, typ)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			out := []facetOut{}
			for rows.Next() {
				var f facetOut
				if err := rows.Scan(&f.Value, &f.Count); err != nil {
					return nil, err
				}
				f.Name = f.Value
				if display != nil {
					f.Name = display(f.Value)
				}
				out = append(out, f)
			}
			return out, rows.Err()
		}
		genres, err := facet(`select g, count(*) as c from titles t, unnest(t.genres) g
			where not t.is_adult and t.type = $1 group by g order by c desc, g asc`, nil)
		if err != nil {
			return nil, err
		}
		years, err := facet(`select year::text, count(*) as c `+base+`
			and t.year is not null group by t.year order by t.year desc`, nil)
		if err != nil {
			return nil, err
		}
		countries, err := facet(`select t.country, count(*) as c `+base+`
			and coalesce(t.country, '') <> '' group by t.country order by c desc, t.country asc`,
			func(v string) string {
				if n, ok := namaNegaraISO[v]; ok {
					return n
				}
				return v
			})
		if err != nil {
			return nil, err
		}
		networks, err := facet(`select t.network, count(*) as c `+base+`
			and coalesce(t.network, '') <> '' group by t.network order by c desc, t.network asc limit 40`, nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"genres": genres, "years": years,
			"countries": countries, "networks": networks,
		}, nil
	})
}

type titleDetail struct {
	titleCard
	Synopsis         *string `json:"synopsis,omitempty"`
	Season           *string `json:"season,omitempty"`
	Network          *string `json:"network,omitempty"`
	Studio           *string `json:"studio,omitempty"`
	DurationMin      *int    `json:"duration_min,omitempty"`
	Country          *string `json:"country,omitempty"`
	FetchMode        string  `json:"fetch_mode"`
	SourceDomain     *string `json:"source_domain,omitempty"`
	SourceURLPattern *string `json:"source_url_template,omitempty"`
	IsHidden         bool    `json:"-"`
}

type episodeItem struct {
	ID          string  `json:"id"`
	EpisodeNo   int     `json:"episode_no"`
	Title       *string `json:"title,omitempty"`
	DurationSec *int    `json:"duration_sec,omitempty"`
	// Locked: episode ini tidak boleh ditonton tier penonton saat
	// ini (aturan non-premium hanya episode pertama anime/donghua).
	Locked bool `json:"locked"`
}

type chapterItem struct {
	ID        string  `json:"id"`
	ChapterNo float64 `json:"chapter_no"`
	Title     *string `json:"title,omitempty"`
	PageCount int     `json:"page_count"`
}

// loadTitleDetail membaca detail judul + episode + chapter dari DB.
func (a *API) loadTitleDetail(ctx context.Context, id string) (titleDetail, []episodeItem, []chapterItem, error) {
	var detail titleDetail
	err := a.pool.QueryRow(ctx, `
		select `+titleCardCols+`, synopsis, season, network, studio,
		       duration_min, country, fetch_mode, source_domain,
		       source_url_template, is_hidden from titles where id = $1::uuid`, id,
	).Scan(&detail.ID, &detail.Slug, &detail.Type, &detail.Title, &detail.TitleAlt,
		&detail.Poster, &detail.Year, &detail.Status, &detail.Genres, &detail.Rating,
		&detail.IsAdult, &detail.Synopsis, &detail.Season, &detail.Network,
		&detail.Studio, &detail.DurationMin, &detail.Country, &detail.FetchMode,
		&detail.SourceDomain, &detail.SourceURLPattern, &detail.IsHidden)
	if err != nil {
		return detail, nil, nil, err
	}
	if detail.Genres == nil {
		detail.Genres = []string{}
	}
	episodes := []episodeItem{}
	erows, err := a.pool.Query(ctx, `
		select id, episode_no, title, duration_sec from episodes
		where title_id = $1::uuid order by episode_no`, id)
	if err == nil {
		defer erows.Close()
		for erows.Next() {
			var e episodeItem
			if err := erows.Scan(&e.ID, &e.EpisodeNo, &e.Title, &e.DurationSec); err == nil {
				episodes = append(episodes, e)
			}
		}
	}
	chapters := []chapterItem{}
	crows, err := a.pool.Query(ctx, `
		select id, chapter_no::float8, title, page_count from chapters
		where title_id = $1::uuid order by chapter_no`, id)
	if err == nil {
		defer crows.Close()
		for crows.Next() {
			var c chapterItem
			if err := crows.Scan(&c.ID, &c.ChapterNo, &c.Title, &c.PageCount); err == nil {
				chapters = append(chapters, c)
			}
		}
	}
	return detail, episodes, chapters, nil
}

// handleTitleDetail: detail judul + daftar episode/chapter. Bila
// detail judul server masih kosong (sinopsis belum ada / episode
// masih 0), jalur on-demand mencoba mengisinya langsung dari
// sumber sebelum menjawab (ondemand.go); kegagalannya tidak pernah
// menjadi error ke user — data DB apa adanya yang dikembalikan.
func (a *API) handleTitleDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	detail, episodes, chapters, err := a.loadTitleDetail(ctx, id)
	if err != nil || detail.IsHidden {
		// Judul yang disembunyikan admin tidak ada bagi aplikasi.
		writeErr(w, http.StatusNotFound, "judul tidak ditemukan")
		return
	}
	synopsis := ""
	if detail.Synopsis != nil {
		synopsis = *detail.Synopsis
	}
	if needMeta, needEps := needsOnDemand(detail.FetchMode, detail.Type, synopsis, len(episodes)); needMeta || needEps {
		if a.fillOnDemand(r, id, needMeta, needEps) {
			if d2, e2, c2, err := a.loadTitleDetail(ctx, id); err == nil {
				detail, episodes, chapters = d2, e2, c2
			}
		}
	}
	// Tandai episode terkunci untuk tier penonton ini.
	aksesTier, _ := a.viewerTier(r)
	if len(episodes) > 0 {
		minNo := episodes[0].EpisodeNo
		for _, e := range episodes {
			if e.EpisodeNo < minNo {
				minNo = e.EpisodeNo
			}
		}
		for i := range episodes {
			episodes[i].Locked = !watchAllowed(aksesTier, detail.Type, episodes[i].EpisodeNo == minNo)
		}
		// Episode yang sudah dibuka lewat iklan tidak tampil terkunci.
		if aksesTier != tierPremium {
			if unlocked := a.adUnlockedEpisodes(r, id); len(unlocked) > 0 {
				for i := range episodes {
					if unlocked[episodes[i].ID] {
						episodes[i].Locked = false
					}
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"title": detail, "episodes": episodes, "chapters": chapters,
		"access": map[string]any{"tier": string(aksesTier), "ads": aksesTier != tierPremium},
	})
}

// handlePlayback: sumber putar sehat (failover O9) + subtitle bahasa asal.
func (a *API) handlePlayback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var ep struct {
		ID        string `json:"id"`
		TitleID   string `json:"title_id"`
		EpisodeNo int    `json:"episode_no"`
		VideoURL  string `json:"-"`
		IntroS    *int   `json:"intro_start_sec,omitempty"`
		IntroE    *int   `json:"intro_end_sec,omitempty"`
		OutroS    *int   `json:"outro_start_sec,omitempty"`
		OutroE    *int   `json:"outro_end_sec,omitempty"`
	}
	err := a.pool.QueryRow(ctx, `
		select id, title_id, episode_no, video_url, intro_start_sec, intro_end_sec,
		       outro_start_sec, outro_end_sec
		from episodes where id = $1::uuid`, id,
	).Scan(&ep.ID, &ep.TitleID, &ep.EpisodeNo, &ep.VideoURL, &ep.IntroS, &ep.IntroE, &ep.OutroS, &ep.OutroE)
	if err != nil {
		writeErr(w, http.StatusNotFound, "episode tidak ditemukan")
		return
	}
	// Gerbang freemium: non-premium hanya episode pertama
	// anime/donghua; film & episode lanjutan khusus premium.
	if !a.gateEpisode(w, r, ep.ID, ep.TitleID, ep.EpisodeNo) {
		return
	}
	aksesTier, _ := a.viewerTier(r)
	type source struct {
		URL   string  `json:"url"`
		Label *string `json:"label,omitempty"`
		Lang  *string `json:"lang,omitempty"` // bahasa hardsub aliran ini (F40); kosong = softsub
	}
	readSources := func() []source {
		out := []source{}
		srows, err := a.pool.Query(ctx, `
			select url, label, lang from episode_sources
			where episode_id = $1::uuid and is_alive order by created_at`, id)
		if err != nil {
			return out
		}
		defer srows.Close()
		for srows.Next() {
			var s source
			if err := srows.Scan(&s.URL, &s.Label, &s.Lang); err == nil {
				out = append(out, s)
			}
		}
		return out
	}
	sources := readSources()
	// Sumber sehat = ada URL http(s). Bila belum ada (atau hanya
	// placeholder "about:blank" dari penyelesaian lama yang
	// rusak), selesaikan on-demand lalu baca ulang.
	healthy := false
	for _, s := range sources {
		if strings.HasPrefix(s.URL, "http://") || strings.HasPrefix(s.URL, "https://") {
			healthy = true
			break
		}
	}
	if !healthy && a.fillEpisodeOnDemand(r, id) {
		sources = readSources()
	}
	if len(sources) == 0 {
		utama := "utama"
		sources = []source{{URL: ep.VideoURL, Label: &utama}}
	}
	type subtitle struct {
		Lang   string  `json:"lang"`
		Source string  `json:"source"`
		Format string  `json:"format"`
		File   *string `json:"file_url,omitempty"`
		Status string  `json:"status"`
	}
	subtitles := []subtitle{}
	urows, err := a.pool.Query(ctx, `
		select lang, source, format, file_url, status from subtitles
		where episode_id = $1::uuid order by lang`, id)
	if err == nil {
		defer urows.Close()
		for urows.Next() {
			var s subtitle
			if err := urows.Scan(&s.Lang, &s.Source, &s.Format, &s.File, &s.Status); err == nil {
				subtitles = append(subtitles, s)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"episode":   ep,
		"sources":   sources,
		"subtitles": subtitles,
		"access":    map[string]any{"tier": string(aksesTier), "ads": aksesTier != tierPremium},
	})
}

// handleChapter: isi chapter (halaman) untuk mode baca.
func (a *API) handleChapter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var ch struct {
		ID        string  `json:"id"`
		TitleID   string  `json:"title_id"`
		ChapterNo float64 `json:"chapter_no"`
		Title     *string `json:"title,omitempty"`
	}
	err := a.pool.QueryRow(ctx, `
		select id, title_id, chapter_no::float8, title from chapters where id = $1::uuid`, id,
	).Scan(&ch.ID, &ch.TitleID, &ch.ChapterNo, &ch.Title)
	if err != nil {
		writeErr(w, http.StatusNotFound, "chapter tidak ditemukan")
		return
	}
	// Gerbang freemium: tamu maks. 1 chapter.
	if !a.gateChapter(w, r, ch.ID, ch.TitleID) {
		return
	}
	aksesTier, _ := a.viewerTier(r)
	type page struct {
		PageNo    int     `json:"page_no"`
		ImageURL  string  `json:"image_url"`
		LightURL  *string `json:"light_url,omitempty"`
		OCRStatus string  `json:"ocr_status"`
	}
	pages := []page{}
	prows, err := a.pool.Query(ctx, `
		select page_no, image_url, light_url, ocr_status from chapter_pages
		where chapter_id = $1::uuid order by page_no`, id)
	if err == nil {
		defer prows.Close()
		for prows.Next() {
			var pg page
			if err := prows.Scan(&pg.PageNo, &pg.ImageURL, &pg.LightURL, &pg.OCRStatus); err == nil {
				pages = append(pages, pg)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"chapter": ch,
		"pages":   pages,
		"access":  map[string]any{"tier": string(aksesTier), "ads": aksesTier != tierPremium},
	})
}
