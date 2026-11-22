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
//   - Daftar episode di anichin.my dimuat sisi klien (tidak ada di
//     HTML statis), tapi mirror anichin.moe menyajikannya sebagai
//     eplister statis — Episodes() memakai REST dulu, lalu eplister
//     HTML sebagai cadangan.
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

var (
	tsImgRe  = regexp.MustCompile(`(?is)<img[^>]+class="[^"]*ts-post-image[^"]*"[^>]*>`)
	epTitleRe = regexp.MustCompile(`(?i)\s+Episode\s+\d+.*$`)
)

// attr mengambil nilai atribut dari satu tag HTML ("" bila tidak ada).
// Nama atribut harus didahului spasi agar "src" tidak tertukar dengan
// "data-src".
func attr(tag, name string) string {
	m := regexp.MustCompile(`(?i)\s` + name + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1]
}

// parsePosterMap mengurai halaman daftar Animestream menjadi peta
// poster: tiap kartu episode membawa gambar poster serinya dengan
// title "<Seri> Episode <n> Subtitle Indonesia" — sufiks episode
// dibuang, kunci dinormalisasi, URL relatif dibuat absolut ke mirror.
func parsePosterMap(page, mirrorBase string) map[string]string {
	out := map[string]string{}
	for _, tag := range tsImgRe.FindAllString(page, -1) {
		img := attr(tag, "src")
		if img == "" || strings.HasPrefix(img, "data:") {
			img = attr(tag, "data-src")
		}
		name := attr(tag, "title")
		if name == "" {
			name = attr(tag, "alt")
		}
		name = strings.TrimSuffix(epTitleRe.ReplaceAllString(cleanText(name), ""), " Subtitle Indonesia")
		if img == "" || name == "" {
			continue
		}
		if strings.HasPrefix(img, "/") {
			img = mirrorBase + img
		}
		out[NormalizeTitleKey(name)] = img
	}
	return out
}

// posterMapBase menentukan basis halaman daftar untuk peta poster
// per resolver: Anichin memakai mirror resminya anichin.moe (halaman
// HTML anichin.my tidak bisa diandalkan dari server — WAF), sedangkan
// Kuronime memakai base URL sumbernya sendiri (tema Animestream yang
// sama, kartu episodenya membawa poster seri dengan pola yang sama).
func posterMapBase(key string, src Source) (string, bool) {
	switch key {
	case "anichin":
		return "https://anichin.moe", true
	case "kuronime":
		base := strings.TrimRight(src.BaseURL, "/")
		if base == "" {
			return "", false
		}
		return base, true
	}
	return "", false
}

// Posters menyediakan peta poster massal dari halaman daftar tema
// Animestream: kartu episode di halaman daftar membawa poster serinya
// (parsePosterMap) — halaman beranda + /page/2..10 cukup untuk
// judul-judul aktif.
func (a *animestreamResolver) Posters(ctx context.Context, src Source) (map[string]string, error) {
	mirror, ok := posterMapBase(a.key, src)
	if !ok {
		return nil, fmt.Errorf("peta poster tidak tersedia untuk %s", a.key)
	}
	out := map[string]string{}
	pages := []string{mirror + "/"}
	for p := 2; p <= 10; p++ {
		pages = append(pages, fmt.Sprintf("%s/page/%d/", mirror, p))
	}
	for _, u := range pages {
		page, err := Fetch(ctx, u)
		if err != nil {
			continue
		}
		for k, v := range parsePosterMap(page, mirror) {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("tidak ada poster terbaca dari %s", mirror)
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
// Episodes mengambil daftar episode lewat WP REST (jalur utama);
// bila jalur itu gagal/kosong, jatuh ke daftar episode HTML
// (eplister) di halaman series — mirror anichin.moe menyajikan
// eplister sebagai HTML statis (terverifikasi 2026-11-22 pada
// "Tales of Herding Gods": 108 episode tertaut dari halaman seri),
// tidak seperti anichin.my yang memuatnya sisi klien.
func (a *animestreamResolver) Episodes(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error) {
	// Anichin: eplister HTML (.moe) DIDAHULUKAN — WP REST anichin.my
	// terbukti timeout dari VPS (log worker 2026-11-22), sehingga
	// jalur WP dulu membakar 30 dtk per judul sebelum cadangan
	// sempat dicoba. Resolver Animestream lain tetap WP dulu.
	if a.key == "anichin" {
		if htmlEps, herr := a.episodesHTML(ctx, titleURL); herr == nil && len(htmlEps) > 0 {
			return htmlEps, nil
		}
		return a.episodesWP(ctx, src, titleURL)
	}
	eps, err := a.episodesWP(ctx, src, titleURL)
	if err == nil && len(eps) > 0 {
		return eps, nil
	}
	if htmlEps, herr := a.episodesHTML(ctx, titleURL); herr == nil && len(htmlEps) > 0 {
		return htmlEps, nil
	}
	return eps, err
}

var eplisterRe = regexp.MustCompile(`href="([^"]*-episode-(\d+)[^"]*)"`)

// parseEplister mengurai tautan episode dari HTML halaman series
// tema Animestream. Dua penjaga agar episode seri LAIN dari widget
// "rilisan terbaru" di bilah sisi tidak ikut tertangkap: (1) hanya
// blok eplister yang dibaca (dari penanda "eplister" sampai </ul>
// pertamanya); (2) slug episode harus berawalan slug seri ini —
// situs memakai varian jamak untuk slug episode (seri
// "tales-of-herding-god" → episode "tales-of-herding-gods-episode-…"),
// jadi awalan yang diterima: slug, slug+s, dan slug tanpa s akhir.
func parseEplister(html, pageURL string) []EpisodeRef {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	seg := html
	if i := strings.Index(seg, "eplister"); i >= 0 {
		seg = seg[i:]
		if j := strings.Index(seg, "</ul>"); j >= 0 {
			seg = seg[:j]
		}
	}
	parts := strings.Split(strings.Trim(base.Path, "/"), "/")
	slug := parts[len(parts)-1]
	prefixes := map[string]bool{slug: true, slug + "s": true}
	if strings.HasSuffix(slug, "s") {
		prefixes[strings.TrimSuffix(slug, "s")] = true
	}
	byNo := map[int]EpisodeRef{}
	for _, m := range eplisterRe.FindAllStringSubmatch(seg, -1) {
		ref, err := url.Parse(m[1])
		if err != nil {
			continue
		}
		segPath := strings.Trim(ref.Path, "/")
		ok := false
		for p := range prefixes {
			if strings.HasPrefix(segPath, p+"-episode-") {
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil || n <= 0 {
			continue
		}
		if _, dup := byNo[n]; dup {
			continue
		}
		byNo[n] = EpisodeRef{Number: n, URL: base.ResolveReference(ref).String()}
	}
	out := make([]EpisodeRef, 0, len(byNo))
	for _, e := range byNo {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// mirrorCandidates mengembalikan URL halaman apa adanya plus
// mirror anichin.moe dengan path yang sama (mirror menyajikan HTML
// statis lengkap; domain utama kerap membatasi akses server).
func mirrorCandidates(titleURL string) []string {
	candidates := []string{titleURL}
	if u, err := url.Parse(titleURL); err == nil && u.Host != "anichin.moe" {
		u.Host = "anichin.moe"
		u.Scheme = "https"
		candidates = append(candidates, u.String())
	}
	return candidates
}

// moeFirstCandidates mendahulukan mirror anichin.moe (domain utama
// kerap timeout dari server; mirror menjawab cepat).
func moeFirstCandidates(titleURL string) []string {
	c := mirrorCandidates(titleURL)
	if len(c) == 2 {
		return []string{c[1], c[0]}
	}
	return c
}

// episodesHTML mencoba halaman series (untuk Anichin: mirror
// anichin.moe dulu), lalu kandidat berikutnya.
func (a *animestreamResolver) episodesHTML(ctx context.Context, titleURL string) ([]EpisodeRef, error) {
	candidates := mirrorCandidates(titleURL)
	if a.key == "anichin" {
		candidates = moeFirstCandidates(titleURL)
	}
	var lastErr error
	for _, cand := range candidates {
		page, err := Fetch(ctx, cand)
		if err != nil {
			lastErr = err
			continue
		}
		if eps := parseEplister(page, cand); len(eps) > 0 {
			return eps, nil
		}
	}
	return nil, lastErr
}

var (
	alterRe   = regexp.MustCompile(`(?is)<span class="alter">(.*?)</span>`)
	speSpanRe = regexp.MustCompile(`(?is)<span[^>]*><b>([^<]+):</b>(.*?)</span>`)
	genxedRe  = regexp.MustCompile(`(?is)<div class="genxed">(.*?)</div>`)
	genreARe  = regexp.MustCompile(`(?is)<a[^>]*>([^<]+)</a>`)
	entryRe   = regexp.MustCompile(`(?is)<div class="entry-content"[^>]*>(.*?)</div>`)
	firstH4Re = regexp.MustCompile(`(?is)^\s*<h4>(.*?)</h4>`)
	yearRe    = regexp.MustCompile(`(19|20)\d{2}`)
	durRe     = regexp.MustCompile(`(\d+)\s*min`)
)

// parseSeriesPage mengurai metadata seri dari HTML halaman seri
// Animestream: judul alternatif (span.alter), blok info (div.spe:
// Status/Network/Studio/Tanggal rilis/Durasi/Season/Negara), genre
// (div.genxed), dan sinopsis (div.entry-content — h4 pembuka berisi
// judul berkurung dibuang; teks sesudahnya adalah sinopsis asli,
// sedangkan blok .desc terpisah adalah boilerplate SEO situs).
func parseSeriesPage(html string) *SeriesMeta {
	meta := &SeriesMeta{}
	if m := alterRe.FindStringSubmatch(html); m != nil {
		meta.TitleAlt = cleanText(m[1])
	}
	for _, m := range speSpanRe.FindAllStringSubmatch(html, -1) {
		label := strings.ToLower(strings.TrimSpace(m[1]))
		val := cleanText(m[2])
		switch label {
		case "network":
			meta.Network = val
		case "studio":
			meta.Studio = val
		case "negara":
			meta.Country = val
		case "durasi":
			if dm := durRe.FindStringSubmatch(val); dm != nil {
				meta.DurationMin, _ = strconv.Atoi(dm[1])
			}
		case "tanggal rilis":
			if ym := yearRe.FindString(val); ym != "" {
				meta.Year, _ = strconv.Atoi(ym)
			}
		case "season":
			fields := strings.Fields(val)
			if len(fields) > 0 {
				switch s := strings.ToLower(fields[0]); s {
				case "winter", "spring", "summer", "fall":
					meta.Season = s
				}
			}
			if meta.Year == 0 {
				if ym := yearRe.FindString(val); ym != "" {
					meta.Year, _ = strconv.Atoi(ym)
				}
			}
		}
	}
	if m := genxedRe.FindStringSubmatch(html); m != nil {
		for _, g := range genreARe.FindAllStringSubmatch(m[1], -1) {
			if t := cleanText(g[1]); t != "" {
				meta.Genres = append(meta.Genres, t)
			}
		}
	}
	if m := entryRe.FindStringSubmatch(html); m != nil {
		body := m[1]
		// Buang HANYA h4 pertama bila isinya judul berkurung
		// ("[Tales of Herding Gods]") — sinopsisnya sendiri dibungkus
		// h4 berikutnya oleh tema ini, jadi membuang semua h4 akan
		// menghapus sinopsisnya.
		if fm := firstH4Re.FindStringSubmatch(body); fm != nil && strings.Contains(fm[1], "[") {
			body = firstH4Re.ReplaceAllString(body, " ")
		}
		if syn := cleanText(body); len(syn) > 80 {
			if len(syn) > 2000 {
				syn = syn[:2000]
			}
			meta.Synopsis = syn
		}
	}
	return meta
}

// SeriesInfo membaca metadata seri dari halaman serinya (mirror
// HTML statis dipakai sebagai cadangan seperti daftar episode).
func (a *animestreamResolver) SeriesInfo(ctx context.Context, src Source, titleURL string) (*SeriesMeta, error) {
	candidates := mirrorCandidates(titleURL)
	if a.key == "anichin" {
		candidates = moeFirstCandidates(titleURL)
	}
	var lastErr error
	for _, cand := range candidates {
		page, err := Fetch(ctx, cand)
		if err != nil {
			lastErr = err
			continue
		}
		meta := parseSeriesPage(page)
		if meta.Synopsis != "" || meta.Studio != "" || len(meta.Genres) > 0 {
			return meta, nil
		}
	}
	return nil, lastErr
}

func (a *animestreamResolver) episodesWP(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error) {
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
