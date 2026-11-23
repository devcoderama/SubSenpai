package sources

// Pencarian judul langsung di halaman pencarian situs sumber
// (permintaan Rams 2026-11-23): bila kata yang dicari pengguna
// belum ada di katalog, backend mengais halaman pencarian sumber
// (Kuronime /?s=, Anichin /?s=, Hashflix /database/search/movie)
// dan menyimpan hasilnya ke katalog — pencarian kedua untuk kata
// yang sama langsung tersedia dari database.
//
// Jalur lama resolver.Search (wp-json) tidak dipakai di sini:
// wp-json Anichin mati (Cloudflare 403) dan saringan "/series/"-nya
// tidak cocok dengan URL seri Kuronime (/anime/<slug>/), sedangkan
// Search Hashflix memang belum ada. Halaman pencarian HTML/JSON
// sumber memakai pengurai kartu yang sama dengan arsip & genre.

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// maxSearchTitles membatasi hasil pencarian live yang disimpan per
// kata: hasil teratas halaman pencarian adalah yang paling relevan
// (sisanya, termasuk kartu sampingan "Top Anime", tidak perlu ikut
// memenuhi katalog).
const maxSearchTitles = 15

func capSearchTitles(items []DiscoveredTitle) []DiscoveredTitle {
	if len(items) > maxSearchTitles {
		return items[:maxSearchTitles]
	}
	return items
}

// SearchTitles Kuronime & Anichin: halaman hasil pencarian tema
// yang sama dengan arsipnya (/?s=<kata>), jadi pengurai kartunya
// dipakai ulang persis.
func (a *animestreamResolver) SearchTitles(ctx context.Context, src Source, query string) ([]DiscoveredTitle, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	switch a.key {
	case "kuronime":
		base := strings.TrimRight(src.BaseURL, "/")
		body, err := Fetch(ctx, base+"/?s="+url.QueryEscape(q))
		if err != nil {
			return nil, err
		}
		return capSearchTitles(parseKuroArchivePage(body, base)), nil
	case "anichin":
		base := moeBase(src.BaseURL)
		body, err := Fetch(ctx, base+"/?s="+url.QueryEscape(q))
		if err != nil {
			return nil, err
		}
		return capSearchTitles(parseArchivePage(body, base)), nil
	}
	return nil, nil
}

// SearchTitles Hashflix: endpoint pencarian TMDB-nya
// (/database/search/movie?query=) — bentuk JSON-nya sama persis
// dengan trending, jadi pengurainya dipakai ulang.
func (h *hashflixResolver) SearchTitles(ctx context.Context, src Source, query string) ([]DiscoveredTitle, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	base := strings.TrimRight(src.BaseURL, "/")
	body, err := Fetch(ctx, fmt.Sprintf("%s/database/search/movie?query=%s&page=1", base, url.QueryEscape(q)))
	if err != nil {
		return nil, err
	}
	items, _ := parseHFTrending(body, base)
	return capSearchTitles(items), nil
}
