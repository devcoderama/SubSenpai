package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Resolver hashflix (film): katalog situs adalah aplikasi Next.js yang
// memuat data dari API internal bergaya TMDB di /database/* situs itu
// sendiri (ditemukan dari potongan JS publiknya, 2026-11-22):
//   GET /database/trending/movie/{day|week}?page=N
// membalas JSON TMDB ({page, results: [{id, title, poster_path,
// release_date, genre_ids, adult, ...}], total_pages}). Poster adalah
// berkas CDN TMDB: https://image.tmdb.org/t/p/w500<poster_path>.
//   GET /database/details/movie/{tmdbID}
// membalas JSON TMDB lengkap satu film (overview, runtime, genres,
// production_companies, ...) — dipakai SeriesInfo untuk metadata
// halaman detail (2026-11-22 malam, keluhan Rams: metadata film
// kosong).
// Halaman tonton situs (/watch/movie/{id}) memutar embed pihak ketiga
// berbasis id TMDB (ditemukan di potongan JS pemutarnya): server
// utama vidstuck.cc (tanpa iklan menurut situs), cadangan zxcstream,
// vidbolt, vidfast — Resolve mengembalikan keempatnya sebagai aliran
// embed; satu film = satu "episode" nomor 1.
type hashflixResolver struct{}

// NewHashflix membuat resolver hashflix (didaftarkan di sources.go).
func NewHashflix() *hashflixResolver { return &hashflixResolver{} }

func (h *hashflixResolver) Key() string { return "hashflix" }

func (h *hashflixResolver) Search(_ context.Context, _ Source, _ string) ([]Candidate, error) {
	return nil, fmt.Errorf("hashflix: pencarian belum didukung")
}

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
func (h *hashflixResolver) Episodes(_ context.Context, _ Source, titleURL string) ([]EpisodeRef, error) {
	id := tmdbIDFromRef(titleURL)
	if id == "" {
		return nil, fmt.Errorf("hashflix: id TMDB tidak terbaca dari %s", titleURL)
	}
	return []EpisodeRef{{Number: 1, Title: "Film", URL: titleURL, Ref: id}}, nil
}

// hfServers adalah daftar server embed halaman tonton hashflix
// (nama persis seperti di situsnya; urutan = prioritas situs).
var hfServers = []struct{ name, movie string }{
	{"Main Server", "https://vidstuck.cc/embed/movie/%s"},
	{"Backup I", "https://zxcstream.cc/player/movie/%s"},
	{"Backup II", "https://vidbolt.xyz/movie/%s"},
	{"Backup III", "https://vidfast.vc/movie/%s"},
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
		out.Streams = append(out.Streams, Stream{
			URL:   fmt.Sprintf(s.movie, id),
			Label: s.name,
		})
	}
	return out, nil
}

type hfDetails struct {
	Title            string `json:"title"`
	OriginalTitle    string `json:"original_title"`
	Overview         string `json:"overview"`
	Runtime          int    `json:"runtime"`
	ReleaseDate      string `json:"release_date"`
	Genres           []struct {
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
	meta := &SeriesMeta{
		Synopsis:    strings.TrimSpace(d.Overview),
		DurationMin: d.Runtime,
	}
	if d.OriginalTitle != "" && d.OriginalTitle != d.Title {
		meta.TitleAlt = d.OriginalTitle
	}
	if len(d.ReleaseDate) >= 4 {
		fmt.Sscanf(d.ReleaseDate[:4], "%d", &meta.Year)
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
		meta.Country = d.ProductionCountries[0].ISO
	}
	if meta.Synopsis == "" && meta.DurationMin == 0 && len(meta.Genres) == 0 {
		return nil, fmt.Errorf("hashflix: detail film kosong")
	}
	return meta, nil
}

// SeriesInfo membaca metadata film dari API detail TMDB situs.
func (h *hashflixResolver) SeriesInfo(ctx context.Context, src Source, titleURL string) (*SeriesMeta, error) {
	id := tmdbIDFromRef(titleURL)
	if id == "" {
		return nil, fmt.Errorf("hashflix: id TMDB tidak terbaca dari %s", titleURL)
	}
	base := strings.TrimRight(src.BaseURL, "/")
	body, err := Fetch(ctx, fmt.Sprintf("%s/database/details/movie/%s", base, id))
	if err != nil {
		return nil, err
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
	Page int `json:"page"`
	Results []struct {
		ID int `json:"id"`
		Title string `json:"title"`
		Name string `json:"name"`
		PosterPath string `json:"poster_path"`
		ReleaseDate string `json:"release_date"`
		GenreIDs []int `json:"genre_ids"`
		Adult bool `json:"adult"`
	} `json:"results"`
	TotalPages int `json:"total_pages"`
}

// parseHFTrending mengurai satu halaman tren TMDB menjadi judul katalog.
func parseHFTrending(body string, base string) ([]DiscoveredTitle, int) {
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
			Title: title,
			Slug: fmt.Sprintf("%s-%d", slugify(title), r.ID),
			URL: fmt.Sprintf("%s/movie/%d", base, r.ID),
			Completed: true,
		}
		if r.PosterPath != "" {
			d.Poster = "https://image.tmdb.org/t/p/w500" + r.PosterPath
		}
		if len(r.ReleaseDate) >= 4 {
			fmt.Sscanf(r.ReleaseDate[:4], "%d", &d.Year)
		}
		for _, gid := range r.GenreIDs {
			if name, ok := tmdbMovieGenres[gid]; ok {
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
// halaman, deduplikasi per id TMDB di slug).
func (h *hashflixResolver) Discover(ctx context.Context, src Source, limit int) ([]DiscoveredTitle, error) {
	base := strings.TrimRight(src.BaseURL, "/")
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
		body, err := Fetch(ctx, fmt.Sprintf("%s/database/trending/movie/week?page=%d", base, p))
		if err != nil {
			lastErr = err
			break
		}
		items, _ := parseHFTrending(body, base)
		if len(items) == 0 {
			break
		}
		add(items)
	}
	if len(out) < limit {
		if body, err := Fetch(ctx, base+"/database/trending/movie/day?page=1"); err == nil {
			items, _ := parseHFTrending(body, base)
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
