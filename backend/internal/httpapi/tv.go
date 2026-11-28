package httpapi

// TV Langsung (permintaan Rams 2026-11-28): kanal TV live HLS yang
// diputar langsung dari perangkat pengguna ke sumber stream (VPS
// tidak mem-proxy video). Gratis untuk semua tier — TV adalah umpan
// retensi harian, bukan fitur premium (v1). Label stabilitas:
// 'bagus' = stream resmi + HTTPS + terverifikasi hidup;
// 'tidak stabil' = restream IP mentah / belum terverifikasi —
// klien menampilkan lencana itu dan tidak menjanjikan kelancaran.
// Kanal yang health-check-nya gagal 3x beruntun disembunyikan
// otomatis (is_active=false) sampai hidup lagi.

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type tvChannel struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Slug          string  `json:"slug"`
	LogoURL       *string `json:"logo_url"`
	StreamURL     string  `json:"stream_url"`
	Country       string  `json:"country"`
	Category      string  `json:"category"`
	Quality       *string `json:"quality,omitempty"`
	Stability     string  `json:"stability"`
	Source        string  `json:"source"`
	HTTPReferer   *string `json:"http_referer,omitempty"`
	HTTPUserAgent *string `json:"http_user_agent,omitempty"`
	// WebOK: true bila pemutar web (hls.js) punya peluang memutar
	// kanal ini; false = hanya aplikasi (http mixed content / DASH).
	WebOK bool `json:"web_ok"`
}

const tvChannelCols = `id, name, slug, logo_url, stream_url, country,
	category, quality, stability, source, http_referer, http_user_agent, web_ok`

func scanTVChannel(rows interface {
	Scan(...any) error
}) (tvChannel, error) {
	var c tvChannel
	err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.LogoURL, &c.StreamURL,
		&c.Country, &c.Category, &c.Quality, &c.Stability, &c.Source,
		&c.HTTPReferer, &c.HTTPUserAgent, &c.WebOK)
	return c, err
}

func (a *API) handleTVChannels(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	args := []any{}
	where := "where is_active"
	if cat := q.Get("category"); cat != "" {
		args = append(args, cat)
		where += " and category = $" + strconv.Itoa(len(args))
	}
	if st := q.Get("stability"); st == "bagus" || st == "tidak stabil" {
		args = append(args, st)
		where += " and stability = $" + strconv.Itoa(len(args))
	}
	if country := q.Get("country"); country != "" {
		args = append(args, country)
		where += " and country = $" + strconv.Itoa(len(args))
	}
	rows, err := a.pool.Query(r.Context(),
		`select `+tvChannelCols+` from tv_channels `+where+` order by sort_order, name`, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat kanal TV")
		return
	}
	defer rows.Close()
	out := []tvChannel{}
	for rows.Next() {
		c, err := scanTVChannel(rows)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat kanal TV")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (a *API) handleTVChannel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// Cari per slug dulu, lalu per UUID — $1::uuid langsung pada
	// slug ('transtv') membuat Postgres error cast, jadi dua tahap.
	var c tvChannel
	err := a.pool.QueryRow(r.Context(),
		`select `+tvChannelCols+` from tv_channels
		 where is_active and slug = $1`, id).
		Scan(&c.ID, &c.Name, &c.Slug, &c.LogoURL, &c.StreamURL,
			&c.Country, &c.Category, &c.Quality, &c.Stability, &c.Source,
			&c.HTTPReferer, &c.HTTPUserAgent, &c.WebOK)
	if err != nil {
		err = a.pool.QueryRow(r.Context(),
			`select `+tvChannelCols+` from tv_channels
			 where is_active and id = $1::uuid`, id).
			Scan(&c.ID, &c.Name, &c.Slug, &c.LogoURL, &c.StreamURL,
				&c.Country, &c.Category, &c.Quality, &c.Stability, &c.Source,
				&c.HTTPReferer, &c.HTTPUserAgent, &c.WebOK)
	}
	if err != nil {
		writeErr(w, http.StatusNotFound, "kanal TV tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channel": c})
}
