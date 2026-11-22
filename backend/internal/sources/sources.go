// Package sources berisi resolver situs sumber konten (F40): untuk tiap
// situs yang terdaftar di registri content_sources, resolver tahu cara
// mencari judul, mengambil daftar episode, dan menyelesaikan aliran video
// + trek subtitle siap pakai dari halaman sumbernya.
//
// Prinsip F40: tidak ada transkripsi dan tidak ada terjemahan di server —
// yang diambil adalah stream & subtitle yang memang sudah disediakan
// situs sumber. Sumber softsub menghasilkan trek subtitle (berkas .vtt/.srt
// di CDN sumber, dirujuk langsung); sumber hardsub menghasilkan aliran
// video per bahasa (subtitle menyatu di gambar).
package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Source adalah satu baris registri content_sources.
type Source struct {
	ID           string
	Name         string
	ContentType  string
	Lang         string // BCP-47; "multi" untuk softsub multi-trek
	BaseURL      string
	Resolver     string
	FetchMode    string // server | device
	SubtitleKind string // soft | hard | none
	Priority     int
}

// Candidate adalah hasil pencarian judul di situs sumber.
type Candidate struct {
	Title string
	URL   string
	Ref   string // id/slug internal situs sumber bila ada
}

// EpisodeRef adalah satu episode pada halaman judul sumber.
type EpisodeRef struct {
	Number int
	Title  string
	URL    string
	Ref    string // id episode internal situs sumber bila ada
}

// Stream adalah aliran video hasil resolusi satu episode.
type Stream struct {
	URL   string
	Label string
	Lang  string // bahasa subtitle aliran ini (hardsub); kosong bila softsub
}

// Track adalah trek subtitle softsub hasil resolusi satu episode.
type Track struct {
	Lang   string
	Format string // vtt | srt | ass
	URL    string // URL berkas di CDN sumber (dirujuk langsung, tidak diunduh server)
}

// Resolved adalah hasil resolusi satu episode.
type Resolved struct {
	Streams []Stream
	Tracks  []Track
}

// Resolver adalah perilaku per situs sumber.
type Resolver interface {
	Key() string
	Search(ctx context.Context, src Source, query string) ([]Candidate, error)
	Episodes(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error)
	Resolve(ctx context.Context, src Source, ep EpisodeRef) (Resolved, error)
}

// ByKey mengembalikan resolver terdaftar untuk kunci registri.
func ByKey(key string) (Resolver, bool) {
	r, ok := registry[key]
	return r, ok
}

var registry = map[string]Resolver{}

func register(r Resolver) { registry[r.Key()] = r }

func init() {
	register(NewKuronime())
	register(NewAnichin())
	register(NewHiAnime())
	register(NewGeneric())
}

// httpClient bersama untuk semua resolver: UA peramban wajar + timeout.
var httpClient = &http.Client{Timeout: 30 * time.Second}

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"

// Fetch mengambil isi URL sebagai teks (HTML/JSON) dengan UA peramban.
func Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "text/html,application/json,*/*")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d dari %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// Health memeriksa apakah base URL sumber menjawab HTTP < 400.
func Health(ctx context.Context, baseURL string) bool {
	if baseURL == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", browserUA)
	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode < 400
}

// LangFromLabel memetakan label bahasa bebas (mis. dari trek HiAnime
// "English", "Spanish", "Bahasa Indonesia") ke kode BCP-47 sederhana.
func LangFromLabel(label string) string {
	l := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(l, "english"), l == "en":
		return "en"
	case strings.Contains(l, "indonesia"), l == "id":
		return "id"
	case strings.Contains(l, "malay"), strings.Contains(l, "melayu"), l == "ms":
		return "ms"
	case strings.Contains(l, "spanish"), strings.Contains(l, "español"), l == "es":
		return "es"
	case strings.Contains(l, "portugu"), l == "pt":
		return "pt"
	case strings.Contains(l, "french"), strings.Contains(l, "français"), l == "fr":
		return "fr"
	case strings.Contains(l, "viet"), l == "vi":
		return "vi"
	case strings.Contains(l, "thai"), l == "th":
		return "th"
	case strings.Contains(l, "arab"), l == "ar":
		return "ar"
	case strings.Contains(l, "chinese"), l == "zh", strings.HasPrefix(l, "zh"):
		return "zh-CN"
	case strings.Contains(l, "korean"), l == "ko":
		return "ko"
	case strings.Contains(l, "japan"), l == "ja":
		return "ja"
	default:
		return l
	}
}
