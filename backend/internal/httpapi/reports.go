package httpapi

// Laporan user dari pemutar (tombol Laporkan — permintaan Rams
// 2026-11-23 malam) + tindak lanjut admin dari dashboard:
// perbaiki link streaming (buang sumber lama episode lalu resolve
// ulang dari situs sumber), sembunyikan satu episode, atau
// sembunyikan judulnya. Laporan boleh dari user login maupun
// tamu (kunci perangkat), didedupe 10 menit per pelapor+episode.

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/jobs"
)

var reportReasons = map[string]bool{
	"semua_server_gagal": true,
	"link_rusak":         true,
	"video_salah":        true,
	"subtitle":           true,
	"lainnya":            true,
}

var reportStatuses = map[string]bool{
	"baru": true, "diproses": true, "selesai": true, "diabaikan": true,
}

/* ---------- sisi user ---------- */

func (a *API) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EpisodeID string `json:"episode_id"`
		Reason    string `json:"reason"`
		Note      string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.EpisodeID == "" {
		writeErr(w, http.StatusBadRequest, "episode_id wajib diisi")
		return
	}
	if !reportReasons[req.Reason] {
		req.Reason = "link_rusak"
	}
	if n := []rune(req.Note); len(n) > 500 {
		req.Note = string(n[:500])
	}
	ctx := r.Context()
	var titleID string
	if err := a.pool.QueryRow(ctx,
		`select title_id from episodes where id = $1::uuid`, req.EpisodeID).Scan(&titleID); err != nil {
		writeErr(w, http.StatusNotFound, "episode tidak ditemukan")
		return
	}
	_, profile := a.viewerTier(r)
	var userID *string
	reporterKey := "d:" + deviceKey(r)
	if profile != nil {
		userID = &profile.ID
		reporterKey = "u:" + profile.ID
	}
	// Dedupe: laporan terbuka yang sama dari pelapor yang sama
	// dalam 10 menit terakhir dikembalikan apa adanya.
	var existing string
	if err := a.pool.QueryRow(ctx, `
		select id from user_reports
		where episode_id = $1::uuid and reporter_key = $2
		  and status in ('baru','diproses')
		  and created_at > now() - interval '10 minutes'
		order by created_at desc limit 1`, req.EpisodeID, reporterKey).Scan(&existing); err == nil {
		writeJSON(w, http.StatusOK, map[string]any{"id": existing, "duplicated": true})
		return
	}
	var id string
	err := a.pool.QueryRow(ctx, `
		insert into user_reports (user_id, device_key, reporter_key, title_id, episode_id, reason, note)
		values ($1::uuid, $2, $3, $4::uuid, $5::uuid, $6, $7) returning id`,
		userID, deviceKey(r), reporterKey, titleID, req.EpisodeID, req.Reason, req.Note).Scan(&id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan laporan")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

/* ---------- sisi admin ---------- */

type adminReport struct {
	ID        string    `json:"id"`
	TitleID   *string   `json:"title_id,omitempty"`
	Title     string    `json:"title"`
	EpisodeID *string   `json:"episode_id,omitempty"`
	EpisodeNo *int      `json:"episode_no,omitempty"`
	Reason    string    `json:"reason"`
	Note      string    `json:"note"`
	Status    string    `json:"status"`
	Reporter  string    `json:"reporter"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *API) handleAdminReports(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	status := r.URL.Query().Get("status")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	ctx := r.Context()
	var nBaru, nProses, nSelesai, nAbaikan int
	_ = a.pool.QueryRow(ctx, `
		select count(*) filter (where status = 'baru'),
		       count(*) filter (where status = 'diproses'),
		       count(*) filter (where status = 'selesai'),
		       count(*) filter (where status = 'diabaikan')
		from user_reports`).Scan(&nBaru, &nProses, &nSelesai, &nAbaikan)
	where := ""
	args := []any{}
	if reportStatuses[status] {
		args = append(args, status)
		where = "where ur.status = $1"
	}
	var total int
	_ = a.pool.QueryRow(ctx, "select count(*) from user_reports ur "+where, args...).Scan(&total)
	args = append(args, limit, offset)
	rows, err := a.pool.Query(ctx, `
		select ur.id, ur.title_id, coalesce(t.title, '(judul terhapus)'),
		       ur.episode_id, e.episode_no, ur.reason, ur.note, ur.status,
		       coalesce(p.email, 'Tamu'), ur.created_at
		from user_reports ur
		left join titles t on t.id = ur.title_id
		left join episodes e on e.id = ur.episode_id
		left join profiles p on p.id = ur.user_id
		`+where+`
		order by ur.created_at desc
		limit $`+strconv.Itoa(len(args)-1)+` offset $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat laporan")
		return
	}
	defer rows.Close()
	out := []adminReport{}
	for rows.Next() {
		var rp adminReport
		if err := rows.Scan(&rp.ID, &rp.TitleID, &rp.Title, &rp.EpisodeID, &rp.EpisodeNo,
			&rp.Reason, &rp.Note, &rp.Status, &rp.Reporter, &rp.CreatedAt); err == nil {
			out = append(out, rp)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"reports": out, "total": total,
		"counts": map[string]any{
			"baru": nBaru, "diproses": nProses, "selesai": nSelesai,
			"diabaikan": nAbaikan, "total": nBaru + nProses + nSelesai + nAbaikan,
		},
	})
}

// loadReport memuat target satu laporan untuk aksi admin.
func (a *API) loadReport(ctx context.Context, id string) (titleID, episodeID *string, err error) {
	err = a.pool.QueryRow(ctx,
		`select title_id, episode_id from user_reports where id = $1::uuid`, id).
		Scan(&titleID, &episodeID)
	return titleID, episodeID, err
}

func (a *API) setReportStatus(ctx context.Context, adminID, id, status string) {
	_, _ = a.pool.Exec(ctx, `
		update user_reports set status = $2, handled_by = $3::uuid, updated_at = now()
		where id = $1::uuid`, id, status, adminID)
}

func (a *API) handleAdminReportUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !reportStatuses[req.Status] {
		writeErr(w, http.StatusBadRequest, "status tidak dikenal")
		return
	}
	ct, err := a.pool.Exec(r.Context(), `
		update user_reports set status = $2, handled_by = $3::uuid, updated_at = now()
		where id = $1::uuid`, id, req.Status, p.ID)
	if err != nil || ct.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "laporan tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(r.Context(), `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'report_status', $2, $3)`, p.ID, id, map[string]any{"status": req.Status})
	writeJSON(w, http.StatusOK, map[string]any{"status": req.Status})
}

// handleAdminReportFixLinks: buang semua sumber putar lama
// episode yang dilaporkan, lalu resolve ulang dari situs sumber
// di latar; laporan selesai otomatis bila resolve berhasil.
func (a *API) handleAdminReportFixLinks(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	_, episodeID, err := a.loadReport(ctx, id)
	if err != nil || episodeID == nil {
		writeErr(w, http.StatusNotFound, "laporan/episode tidak ditemukan")
		return
	}
	if _, err := a.pool.Exec(ctx,
		`delete from episode_sources where episode_id = $1::uuid`, *episodeID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuang sumber lama")
		return
	}
	_, _ = a.pool.Exec(ctx,
		`update episodes set video_url = '' where id = $1::uuid`, *episodeID)
	a.setReportStatus(ctx, p.ID, id, "diproses")
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'report_fix_links', $2, $3)`, p.ID, id, map[string]any{"episode_id": *episodeID})
	epID := *episodeID
	go func() {
		bctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := jobs.ResolveOneEpisode(bctx, a.pool, epID); err != nil {
			log.Printf("perbaiki link laporan %s (episode %s): %v", id, epID, err)
			return
		}
		_, _ = a.pool.Exec(bctx, `
			update user_reports set status = 'selesai', updated_at = now()
			where id = $1::uuid and status = 'diproses'`, id)
	}()
	writeJSON(w, http.StatusOK, map[string]any{"status": "diproses"})
}

func (a *API) handleAdminReportHideTitle(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	titleID, _, err := a.loadReport(ctx, id)
	if err != nil || titleID == nil {
		writeErr(w, http.StatusNotFound, "laporan/judul tidak ditemukan")
		return
	}
	if _, err := a.pool.Exec(ctx,
		`update titles set is_hidden = true, updated_at = now() where id = $1::uuid`, *titleID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyembunyikan judul")
		return
	}
	a.setReportStatus(ctx, p.ID, id, "selesai")
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'report_hide_title', $2, $3)`, p.ID, id, map[string]any{"title_id": *titleID})
	writeJSON(w, http.StatusOK, map[string]any{"status": "selesai", "hidden": "title"})
}

func (a *API) handleAdminReportHideEpisode(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	_, episodeID, err := a.loadReport(ctx, id)
	if err != nil || episodeID == nil {
		writeErr(w, http.StatusNotFound, "laporan/episode tidak ditemukan")
		return
	}
	if _, err := a.pool.Exec(ctx,
		`update episodes set is_hidden = true where id = $1::uuid`, *episodeID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyembunyikan episode")
		return
	}
	a.setReportStatus(ctx, p.ID, id, "selesai")
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'report_hide_episode', $2, $3)`, p.ID, id, map[string]any{"episode_id": *episodeID})
	writeJSON(w, http.StatusOK, map[string]any{"status": "selesai", "hidden": "episode"})
}

func (a *API) handleAdminReportDelete(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	ct, err := a.pool.Exec(r.Context(),
		`delete from user_reports where id = $1::uuid`, id)
	if err != nil || ct.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "laporan tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(r.Context(), `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'report_delete', $2, $3)`, p.ID, id, map[string]any{})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
