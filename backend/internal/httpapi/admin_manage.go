package httpapi

// Peralatan admin & developer (permintaan Rams 2026-11-23):
// kelola visibilitas judul, laporan crash aplikasi, pengaturan
// aplikasi (perawatan / paksa perbarui / pengumuman), log aktivitas
// admin, dan info sistem. Semua endpoint /admin/* di belakang
// requireAdmin; POST /crashes dan GET /app-config untuk aplikasi.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/jobs"
)

/* ---------- Kelola judul (tampilkan / sembunyikan) ---------- */

type adminTitle struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Type         string   `json:"type"`
	Title        string   `json:"title"`
	Poster       *string  `json:"poster_url,omitempty"`
	Year         *int     `json:"year,omitempty"`
	Status       string   `json:"status"`
	IsHidden     bool     `json:"is_hidden"`
	EpisodeCount int      `json:"episode_count"`
	Complete     bool     `json:"complete"`
	Missing      []string `json:"missing"`
	// Dua kolom berikut hanya diisi endpoint detail judul.
	Synopsis string   `json:"synopsis"`
	Genres   []string `json:"genres"`
}

// missingLabels menyusun label kekurangan satu judul (definisi
// sama dengan audit RepairCatalog di paket jobs).
func missingLabels(noPoster, noSynopsis, noYear, noGenres bool, epCount, unsourced int) []string {
	m := []string{}
	if noPoster {
		m = append(m, "poster")
	}
	if noSynopsis {
		m = append(m, "sinopsis")
	}
	if noYear {
		m = append(m, "tahun")
	}
	if noGenres {
		m = append(m, "genre")
	}
	if epCount == 0 {
		m = append(m, "episode kosong")
	} else if unsourced > 0 {
		m = append(m, fmt.Sprintf("%d episode tanpa sumber", unsourced))
	}
	return m
}

// handleAdminTitles: cari judul TERMASUK yang tersembunyi (untuk
// dikelola admin) — berbeda dari katalog aplikasi yang menyaringnya.
func (a *API) handleAdminTitles(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	typ := r.URL.Query().Get("type")
	filter := r.URL.Query().Get("filter") // "" | komplit | belum | kosong
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := "where 1=1"
	if q != "" {
		args = append(args, "%"+q+"%")
		where += " and (t.title ilike $" + strconv.Itoa(len(args)) + " or t.title_alt ilike $" + strconv.Itoa(len(args)) + ")"
	}
	if typ != "" {
		args = append(args, typ)
		where += " and t.type = $" + strconv.Itoa(len(args))
	}
	// Basis kelengkapan (definisi sama dengan audit RepairCatalog):
	// komplit = poster + sinopsis + tahun + genre + ada episode dan
	// semua episode punya sumber putar; kosong ("judul saja") =
	// belum ada sinopsis dan belum ada episode — umpan antrean
	// penyelesaian worker.
	base := `
		select t.id, t.slug, t.type, t.title, t.poster_url, t.year, t.status,
		       t.is_hidden, t.updated_at,
		       coalesce(t.poster_url,'') = '' as no_poster,
		       coalesce(t.synopsis,'') = '' as no_synopsis,
		       t.year is null as no_year,
		       coalesce(cardinality(t.genres),0) = 0 as no_genres,
		       ec.cnt as ep_count, ec.unsourced as unsourced,
		       (coalesce(t.poster_url,'') <> '' and coalesce(t.synopsis,'') <> ''
		        and t.year is not null and coalesce(cardinality(t.genres),0) > 0
		        and ec.cnt > 0 and ec.unsourced = 0) as complete,
		       (ec.cnt = 0 and coalesce(t.synopsis,'') = '') as bare
		from titles t
		join lateral (
			select count(*) as cnt,
			       count(*) filter (
			           where not exists (select 1 from episode_sources es where es.episode_id = e.id)
			             and coalesce(e.video_url,'') = ''
			       ) as unsourced
			from episodes e where e.title_id = t.id
		) ec on true
		` + where
	var nTotal, nKomplit, nBelum, nKosong int
	if err := a.pool.QueryRow(r.Context(), `
		select count(*), count(*) filter (where complete),
		       count(*) filter (where not complete),
		       count(*) filter (where bare)
		from (`+base+`) b`, args...).Scan(&nTotal, &nKomplit, &nBelum, &nKosong); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghitung judul")
		return
	}
	total := nTotal
	filterSQL := ""
	switch filter {
	case "komplit":
		filterSQL, total = " where complete", nKomplit
	case "belum":
		filterSQL, total = " where not complete", nBelum
	case "kosong":
		filterSQL, total = " where bare", nKosong
	}
	args = append(args, limit, offset)
	rows, err := a.pool.Query(r.Context(), `
		select id, slug, type, title, poster_url, year, status, is_hidden, ep_count,
		       no_poster, no_synopsis, no_year, no_genres, unsourced, complete
		from (`+base+`) b`+filterSQL+`
		order by updated_at desc
		limit $`+strconv.Itoa(len(args)-1)+` offset $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat judul")
		return
	}
	defer rows.Close()
	out := []adminTitle{}
	for rows.Next() {
		var t adminTitle
		var noPoster, noSynopsis, noYear, noGenres bool
		var unsourced int
		if err := rows.Scan(&t.ID, &t.Slug, &t.Type, &t.Title, &t.Poster,
			&t.Year, &t.Status, &t.IsHidden, &t.EpisodeCount,
			&noPoster, &noSynopsis, &noYear, &noGenres, &unsourced, &t.Complete); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca judul")
			return
		}
		t.Missing = missingLabels(noPoster, noSynopsis, noYear, noGenres, t.EpisodeCount, unsourced)
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"titles": out, "total": total,
		"counts": map[string]any{
			"total": nTotal, "komplit": nKomplit, "belum": nBelum, "kosong": nKosong,
		},
	})
}

// handleAdminTitleGet: satu judul lengkap untuk halaman detail
// Kelola Judul (sinopsis, genre, kelengkapan) — tabel daftar kini
// hanya memuat poster + judul + tombol Detail.
func (a *API) handleAdminTitleGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	var t adminTitle
	var noPoster, noSynopsis, noYear, noGenres bool
	var unsourced int
	err := a.pool.QueryRow(r.Context(), `
		select t.id, t.slug, t.type, t.title, t.poster_url, t.year, t.status, t.is_hidden,
		       coalesce(t.synopsis, ''), coalesce(t.genres, '{}'),
		       ec.cnt,
		       coalesce(t.poster_url,'') = '' as no_poster,
		       coalesce(t.synopsis,'') = '' as no_synopsis,
		       t.year is null as no_year,
		       coalesce(cardinality(t.genres),0) = 0 as no_genres,
		       ec.unsourced,
		       (coalesce(t.poster_url,'') <> '' and coalesce(t.synopsis,'') <> ''
		        and t.year is not null and coalesce(cardinality(t.genres),0) > 0
		        and ec.cnt > 0 and ec.unsourced = 0) as complete
		from titles t
		join lateral (
			select count(*) as cnt,
			       count(*) filter (
			           where not exists (select 1 from episode_sources es where es.episode_id = e.id)
			             and coalesce(e.video_url,'') = ''
			       ) as unsourced
			from episodes e where e.title_id = t.id
		) ec on true
		where t.id = $1::uuid`, chi.URLParam(r, "id")).Scan(
		&t.ID, &t.Slug, &t.Type, &t.Title, &t.Poster, &t.Year, &t.Status, &t.IsHidden,
		&t.Synopsis, &t.Genres, &t.EpisodeCount,
		&noPoster, &noSynopsis, &noYear, &noGenres, &unsourced, &t.Complete)
	if err != nil {
		writeErr(w, http.StatusNotFound, "judul tidak ditemukan")
		return
	}
	t.Missing = missingLabels(noPoster, noSynopsis, noYear, noGenres, t.EpisodeCount, unsourced)
	writeJSON(w, http.StatusOK, map[string]any{"title": t})
}

// handleAdminWorker: status proses worker (fase yang sedang
// berjalan + judul yang sedang diproses) dan antrean kelengkapan
// katalog, untuk kartu pantauan di halaman Kelola Judul.
func (a *API) handleAdminWorker(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	var total, komplit, kosong int
	_ = a.pool.QueryRow(ctx, `
		select count(*),
		  count(*) filter (where coalesce(poster_url,'') <> '' and coalesce(synopsis,'') <> ''
		     and year is not null and coalesce(cardinality(genres),0) > 0
		     and exists (select 1 from episodes e where e.title_id = titles.id)
		     and not exists (select 1 from episodes e where e.title_id = titles.id
		        and not exists (select 1 from episode_sources es where es.episode_id = e.id)
		        and coalesce(e.video_url,'') = '')),
		  count(*) filter (where coalesce(synopsis,'') = ''
		     and not exists (select 1 from episodes e where e.title_id = titles.id))
		from titles where type in ('anime','donghua','film')`).Scan(&total, &komplit, &kosong)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": jobs.ReadWorkerStatus(ctx, a.pool),
		"queue": map[string]any{
			"total": total, "komplit": komplit, "belum": total - komplit, "kosong": kosong,
		},
	})
}

// handleAdminTitleUpdate: tampilkan/sembunyikan judul dari aplikasi.
func (a *API) handleAdminTitleUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		IsHidden *bool `json:"is_hidden"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IsHidden == nil {
		writeErr(w, http.StatusBadRequest, "is_hidden wajib diisi")
		return
	}
	ctx := r.Context()
	var title string
	err := a.pool.QueryRow(ctx, `
		update titles set is_hidden = $2, updated_at = now()
		where id = $1::uuid returning title`, id, *req.IsHidden).Scan(&title)
	if err != nil {
		writeErr(w, http.StatusNotFound, "judul tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'title_visibility', $2, $3)`,
		p.ID, id, map[string]any{"title": title, "is_hidden": *req.IsHidden})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "is_hidden": *req.IsHidden})
}

/* ---------- Laporan crash ---------- */

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

// handleCrashReport: aplikasi mengunggah laporan crash yang
// disimpannya saat crash (diunggah pada peluncuran berikutnya).
// Login tidak wajib (crash bisa terjadi sebelum login).
func (a *API) handleCrashReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppVersion     string `json:"app_version"`
		VersionCode    int    `json:"version_code"`
		AndroidVersion string `json:"android_version"`
		Device         string `json:"device"`
		Message        string `json:"message"`
		StackTrace     string `json:"stack_trace"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format laporan tidak valid")
		return
	}
	var userID any
	if p := auth.FromContext(r.Context()); p != nil {
		userID = p.ID
	}
	_, err := a.pool.Exec(r.Context(), `
		insert into crash_reports (user_id, app_version, version_code,
			android_version, device, message, stack_trace)
		values ($1, $2, $3, $4, $5, $6, $7)`,
		userID, truncate(req.AppVersion, 50), req.VersionCode,
		truncate(req.AndroidVersion, 100), truncate(req.Device, 200),
		truncate(req.Message, 500), truncate(req.StackTrace, 8000))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan laporan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type adminCrash struct {
	ID             string     `json:"id"`
	UserName       string     `json:"user_name"`
	UserEmail      string     `json:"user_email,omitempty"`
	AppVersion     string     `json:"app_version"`
	VersionCode    int        `json:"version_code"`
	AndroidVersion string     `json:"android_version"`
	Device         string     `json:"device"`
	Message        string     `json:"message"`
	StackTrace     string     `json:"stack_trace"`
	IsFixed        bool       `json:"is_fixed"`
	FixedAt        *time.Time `json:"fixed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (a *API) handleAdminCrashes(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	fixedFilter := r.URL.Query().Get("fixed") // "" | belum | sudah
	where := ""
	switch fixedFilter {
	case "belum":
		where = "where not c.is_fixed"
	case "sudah":
		where = "where c.is_fixed"
	}
	ctx := r.Context()
	var nTotal, nBelum, nSudah int
	_ = a.pool.QueryRow(ctx, `
		select count(*), count(*) filter (where not is_fixed),
		       count(*) filter (where is_fixed)
		from crash_reports`).Scan(&nTotal, &nBelum, &nSudah)
	rows, err := a.pool.Query(ctx, `
		select c.id, coalesce(p.display_name, ''), coalesce(p.email, ''), c.app_version, c.version_code,
		       c.android_version, c.device, c.message, c.stack_trace,
		       c.is_fixed, c.fixed_at, c.created_at
		from crash_reports c
		left join profiles p on p.id = c.user_id
		`+where+`
		order by c.created_at desc limit $1`, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat laporan crash")
		return
	}
	defer rows.Close()
	out := []adminCrash{}
	for rows.Next() {
		var c adminCrash
		if err := rows.Scan(&c.ID, &c.UserName, &c.UserEmail, &c.AppVersion, &c.VersionCode,
			&c.AndroidVersion, &c.Device, &c.Message, &c.StackTrace,
			&c.IsFixed, &c.FixedAt, &c.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca laporan crash")
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"crashes": out,
		"counts":  map[string]any{"total": nTotal, "belum": nBelum, "sudah": nSudah},
	})
}

// handleAdminCrashUpdate: tandai laporan crash sudah diperbaiki
// atau kembalikan ke belum (permintaan Rams 2026-11-23 malam).
func (a *API) handleAdminCrashUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		IsFixed *bool `json:"is_fixed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IsFixed == nil {
		writeErr(w, http.StatusBadRequest, "is_fixed wajib diisi")
		return
	}
	ct, err := a.pool.Exec(r.Context(), `
		update crash_reports
		set is_fixed = $2, fixed_at = case when $2 then now() else null end
		where id = $1::uuid`, id, *req.IsFixed)
	if err != nil || ct.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "laporan crash tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(r.Context(), `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'crash_fixed', $2, $3)`,
		p.ID, id, map[string]any{"is_fixed": *req.IsFixed})
	writeJSON(w, http.StatusOK, map[string]any{"is_fixed": *req.IsFixed})
}

func (a *API) handleAdminCrashDelete(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	if _, err := a.pool.Exec(r.Context(),
		`delete from crash_reports where id = $1::uuid`, chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus laporan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAdminCrashesClear: kosongkan semua laporan crash.
func (a *API) handleAdminCrashesClear(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	tag, err := a.pool.Exec(r.Context(), `delete from crash_reports`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal mengosongkan laporan")
		return
	}
	_, _ = a.pool.Exec(r.Context(), `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'crash_clear', '', $2)`,
		p.ID, map[string]any{"deleted": tag.RowsAffected()})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": tag.RowsAffected()})
}

/* ---------- Pengaturan aplikasi ---------- */

var settingKeys = []string{
	"maintenance_mode", "maintenance_message",
	"min_version_code", "min_version_name", "update_url", "announcement",
}

// validVersionName: nama versi seperti "0.22.2" (2–4 segmen angka).
func validVersionName(s string) bool {
	if s == "" {
		return true
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 4 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func (a *API) loadSettings(r *http.Request) map[string]string {
	out := map[string]string{}
	rows, err := a.pool.Query(r.Context(), `select key, value from app_settings`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err == nil {
				out[k] = v
			}
		}
	}
	return out
}

// handleAppConfig: konfigurasi publik untuk aplikasi — mode
// perawatan, versi minimum (paksa perbarui), dan pengumuman.
func (a *API) handleAppConfig(w http.ResponseWriter, r *http.Request) {
	s := a.loadSettings(r)
	minVer, _ := strconv.Atoi(s["min_version_code"])
	writeJSON(w, http.StatusOK, map[string]any{
		"maintenance_mode":    s["maintenance_mode"] == "true",
		"maintenance_message": s["maintenance_message"],
		"min_version_code":    minVer,
		"min_version_name":    s["min_version_name"],
		"update_url":          s["update_url"],
		"announcement":        s["announcement"],
	})
}

func (a *API) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": a.loadSettings(r)})
}

func (a *API) handleAdminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		MaintenanceMode    *bool   `json:"maintenance_mode"`
		MaintenanceMessage *string `json:"maintenance_message"`
		MinVersionCode     *int    `json:"min_version_code"`
		MinVersionName     *string `json:"min_version_name"`
		UpdateURL          *string `json:"update_url"`
		Announcement       *string `json:"announcement"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	if req.MinVersionCode != nil && *req.MinVersionCode < 0 {
		writeErr(w, http.StatusBadRequest, "min_version_code tidak boleh negatif")
		return
	}
	if req.MinVersionName != nil && !validVersionName(strings.TrimSpace(*req.MinVersionName)) {
		writeErr(w, http.StatusBadRequest, "format versi tidak valid — contoh: 0.22.2")
		return
	}
	updates := map[string]string{}
	if req.MaintenanceMode != nil {
		updates["maintenance_mode"] = strconv.FormatBool(*req.MaintenanceMode)
	}
	if req.MaintenanceMessage != nil {
		updates["maintenance_message"] = truncate(*req.MaintenanceMessage, 500)
	}
	if req.MinVersionCode != nil {
		updates["min_version_code"] = strconv.Itoa(*req.MinVersionCode)
	}
	if req.MinVersionName != nil {
		updates["min_version_name"] = strings.TrimSpace(*req.MinVersionName)
	}
	if req.UpdateURL != nil {
		updates["update_url"] = truncate(*req.UpdateURL, 500)
	}
	if req.Announcement != nil {
		updates["announcement"] = truncate(*req.Announcement, 500)
	}
	ctx := r.Context()
	for k, v := range updates {
		if _, err := a.pool.Exec(ctx, `
			insert into app_settings (key, value) values ($1, $2)
			on conflict (key) do update set value = excluded.value, updated_at = now()`,
			k, v); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal menyimpan pengaturan")
			return
		}
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'settings_update', '', $2)`, p.ID, updates)
	// Pengumuman update APK ke Discord kini OTOMATIS dari rilis
	// GitHub (jobs.PostUpdateToDiscordIfNew di worker) — menaikkan
	// min_version_name di sini tidak lagi memposting apa pun.
	writeJSON(w, http.StatusOK, map[string]any{"settings": a.loadSettings(r)})
}

/* ---------- Log admin & info sistem ---------- */

func (a *API) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := a.pool.Query(r.Context(), `
		select l.id, coalesce(p.email, ''), l.action, coalesce(l.target, ''),
		       coalesce(l.detail::text, '{}'), l.created_at
		from admin_logs l
		left join profiles p on p.id = l.admin_id
		order by l.created_at desc limit $1`, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat log")
		return
	}
	defer rows.Close()
	type logRow struct {
		ID        string          `json:"id"`
		Admin     string          `json:"admin_email"`
		Action    string          `json:"action"`
		Target    string          `json:"target"`
		Detail    json.RawMessage `json:"detail"`
		CreatedAt time.Time       `json:"created_at"`
	}
	out := []logRow{}
	for rows.Next() {
		var l logRow
		var detailText string
		if err := rows.Scan(&l.ID, &l.Admin, &l.Action, &l.Target, &detailText, &l.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca log")
			return
		}
		l.Detail = json.RawMessage(detailText)
		out = append(out, l)
	}
	writeJSON(w, http.StatusOK, map[string]any{"logs": out})
}

// handleAdminSystem: info sistem untuk developer — versi runtime,
// migrasi terakhir, jumlah data, dan status konfigurasi (tanpa
// nilai rahasia, hanya terpasang/tidak).
func (a *API) handleAdminSystem(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	count := func(query string) int {
		var n int
		_ = a.pool.QueryRow(ctx, query).Scan(&n)
		return n
	}
	var migration string
	_ = a.pool.QueryRow(ctx, `
		select version from schema_migrations order by version desc limit 1`).Scan(&migration)
	var dbSize int64
	_ = a.pool.QueryRow(ctx, `select pg_database_size(current_database())`).Scan(&dbSize)
	srvMetrics, srvHistory := collectServerMetrics()
	writeJSON(w, http.StatusOK, map[string]any{
		"go_version":       runtime.Version(),
		"server_time":      time.Now(),
		"latest_migration": migration,
		"server":           srvMetrics,
		"history":          srvHistory,
		"db_size_bytes":    dbSize,
		"counts": map[string]any{
			"users":           count(`select count(*) from profiles`),
			"admins":          count(`select count(*) from profiles where role = 'admin'`),
			"premium_users":   count(`select count(*) from profiles where premium_until > now()`),
			"titles":          count(`select count(*) from titles`),
			"titles_hidden":   count(`select count(*) from titles where is_hidden`),
			"episodes":        count(`select count(*) from episodes`),
			"transactions":    count(`select count(*) from payment_sessions`),
			"vouchers_active": count(`select count(*) from vouchers where status = 'active'`),
			"crash_reports":   count(`select count(*) from crash_reports`),
			"crash_unfixed":   count(`select count(*) from crash_reports where not is_fixed`),
			"feed_posts":      count(`select count(*) from feed_posts`),
		},
		"config": map[string]any{
			"google_login": a.cfg.GoogleClientID != "",
			"tokopay":      a.cfg.TokopayMerchantID != "" && a.cfg.TokopaySecret != "",
			"imgbb":        a.cfg.ImgBBAPIKey != "",
			"discord":      a.cfg.DiscordBotToken != "",
			"telegram":     a.cfg.TelegramBotToken != "",
		},
	})
}
