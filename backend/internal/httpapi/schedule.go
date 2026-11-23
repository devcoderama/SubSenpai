package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"subsenpai/backend/internal/sources"
)

// Jadwal rilis (permintaan Rams 2026-11-23): GET /api/v1/schedule
// ?type=anime|donghua|film. Anime & donghua diambil dari halaman
// jadwal sumber (Kuronime / Anichin) lalu di-cache 30 menit di
// memori — kegagalan ambil menyajikan cache basi bila ada. Film
// tidak punya jadwal mingguan di sumbernya, jadi tab film berisi
// rilis terbaru dari katalog sendiri. Entri yang slug-nya cocok
// dengan katalog membawa title_id agar bisa diketuk di aplikasi.

type schedCache struct {
	mu     sync.Mutex
	at     time.Time
	groups []sources.ScheduleGroup
}

func (c *schedCache) get(fetch func() ([]sources.ScheduleGroup, error)) ([]sources.ScheduleGroup, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.groups != nil && time.Since(c.at) < 30*time.Minute {
		return c.groups, nil
	}
	groups, err := fetch()
	if err != nil || len(groups) == 0 {
		if c.groups != nil {
			return c.groups, nil
		}
		if err == nil {
			err = errScheduleEmpty
		}
		return nil, err
	}
	c.groups, c.at = groups, time.Now()
	return groups, nil
}

var errScheduleEmpty = &scheduleError{"jadwal kosong dari sumber"}

type scheduleError struct{ s string }

func (e *scheduleError) Error() string { return e.s }

type schedItemOut struct {
	Title   string  `json:"title"`
	Slug    string  `json:"slug"`
	Time    string  `json:"time"`
	Episode int     `json:"episode,omitempty"`
	Poster  string  `json:"poster,omitempty"`
	TitleID *string `json:"title_id,omitempty"`
}

type schedGroupOut struct {
	Day   string         `json:"day"`
	Items []schedItemOut `json:"items"`
}

// matchTitleIDs mengisi title_id untuk item yang slug-nya ada di
// katalog pada tipe yang sama.
func (a *API) matchTitleIDs(r *http.Request, typ string, groups []sources.ScheduleGroup) []schedGroupOut {
	slugs := []string{}
	seen := map[string]bool{}
	for _, g := range groups {
		for _, it := range g.Items {
			if !seen[it.Slug] {
				seen[it.Slug] = true
				slugs = append(slugs, it.Slug)
			}
		}
	}
	ids := map[string]string{}
	if len(slugs) > 0 {
		rows, err := a.pool.Query(r.Context(), `
			select slug, id from titles where type = $1 and slug = any($2::text[]) and not is_hidden`, typ, slugs)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var slug, id string
				if err := rows.Scan(&slug, &id); err == nil {
					ids[slug] = id
				}
			}
		}
	}
	out := make([]schedGroupOut, 0, len(groups))
	for _, g := range groups {
		go2 := schedGroupOut{Day: g.Day, Items: []schedItemOut{}}
		for _, it := range g.Items {
			o := schedItemOut{Title: it.Title, Slug: it.Slug, Time: it.Time, Episode: it.Episode, Poster: it.Poster}
			if id, ok := ids[it.Slug]; ok {
				o.TitleID = &id
			}
			go2.Items = append(go2.Items, o)
		}
		out = append(out, go2)
	}
	return out
}

func (a *API) handleSchedule(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	switch typ {
	case "anime", "donghua":
		var cache *schedCache
		var fetch func() ([]sources.ScheduleGroup, error)
		if typ == "anime" {
			cache = &a.schedAnime
			fetch = func() ([]sources.ScheduleGroup, error) { return sources.FetchKuronimeSchedule(r.Context()) }
		} else {
			cache = &a.schedDonghua
			fetch = func() ([]sources.ScheduleGroup, error) { return sources.FetchAnichinSchedule(r.Context()) }
		}
		groups, err := cache.get(fetch)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "jadwal belum bisa dimuat — coba lagi nanti")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"type":   typ,
			"groups": a.matchTitleIDs(r, typ, groups),
		})
	case "film":
		rows, err := a.pool.Query(r.Context(), `
			select id, slug, title, poster_url, year from titles
			where type = 'film' and not is_hidden order by updated_at desc limit 40`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		defer rows.Close()
		items := []schedItemOut{}
		for rows.Next() {
			var id, slug, title string
			var poster *string
			var year *int
			if err := rows.Scan(&id, &slug, &title, &poster, &year); err != nil {
				continue
			}
			o := schedItemOut{Title: title, Slug: slug, TitleID: &id}
			if poster != nil {
				o.Poster = *poster
			}
			if year != nil {
				o.Time = strconv.Itoa(*year)
			}
			items = append(items, o)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"type":   "film",
			"groups": []schedGroupOut{{Day: "Rilis Terbaru", Items: items}},
		})
	default:
		writeErr(w, http.StatusBadRequest, "type harus anime, donghua, atau film")
	}
}
