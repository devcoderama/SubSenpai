package sources

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// wpResolver adalah resolver generik untuk situs streaming berbasis
// WordPress (pola tema anime yang dipakai Kuronime, Anichin, dan banyak
// situs fansub lain): pencarian lewat /?s=, daftar episode sebagai tautan
// di halaman judul, dan video sebagai embed iframe di halaman episode.
//
// Heuristik v1 berbasis pola tautan — cukup untuk struktur tema umum;
// penyimpangan per situs ditangani resolver khusus bila diperlukan.
type wpResolver struct {
	key string
}

func NewGeneric() Resolver { return &wpResolver{key: "generic"} }

func (w *wpResolver) Key() string { return w.key }

var (
	anchorRe  = regexp.MustCompile(`(?is)<a\s[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	iframeRe  = regexp.MustCompile(`(?is)<iframe\s[^>]*(?:src|data-src)="([^"]+)"`)
	sourceRe  = regexp.MustCompile(`(?is)<source\s[^>]*src="([^"]+)"`)
	tagRe     = regexp.MustCompile(`(?is)<[^>]+>`)
	epInURLRe = regexp.MustCompile(`(?i)episode[-_ ]?(\d+)`)
	epInTxtRe = regexp.MustCompile(`(?i)(?:episode|eps|ep)[. ]?\s*(\d+)`)
	trailNoRe = regexp.MustCompile(`-(\d+)/?$`)
)

func cleanText(s string) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

func sameHost(base, link string) bool {
	b, err1 := url.Parse(base)
	l, err2 := url.Parse(link)
	return err1 == nil && err2 == nil && l.Host != "" && l.Host == b.Host
}

func episodeNumber(href, text string) (int, bool) {
	if m := epInURLRe.FindStringSubmatch(href); m != nil {
		n, err := strconv.Atoi(m[1])
		return n, err == nil
	}
	if m := epInTxtRe.FindStringSubmatch(text); m != nil {
		n, err := strconv.Atoi(m[1])
		return n, err == nil
	}
	if m := trailNoRe.FindStringSubmatch(href); m != nil {
		n, err := strconv.Atoi(m[1])
		return n, err == nil
	}
	return 0, false
}

func (w *wpResolver) Search(ctx context.Context, src Source, query string) ([]Candidate, error) {
	u := strings.TrimRight(src.BaseURL, "/") + "/?s=" + url.QueryEscape(query)
	page, err := Fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []Candidate{}
	for _, m := range anchorRe.FindAllStringSubmatch(page, -1) {
		href, text := m[1], cleanText(m[2])
		if text == "" || !sameHost(src.BaseURL, href) || seen[href] {
			continue
		}
		if _, isEp := episodeNumber(href, ""); isEp {
			continue // tautan episode, bukan halaman judul
		}
		if strings.Contains(href, "/category/") || strings.Contains(href, "/tag/") ||
			strings.Contains(href, "/page/") || strings.Contains(href, "#") {
			continue
		}
		seen[href] = true
		out = append(out, Candidate{Title: text, URL: href})
		if len(out) >= 10 {
			break
		}
	}
	return out, nil
}

func (w *wpResolver) Episodes(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error) {
	page, err := Fetch(ctx, titleURL)
	if err != nil {
		return nil, err
	}
	byNo := map[int]EpisodeRef{}
	for _, m := range anchorRe.FindAllStringSubmatch(page, -1) {
		href, text := m[1], cleanText(m[2])
		if !sameHost(src.BaseURL, href) && !strings.HasPrefix(href, "/") {
			continue
		}
		n, ok := episodeNumber(href, text)
		if !ok {
			continue
		}
		if strings.HasPrefix(href, "/") {
			href = strings.TrimRight(src.BaseURL, "/") + href
		}
		if _, dup := byNo[n]; !dup {
			byNo[n] = EpisodeRef{Number: n, Title: text, URL: href}
		}
	}
	out := make([]EpisodeRef, 0, len(byNo))
	for _, e := range byNo {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

var hostLabels = map[string]string{
	"ok.ru": "OK.ru", "dailymotion.com": "Dailymotion", "dai.ly": "Dailymotion",
	"blogger.com": "Blogger", "streamtape.com": "Streamtape",
	"filemoon.sx": "Filemoon", "vidhide.com": "Vidhide", "streamwish.to": "StreamWish",
	"mp4upload.com": "Mp4Upload", "gdriveplayer.to": "GDrivePlayer",
}

func (w *wpResolver) Resolve(ctx context.Context, src Source, ep EpisodeRef) (Resolved, error) {
	page, err := Fetch(ctx, ep.URL)
	if err != nil {
		return Resolved{}, err
	}
	res := Resolved{}
	seen := map[string]bool{}
	add := func(raw string) {
		raw = html.UnescapeString(strings.TrimSpace(raw))
		if raw == "" || seen[raw] || strings.HasPrefix(raw, "about:") {
			return
		}
		if strings.HasPrefix(raw, "//") {
			raw = "https:" + raw
		}
		seen[raw] = true
		label := raw
		if u, err := url.Parse(raw); err == nil {
			host := strings.TrimPrefix(u.Host, "www.")
			if l, ok := hostLabels[host]; ok {
				label = l
			} else {
				label = host
			}
		}
		// Aliran dari situs hardsub: bahasanya bahasa sumber ini.
		res.Streams = append(res.Streams, Stream{URL: raw, Label: label, Lang: src.Lang})
	}
	for _, m := range iframeRe.FindAllStringSubmatch(page, -1) {
		add(m[1])
	}
	for _, m := range sourceRe.FindAllStringSubmatch(page, -1) {
		add(m[1])
	}
	if len(res.Streams) == 0 {
		return res, fmt.Errorf("tidak ada embed video ditemukan di %s", ep.URL)
	}
	return res, nil
}
