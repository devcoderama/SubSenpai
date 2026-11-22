package sources

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
//   - Halaman episode: TANPA <select class="mirror"> di HTML
//     mentah — <iframe id="iframedc"> hanya placeholder
//     data-src='about:blank'. Aliran aslinya datang dari API
//     Animeku: script pintar.js situs mem-POST blob per-episode
//     (var _0xa100d42aa di halaman) ke
//     https://animeku.org/api/v9/sources dan menerima token
//     player (src/src_sd) + peta embed mirror terenkripsi
//     CryptoJS (kunci di pintar.js). Resolver meniru persis
//     alur itu di resolveKuro. Iframe lain di halaman (chat
//     cbox.ws) bukan pemutar dan harus disingkirkan.
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
var kuroAnimekuRe = regexp.MustCompile(`https?://player\.animeku\.org/[^"'\s<>\\]+`)

// parseKuroEmbed mengekstrak URL embed player.animeku.org dari
// HTML halaman episode Kuronime. Pelajaran produksi 2026-11-22:
// tag mentahnya adalah <iframe id="iframedc" src="about:blank"
// …> — src hanya placeholder lazy-load dan atribut pembawa URL
// aslinya tidak selalu "data-src", jadi pola lama (data-src lalu
// src) menyimpan "about:blank" sebagai aliran. Pola baru:
//  1. cari URL player.animeku.org di SELURUH halaman (atribut
//     data-src/data-lazy-src, string JS, noscript — di mana pun
//     ia tertulis);
//  2. cadangan: tag iframe#iframedc, coba data-src, data-lazy-src,
//     lalu src — nilai kosong/"about:blank"/non-http selalu
//     ditolak.
func parseKuroEmbed(html string) string {
	if m := kuroAnimekuRe.FindString(html); m != "" {
		return strings.TrimSpace(m)
	}
	tag := kuroIframeRe.FindString(html)
	if tag == "" {
		return ""
	}
	for _, name := range []string{"data-src", "data-lazy-src", "src"} {
		u := strings.TrimSpace(attrAny(tag, name))
		if strings.HasPrefix(u, "//") {
			u = "https:" + u
		}
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			return u
		}
	}
	return ""
}

// --- API sumber Animeku (alur pintar.js situs) -----------------
//
// Halaman episode Kuronime tidak memuat URL aliran sama sekali:
// script pintar.js mem-POST {"id": <blob>} — blob per-episode dari
// var _0xa100d42aa di halaman — ke API di bawah, lalu memasang
// https://player.animeku.org/?data=<src> sebagai aliran utama.
// Field mirror respons adalah base64 dari dokumen JSON CryptoJS
// {ct, iv, s} yang terenkripsi AES-256-CBC dengan passphrase di
// bawah (EVP_BytesToKey/MD5 ala OpenSSL, garam dari field s,
// IV dari field iv) dan berisi peta embed per kualitas/host.

const (
	kuroAPIURL     = "https://animeku.org/api/v9/sources"
	kuroPlayerBase = "https://player.animeku.org/?data="
	kuroBlogBase   = "https://blog.animeku.org/player2.php?id="
	kuroMirrorPass = "3&!Z0M,VIZ;dZW=="
)

var (
	kuroBlobRe = regexp.MustCompile(`var _0xa100d42aa = "([^"]+)"`)
	kuroXenRe  = regexp.MustCompile(`var xenHash = "([^"]*)"`)
)

// parseKuroBlob mengambil blob id episode + nilai xenHash dari
// HTML mentah halaman episode.
func parseKuroBlob(page string) (blob, xen string) {
	if m := kuroBlobRe.FindStringSubmatch(page); m != nil {
		blob = m[1]
	}
	if m := kuroXenRe.FindStringSubmatch(page); m != nil {
		xen = m[1]
	}
	return blob, xen
}

// kuroAPIResp adalah respons API sumber Animeku.
type kuroAPIResp struct {
	Status int    `json:"status"`
	Src    string `json:"src"`
	SrcSD  string `json:"src_sd"`
	Blog   string `json:"blog"`
	Mirror string `json:"mirror"`
}

// fetchKuroSources memanggil API sumber Animeku persis seperti
// pintar.js: POST JSON {"id": blob} dari halaman episode.
func fetchKuroSources(ctx context.Context, blob, pageURL string) (*kuroAPIResp, error) {
	payload, err := json.Marshal(map[string]string{"id": blob})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, kuroAPIURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json,*/*")
	if u, err := url.Parse(pageURL); err == nil && u.Host != "" {
		req.Header.Set("Origin", u.Scheme+"://"+u.Host)
		req.Header.Set("Referer", pageURL)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API Animeku HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var out kuroAPIResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("respons API Animeku tidak terbaca: %w", err)
	}
	if out.Status != 200 {
		return nil, fmt.Errorf("API Animeku status %d", out.Status)
	}
	return &out, nil
}

// evpBytesToKey menurunkan kunci ala OpenSSL EVP_BytesToKey
// (rantai MD5) seperti yang dipakai CryptoJS untuk passphrase.
func evpBytesToKey(password, salt []byte, keyLen int) []byte {
	var out, prev []byte
	for len(out) < keyLen {
		h := md5.New()
		h.Write(prev)
		h.Write(password)
		h.Write(salt)
		prev = h.Sum(nil)
		out = append(out, prev...)
	}
	return out[:keyLen]
}

// kuroMirrorData adalah hasil dekripsi field mirror API.
type kuroMirrorData struct {
	Embed     map[string]map[string]*string `json:"embed"`
	Filelions string                        `json:"filelions"`
}

// decryptKuroMirror mendekripsi field mirror: base64 → JSON
// CryptoJS {ct, iv, s} → AES-256-CBC (kunci EVP dari passphrase,
// IV dari field iv) → JSON peta embed.
func decryptKuroMirror(mirrorB64 string) (*kuroMirrorData, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(mirrorB64))
	if err != nil {
		return nil, fmt.Errorf("mirror bukan base64: %w", err)
	}
	var doc struct {
		CT string `json:"ct"`
		IV string `json:"iv"`
		S  string `json:"s"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("mirror bukan JSON CryptoJS: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(doc.CT)
	if err != nil {
		return nil, fmt.Errorf("ct mirror rusak: %w", err)
	}
	salt, err := hex.DecodeString(doc.S)
	if err != nil {
		return nil, fmt.Errorf("salt mirror rusak: %w", err)
	}
	iv, err := hex.DecodeString(doc.IV)
	if err != nil {
		return nil, fmt.Errorf("iv mirror rusak: %w", err)
	}
	if len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ct mirror tidak sejajar blok AES")
	}
	key := evpBytesToKey([]byte(kuroMirrorPass), salt, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ct)
	if len(plain) == 0 {
		return nil, fmt.Errorf("hasil dekripsi mirror kosong")
	}
	pad := int(plain[len(plain)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(plain) {
		return nil, fmt.Errorf("padding mirror tidak sah")
	}
	plain = plain[:len(plain)-pad]
	var data kuroMirrorData
	if err := json.Unmarshal(plain, &data); err != nil {
		return nil, fmt.Errorf("isi mirror tidak terbaca: %w", err)
	}
	return &data, nil
}

// kuroHostLabel merapikan nama host mirror untuk label aliran.
func kuroHostLabel(host string) string {
	switch host {
	case "doodstream":
		return "Doodstream"
	case "mp4upload":
		return "Mp4Upload"
	case "krakenfiles":
		return "KrakenFiles"
	case "pixeldrain":
		return "Pixeldrain"
	case "filelions":
		return "Filelions"
	}
	if host == "" {
		return host
	}
	return strings.ToUpper(host[:1]) + host[1:]
}

// kuroQualityRank mengurutkan kunci kualitas v360p < v480p < …
// (kunci tak dikenal di belakang, menurut abjad).
func kuroQualityRank(q string) int {
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(q, "v"), "p"))
	if err != nil {
		return 1 << 30
	}
	return n
}

// kuroStreamsFromAPI menyusun daftar aliran dari respons API
// Animeku: token player utama (HD + SD), player blog khusus
// situs xenHash "rawa", lalu peta embed mirror terdekripsi
// (per kualitas menaik, host menurut abjad) dan Filelions.
func kuroStreamsFromAPI(resp *kuroAPIResp, xen, lang string) []Stream {
	var out []Stream
	seen := map[string]bool{}
	add := func(u, label string) {
		u = strings.TrimSpace(u)
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return
		}
		if seen[u] {
			return
		}
		seen[u] = true
		out = append(out, Stream{URL: u, Label: label, Lang: lang})
	}
	if resp.Src != "" {
		add(kuroPlayerBase+resp.Src, "Animeku")
	}
	if resp.SrcSD != "" {
		add(kuroPlayerBase+resp.SrcSD, "Animeku SD")
	}
	if xen == "rawa" && resp.Blog != "" {
		add(kuroBlogBase+resp.Blog, "Blog")
	}
	if resp.Mirror != "" {
		if data, err := decryptKuroMirror(resp.Mirror); err == nil {
			qualities := make([]string, 0, len(data.Embed))
			for q := range data.Embed {
				qualities = append(qualities, q)
			}
			sort.Slice(qualities, func(i, j int) bool {
				ri, rj := kuroQualityRank(qualities[i]), kuroQualityRank(qualities[j])
				if ri != rj {
					return ri < rj
				}
				return qualities[i] < qualities[j]
			})
			for _, q := range qualities {
				hosts := make([]string, 0, len(data.Embed[q]))
				for h := range data.Embed[q] {
					hosts = append(hosts, h)
				}
				sort.Strings(hosts)
				qlabel := strings.TrimPrefix(q, "v")
				for _, h := range hosts {
					if u := data.Embed[q][h]; u != nil {
						add(*u, kuroHostLabel(h)+" "+qlabel)
					}
				}
			}
			add(data.Filelions, "Filelions")
		}
	}
	return out
}

// resolveKuro adalah Resolve Kuronime. Jalur utama meniru
// pintar.js: blob episode dari halaman → API sumber Animeku →
// token player + mirror terdekripsi. Cadangan lama (embed yang
// tertulis langsung di halaman / pengurai umum) dipertahankan
// untuk berjaga bila API tidak menjawab.
func (a *animestreamResolver) resolveKuro(ctx context.Context, src Source, ep EpisodeRef) (Resolved, error) {
	page, err := Fetch(ctx, ep.URL)
	if err != nil {
		return Resolved{}, err
	}
	if blob, xen := parseKuroBlob(page); blob != "" {
		if resp, err := fetchKuroSources(ctx, blob, ep.URL); err == nil {
			if streams := kuroStreamsFromAPI(resp, xen, src.Lang); len(streams) > 0 {
				return Resolved{Streams: streams}, nil
			}
		}
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
