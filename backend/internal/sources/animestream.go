package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// animestreamResolver menarget tema WordPress "Animestream" (dipakai
// Anichin, Kuronime, dan banyak situs fansub lain). Tiga fakta tema yang
// terverifikasi hidup di anichin.my (2026-11-22):
//   - WP REST API terbuka: pencarian judul lewat /wp-json/wp/v2/search
//     (halaman series berawalan /series/); episode adalah post biasa
//     berslug "<slug-series>-episode-<n>-subtitle-indonesia" yang semuanya
//     berkategori series-nya, jadi daftar episode diambil lewat
//     /wp-json/wp/v2/posts?categories=<id> (parameter search pada /posts
//     dinonaktifkan situs ini — selalu mengembalikan kosong).
//   - Daftar episode TIDAK ada di HTML statis halaman series (dimuat
//     sisi klien), jadi jalur REST adalah satu-satunya jalur server.
//   - Halaman episode menyediakan <select class="mirror"> berisi opsi
//     server sebagai HTML embed ter-base64, plus satu iframe bawaan.
type animestreamResolver struct{ key string }

func NewAnichin() Resolver  { return &animestreamResolver{key: "anichin"} }
func NewKuronime() Resolver { return &animestreamResolver{key: "kuronime"} }

func (a *animestreamResolver) Key() string { return a.key }

type wpSearchItem struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type wpPost struct {
	Slug string `json:"slug"`
	Link string `json:"link"`
	Titl struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
}

func (a *animestreamResolver) Search(ctx context.Context, src Source, query string) ([]Candidate, error) {
	base := strings.TrimRight(src.BaseURL, "/")
	u := base + "/wp-json/wp/v2/search?per_page=25&search=" + url.QueryEscape(query)
	raw, err := Fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	var items []wpSearchItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("hasil pencarian WP tidak terbaca: %w", err)
	}
	out := []Candidate{}
	for _, it := range items {
		if !strings.Contains(it.URL, "/series/") {
			continue // hanya halaman series; post episode dilewati
		}
		out = append(out, Candidate{Title: cleanText(it.Title), URL: it.URL})
		if len(out) >= 10 {
			break
		}
	}
	return out, nil
}

type wpCategoryFull struct {
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Count int    `json:"count"`
}

// Discover menemukan seri dari indeks kategori WordPress: di tema
// Animestream tiap seri adalah satu kategori (slug kategori = slug seri,
// terbukti di anichin.my: 573 kategori seri) dan semua post episodenya
// berkategori itu — jadi daftar kategori terurut jumlah post terbanyak
// adalah indeks seri situs ini. URL seri dibangun sebagai
// <base>/series/<slug>/ sesuai pola permalink tema.
func (a *animestreamResolver) Discover(ctx context.Context, src Source, limit int) ([]DiscoveredTitle, error) {
	base := strings.TrimRight(src.BaseURL, "/")
	out := []DiscoveredTitle{}
	for page := 1; page <= 20 && len(out) < limit; page++ {
		u := fmt.Sprintf("%s/wp-json/wp/v2/categories?per_page=100&page=%d&orderby=count&order=desc&_fields=name,slug,count",
			base, page)
		raw, err := Fetch(ctx, u)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		var cats []wpCategoryFull
		if err := json.Unmarshal([]byte(raw), &cats); err != nil {
			return nil, fmt.Errorf("daftar kategori WP tidak terbaca: %w", err)
		}
		for _, c := range cats {
			if c.Count <= 0 || c.Slug == "" {
				continue
			}
			name := strings.TrimSuffix(cleanText(c.Name), " Subtitle Indonesia")
			if name == "" {
				continue
			}
			out = append(out, DiscoveredTitle{
				Title: name,
				Slug:  c.Slug,
				URL:   base + "/series/" + c.Slug + "/",
			})
			if len(out) >= limit {
				break
			}
		}
		if len(cats) < 100 {
			break
		}
	}
	return out, nil
}

var epSlugRe = regexp.MustCompile(`(?i)-episode-(\d+)`)

type wpCategory struct {
	ID int `json:"id"`
}

// Episodes menemukan episode lewat kategori WordPress: di tema ini tiap
// series adalah satu kategori (slug kategori = slug series) dan semua
// post episodenya berkategori itu. Parameter search pada /posts dinonaktifkan
// situs (selalu kosong), jadi jalur kategori adalah jalur yang andal.
func (a *animestreamResolver) Episodes(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error) {
	base := strings.TrimRight(src.BaseURL, "/")
	u, err := url.Parse(titleURL)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	slug := parts[len(parts)-1]

	raw, err := Fetch(ctx, base+"/wp-json/wp/v2/categories?slug="+url.QueryEscape(slug)+"&_fields=id")
	if err != nil {
		return nil, err
	}
	var cats []wpCategory
	if err := json.Unmarshal([]byte(raw), &cats); err != nil || len(cats) == 0 {
		if err == nil {
			err = fmt.Errorf("kategori series %s tidak ditemukan", slug)
		}
		return nil, err
	}
	catID := cats[0].ID

	byNo := map[int]EpisodeRef{}
	var single *EpisodeRef // cadangan film: satu post tanpa nomor episode
	for page := 1; page <= 20; page++ {
		pu := fmt.Sprintf("%s/wp-json/wp/v2/posts?categories=%d&per_page=100&page=%d&_fields=slug,link,title",
			base, catID, page)
		raw, err := Fetch(ctx, pu)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		var posts []wpPost
		if err := json.Unmarshal([]byte(raw), &posts); err != nil {
			return nil, fmt.Errorf("daftar post WP tidak terbaca: %w", err)
		}
		for _, p := range posts {
			m := epSlugRe.FindStringSubmatch(p.Slug)
			if m == nil {
				// Film di tema ini adalah satu post berslug
				// "<slug-series>-subtitle-indonesia" tanpa nomor episode.
				if single == nil && strings.HasPrefix(strings.ToLower(p.Slug), strings.ToLower(slug)+"-") {
					single = &EpisodeRef{Number: 1, Title: cleanText(p.Titl.Rendered), URL: p.Link}
				}
				continue
			}
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if _, dup := byNo[n]; !dup {
				byNo[n] = EpisodeRef{Number: n, Title: cleanText(p.Titl.Rendered), URL: p.Link}
			}
		}
		if len(posts) < 100 {
			break
		}
	}
	if len(byNo) == 0 && single != nil {
		return []EpisodeRef{*single}, nil
	}
	out := make([]EpisodeRef, 0, len(byNo))
	for _, e := range byNo {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

var (
	mirrorOptRe     = regexp.MustCompile(`(?is)<option\s+value="([A-Za-z0-9+/=\s]{40,})"[^>]*>(.*?)</option>`)
	iframeSrcRe     = regexp.MustCompile(`(?is)src="([^"]+)"`)
	labelBracketRe  = regexp.MustCompile(`\s*\[[^\]]*\]\s*$`)
)

func (a *animestreamResolver) Resolve(ctx context.Context, src Source, ep EpisodeRef) (Resolved, error) {
	page, err := Fetch(ctx, ep.URL)
	if err != nil {
		return Resolved{}, err
	}
	res := Resolved{}
	seen := map[string]bool{}
	add := func(raw, label string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			return
		}
		if strings.HasPrefix(raw, "//") {
			raw = "https:" + raw
		}
		seen[raw] = true
		if label == "" {
			label = raw
			if u, err := url.Parse(raw); err == nil {
				host := strings.TrimPrefix(u.Host, "www.")
				if l, ok := hostLabels[host]; ok {
					label = l
				} else {
					label = host
				}
			}
		}
		res.Streams = append(res.Streams, Stream{URL: raw, Label: label, Lang: src.Lang})
	}
	// Server-server dari <select class="mirror">: nilai opsi adalah HTML
	// embed ter-base64; teks opsi adalah nama servernya.
	for _, m := range mirrorOptRe.FindAllStringSubmatch(page, -1) {
		enc := strings.Join(strings.Fields(m[1]), "")
		decoded, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			continue
		}
		if sm := iframeSrcRe.FindSubmatch(decoded); sm != nil {
			label := labelBracketRe.ReplaceAllString(cleanText(m[2]), "")
			add(string(sm[1]), label)
		}
	}
	// Iframe bawaan yang langsung tampil di pemutar.
	for _, m := range iframeRe.FindAllStringSubmatch(page, -1) {
		add(m[1], "")
	}
	if len(res.Streams) == 0 {
		return res, fmt.Errorf("tidak ada embed video ditemukan di %s", ep.URL)
	}
	return res, nil
}
