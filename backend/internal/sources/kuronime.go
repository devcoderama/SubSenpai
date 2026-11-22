package sources

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Kuronime (kuronime.sbs) memakai tema "animenonton" — sepupu tema
// Animestream, tetapi markup-nya berbeda di semua titik penting
// (terverifikasi hidup 2026-11-22 dari halaman aslinya):
//   - Arsip seri: <base>/anime/ dengan paginasi PATH
//     /anime/page/2/, /anime/page/3/, … (84 halaman, ±30 kartu per
//     halaman). Kartu <article class="bs"> menaut ke /anime/<slug>/;
//     posternya <img itemprop="image"> yang atribut src-nya TAK
//     berpetik di HTML mentah dan memakai CDN Photon (i*.wp.com)
//     dengan ekor "?resize=…" yang harus dibuang; judul di .tt h2;
//     lencana tipe di span.type; rating di .tt .rating i.
//   - Halaman seri: satu <article class="hentry"> — judul di
//     h1.entry-title; info sebagai label teks (Judul: / Genre: /
//     Status: / Studio: / Tayang: / Season: / Tipe: / Durasi:);
//     sinopsis di <h3> tepat sesudah <h2>Sinopsis …</h2> (diakhiri
//     sufiks " Subtitle Indonesia"); daftar episode sesudah
//     <h4>Episode …</h4> sebagai tautan /nonton-<slug>-episode-N/
//     yang slug-nya bisa berbeda dari slug seri (seri
//     "one-piece-op" → episode "one-piece-episode-1180").
//   - Halaman episode: TANPA <select class="mirror"> — pemutarnya
//     satu <iframe id="iframedc"> yang URL embed-nya ada di
//     data-src (player.animeku.org, token terenkripsi), src-nya
//     kosong. Iframe lain di halaman itu (chat cbox.ws) bukan
//     pemutar dan harus disingkirkan.
//
// Seperti pembangunan ulang donghua (anichin.moe), semua jalur
// Kuronime kini HTML-first; jalur WP REST lama tetap menjadi
// cadangan di pemanggil (Discover/Episodes/SeriesInfo/Resolve)
// karena wp-json kuronime.sbs masih hidup.

var (
	kuroCardRe   = regexp.MustCompile(`(?is)<article\s[^>]*class="bs(?:\s[^"]*)?"[^>]*>(.*?)</article>`)
	kuroCardEpRe = regexp.MustCompile(`(?is)<article\s[^>]*class="bsu(?:\s[^"]*)?"[^>]*>(.*?)</article>`)
	kuroTTRe     = regexp.MustCompile(`(?is)<div class="tt">(.*?)</div>\s*</a>`)
	kuroH2Re     = regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`)
	kuroImgTagRe = regexp.MustCompile(`(?is)<img\s[^>]*>`)
)

// attrAny mengambil nilai atribut dari satu tag HTML, menerima
// bentuk berpetik (name="…") maupun tak berpetik (name=https://…
// yang nilainya berakhir di spasi atau ">") — HTML mentah
// Kuronime memakai bentuk tak berpetik untuk poster kartu arsip.
func attrAny(tag, name string) string {
	if v := attr(tag, name); v != "" {
		return v
	}
	m := regexp.MustCompile(`(?i)\s` + name + `=([^\s>]+)`).FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return strings.Trim(m[1], `"'`)
}

// cleanKuroPosterURL membersihkan URL poster mentah Kuronime:
// ekor Photon "?resize=…" dibuang (URL dasarnya tetap melayani
// gambar utuh), lalu URL relatif dibuat absolut ke base.
func cleanKuroPosterURL(raw, base string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "data:") {
		return ""
	}
	if i := strings.Index(raw, "?resize"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimSuffix(raw, "?")
	baseURL, err := url.Parse(strings.TrimRight(base, "/") + "/")
	if err != nil {
		return raw
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return baseURL.ResolveReference(ref).String()
}

// kuroCardPoster mengambil URL poster dari potongan kartu: img
// pertama yang bukan ikon pemutar (ikon controls-play.svg dan
// dashicons disingkirkan); src data:/kosong → data-src.
func kuroCardPoster(card, base string) string {
	for _, tag := range kuroImgTagRe.FindAllString(card, -1) {
		if strings.Contains(tag, "controls-play") || strings.Contains(tag, "dashicons") {
			continue
		}
		img := attrAny(tag, "src")
		if img == "" || strings.HasPrefix(img, "data:") {
			img = attrAny(tag, "data-src")
		}
		if img == "" || strings.HasPrefix(img, "data:") {
			continue
		}
		if p := cleanKuroPosterURL(img, base); p != "" {
			return p
		}
	}
	return ""
}

// parseKuroArchivePage mengurai satu halaman arsip seri Kuronime
// (/anime/ dan /anime/page/N/): tiap kartu <article class="bs">
// menaut ke /anime/<slug>/ dan membawa poster + judul (.tt h2).
// Status tamat dibaca dari penanda div.status / span.epx bila ada
// (seperti kartu Animestream); kartu Kuronime umumnya tidak
// membawanya, jadi Completed dibiarkan false (status rinci seri
// diisi jalur lain).
func parseKuroArchivePage(page, base string) []DiscoveredTitle {
	base = strings.TrimRight(base, "/")
	out := []DiscoveredTitle{}
	seen := map[string]bool{}
	for _, m := range kuroCardRe.FindAllStringSubmatch(page, -1) {
		card := m[1]
		am := archiveAnchorRe.FindStringSubmatch(card)
		if am == nil {
			continue
		}
		ref, err := url.Parse(am[2])
		if err != nil {
			continue
		}
		parts := strings.Split(strings.Trim(ref.Path, "/"), "/")
		if len(parts) != 2 || parts[0] != "anime" {
			continue // hanya tautan seri /anime/<slug>/
		}
		slug := parts[1]
		if slug == "" || seen[slug] {
			continue
		}
		var title string
		if hm := kuroH2Re.FindStringSubmatch(card); hm != nil {
			title = cleanText(hm[1])
		}
		if title == "" {
			title = cleanText(attr(am[1], "title"))
		}
		title = strings.TrimSuffix(title, " Subtitle Indonesia")
		if title == "" {
			continue
		}
		seen[slug] = true
		d := DiscoveredTitle{
			Title:  title,
			Slug:   slug,
			URL:    base + "/anime/" + slug + "/",
			Poster: kuroCardPoster(card, base),
		}
		if sm := archiveStatusRe.FindStringSubmatch(card); sm != nil &&
			(strings.Contains(sm[1], "Completed") || strings.EqualFold(cleanText(sm[2]), "Completed")) {
			d.Completed = true
		}
		if em := archiveEpxRe.FindStringSubmatch(card); em != nil &&
			strings.EqualFold(cleanText(em[1]), "Completed") {
			d.Completed = true
		}
		out = append(out, d)
	}
	return out
}

// discoverKuroArchive adalah discovery Kuronime: scraping arsip
// seri HTML — <base>/anime/ lalu <base>/anime/page/2/, 3, …
// (paginasi path terverifikasi persis dari tautan halamannya;
// 84 halaman saat dibangun), berhenti bila halaman memberi 0
// kartu, halaman hanya berisi kartu yang sudah terlihat, limit
// pemanggil tercapai, atau 90 halaman. Meniru discoverArchive
// milik Anichin (donghua) dengan bentuk paginasi situs ini.
func (a *animestreamResolver) discoverKuroArchive(ctx context.Context, src Source, limit int) ([]DiscoveredTitle, error) {
	base := strings.TrimRight(src.BaseURL, "/")
	out := []DiscoveredTitle{}
	seen := map[string]bool{}
	for page := 1; page <= 90 && len(out) < limit; page++ {
		u := base + "/anime/"
		if page > 1 {
			u = fmt.Sprintf("%s/anime/page/%d/", base, page)
		}
		pageHTML, err := Fetch(ctx, u)
		if err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		cards := parseKuroArchivePage(pageHTML, base)
		if len(cards) == 0 {
			break
		}
		added := 0
		for _, d := range cards {
			if seen[d.Slug] {
				continue
			}
			seen[d.Slug] = true
			out = append(out, d)
			added++
			if len(out) >= limit {
				break
			}
		}
		if added == 0 {
			break
		}
	}
	return out, nil
}

var (
	kuroBlockEndRe = regexp.MustCompile(`(?i)</(div|li|tr|td|span|p|h[1-6])>|<br[^>]*>`)
	kuroGenreARe   = regexp.MustCompile(`(?is)<a\s[^>]*href="[^"]*/genres/[^"]*"[^>]*>([^<]+)</a>`)
	kuroArticleRe  = regexp.MustCompile(`(?is)<article[^>]*>(.*)</article>`)
)

// kuroArticle memotong HTML ke isi <article> halaman (seri maupun
// episode) agar widget bilah sisi (rilisan terbaru seri LAIN) tidak
// ikut terurai. Tanpa article, HTML apa adanya dikembalikan.
func kuroArticle(html string) string {
	if m := kuroArticleRe.FindStringSubmatch(html); m != nil {
		return m[1]
	}
	return html
}

// kuroInfoFields membaca blok info seri Kuronime yang berbentuk
// label teks ("Judul: …", "Genre: …", "Studio: …", dst.) di dalam
// article: penutup elemen blok diganti pemisah "|" sebelum tag
// dibuang, lalu tiap potongan "Label: nilai" dicatat (kemunculan
// pertama yang menang). Nilai dibatasi 200 aksara agar teks
// sinopsis yang mengandung titik dua tidak tertangkap sebagai
// nilai label.
func kuroInfoFields(articleHTML string) map[string]string {
	s := kuroBlockEndRe.ReplaceAllString(articleHTML, "|")
	s = cleanText(s)
	fields := map[string]string{}
	for _, part := range strings.Split(s, "|") {
		part = strings.TrimSpace(part)
		i := strings.Index(part, ":")
		if i <= 0 || i > 20 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(part[:i]))
		val := strings.TrimSpace(part[i+1:])
		if val == "" || len(val) > 200 {
			continue
		}
		if _, dup := fields[label]; !dup {
			fields[label] = val
		}
	}
	return fields
}

// parseKuroSeriesPage mengurai metadata seri dari HTML halaman
// seri Kuronime (di dalam <article>): judul alternatif (label
// "Judul"), genre (tautan /genres/ di article, cadangan label
// "Genre" yang dipisah koma), studio, season + tahun (label
// "Season": "Fall 1999"), tahun cadangan dari label "Tayang"
// ("Oct 20, 1999 to ?"), durasi (label "Durasi"), dan sinopsis
// (<h3> sesudah <h2>Sinopsis …</h2>, sufiks " Subtitle Indonesia"
// dibuang, boilerplate iklan ditolak seperti jalur donghua).
func parseKuroSeriesPage(html string) *SeriesMeta {
	article := kuroArticle(html)
	meta := &SeriesMeta{}
	fields := kuroInfoFields(article)
	if v := fields["judul"]; v != "" {
		meta.TitleAlt = v
	}
	for _, m := range kuroGenreARe.FindAllStringSubmatch(article, -1) {
		if t := cleanText(m[1]); t != "" {
			meta.Genres = append(meta.Genres, t)
		}
	}
	if len(meta.Genres) == 0 {
		for _, g := range strings.Split(fields["genre"], ",") {
			if t := strings.TrimSpace(g); t != "" {
				meta.Genres = append(meta.Genres, t)
			}
		}
	}
	meta.Studio = fields["studio"]
	if v := fields["season"]; v != "" {
		parts := strings.Fields(v)
		if len(parts) > 0 {
			switch s := strings.ToLower(parts[0]); s {
			case "winter", "spring", "summer", "fall":
				meta.Season = s
			}
		}
		if ym := yearRe.FindString(v); ym != "" {
			meta.Year, _ = strconv.Atoi(ym)
		}
	}
	if meta.Year == 0 {
		if ym := yearRe.FindString(fields["tayang"]); ym != "" {
			meta.Year, _ = strconv.Atoi(ym)
		}
	}
	meta.DurationMin = parseDurationMin(fields["durasi"])
	meta.Synopsis = kuroSynopsis(article)
	return meta
}

var (
	kuroSynopsisHeadRe = regexp.MustCompile(`(?is)<h2[^>]*>\s*Sinopsis[^<]*</h2>`)
	kuroH3Re           = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>`)
	kuroPRe            = regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	// Ekor sisipan kata kunci yang ditempel situs sesudah sinopsis
	// asli, mis. "… Fate Apocrypha Subtitle Indonesia,
	// Fate/Apocrypha Subtitle Indonesia" di akhir teks <h3>.
	kuroKeywordTailRe = regexp.MustCompile(`(?i)[\s,]+[^\s,][^.,]*?subtitle indonesia\s*$`)
)

// kuroSynopsis mengambil teks sinopsis dari blok sesudah judul
// <h2>Sinopsis …</h2>: elemen teks pertama (<h3>, cadangan <p>)
// dalam jendela sesudahnya — di antara judul dan isi boleh ada
// markup lain, jadi pola lama yang menuntut <h3> menempel persis
// sesudah </h2> diganti pencarian berjendela ini. Ekor kata kunci
// "Subtitle Indonesia" dibuang berulang; hasil harus lolos uji
// boilerplate dan panjang minimum seperti sebelumnya.
func kuroSynopsis(article string) string {
	loc := kuroSynopsisHeadRe.FindStringIndex(article)
	if loc == nil {
		return ""
	}
	window := article[loc[1]:]
	if len(window) > 12000 {
		window = window[:12000]
	}
	var raw string
	if m := kuroH3Re.FindStringSubmatch(window); m != nil {
		raw = m[1]
	} else if m := kuroPRe.FindStringSubmatch(window); m != nil {
		raw = m[1]
	} else {
		return ""
	}
	syn := cleanText(raw)
	for {
		trimmed := kuroKeywordTailRe.ReplaceAllString(syn, "")
		if trimmed == syn {
			break
		}
		syn = trimmed
	}
	syn = strings.TrimSuffix(syn, " Subtitle Indonesia")
	syn = strings.TrimSuffix(syn, " Sub Indo")
	syn = strings.TrimSpace(syn)
	if len(syn) <= 80 || IsBoilerplateSynopsis(syn) {
		return ""
	}
	if len(syn) > 2000 {
		syn = syn[:2000]
	}
	return syn
}

// seriesInfoKuro adalah SeriesInfo Kuronime: ambil halaman seri
// (URL tersimpan lama berbentuk /series/<slug>/ dialihkan situs ke
// /anime/<slug>/ dan klien HTTP mengikutinya), urai dengan
// parseKuroSeriesPage. Hasil urai kosong total = error deskriptif
// agar kegagalannya tercatat, bukan senyap.
func (a *animestreamResolver) seriesInfoKuro(ctx context.Context, titleURL string) (*SeriesMeta, error) {
	page, err := Fetch(ctx, titleURL)
	if err != nil {
		return nil, err
	}
	meta := parseKuroSeriesPage(page)
	if seriesMetaScore(meta) == 0 {
		return nil, fmt.Errorf("SeriesInfo kuronime: tidak ada metadata terbaca dari %s", titleURL)
	}
	return meta, nil
}

var (
	// href dalam segala bentuk petik (tema ini terbukti memakai
	// atribut tanpa petik di tempat lain).
	kuroHrefRe = regexp.MustCompile(`(?i)href\s*=\s*(?:"([^"]+)"|'([^']+)'|([^\s>]+))`)
	// Path episode: /nonton-<slug>-episode-<nomor>/
	kuroEpPathRe = regexp.MustCompile(`^/nonton-(.+)-episode-(\d+)/?$`)
)

// kuroNormSlug menormalkan slug untuk pencocokan seri↔episode:
// huruf kecil, tanpa tanda hubung/garis bawah ("fateapocrypha"
// seri ↔ "fate-apocrypha" episode harus dianggap sama).
func kuroNormSlug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

// kuroSlugMatch: slug episode cocok dengan slug seri bila sesudah
// normalisasi keduanya sama, atau yang satu awalan yang lain
// (seri "one-piece-op" ↔ episode "one-piece").
func kuroSlugMatch(seriesSlug, epSlug string) bool {
	a, b := kuroNormSlug(seriesSlug), kuroNormSlug(epSlug)
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

// parseKuroEpisodes mengurai daftar episode dari HTML halaman
// seri Kuronime. Pelajaran dari halaman nyata (Fate/Apocrypha,
// terverifikasi 2026-11-22): daftar episode TIDAK selalu berada
// di dalam <article> dan slug episode bisa berbeda dari slug
// seri, jadi pola lama (wilayah article + href berpetik dua)
// menghasilkan 0 episode. Pola baru: kumpulkan SEMUA tautan
// /nonton-<slug>-episode-N/ dari seluruh halaman (href segala
// bentuk petik), kelompokkan per slug episode, lalu pilih
// kelompok terbesar yang slug-nya cocok dengan slug seri —
// episode seri lain dari widget bilah sisi tersaring oleh
// pengelompokan itu (jebakan yang sama seperti eplister
// donghua). Tiap episode tertaut dua kali ("Episode N" dan
// "Nonton", href sama) — dedupe per nomor.
func parseKuroEpisodes(html, pageURL string) []EpisodeRef {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	seriesSlug := ""
	if segs := strings.Split(strings.Trim(base.Path, "/"), "/"); len(segs) > 0 {
		seriesSlug = segs[len(segs)-1]
	}
	groups := map[string]map[int]EpisodeRef{}
	for _, m := range kuroHrefRe.FindAllStringSubmatch(html, -1) {
		href := m[1]
		if href == "" {
			href = m[2]
		}
		if href == "" {
			href = m[3]
		}
		if href == "" {
			continue
		}
		ref, err := url.Parse(href)
		if err != nil {
			continue
		}
		pm := kuroEpPathRe.FindStringSubmatch(ref.Path)
		if pm == nil {
			continue
		}
		n, err := strconv.Atoi(pm[2])
		if err != nil || n <= 0 {
			continue
		}
		slug := pm[1]
		g, ok := groups[slug]
		if !ok {
			g = map[int]EpisodeRef{}
			groups[slug] = g
		}
		if _, dup := g[n]; !dup {
			g[n] = EpisodeRef{
				Number: n,
				Title:  fmt.Sprintf("Episode %d", n),
				URL:    base.ResolveReference(ref).String(),
			}
		}
	}
	var best map[int]EpisodeRef
	for slug, g := range groups {
		if kuroSlugMatch(seriesSlug, slug) && (best == nil || len(g) > len(best)) {
			best = g
		}
	}
	if best == nil {
		return nil
	}
	out := make([]EpisodeRef, 0, len(best))
	for _, e := range best {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// episodesKuroHTML mengambil daftar episode dari HTML halaman
// seri Kuronime (jalur utama baru; WP REST menjadi cadangan
// pemanggil).
func (a *animestreamResolver) episodesKuroHTML(ctx context.Context, titleURL string) ([]EpisodeRef, error) {
	page, err := Fetch(ctx, titleURL)
	if err != nil {
		return nil, err
	}
	eps := parseKuroEpisodes(page, titleURL)
	if len(eps) == 0 {
		return nil, fmt.Errorf("tidak ada tautan episode terbaca dari %s", titleURL)
	}
	return eps, nil
}

var kuroIframeRe = regexp.MustCompile(`(?is)<iframe[^>]*id="iframedc"[^>]*>`)

// parseKuroEmbed mengambil URL pemutar dari HTML halaman episode
// Kuronime: iframe#iframedc, URL embed di data-src (src kosong).
// String kosong bila iframe itu tidak ada.
func parseKuroEmbed(html string) string {
	tag := kuroIframeRe.FindString(html)
	if tag == "" {
		return ""
	}
	u := attrAny(tag, "data-src")
	if u == "" {
		u = attrAny(tag, "src")
	}
	u = strings.TrimSpace(u)
	if strings.HasPrefix(u, "//") {
		u = "https:" + u
	}
	return u
}

// resolveKuro adalah Resolve Kuronime: pemutar tunggal
// player.animeku.org dari iframe#iframedc (label "Animeku").
// Bila iframe itu tidak ditemukan, halaman yang sama diurai
// dengan pengurai mirror/iframe umum (resolveMirrorPage) sebagai
// cadangan — iframe chat cbox.ws disingkirkan dari hasil umum.
func (a *animestreamResolver) resolveKuro(ctx context.Context, src Source, ep EpisodeRef) (Resolved, error) {
	page, err := Fetch(ctx, ep.URL)
	if err != nil {
		return Resolved{}, err
	}
	if u := parseKuroEmbed(page); u != "" {
		return Resolved{Streams: []Stream{{URL: u, Label: "Animeku", Lang: src.Lang}}}, nil
	}
	res, err := resolveMirrorPage(page, src, ep.URL)
	if err != nil {
		return res, err
	}
	filtered := res.Streams[:0]
	for _, s := range res.Streams {
		if strings.Contains(s.URL, "cbox.ws") {
			continue
		}
		filtered = append(filtered, s)
	}
	res.Streams = filtered
	if len(res.Streams) == 0 {
		return res, fmt.Errorf("tidak ada embed video ditemukan di %s", ep.URL)
	}
	return res, nil
}

// parseKuroPosterMap melengkapi peta poster Kuronime dari kartu
// seri arsip (judul → poster serinya sendiri) dan kartu episode
// beranda (judul dari .bsuxtt h2 dengan sufiks episode dibuang →
// poster seri) — kartu Kuronime tidak memakai img.ts-post-image
// seperti Animestream, jadi parsePosterMap lama tidak menangkapnya.
func parseKuroPosterMap(page, base string) map[string]string {
	out := map[string]string{}
	for _, d := range parseKuroArchivePage(page, base) {
		if d.Poster != "" {
			out[NormalizeTitleKey(d.Title)] = d.Poster
		}
	}
	base = strings.TrimRight(base, "/")
	for _, m := range kuroCardEpRe.FindAllStringSubmatch(page, -1) {
		card := m[1]
		var title string
		if hm := kuroH2Re.FindStringSubmatch(card); hm != nil {
			title = cleanText(hm[1])
		}
		title = strings.TrimSuffix(epTitleRe.ReplaceAllString(title, ""), " Subtitle Indonesia")
		if title == "" {
			continue
		}
		if p := kuroCardPoster(card, base); p != "" {
			key := NormalizeTitleKey(title)
			if _, dup := out[key]; !dup {
				out[key] = p
			}
		}
	}
	return out
}
