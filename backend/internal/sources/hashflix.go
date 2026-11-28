package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Resolver hashflix (film + serial TV): katalog situs adalah aplikasi
// Next.js yang memuat data dari API internal bergaya TMDB di
// /database/* situs itu sendiri (ditemukan dari potongan JS publiknya,
// 2026-11-22):
//
//	GET /database/trending/movie/{day|week}?page=N
//
// membalas JSON TMDB ({page, results: [{id, title, poster_path,
// release_date, genre_ids, adult, ...}], total_pages}). Poster adalah
// berkas CDN TMDB: https://image.tmdb.org/t/p/w500<poster_path>.
//
//	GET /database/details/movie/{tmdbID}
//
// membalas JSON TMDB lengkap satu film (overview, runtime, genres,
// production_companies, ...) — dipakai SeriesInfo untuk metadata
// halaman detail (2026-11-22 malam, keluhan Rams: metadata film
// kosong).
// Serial TV memakai endpoint yang sama dengan kata "tv" (terverifikasi
// live 2026-11-28, permintaan Rams): /database/trending/tv/{day|week},
// /database/discover/tv, /database/search/tv, /database/details/tv/{id}
// (lengkap seasons [{season_number, episode_count}] + number_of_seasons
// + number_of_episodes; /database/tv/{id}/season/{n} TIDAK diekspos —
// judul per episode tidak tersedia, episode memakai "Episode N").
// Halaman tonton situs (/watch/movie/{id}) memutar embed pihak ketiga
// berbasis id TMDB (ditemukan di potongan JS pemutarnya): server
// utama vidstuck.cc (tanpa iklan menurut situs), cadangan zxcstream,
// vidbolt, vidfast — Resolve mengembalikan keempatnya sebagai aliran
// embed; satu film = satu "episode" nomor 1. Serial TV memakai pola
// yang sama per season/episode: /embed/tv/{id}/{s}/{e} (vidstuck),
// /player/tv/{id}/{s}/{e} (zxcstream), /tv/{id}/{s}/{e}
// (vidbolt, vidfast) — semuanya terverifikasi 200 live 2026-11-28.
// Akses serial TV: GRATIS untuk semua tier (keputusan Rams 2026-11-28)
// — aturan itu hidup di httpapi.watchAllowed, bukan di sini.
type hashflixResolver struct{}

// NewHashflix membuat resolver hashflix (didaftarkan di sources.go).
func NewHashflix() *hashflixResolver { return &hashflixResolver{} }

func (h *hashflixResolver) Key() string { return "hashflix" }

func (h *hashflixResolver) Search(_ context.Context, _ Source, _ string) ([]Candidate, error) {
	return nil, fmt.Errorf("hashflix: pencarian belum didukung")
}

// donkeySort memetakan urutan ala donkey.to/filters ke sort_by TMDB.
var donkeySort = map[string]string{
	"popular":    "popularity.desc",
	"latest":     "primary_release_date.desc",
	"name_asc":   "title.asc",
	"name_desc":  "title.desc",
	"most_voted": "vote_count.desc",
}

// donkeySortTV adalah pasangan donkeySort untuk serial TV: TMDB
// discover/tv memakai first_air_date (bukan primary_release_date)
// dan name (bukan title).
var donkeySortTV = map[string]string{
	"popular":    "popularity.desc",
	"latest":     "first_air_date.desc",
	"name_asc":   "name.asc",
	"name_desc":  "name.desc",
	"most_voted": "vote_count.desc",
}

// tmdbTVGenres adalah peta genre TMDB untuk serial TV (id -> nama);
// dipakai parseHFTrending + pengurai detail TV.
var tmdbTVGenres = map[int]string{
	10759: "Action & Adventure", 16: "Animation", 35: "Comedy",
	80: "Crime", 99: "Documentary", 18: "Drama", 10751: "Family",
	10762: "Kids", 9648: "Mystery", 10763: "News", 10764: "Reality",
	10765: "Sci-Fi & Fantasy", 10766: "Soap", 10767: "Talk",
	10768: "War & Politics", 37: "Western",
}

// tmdbTVGenreIDs membalik tmdbTVGenres (nama kecil -> id) untuk
// parameter with_genres Hashflix — pasangan tmdbGenreIDs film.
var tmdbTVGenreIDs = func() map[string]int {
	out := map[string]int{}
	for id, name := range tmdbTVGenres {
		out[strings.ToLower(name)] = id
	}
	return out
}()

// hfDiscoverURL merakit URL discover TMDB Hashflix dari filter.
// Genre banyak = OR (pemisah "|" — TMDB: koma = AND, pipa = OR).
// Mengembalikan ok=false bila genre diisi tapi tak satu pun dikenal
// TMDB (pemanggil mengembalikan kosong, seperti BrowseGenre lama).
// kind "movie" atau "tv": TV memakai peta genre + urutan sendiri.
func hfDiscoverURL(base, kind string, f BrowseFilter, page int) (string, bool) {
	if page < 1 {
		page = 1
	}
	if kind != "tv" {
		kind = "movie"
	}
	u := fmt.Sprintf("%s/database/discover/%s?page=%d", strings.TrimRight(base, "/"), kind, page)
	genreIDs := tmdbGenreIDs
	sortMap := donkeySort
	if kind == "tv" {
		genreIDs = tmdbTVGenreIDs
		sortMap = donkeySortTV
	}
	if len(f.Genres) > 0 {
		var ids []string
		for _, g := range f.Genres {
			if id, ok := genreIDs[strings.ToLower(strings.TrimSpace(g))]; ok {
				ids = append(ids, strconv.Itoa(id))
			}
		}
		if len(ids) == 0 {
			return "", false
		}
		u += "&with_genres=" + strings.Join(ids, "%7C")
	}
	if f.Year > 0 {
		if kind == "tv" {
			u += fmt.Sprintf("&first_air_date_year=%d", f.Year)
		} else {
			u += fmt.Sprintf("&primary_release_year=%d", f.Year)
		}
	}
	if c := strings.ToUpper(strings.TrimSpace(f.Country)); c != "" {
		u += "&with_origin_country=" + c
	}
	if sb, ok := sortMap[strings.ToLower(strings.TrimSpace(f.Sort))]; ok {
		u += "&sort_by=" + sb
	}
	return u, true
}

// hfKind mengembalikan kata endpoint Hashflix untuk sumber ini:
// "tv" bila content_type-nya series, "movie" bila film.
func hfKind(src Source) string {
	if src.ContentType == "series" {
		return "tv"
	}
	return "movie"
}

// BrowseFiltered Hashflix: discover TMDB dengan filter lengkap
// ala donkey.to (genre/tahun/negara/urutan). Filter kosong = semua
// film (sama seperti BrowseGenre tanpa genre).
func (h *hashflixResolver) BrowseFiltered(ctx context.Context, src Source, f BrowseFilter, page int) ([]DiscoveredTitle, error) {
	if page < 1 {
		return nil, nil
	}
	u, ok := hfDiscoverURL(src.BaseURL, hfKind(src), f, page)
	if !ok {
		return nil, nil
	}
	body, err := Fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	items, _ := parseHFTrending(body, strings.TrimRight(src.BaseURL, "/"), hfKind(src))
	return items, nil
}

// (SearchTitles Hashflix sudah ada di searchtitles.go —
// /database/search/movie?query= — jadi pencarian kata kunci film
// sudah hidup lewat liveSearch; di sini tak diduplikasi.)

// tmdbIDFromRef mengekstrak id TMDB dari URL halaman judul hashflix
// ({base}/movie/83533), slug katalog (avatar-fire-and-ash-83533),
// atau ref polos (83533). Kosong bila tidak ada ekor numerik.
func tmdbIDFromRef(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "-"); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return ""
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return s
}

// Episodes: satu film adalah satu episode bernomor 1; id TMDB
// disimpan di Ref agar Resolve tidak perlu mengurai URL lagi.
// Serial TV: daftar episode dibangun dari seasons detail TV
// (details/tv/{id} — season 0/Specials dilewati, episode memakai
// "Episode N" karena judul per episode tidak diekspos API-nya);
// Ref membawa id TMDB, Season + Number membawa posisi tayangnya.
func (h *hashflixResolver) Episodes(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error) {
	id := tmdbIDFromRef(titleURL)
	if id == "" {
		return nil, fmt.Errorf("hashflix: id TMDB tidak terbaca dari %s", titleURL)
	}
	if hfKind(src) == "tv" {
		base := strings.TrimRight(src.BaseURL, "/")
		body, err := Fetch(ctx, fmt.Sprintf("%s/database/details/tv/%s", base, id))
		if err != nil {
			return nil, err
		}
		var d hfDetails
		if err := json.Unmarshal([]byte(body), &d); err != nil {
			return nil, err
		}
		out := []EpisodeRef{}
		for _, s := range d.Seasons {
			if s.SeasonNumber < 1 || s.EpisodeCount < 1 {
				continue
			}
			for e := 1; e <= s.EpisodeCount; e++ {
				out = append(out, EpisodeRef{
					Season: s.SeasonNumber,
					Number: e,
					Title:  fmt.Sprintf("Episode %d", e),
					URL:    titleURL,
					Ref:    id,
				})
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("hashflix: serial TV %s tidak punya season/episode", id)
		}
		return out, nil
	}
	return []EpisodeRef{{Number: 1, Title: "Film", URL: titleURL, Ref: id}}, nil
}

// hfServers adalah daftar server embed halaman tonton hashflix
// (nama persis seperti di situsnya; urutan = prioritas situs).
var hfServers = []struct{ name, movie, tv string }{
	{"Main Server", "https://vidstuck.cc/embed/movie/%s", "https://vidstuck.cc/embed/tv/%s/%d/%d"},
	{"Backup I", "https://zxcstream.cc/player/movie/%s", "https://zxcstream.cc/player/tv/%s/%d/%d"},
	{"Backup II", "https://vidbolt.xyz/movie/%s", "https://vidbolt.xyz/tv/%s/%d/%d"},
	{"Backup III", "https://vidfast.vc/movie/%s", "https://vidfast.vc/tv/%s/%d/%d"},
}

// Resolve mengembalikan aliran embed semua server situs untuk id
// TMDB film ini. Aliran adalah URL halaman embed (diputar WebView
// aplikasi), bukan berkas video langsung.
func (h *hashflixResolver) Resolve(_ context.Context, _ Source, ep EpisodeRef) (Resolved, error) {
	id := tmdbIDFromRef(ep.Ref)
	if id == "" {
		id = tmdbIDFromRef(ep.URL)
	}
	if id == "" {
		return Resolved{}, fmt.Errorf("hashflix: id TMDB tidak terbaca dari ref %q url %q", ep.Ref, ep.URL)
	}
	out := Resolved{}
	for _, s := range hfServers {
		u := fmt.Sprintf(s.movie, id)
		if ep.Season > 0 {
			u = fmt.Sprintf(s.tv, id, ep.Season, ep.Number)
		}
		out.Streams = append(out.Streams, Stream{
			URL:   u,
			Label: s.name,
		})
	}
	return out, nil
}

type hfDetails struct {
	Title            string `json:"title"`
	Name             string `json:"name"`
	OriginalTitle    string `json:"original_title"`
	OriginalName     string `json:"original_name"`
	Overview         string `json:"overview"`
	Runtime          int    `json:"runtime"`
	EpisodeRunTime   []int  `json:"episode_run_time"`
	ReleaseDate      string `json:"release_date"`
	FirstAirDate     string `json:"first_air_date"`
	Status           string `json:"status"`
	NumberOfSeasons  int    `json:"number_of_seasons"`
	NumberOfEpisodes int    `json:"number_of_episodes"`
	Seasons          []struct {
		SeasonNumber int    `json:"season_number"`
		EpisodeCount int    `json:"episode_count"`
		Name         string `json:"name"`
	} `json:"seasons"`
	Genres []struct {
		Name string `json:"name"`
	} `json:"genres"`
	ProductionCompanies []struct {
		Name string `json:"name"`
	} `json:"production_companies"`
	ProductionCountries []struct {
		ISO string `json:"iso_3166_1"`
	} `json:"production_countries"`
}

// parseHFDetails mengurai JSON /database/details/movie/{id} menjadi
// metadata seri katalog.
func parseHFDetails(body string) (*SeriesMeta, error) {
	var d hfDetails
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		return nil, err
	}
	meta := hfDetailsToMeta(&d, false)
	if meta.Synopsis == "" && meta.DurationMin == 0 && len(meta.Genres) == 0 {
		return nil, fmt.Errorf("hashflix: detail film kosong")
	}
	return meta, nil
}

// parseHFDetailsTV mengurai JSON /database/details/tv/{id}: judul
// dari `name`, tahun dari first_air_date, durasi per episode dari
// episode_run_time pertama. Season/episode tidak masuk SeriesMeta —
// daftar episode dibangun Episodes() dari d.Seasons.
func parseHFDetailsTV(body string) (*SeriesMeta, error) {
	var d hfDetails
	if err := json.Unmarshal([]byte(body), &d); err != nil {
		return nil, err
	}
	meta := hfDetailsToMeta(&d, true)
	if meta.Synopsis == "" && meta.DurationMin == 0 && len(meta.Genres) == 0 {
		return nil, fmt.Errorf("hashflix: detail serial TV kosong")
	}
	return meta, nil
}

func hfDetailsToMeta(d *hfDetails, isTV bool) *SeriesMeta {
	meta := &SeriesMeta{
		Synopsis: strings.TrimSpace(d.Overview),
	}
	title := d.Title
	origTitle := d.OriginalTitle
	airDate := d.ReleaseDate
	if isTV {
		title = d.Name
		origTitle = d.OriginalName
		airDate = d.FirstAirDate
		if len(d.EpisodeRunTime) > 0 {
			meta.DurationMin = d.EpisodeRunTime[0]
		}
	} else {
		meta.DurationMin = d.Runtime
	}
	_ = title
	if origTitle != "" && origTitle != title {
		meta.TitleAlt = origTitle
	}
	if len(airDate) >= 4 {
		fmt.Sscanf(airDate[:4], "%d", &meta.Year)
	}
	for _, g := range d.Genres {
		if g.Name != "" {
			meta.Genres = append(meta.Genres, g.Name)
		}
	}
	if len(d.ProductionCompanies) > 0 {
		meta.Studio = d.ProductionCompanies[0].Name
	}
	if len(d.ProductionCountries) > 0 {
		// Simpan NAMA Indonesia ("Indonesia"), bukan kode ISO ("ID"):
		// kolom country katalog berisi nama, dan filter negara
		// membandingkan dengan nama — kode membuat film tak ketemu
		// (diagnosis 2026-11-28: Ghost in the Cell tersimpan "ID").
		meta.Country = CountryName(d.ProductionCountries[0].ISO)
	}
	return meta
}

// SeriesInfo membaca metadata film dari API detail TMDB situs.
// Serial TV memakai endpoint details/tv yang sama bentuknya.
func (h *hashflixResolver) SeriesInfo(ctx context.Context, src Source, titleURL string) (*SeriesMeta, error) {
	id := tmdbIDFromRef(titleURL)
	if id == "" {
		return nil, fmt.Errorf("hashflix: id TMDB tidak terbaca dari %s", titleURL)
	}
	base := strings.TrimRight(src.BaseURL, "/")
	kind := hfKind(src)
	body, err := Fetch(ctx, fmt.Sprintf("%s/database/details/%s/%s", base, kind, id))
	if err != nil {
		return nil, err
	}
	if kind == "tv" {
		return parseHFDetailsTV(body)
	}
	return parseHFDetails(body)
}

var tmdbMovieGenres = map[int]string{
	28: "Action", 12: "Adventure", 16: "Animation", 35: "Comedy",
	80: "Crime", 99: "Documentary", 18: "Drama", 10751: "Family",
	14: "Fantasy", 36: "History", 27: "Horror", 10402: "Music",
	9648: "Mystery", 10749: "Romance", 878: "Science Fiction",
	10770: "TV Movie", 53: "Thriller", 10752: "War", 37: "Western",
}

type hfTrending struct {
	Page    int `json:"page"`
	Results []struct {
		ID           int    `json:"id"`
		Title        string `json:"title"`
		Name         string `json:"name"`
		PosterPath   string `json:"poster_path"`
		ReleaseDate  string `json:"release_date"`
		FirstAirDate string `json:"first_air_date"`
		GenreIDs     []int  `json:"genre_ids"`
		Adult        bool   `json:"adult"`
	} `json:"results"`
	TotalPages int `json:"total_pages"`
}

// parseHFTrending mengurai satu halaman tren TMDB menjadi judul
// katalog. kind "movie" atau "tv": judul TV datang di `name` +
// `first_air_date`, dan URL halamannya /tv/{id}; film di `title` +
// `release_date` + /movie/{id}. Serial TV tidak ditandai Completed
// (status tamat tidak diketahui dari daftar tren — tetap ongoing).
func parseHFTrending(body string, base, kind string) ([]DiscoveredTitle, int) {
	if kind != "tv" {
		kind = "movie"
	}
	var page hfTrending
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		return nil, 0
	}
	out := []DiscoveredTitle{}
	for _, r := range page.Results {
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = strings.TrimSpace(r.Name)
		}
		if title == "" || r.ID == 0 || r.Adult {
			continue
		}
		d := DiscoveredTitle{
			Title:     title,
			Slug:      fmt.Sprintf("%s-%d", slugify(title), r.ID),
			URL:       fmt.Sprintf("%s/%s/%d", base, kind, r.ID),
			Completed: kind == "movie",
		}
		if r.PosterPath != "" {
			d.Poster = "https://image.tmdb.org/t/p/w500" + r.PosterPath
		}
		airDate := r.ReleaseDate
		if airDate == "" {
			airDate = r.FirstAirDate
		}
		if len(airDate) >= 4 {
			fmt.Sscanf(airDate[:4], "%d", &d.Year)
		}
		genreMap := tmdbMovieGenres
		if kind == "tv" {
			genreMap = tmdbTVGenres
		}
		for _, gid := range r.GenreIDs {
			if name, ok := genreMap[gid]; ok {
				d.Genres = append(d.Genres, name)
			}
		}
		out = append(out, d)
	}
	return out, page.TotalPages
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// Discover mengumpulkan film tren (mingguan 5 halaman + harian 1
// halaman, deduplikasi per id TMDB di slug). Serial TV memakai
// endpoint trending/tv yang sama.
func (h *hashflixResolver) Discover(ctx context.Context, src Source, limit int) ([]DiscoveredTitle, error) {
	base := strings.TrimRight(src.BaseURL, "/")
	kind := hfKind(src)
	out := []DiscoveredTitle{}
	seen := map[string]bool{}
	add := func(items []DiscoveredTitle) {
		for _, d := range items {
			if seen[d.Slug] {
				continue
			}
			seen[d.Slug] = true
			out = append(out, d)
		}
	}
	var lastErr error
	for p := 1; p <= 5 && len(out) < limit; p++ {
		body, err := Fetch(ctx, fmt.Sprintf("%s/database/trending/%s/week?page=%d", base, kind, p))
		if err != nil {
			lastErr = err
			break
		}
		items, _ := parseHFTrending(body, base, kind)
		if len(items) == 0 {
			break
		}
		add(items)
	}
	if len(out) < limit {
		if body, err := Fetch(ctx, base+"/database/trending/"+kind+"/day?page=1"); err == nil {
			items, _ := parseHFTrending(body, base, kind)
			add(items)
		} else {
			lastErr = err
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	return out, nil
}
