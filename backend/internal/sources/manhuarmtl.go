package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// manhuarmtlResolver menarget manhuarmtl.com — sumber baca (F39) yang
// halaman HTML-nya dilindungi Cloudflare, TETAPI menyediakan API JSON
// publiknya sendiri di /wp-json/manhuarmtl/v1 (API yang sama yang
// dipakai aplikasi resmi situs itu). API ini lolos dari server karena
// memang dibuka untuk klien aplikasi:
//
//   - GET /wp-json/manhuarmtl/v1/latest?page=N — umpan seri terbaru
//     (24/halaman, ±400 seri): judul, URL seri, poster (thumbnail),
//     type (manga|manhua|manhwa), status, tahun, genre.
//   - GET /wp-json/manhuarmtl/v1/manga?url=<seri> — detail penuh
//     termasuk daftar chapter (tidak dipakai di sini; chapter tetap
//     diselesaikan di perangkat sesuai F39).
//
// Resolver ini HANYA mendukung penemuan katalog (Discover): judul masuk
// katalog sebagai judul mode perangkat (fetch_mode 'device'), dan
// daftar chapter + halaman dibaca aplikasi langsung dari situsnya.
type manhuarmtlResolver struct{}

func NewManhuarmtl() Resolver { return &manhuarmtlResolver{} }

func (m *manhuarmtlResolver) Key() string { return "manhuarmtl" }

func (m *manhuarmtlResolver) Search(ctx context.Context, src Source, query string) ([]Candidate, error) {
	return nil, fmt.Errorf("manhuarmtl adalah sumber perangkat — pencarian server tidak dipakai")
}

func (m *manhuarmtlResolver) Episodes(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error) {
	return nil, fmt.Errorf("manhuarmtl adalah sumber perangkat — chapter diselesaikan di aplikasi (F39)")
}

func (m *manhuarmtlResolver) Resolve(ctx context.Context, src Source, ep EpisodeRef) (Resolved, error) {
	return Resolved{}, fmt.Errorf("manhuarmtl adalah sumber perangkat — resolusi di aplikasi (F39)")
}

type mtLatestItem struct {
	Title     string   `json:"title"`
	MangaURL  string   `json:"mangaUrl"`
	Thumbnail string   `json:"thumbnail"`
	Type      string   `json:"type"`
	Status    string   `json:"status"`
	Year      int      `json:"year"`
	Genres    []string `json:"genreSlugs"`
}

type mtLatestResp struct {
	Items      []mtLatestItem `json:"items"`
	TotalPages int            `json:"total_pages"`
}

var comicTypes = map[string]bool{"manga": true, "manhua": true, "manhwa": true}

// parseMTLatest mengubah satu halaman respons /latest menjadi judul
// katalog. Item tanpa type komik yang dikenal dilewati (umpan situs
// sesekali memuat type kosong).
func parseMTLatest(raw string) ([]DiscoveredTitle, int, error) {
	var resp mtLatestResp
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		return nil, 0, fmt.Errorf("respons latest manhuarmtl tidak terbaca: %w", err)
	}
	out := []DiscoveredTitle{}
	for _, it := range resp.Items {
		if !comicTypes[it.Type] || it.MangaURL == "" || it.Title == "" {
			continue
		}
		slug := strings.TrimSuffix(it.MangaURL, "/")
		if i := strings.LastIndex(slug, "/"); i >= 0 {
			slug = slug[i+1:]
		}
		if slug == "" {
			continue
		}
		genres := make([]string, 0, len(it.Genres))
		for _, g := range it.Genres {
			g = strings.TrimSpace(strings.ReplaceAll(g, "-", " "))
			if g != "" {
				genres = append(genres, strings.ToUpper(g[:1])+g[1:])
			}
		}
		out = append(out, DiscoveredTitle{
			Title:     cleanText(it.Title),
			Slug:      slug,
			URL:       it.MangaURL,
			Type:      it.Type,
			Poster:    it.Thumbnail,
			Year:      it.Year,
			Genres:    genres,
			Completed: it.Status == "end" || it.Status == "completed",
		})
	}
	return out, resp.TotalPages, nil
}

// Discover menelusuri umpan /latest situs (24 seri per halaman) sampai
// batas judul atau halaman terakhir.
func (m *manhuarmtlResolver) Discover(ctx context.Context, src Source, limit int) ([]DiscoveredTitle, error) {
	base := strings.TrimRight(src.BaseURL, "/")
	out := []DiscoveredTitle{}
	for page := 1; page <= 30 && len(out) < limit; page++ {
		raw, err := Fetch(ctx, fmt.Sprintf("%s/wp-json/manhuarmtl/v1/latest?page=%d", base, page))
		if err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		items, totalPages, err := parseMTLatest(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if page >= totalPages || len(items) == 0 {
			break
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
