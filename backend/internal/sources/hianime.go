package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// hiAnimeResolver mengikuti alur ajax yang dipakai situs keluarga
// HiAnime (penerus AniWatch/Zoro): halaman tontonan memuat daftar episode
// via /ajax/v2/episode/list/{id}, server via /ajax/v2/episode/servers,
// dan sumber sebenarnya adalah embed MegaCloud yang endpoint
// getSources-nya mengembalikan aliran HLS + trek subtitle multi-bahasa
// (softsub — satu video, banyak bahasa).
type hiAnimeResolver struct{}

func NewHiAnime() Resolver { return &hiAnimeResolver{} }

func (h *hiAnimeResolver) Key() string { return "hianime" }

func fetchH(ctx context.Context, rawURL string, headers map[string]string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", browserUA)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d dari %s", resp.StatusCode, rawURL)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func ajaxHeaders(referer string) map[string]string {
	return map[string]string{
		"X-Requested-With": "XMLHttpRequest",
		"Referer":          referer,
		"Accept":           "application/json, text/javascript, */*; q=0.01",
	}
}

var (
	hiCardRe   = regexp.MustCompile(`(?is)<div class="flw-item">(.*?)<div class="film-detail`)
	hiHrefRe   = regexp.MustCompile(`href="(/watch/[^"]+)"`)
	hiDataIDRe = regexp.MustCompile(`data-id="(\d+)"`)
	hiNameRe   = regexp.MustCompile(`(?is)<h3 class="film-name"><a[^>]*>(.*?)</a>`)
	hiEpItemRe = regexp.MustCompile(`(?is)<a[^>]*class="[^"]*ep-item[^"]*"[^>]*data-number="(\d+)"[^>]*data-id="(\d+)"[^>]*href="([^"]+)"`)
	hiSrvRe    = regexp.MustCompile(`(?is)<div class="item server-item"[^>]*data-type="([^"]+)"[^>]*data-id="(\d+)"[^>]*>(.*?)</div>`)
	hiMovieRe  = regexp.MustCompile(`(?:movie_id"?\s*(?:value=|:)\s*"?|episode/list/)(\d+)`)
)

func (h *hiAnimeResolver) Search(ctx context.Context, src Source, query string) ([]Candidate, error) {
	u := strings.TrimRight(src.BaseURL, "/") + "/search?keyword=" + url.QueryEscape(query)
	page, err := Fetch(ctx, u)
	if err != nil {
		return nil, err
	}
	out := []Candidate{}
	for _, block := range hiCardRe.FindAllStringSubmatch(page, -1) {
		b := block[1]
		href := hiHrefRe.FindStringSubmatch(b)
		name := hiNameRe.FindStringSubmatch(b)
		if href == nil || name == nil {
			continue
		}
		c := Candidate{Title: cleanText(name[1]), URL: strings.TrimRight(src.BaseURL, "/") + href[1]}
		if idm := hiDataIDRe.FindStringSubmatch(b); idm != nil {
			c.Ref = idm[1]
		}
		out = append(out, c)
		if len(out) >= 10 {
			break
		}
	}
	return out, nil
}

func (h *hiAnimeResolver) animeID(ctx context.Context, src Source, titleURL string) (string, error) {
	page, err := Fetch(ctx, titleURL)
	if err != nil {
		return "", err
	}
	if m := hiMovieRe.FindStringSubmatch(page); m != nil {
		return m[1], nil
	}
	return "", fmt.Errorf("id anime tidak ditemukan di %s", titleURL)
}

func (h *hiAnimeResolver) Episodes(ctx context.Context, src Source, titleURL string) ([]EpisodeRef, error) {
	id, err := h.animeID(ctx, src, titleURL)
	if err != nil {
		return nil, err
	}
	u := strings.TrimRight(src.BaseURL, "/") + "/ajax/v2/episode/list/" + id
	raw, err := fetchH(ctx, u, ajaxHeaders(titleURL))
	if err != nil {
		return nil, err
	}
	var wrap struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal([]byte(raw), &wrap); err != nil {
		return nil, fmt.Errorf("daftar episode HiAnime tidak terbaca: %w", err)
	}
	out := []EpisodeRef{}
	for _, m := range hiEpItemRe.FindAllStringSubmatch(wrap.HTML, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, EpisodeRef{
			Number: n,
			Ref:    m[2],
			URL:    strings.TrimRight(src.BaseURL, "/") + html.UnescapeString(m[3]),
		})
	}
	return out, nil
}

type megaSources struct {
	Sources []struct {
		File string `json:"file"`
		Type string `json:"type"`
	} `json:"sources"`
	Tracks []struct {
		File    string `json:"file"`
		Label   string `json:"label"`
		Kind    string `json:"kind"`
		Default bool   `json:"default"`
	} `json:"tracks"`
}

// resolveEmbed mengambil aliran + trek dari satu embed MegaCloud.
func (h *hiAnimeResolver) resolveEmbed(ctx context.Context, embedURL, serverName string) (Resolved, error) {
	res := Resolved{}
	// Bentuk embed: https://megacloud.tv/embed-2/e-1/{id}?k=1
	parts := strings.Split(strings.TrimSuffix(embedURL, "?k=1"), "/")
	id := parts[len(parts)-1]
	u, err := url.Parse(embedURL)
	if err != nil {
		return res, err
	}
	api := u.Scheme + "://" + u.Host + "/embed-2/ajax/e-1/getSources?id=" + id
	raw, err := fetchH(ctx, api, map[string]string{"Referer": embedURL, "X-Requested-With": "XMLHttpRequest"})
	if err != nil {
		return res, err
	}
	var ms megaSources
	if err := json.Unmarshal([]byte(raw), &ms); err != nil {
		return res, fmt.Errorf("getSources MegaCloud tidak terbaca: %w", err)
	}
	for _, s := range ms.Sources {
		if s.File != "" {
			res.Streams = append(res.Streams, Stream{URL: s.File, Label: serverName})
		}
	}
	for _, t := range ms.Tracks {
		if t.Kind != "captions" || t.File == "" {
			continue
		}
		format := "vtt"
		if strings.HasSuffix(t.File, ".srt") {
			format = "srt"
		}
		res.Tracks = append(res.Tracks, Track{Lang: LangFromLabel(t.Label), Format: format, URL: t.File})
	}
	return res, nil
}

func (h *hiAnimeResolver) Resolve(ctx context.Context, src Source, ep EpisodeRef) (Resolved, error) {
	if ep.Ref == "" {
		return Resolved{}, fmt.Errorf("episode HiAnime tanpa id")
	}
	base := strings.TrimRight(src.BaseURL, "/")
	u := base + "/ajax/v2/episode/servers?episodeId=" + ep.Ref
	raw, err := fetchH(ctx, u, ajaxHeaders(base+"/"))
	if err != nil {
		return Resolved{}, err
	}
	var wrap struct {
		HTML string `json:"html"`
	}
	if err := json.Unmarshal([]byte(raw), &wrap); err != nil {
		return Resolved{}, fmt.Errorf("daftar server HiAnime tidak terbaca: %w", err)
	}
	type server struct{ id, name string }
	subs := []server{}
	for _, m := range hiSrvRe.FindAllStringSubmatch(wrap.HTML, -1) {
		if m[1] == "sub" {
			subs = append(subs, server{id: m[2], name: cleanText(m[3])})
		}
	}
	if len(subs) == 0 {
		return Resolved{}, fmt.Errorf("tidak ada server sub untuk episode %s", ep.Ref)
	}
	out := Resolved{}
	for _, sv := range subs {
		if len(out.Streams) >= 2 {
			break
		}
		sraw, err := fetchH(ctx, base+"/ajax/v2/episode/sources?id="+sv.id, ajaxHeaders(base+"/"))
		if err != nil {
			continue
		}
		var link struct {
			Link string `json:"link"`
		}
		if err := json.Unmarshal([]byte(sraw), &link); err != nil || link.Link == "" {
			continue
		}
		r, err := h.resolveEmbed(ctx, link.Link, sv.name)
		if err != nil {
			continue
		}
		out.Streams = append(out.Streams, r.Streams...)
		if len(out.Tracks) == 0 {
			out.Tracks = r.Tracks
		}
	}
	if len(out.Streams) == 0 {
		return out, fmt.Errorf("semua server sub gagal diselesaikan untuk episode %s", ep.Ref)
	}
	return out, nil
}
