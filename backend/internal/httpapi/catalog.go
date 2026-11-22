package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

var titleTypes = map[string]bool{
	"anime": true, "donghua": true, "film": true,
	"manga": true, "manhua": true, "manhwa": true,
}

type titleCard struct {
	ID       string   `json:"id"`
	Slug     string   `json:"slug"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	TitleAlt *string  `json:"title_alt,omitempty"`
	Poster   *string  `json:"poster_url,omitempty"`
	Year     *int     `json:"year,omitempty"`
	Status   string   `json:"status"`
	Genres   []string `json:"genres"`
	Rating   *float64 `json:"rating,omitempty"`
	IsAdult  bool     `json:"is_adult"`
}

const titleCardCols = `id, slug, type, title, title_alt, poster_url, year, status, genres, rating::float8, is_adult`

func scanTitleCard(row pgx.Row) (titleCard, error) {
	var c titleCard
	err := row.Scan(&c.ID, &c.Slug, &c.Type, &c.Title, &c.TitleAlt, &c.Poster,
		&c.Year, &c.Status, &c.Genres, &c.Rating, &c.IsAdult)
	if c.Genres == nil {
		c.Genres = []string{}
	}
	return c, err
}

func (a *API) queryCards(ctx context.Context, query string, args ...any) ([]titleCard, error) {
	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []titleCard{}
	for rows.Next() {
		c, err := scanTitleCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// handleHome: endpoint beranda gabungan (tren + terbaru) — satu panggilan.
func (a *API) handleHome(w http.ResponseWriter, r *http.Request) {
	a.serveCached(w, "home", func() (any, error) {
		ctx := r.Context()
		trending, err := a.queryCards(ctx, `
			select `+titleCardCols+` from titles t
			join trending_cache tc on tc.title_id = t.id
			where tc."window" = 'day' and not t.is_adult
			order by tc.score desc limit 10`)
		if err != nil {
			return nil, err
		}
		if len(trending) == 0 {
			// Cache tren belum terisi worker — cadangan: rating tertinggi.
			trending, err = a.queryCards(ctx, `
				select `+titleCardCols+` from titles
				where not is_adult
				order by rating desc nulls last, created_at desc limit 10`)
			if err != nil {
				return nil, err
			}
		}
		terbaru, err := a.queryCards(ctx, `
			select `+titleCardCols+` from titles
			where not is_adult
			order by created_at desc, id desc limit 10`)
		if err != nil {
			return nil, err
		}
		return map[string]any{"trending": trending, "terbaru": terbaru}, nil
	})
}

type cursor struct {
	C time.Time `json:"c"`
	I string    `json:"i"`
}

func encodeCursor(c cursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (cursor, error) {
	var c cursor
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// handleTitles: telusur katalog dengan pagination kursor (bukan OFFSET).
func (a *API) handleTitles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	if typ != "" && !titleTypes[typ] {
		writeErr(w, http.StatusBadRequest, "type tidak dikenal")
		return
	}
	limit := 20
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= 50 {
		limit = v
	}
	var cur *cursor
	if s := q.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "cursor tidak valid")
			return
		}
		cur = &c
	}
	key := "titles?" + r.URL.RawQuery
	a.serveCached(w, key, func() (any, error) {
		ctx := r.Context()
		args := []any{}
		where := "where not t.is_adult"
		if typ != "" {
			args = append(args, typ)
			where += " and t.type = $" + strconv.Itoa(len(args))
		}
		if g := q.Get("genre"); g != "" {
			args = append(args, []string{g})
			where += " and t.genres @> $" + strconv.Itoa(len(args))
		}
		if s := q.Get("q"); s != "" {
			args = append(args, s)
			where += " and t.search_tsv @@ plainto_tsquery('simple', $" + strconv.Itoa(len(args)) + ")"
		}
		if cur != nil {
			args = append(args, cur.C, cur.I)
			where += " and (t.created_at, t.id) < ($" + strconv.Itoa(len(args)-1) + ", $" + strconv.Itoa(len(args)) + "::uuid)"
		}
		args = append(args, limit+1)
		sql := "select " + titleCardCols + ", t.created_at from titles t " + where +
			" order by t.created_at desc, t.id desc limit $" + strconv.Itoa(len(args))
		rows, err := a.pool.Query(ctx, sql, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := []titleCard{}
		createds := []time.Time{}
		for rows.Next() {
			var c titleCard
			var created time.Time
			if err := rows.Scan(&c.ID, &c.Slug, &c.Type, &c.Title, &c.TitleAlt, &c.Poster,
				&c.Year, &c.Status, &c.Genres, &c.Rating, &c.IsAdult, &created); err != nil {
				return nil, err
			}
			if c.Genres == nil {
				c.Genres = []string{}
			}
			items = append(items, c)
			createds = append(createds, created)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		next := ""
		if len(items) > limit {
			last := items[limit-1]
			next = encodeCursor(cursor{C: createds[limit-1], I: last.ID})
			items = items[:limit]
		}
		return map[string]any{"items": items, "next_cursor": next}, nil
	})
}

// handleTitleDetail: detail judul + daftar episode/chapter.
func (a *API) handleTitleDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var detail struct {
		titleCard
		Synopsis         *string `json:"synopsis,omitempty"`
		Season           *string `json:"season,omitempty"`
		Network          *string `json:"network,omitempty"`
		Studio           *string `json:"studio,omitempty"`
		DurationMin      *int    `json:"duration_min,omitempty"`
		Country          *string `json:"country,omitempty"`
		FetchMode        string  `json:"fetch_mode"`
		SourceDomain     *string `json:"source_domain,omitempty"`
		SourceURLPattern *string `json:"source_url_template,omitempty"`
	}
	err := a.pool.QueryRow(ctx, `
		select `+titleCardCols+`, synopsis, season, network, studio,
		       duration_min, country, fetch_mode, source_domain,
		       source_url_template from titles where id = $1::uuid`, id,
	).Scan(&detail.ID, &detail.Slug, &detail.Type, &detail.Title, &detail.TitleAlt,
		&detail.Poster, &detail.Year, &detail.Status, &detail.Genres, &detail.Rating,
		&detail.IsAdult, &detail.Synopsis, &detail.Season, &detail.Network,
		&detail.Studio, &detail.DurationMin, &detail.Country, &detail.FetchMode,
		&detail.SourceDomain, &detail.SourceURLPattern)
	if err != nil {
		writeErr(w, http.StatusNotFound, "judul tidak ditemukan")
		return
	}
	if detail.Genres == nil {
		detail.Genres = []string{}
	}
	type episode struct {
		ID          string  `json:"id"`
		EpisodeNo   int     `json:"episode_no"`
		Title       *string `json:"title,omitempty"`
		DurationSec *int    `json:"duration_sec,omitempty"`
	}
	episodes := []episode{}
	erows, err := a.pool.Query(ctx, `
		select id, episode_no, title, duration_sec from episodes
		where title_id = $1::uuid order by episode_no`, id)
	if err == nil {
		defer erows.Close()
		for erows.Next() {
			var e episode
			if err := erows.Scan(&e.ID, &e.EpisodeNo, &e.Title, &e.DurationSec); err == nil {
				episodes = append(episodes, e)
			}
		}
	}
	type chapter struct {
		ID        string  `json:"id"`
		ChapterNo float64 `json:"chapter_no"`
		Title     *string `json:"title,omitempty"`
		PageCount int     `json:"page_count"`
	}
	chapters := []chapter{}
	crows, err := a.pool.Query(ctx, `
		select id, chapter_no::float8, title, page_count from chapters
		where title_id = $1::uuid order by chapter_no`, id)
	if err == nil {
		defer crows.Close()
		for crows.Next() {
			var c chapter
			if err := crows.Scan(&c.ID, &c.ChapterNo, &c.Title, &c.PageCount); err == nil {
				chapters = append(chapters, c)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"title": detail, "episodes": episodes, "chapters": chapters,
	})
}

// handlePlayback: sumber putar sehat (failover O9) + subtitle bahasa asal.
func (a *API) handlePlayback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var ep struct {
		ID        string  `json:"id"`
		TitleID   string  `json:"title_id"`
		EpisodeNo int     `json:"episode_no"`
		VideoURL  string  `json:"-"`
		IntroS    *int    `json:"intro_start_sec,omitempty"`
		IntroE    *int    `json:"intro_end_sec,omitempty"`
		OutroS    *int    `json:"outro_start_sec,omitempty"`
		OutroE    *int    `json:"outro_end_sec,omitempty"`
	}
	err := a.pool.QueryRow(ctx, `
		select id, title_id, episode_no, video_url, intro_start_sec, intro_end_sec,
		       outro_start_sec, outro_end_sec
		from episodes where id = $1::uuid`, id,
	).Scan(&ep.ID, &ep.TitleID, &ep.EpisodeNo, &ep.VideoURL, &ep.IntroS, &ep.IntroE, &ep.OutroS, &ep.OutroE)
	if err != nil {
		writeErr(w, http.StatusNotFound, "episode tidak ditemukan")
		return
	}
	// Gerbang freemium (STRUKTUR Bagian 4): tamu maks. 1 episode.
	if !a.gateEpisode(w, r, ep.ID, ep.TitleID) {
		return
	}
	aksesTier, _ := a.viewerTier(r)
	type source struct {
		URL   string  `json:"url"`
		Label *string `json:"label,omitempty"`
		Lang  *string `json:"lang,omitempty"` // bahasa hardsub aliran ini (F40); kosong = softsub
	}
	sources := []source{}
	srows, err := a.pool.Query(ctx, `
		select url, label, lang from episode_sources
		where episode_id = $1::uuid and is_alive order by created_at`, id)
	if err == nil {
		defer srows.Close()
		for srows.Next() {
			var s source
			if err := srows.Scan(&s.URL, &s.Label, &s.Lang); err == nil {
				sources = append(sources, s)
			}
		}
	}
	if len(sources) == 0 {
		utama := "utama"
		sources = []source{{URL: ep.VideoURL, Label: &utama}}
	}
	type subtitle struct {
		Lang   string  `json:"lang"`
		Source string  `json:"source"`
		Format string  `json:"format"`
		File   *string `json:"file_url,omitempty"`
		Status string  `json:"status"`
	}
	subtitles := []subtitle{}
	urows, err := a.pool.Query(ctx, `
		select lang, source, format, file_url, status from subtitles
		where episode_id = $1::uuid order by lang`, id)
	if err == nil {
		defer urows.Close()
		for urows.Next() {
			var s subtitle
			if err := urows.Scan(&s.Lang, &s.Source, &s.Format, &s.File, &s.Status); err == nil {
				subtitles = append(subtitles, s)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"episode":   ep,
		"sources":   sources,
		"subtitles": subtitles,
		"access":    map[string]any{"tier": string(aksesTier), "ads": aksesTier != tierPremium},
	})
}

// handleChapter: isi chapter (halaman) untuk mode baca.
func (a *API) handleChapter(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var ch struct {
		ID        string  `json:"id"`
		TitleID   string  `json:"title_id"`
		ChapterNo float64 `json:"chapter_no"`
		Title     *string `json:"title,omitempty"`
	}
	err := a.pool.QueryRow(ctx, `
		select id, title_id, chapter_no::float8, title from chapters where id = $1::uuid`, id,
	).Scan(&ch.ID, &ch.TitleID, &ch.ChapterNo, &ch.Title)
	if err != nil {
		writeErr(w, http.StatusNotFound, "chapter tidak ditemukan")
		return
	}
	// Gerbang freemium: tamu maks. 1 chapter.
	if !a.gateChapter(w, r, ch.ID, ch.TitleID) {
		return
	}
	aksesTier, _ := a.viewerTier(r)
	type page struct {
		PageNo    int     `json:"page_no"`
		ImageURL  string  `json:"image_url"`
		LightURL  *string `json:"light_url,omitempty"`
		OCRStatus string  `json:"ocr_status"`
	}
	pages := []page{}
	prows, err := a.pool.Query(ctx, `
		select page_no, image_url, light_url, ocr_status from chapter_pages
		where chapter_id = $1::uuid order by page_no`, id)
	if err == nil {
		defer prows.Close()
		for prows.Next() {
			var pg page
			if err := prows.Scan(&pg.PageNo, &pg.ImageURL, &pg.LightURL, &pg.OCRStatus); err == nil {
				pages = append(pages, pg)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"chapter": ch,
		"pages":   pages,
		"access":  map[string]any{"tier": string(aksesTier), "ads": aksesTier != tierPremium},
	})
}
