package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
)

// viewingProfileID menentukan profil tontonan aktif untuk request ini:
// header X-Viewing-Profile-Id bila diberikan (divalidasi milik user),
// selain itu profil tontonan pertama akun.
func (a *API) viewingProfileID(r *http.Request, userID string) (string, error) {
	ctx := r.Context()
	if h := r.Header.Get("X-Viewing-Profile-Id"); h != "" {
		var id string
		err := a.pool.QueryRow(ctx, `
			select id from viewing_profiles
			where id = $1::uuid and account_id = $2::uuid`, h, userID).Scan(&id)
		return id, err
	}
	var id string
	err := a.pool.QueryRow(ctx, `
		select id from viewing_profiles
		where account_id = $1::uuid order by created_at limit 1`, userID).Scan(&id)
	return id, err
}

// ---------- Watchlist ----------

func (a *API) handleWatchlistList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	items, err := a.queryCards(r.Context(), `
		select `+titleCardCols+` from titles t
		join watchlist wl on wl.title_id = t.id
		where wl.user_id = $1::uuid and wl.viewing_profile_id = $2::uuid
		order by wl.created_at desc`, p.ID, vp)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) handleWatchlistCheck(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	var exists bool
	err = a.pool.QueryRow(r.Context(), `
		select exists(select 1 from watchlist
			where user_id = $1::uuid and viewing_profile_id = $2::uuid and title_id = $3::uuid)`,
		p.ID, vp, chi.URLParam(r, "titleId")).Scan(&exists)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "judul tidak valid")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"in_watchlist": exists})
}

func (a *API) handleWatchlistAdd(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	var req struct {
		Notify bool `json:"notify"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	_, err = a.pool.Exec(r.Context(), `
		insert into watchlist (user_id, viewing_profile_id, title_id, notify)
		values ($1::uuid, $2::uuid, $3::uuid, $4)
		on conflict (user_id, viewing_profile_id, title_id)
		do update set notify = excluded.notify`,
		p.ID, vp, chi.URLParam(r, "titleId"), req.Notify)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "judul tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"in_watchlist": true})
}

func (a *API) handleWatchlistRemove(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	_, err = a.pool.Exec(r.Context(), `
		delete from watchlist
		where user_id = $1::uuid and viewing_profile_id = $2::uuid and title_id = $3::uuid`,
		p.ID, vp, chi.URLParam(r, "titleId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "judul tidak valid")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"in_watchlist": false})
}

// ---------- Riwayat ----------

func (a *API) handleHistoryWatchReport(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	var req struct {
		EpisodeID   string `json:"episode_id"`
		PositionSec int    `json:"position_sec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EpisodeID == "" {
		writeErr(w, http.StatusBadRequest, "episode_id wajib diisi")
		return
	}
	if req.PositionSec < 0 {
		req.PositionSec = 0
	}
	_, err = a.pool.Exec(r.Context(), `
		insert into watch_history (user_id, viewing_profile_id, episode_id, position_sec, updated_at)
		values ($1::uuid, $2::uuid, $3::uuid, $4, now())
		on conflict (user_id, viewing_profile_id, episode_id)
		do update set position_sec = excluded.position_sec, updated_at = now()`,
		p.ID, vp, req.EpisodeID, req.PositionSec)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "episode tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true})
}

func (a *API) handleHistoryWatchList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	limit := 20
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 50 {
		limit = v
	}
	type item struct {
		EpisodeID   string    `json:"episode_id"`
		TitleID     string    `json:"title_id"`
		Title       string    `json:"title"`
		PosterURL   *string   `json:"poster_url,omitempty"`
		EpisodeNo   int       `json:"episode_no"`
		PositionSec int       `json:"position_sec"`
		UpdatedAt   time.Time `json:"updated_at"`
	}
	items := []item{}
	rows, err := a.pool.Query(r.Context(), `
		select wh.episode_id, e.title_id, t.title, t.poster_url,
		       e.episode_no, wh.position_sec, wh.updated_at
		from watch_history wh
		join episodes e on e.id = wh.episode_id
		join titles t on t.id = e.title_id
		where wh.user_id = $1::uuid and wh.viewing_profile_id = $2::uuid
		order by wh.updated_at desc limit $3`, p.ID, vp, limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.EpisodeID, &it.TitleID, &it.Title, &it.PosterURL,
				&it.EpisodeNo, &it.PositionSec, &it.UpdatedAt); err == nil {
				items = append(items, it)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) handleHistoryReadReport(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	var req struct {
		ChapterID string `json:"chapter_id"`
		LastPage  int    `json:"last_page"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChapterID == "" {
		writeErr(w, http.StatusBadRequest, "chapter_id wajib diisi")
		return
	}
	if req.LastPage < 1 {
		req.LastPage = 1
	}
	ctx := r.Context()
	// Satu baris per (user, profil, chapter): perbarui bila sudah ada.
	var existing string
	err = a.pool.QueryRow(ctx, `
		select id from read_history
		where user_id = $1::uuid and viewing_profile_id = $2::uuid and chapter_id = $3::uuid`,
		p.ID, vp, req.ChapterID).Scan(&existing)
	if err == nil {
		_, err = a.pool.Exec(ctx, `
			update read_history set last_page = $2, updated_at = now() where id = $1::uuid`,
			existing, req.LastPage)
	} else {
		_, err = a.pool.Exec(ctx, `
			insert into read_history (user_id, viewing_profile_id, chapter_id, last_page)
			values ($1::uuid, $2::uuid, $3::uuid, $4)`, p.ID, vp, req.ChapterID, req.LastPage)
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, "chapter tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true})
}

func (a *API) handleHistoryReadList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	vp, err := a.viewingProfileID(r, p.ID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "profil tontonan tidak ditemukan")
		return
	}
	limit := 20
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 50 {
		limit = v
	}
	type item struct {
		ChapterID *string   `json:"chapter_id,omitempty"`
		TitleID   *string   `json:"title_id,omitempty"`
		Title     *string   `json:"title,omitempty"`
		PosterURL *string   `json:"poster_url,omitempty"`
		ChapterNo *float64  `json:"chapter_no,omitempty"`
		Label     *string   `json:"external_label,omitempty"`
		LastPage  int       `json:"last_page"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	items := []item{}
	rows, err := a.pool.Query(r.Context(), `
		select rh.chapter_id, coalesce(c.title_id, rh.title_id),
		       t.title, t.poster_url, c.chapter_no::float8,
		       rh.external_label, rh.last_page, rh.updated_at
		from read_history rh
		left join chapters c on c.id = rh.chapter_id
		left join titles t on t.id = coalesce(c.title_id, rh.title_id)
		where rh.user_id = $1::uuid and rh.viewing_profile_id = $2::uuid
		order by rh.updated_at desc limit $3`, p.ID, vp, limit)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.ChapterID, &it.TitleID, &it.Title, &it.PosterURL,
				&it.ChapterNo, &it.Label, &it.LastPage, &it.UpdatedAt); err == nil {
				items = append(items, it)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
