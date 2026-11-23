package sources

// Telusur genre langsung dari situs sumber (permintaan Rams
// 2026-11-23): halaman genre Kuronime (/genres/<slug>/), Anichin
// (/genres/<slug>/), dan penemuan TMDB Hashflix
// (/database/discover/movie?with_genres=) dibaca apa adanya agar
// pilihan saat memfilter di aplikasi sebanyak pilihan di situsnya.

import (
	"context"
	"fmt"
	"strings"
)

// BrowseGenre Kuronime & Anichin: halaman genre tema yang sama
// dengan arsipnya, jadi pengurai kartunya dipakai ulang persis.
func (a *animestreamResolver) BrowseGenre(ctx context.Context, src Source, genre string, page int) ([]DiscoveredTitle, error) {
	gs := slugify(genre)
	if gs == "" || page < 1 {
		return nil, nil
	}
	switch a.key {
	case "kuronime":
		base := strings.TrimRight(src.BaseURL, "/")
		u := base + "/genres/" + gs + "/"
		if page > 1 {
			u = fmt.Sprintf("%s/genres/%s/page/%d/", base, gs, page)
		}
		pageHTML, err := Fetch(ctx, u)
		if err != nil {
			return nil, err
		}
		return parseKuroArchivePage(pageHTML, base), nil
	case "anichin":
		base := moeBase(src.BaseURL)
		u := base + "/genres/" + gs + "/"
		if page > 1 {
			u = fmt.Sprintf("%s/genres/%s/page/%d/", base, gs, page)
		}
		pageHTML, err := Fetch(ctx, u)
		if err != nil {
			return nil, err
		}
		return parseArchivePage(pageHTML, base), nil
	}
	return nil, nil
}

// tmdbGenreIDs membalik peta genre TMDB (id -> nama) menjadi
// nama kecil -> id untuk parameter with_genres Hashflix.
var tmdbGenreIDs = func() map[string]int {
	out := map[string]int{}
	for id, name := range tmdbMovieGenres {
		out[strings.ToLower(name)] = id
	}
	return out
}()

// BrowseGenre Hashflix: endpoint discover TMDB-nya menerima
// with_genres berisi id genre; genre kosong = semua film.
func (h *hashflixResolver) BrowseGenre(ctx context.Context, src Source, genre string, page int) ([]DiscoveredTitle, error) {
	if page < 1 {
		return nil, nil
	}
	base := strings.TrimRight(src.BaseURL, "/")
	u := fmt.Sprintf("%s/database/discover/movie?page=%d", base, page)
	if strings.TrimSpace(genre) != "" {
		id, ok := tmdbGenreIDs[strings.ToLower(strings.TrimSpace(genre))]
		if !ok {
			return nil, nil // genre tak dikenal TMDB: kosong, bukan error
		}
		u += fmt.Sprintf("&with_genres=%d", id)
	}
	body, err := Fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	items, _ := parseHFTrending(body, base)
	return items, nil
}

// SlugifyTitle mengekspos pembuat slug katalog (dipakai endpoint
// impor judul agar slug film sama dengan hasil penemuan worker:
// slugify(judul)-idTMDB).
func SlugifyTitle(s string) string { return slugify(s) }
