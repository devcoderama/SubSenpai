package httpapi

// Telusur langsung dari sumber (permintaan Rams 2026-11-23): saat
// pengguna memfilter genre, aplikasi juga menampilkan judul-judul
// dari halaman genre SITUS SUMBER (Kuronime/Anichin/Hashflix),
// bukan hanya isi katalog lokal yang masih sedikit. Judul yang
// diketuk dan belum ada di katalog diimpor saat itu juga (judul +
// pemetaan sumber), lalu halaman detail mengisinya seperti biasa.

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"subsenpai/backend/internal/jobs"
	"subsenpai/backend/internal/sources"
)

// liveSources memuat sumber server aktif satu jenis konten
// (meniru jobs.loadSourcesMode; httpapi tidak berbagi helper itu).
func (a *API) liveSources(r *http.Request, contentType string) ([]sources.Source, error) {
	rows, err := a.pool.Query(r.Context(), `
		select id, name, content_type, lang, coalesce(base_url,''), resolver,
		       fetch_mode, subtitle_kind, priority
		from content_sources
		where is_active and base_url is not null and fetch_mode = 'server'
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
			sources.ApplyBaseOverride(&s)
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

type liveItem struct {
	Title     string   `json:"title"`
	Slug      string   `json:"slug"`
	PosterURL string   `json:"poster_url"`
	Year      int      `json:"year,omitempty"`
	Genres    []string `json:"genres,omitempty"`
	URL       string   `json:"url"`
	CatalogID *string  `json:"catalog_id,omitempty"`
}

type liveCacheEnt struct {
	at     time.Time
	source string
	items  []liveItem
}

var liveCache = struct {
	sync.Mutex
	m map[string]liveCacheEnt
}{m: map[string]liveCacheEnt{}}

func (a *API) handleLiveBrowse(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	if typ != "anime" && typ != "donghua" && typ != "film" {
		writeErr(w, http.StatusBadRequest, "type tidak dikenal")
		return
	}
	genre := strings.TrimSpace(q.Get("genre"))
	// Filter lengkap ala donkey.to/filters (film): genres dipisah
	// koma (genres=Action,Horror); bila kosong, pakai genre tunggal
	// lama agar aplikasi + web lama tetap jalan.
	genres := []string{}
	if gs := strings.TrimSpace(q.Get("genres")); gs != "" {
		for _, g := range strings.Split(gs, ",") {
			if g = strings.TrimSpace(g); g != "" {
				genres = append(genres, g)
			}
		}
	}
	if len(genres) == 0 && genre != "" {
		genres = []string{genre}
	}
	year, _ := strconv.Atoi(q.Get("year"))
	if year < 1900 || year > 2030 {
		year = 0
	}
	country := strings.ToUpper(strings.TrimSpace(q.Get("country")))
	if len(country) != 2 {
		country = ""
	}
	sort := strings.ToLower(strings.TrimSpace(q.Get("sort")))
	switch sort {
	case "popular", "latest", "name_asc", "name_desc", "most_voted":
	default:
		sort = ""
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	// Genre kosong kini berarti "beranda situs sumber" (terbaru /
	// populer versi situsnya) untuk semua type — dipakai halaman
	// web resmi; resolver yang tak mendukungnya mengembalikan
	// kosong. (Dulu anime/donghua tanpa genre dilayani katalog
	// lokal dan endpoint ini mengembalikan kosong.)
	key := typ + "|" + strings.ToLower(strings.Join(genres, ",")) + "|" +
		strconv.Itoa(year) + "|" + country + "|" + sort + "|" + strconv.Itoa(page)
	liveCache.Lock()
	ent, hit := liveCache.m[key]
	liveCache.Unlock()
	if hit && time.Since(ent.at) < 10*time.Minute {
		writeJSON(w, http.StatusOK, map[string]any{
			"source": ent.source, "page": page,
			"has_more": len(ent.items) >= 10, "items": ent.items,
		})
		return
	}

	srcs, err := a.liveSources(r, typ)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	var (
		found  []sources.DiscoveredTitle
		srcUse *sources.Source
	)
	for i := range srcs {
		resolver, ok := sources.ByKey(srcs[i].Resolver)
		if !ok {
			continue
		}
		// Resolver yang mendukung filter lengkap (film/Hashflix)
		// selalu dipakai bila ada; BrowseFiltered dengan filter
		// kosong sama dengan BrowseGenre lama. Resolver lain
		// (anime/donghua) tetap jalur genre tunggal.
		var items []sources.DiscoveredTitle
		var err error
		if fb, ok := resolver.(sources.FilteredBrowser); ok {
			f := sources.BrowseFilter{Genres: genres, Year: year, Country: country, Sort: sort}
			items, err = fb.BrowseFiltered(r.Context(), srcs[i], f, page)
		} else if gb, ok := resolver.(sources.GenreBrowser); ok {
			items, err = gb.BrowseGenre(r.Context(), srcs[i], genre, page)
		} else {
			continue
		}
		if err != nil {
			writeErr(w, http.StatusBadGateway, "sumber tidak terjangkau")
			return
		}
		if len(items) > 0 {
			found, srcUse = items, &srcs[i]
			break
		}
	}
	items := []liveItem{}
	if srcUse != nil {
		// Simpan otomatis ke katalog (permintaan Rams 2026-11-23
		// malam): judul yang tampil dari jelajah live langsung
		// tersimpan agar worker bisa melengkapi metadata +
		// episodenya; penandaan catalog_id di bawah lalu menemui
		// mereka juga, jadi ketukan pertama membuka detail tanpa
		// impor. Kegagalan simpan tidak menggagalkan respons.
		jobs.StoreBrowseHits(r.Context(), a.pool, typ, *srcUse, found)
		slugs := make([]string, 0, len(found))
		for _, d := range found {
			slugs = append(slugs, d.Slug)
		}
		inCatalog := map[string]string{}
		rows, err := a.pool.Query(r.Context(), `
			select slug, id from titles where type = $1 and slug = any($2)`, typ, slugs)
		if err == nil {
			for rows.Next() {
				var slug, id string
				if err := rows.Scan(&slug, &id); err == nil {
					inCatalog[slug] = id
				}
			}
			rows.Close()
		}
		for _, d := range found {
			it := liveItem{
				Title: d.Title, Slug: d.Slug, PosterURL: d.Poster,
				Year: d.Year, Genres: d.Genres, URL: d.URL,
			}
			if id, ok := inCatalog[d.Slug]; ok {
				idCopy := id
				it.CatalogID = &idCopy
			}
			items = append(items, it)
		}
		liveCache.Lock()
		liveCache.m[key] = liveCacheEnt{at: time.Now(), source: srcUse.Name, items: items}
		liveCache.Unlock()
	}
	srcName := ""
	if srcUse != nil {
		srcName = srcUse.Name
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"source": srcName, "page": page,
		"has_more": len(items) >= 10, "items": items,
	})
}

// importSlug menurunkan slug katalog dari URL halaman judul sumber:
// /anime/<slug>/ (Kuronime), /<slug>/ (Anichin), /movie/<idTMDB>
// (Hashflix — slug = slugify(judul)-id, sama seperti penemuan).
func importSlug(rawURL, title string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case len(parts) == 2 && parts[0] == "anime" && parts[1] != "":
		return parts[1]
	case len(parts) == 2 && parts[0] == "movie" && parts[1] != "":
		if _, err := strconv.Atoi(parts[1]); err == nil {
			return sources.SlugifyTitle(title) + "-" + parts[1]
		}
		return ""
	case len(parts) == 1 && parts[0] != "":
		return parts[0]
	}
	return ""
}

func (a *API) handleTitleImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type      string   `json:"type"`
		URL       string   `json:"url"`
		Title     string   `json:"title"`
		PosterURL string   `json:"poster_url"`
		Year      int      `json:"year"`
		Genres    []string `json:"genres"`
		Genre     string   `json:"genre"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "body tidak valid")
		return
	}
	if req.Type != "anime" && req.Type != "donghua" && req.Type != "film" {
		writeErr(w, http.StatusBadRequest, "type tidak dikenal")
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.URL) == "" {
		writeErr(w, http.StatusBadRequest, "judul/url kosong")
		return
	}
	slug := importSlug(req.URL, req.Title)
	if slug == "" {
		writeErr(w, http.StatusBadRequest, "url judul tidak dikenal")
		return
	}
	// URL wajib milik salah satu sumber server aktif jenis ini —
	// impor bukan pintu memasukkan alamat sembarangan ke katalog.
	srcs, err := a.liveSources(r, req.Type)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	var matched *sources.Source
	reqHost := ""
	if u, err := url.Parse(req.URL); err == nil {
		reqHost = strings.ToLower(u.Host)
	}
	for i := range srcs {
		base := strings.TrimRight(srcs[i].BaseURL, "/")
		if strings.HasPrefix(strings.ToLower(req.URL), strings.ToLower(base)) {
			matched = &srcs[i]
			break
		}
		if bu, err := url.Parse(base); err == nil && reqHost != "" &&
			strings.EqualFold(bu.Host, reqHost) {
			matched = &srcs[i]
			break
		}
	}
	if matched == nil {
		writeErr(w, http.StatusBadRequest, "url bukan milik sumber aktif")
		return
	}
	genres := req.Genres
	if len(genres) == 0 && strings.TrimSpace(req.Genre) != "" {
		genres = []string{strings.TrimSpace(req.Genre)}
	}
	if genres == nil {
		genres = []string{}
	}
	var year any
	if req.Year > 0 {
		year = req.Year
	}
	ctx := r.Context()
	var id string
	created := false
	err = a.pool.QueryRow(ctx, `
		insert into titles (slug, type, title, poster_url, year, genres, status)
		values ($1, $2, $3, $4, $5, $6, 'ongoing')
		on conflict (slug) do nothing
		returning id`,
		slug, req.Type, req.Title, nullIfEmptyStr(req.PosterURL), year, genres).Scan(&id)
	if err != nil {
		if err := a.pool.QueryRow(ctx, `
			select id from titles where slug = $1`, slug).Scan(&id); err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
	} else {
		created = true
	}
	if _, err := a.pool.Exec(ctx, `
		insert into title_sources (title_id, source_id, source_url)
		values ($1::uuid, $2::uuid, $3)
		on conflict (title_id, source_id) do nothing`,
		id, matched.ID, req.URL); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "slug": slug, "created": created})
}

// nullIfEmptyStr: string kosong -> nil (NULL) untuk kolom nullable.
func nullIfEmptyStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
