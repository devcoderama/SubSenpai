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
// Penemuan hanya mengisi katalog film; resolusi aliran (player situs
// dengan subtitle bawaan) belum dibangun — Search/Episodes/Resolve
// membalas kosong/tidak didukung.
type hashflixResolver struct{}

// NewHashflix membuat resolver hashflix (didaftarkan di sources.go).
func NewHashflix() *hashflixResolver { return &hashflixResolver{} }

func (h *hashflixResolver) Key() string { return "hashflix" }

func (h *hashflixResolver) Search(_ context.Context, _ Source, _ string) ([]Candidate, error) {
	return nil, fmt.Errorf("hashflix: pencarian belum didukung")
}

func (h *hashflixResolver) Episodes(_ context.Context, _ Source, _ string) ([]EpisodeRef, error) {
	return nil, nil
}

func (h *hashflixResolver) Resolve(_ context.Context, _ Source, _ EpisodeRef) (Resolved, error) {
	return Resolved{}, fmt.Errorf("hashflix: resolusi aliran belum didukung")
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
