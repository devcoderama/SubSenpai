package httpapi

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handleShareEpisodePage: halaman bagikan untuk SATU EPISODE —
// GET /share-episode/{episodeId}, diproksi dari domain resmi
// https://subsenpai.web.id/nonton/{episodeId}. Dipakai
// tautan nonton pada notifikasi jadwal rilis komunitas
// (permintaan Rams 2026-11-24): membuka aplikasi langsung ke
// PEMUTAR episode itu (subsenpai://episode/{id}), bukan ke
// halaman detail judul. Templat sama dengan halaman judul.
func (a *API) handleShareEpisodePage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	data := sharePageData{
		Title:       "SubSenpai",
		Description: "Nonton anime, donghua, film & serial TV subtitle Indonesia di aplikasi SubSenpai.",
		PageURL:     "https://subsenpai.web.id/nonton/" + id,
		ID:          id,
	}
	status := http.StatusOK
	if shareIDRe.MatchString(id) {
		var title, typ string
		var titleID string
		var poster, synopsis *string
		var year *int
		var genres []string
		var epNo int
		err := a.pool.QueryRow(r.Context(), `
			select t.id, t.title, t.poster_url, t.year, t.type, t.genres, t.synopsis, e.episode_no
			from episodes e join titles t on t.id = e.title_id
			where e.id = $1::uuid and not t.is_hidden and not e.is_hidden`, id).
			Scan(&titleID, &title, &poster, &year, &typ, &genres, &synopsis, &epNo)
		if err == nil {
			data.Found = true
			data.Title = fmt.Sprintf("%s — Episode %d", title, epNo)
			if synopsis != nil && strings.TrimSpace(*synopsis) != "" {
				data.Synopsis = *synopsis
			}
			meta := []string{fmt.Sprintf("Episode %d", epNo)}
			if year != nil {
				meta = append(meta, strconv.Itoa(*year))
			}
			switch typ {
			case "anime":
				meta = append(meta, "Anime")
			case "donghua":
				meta = append(meta, "Donghua")
			case "film":
				meta = append(meta, "Film")
			}
			data.MetaLine = strings.Join(meta, " · ")
			data.GenresLine = strings.Join(genres, " • ")
			if poster != nil && strings.TrimSpace(*poster) != "" {
				proxy := "https://" + r.Host + "/api/v1/img-proxy?u=" + url.QueryEscape(*poster)
				data.ImageURL = proxy
				data.PosterURL = proxy
			}
			data.IntentURL = template.URL("intent://episode/" + id +
				"#Intent;scheme=subsenpai;package=com.subsenpai;S.browser_fallback_url=" +
				url.QueryEscape("https://subsenpai.web.id/#download") + ";end")
			data.SchemeURL = template.JS(strconv.Quote("subsenpai://episode/" + id))
			data.SEOTitle = seoTitle(title, year, typ, epNo)
			data.Description = seoDescription(title, year, typ, genres, synopsis)
			data.OGType = "video.tv_show"
			if typ == "film" {
				data.OGType = "video.movie"
			}
			data.JSONLD = shareJSONLD(fmt.Sprintf("%s — Episode %d", title, epNo), data.PageURL, data.ImageURL, data.Description, year, genres, typ, epNo, title, "https://subsenpai.web.id/judul/"+titleID)
		} else {
			status = http.StatusNotFound
		}
	} else {
		status = http.StatusNotFound
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(status)
	_ = shareTpl.Execute(w, data)
}
