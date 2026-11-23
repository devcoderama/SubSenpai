package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
)

// Endpoint admin untuk dashboard web (web_admin/) — semua di balik
// JWT + role 'admin'. Login web memakai POST /auth/google yang sama
// dengan aplikasi; yang boleh membuka dashboard hanya akun yang
// role-nya sudah 'admin' di database (diangkat admin lain lewat
// halaman Pengguna, atau diubah langsung di database) — tidak ada
// pendaftaran admin dari web.

func requireAdmin(w http.ResponseWriter, r *http.Request) (*auth.Profile, bool) {
	p := auth.FromContext(r.Context())
	if p == nil || p.Role != "admin" {
		writeErr(w, http.StatusForbidden, "khusus admin")
		return nil, false
	}
	return p, true
}

// handleAdminStats: ringkasan angka untuk halaman utama dashboard.
func (a *API) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	var users, premiumUsers, titles, paidTrx, pendingTrx int
	var revenue int64
	err := a.pool.QueryRow(ctx, `
		select
		  (select count(*) from profiles),
		  (select count(*) from profiles where premium_until > now()),
		  (select count(*) from titles),
		  (select count(*) from payment_sessions where status = 'paid'),
		  (select count(*) from payment_sessions where status = 'pending'),
		  (select coalesce(sum(amount), 0) from payment_sessions where status = 'paid')`,
	).Scan(&users, &premiumUsers, &titles, &paidTrx, &pendingTrx, &revenue)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat statistik")
		return
	}
	byType := map[string]int{}
	rows, err := a.pool.Query(ctx, `select type, count(*) from titles group by type`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t string
			var n int
			if err := rows.Scan(&t, &n); err == nil {
				byType[t] = n
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users": users, "premium_users": premiumUsers, "titles": titles,
		"titles_by_type": byType, "paid_transactions": paidTrx,
		"pending_transactions": pendingTrx, "revenue": revenue,
	})
}

type adminUser struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"display_name"`
	AvatarURL    string     `json:"avatar_url"`
	Role         string     `json:"role"`
	IsBanned     bool       `json:"is_banned"`
	PremiumAktif bool       `json:"premium_aktif"`
	PremiumUntil *time.Time `json:"premium_until,omitempty"`
}

// handleAdminUsers: daftar pengguna (cari by email/nama).
func (a *API) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	q := r.URL.Query()
	search := q.Get("q")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}
	rows, err := a.pool.Query(r.Context(), `
		select id, coalesce(email, ''), coalesce(display_name, ''),
		       coalesce(avatar_url, ''), role, is_banned,
		       coalesce(premium_until > now(), false), premium_until
		from profiles
		where ($1 = '' or email ilike '%' || $1 || '%' or display_name ilike '%' || $1 || '%')
		order by (role = 'admin') desc, email asc
		limit $2 offset $3`, search, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat pengguna")
		return
	}
	defer rows.Close()
	out := []adminUser{}
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL,
			&u.Role, &u.IsBanned, &u.PremiumAktif, &u.PremiumUntil); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca pengguna")
			return
		}
		out = append(out, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

// handleAdminUserUpdate: ubah role pengguna (invite/promote admin
// dilakukan dari sini oleh admin yang sudah ada).
func (a *API) handleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	targetID := chi.URLParam(r, "id")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		(req.Role != "user" && req.Role != "admin") {
		writeErr(w, http.StatusBadRequest, "role harus 'user' atau 'admin'")
		return
	}
	if targetID == p.ID && req.Role != "admin" {
		writeErr(w, http.StatusBadRequest, "tidak bisa menurunkan role kamu sendiri")
		return
	}
	ctx := r.Context()
	var u adminUser
	err := a.pool.QueryRow(ctx, `
		update profiles set role = $2 where id = $1::uuid
		returning id, coalesce(email, ''), coalesce(display_name, ''),
		          coalesce(avatar_url, ''), role, is_banned,
		          coalesce(premium_until > now(), false), premium_until`,
		targetID, req.Role,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL,
		&u.Role, &u.IsBanned, &u.PremiumAktif, &u.PremiumUntil)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pengguna tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'user_role_change', $2, $3)`,
		p.ID, targetID, map[string]any{"role": req.Role})
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

type adminTransaction struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"display_name"`
	Purpose   string    `json:"purpose"`
	Plan      string    `json:"plan"`
	Provider  string    `json:"provider"`
	Amount    int       `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// handleAdminTransactions: sesi pembayaran terbaru semua pengguna.
func (a *API) handleAdminTransactions(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := a.pool.Query(r.Context(), `
		select s.id, coalesce(p.email, ''), coalesce(p.display_name, ''),
		       s.purpose, coalesce(s.plan, ''), s.provider, s.amount,
		       s.status, s.created_at
		from payment_sessions s
		join profiles p on p.id = s.user_id
		order by s.created_at desc
		limit $1`, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat transaksi")
		return
	}
	defer rows.Close()
	out := []adminTransaction{}
	for rows.Next() {
		var t adminTransaction
		if err := rows.Scan(&t.ID, &t.Email, &t.Name, &t.Purpose, &t.Plan,
			&t.Provider, &t.Amount, &t.Status, &t.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca transaksi")
			return
		}
		out = append(out, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": out})
}

type adminPlan struct {
	ID        string `json:"id"`
	Days      int    `json:"days"`
	Price     int    `json:"price"`
	Label     string `json:"label"`
	IsActive  bool   `json:"is_active"`
	SortOrder int    `json:"sort_order"`
}

// handleAdminPlans: daftar paket premium (tabel subscription_plans).
func (a *API) handleAdminPlans(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		select id, days, price, label, is_active, sort_order
		from subscription_plans order by sort_order, days`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat paket")
		return
	}
	defer rows.Close()
	out := []adminPlan{}
	for rows.Next() {
		var pl adminPlan
		if err := rows.Scan(&pl.ID, &pl.Days, &pl.Price, &pl.Label,
			&pl.IsActive, &pl.SortOrder); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca paket")
			return
		}
		out = append(out, pl)
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": out})
}

// handleAdminPlanUpdate: ubah harga/label/status aktif satu paket.
func (a *API) handleAdminPlanUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		Price    *int    `json:"price"`
		Label    *string `json:"label"`
		IsActive *bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	if req.Price != nil && *req.Price < 0 {
		writeErr(w, http.StatusBadRequest, "harga tidak boleh negatif")
		return
	}
	ctx := r.Context()
	var pl adminPlan
	err := a.pool.QueryRow(ctx, `
		update subscription_plans
		set price = coalesce($2, price),
		    label = coalesce($3, label),
		    is_active = coalesce($4, is_active),
		    updated_at = now()
		where id = $1
		returning id, days, price, label, is_active, sort_order`,
		id, req.Price, req.Label, req.IsActive,
	).Scan(&pl.ID, &pl.Days, &pl.Price, &pl.Label, &pl.IsActive, &pl.SortOrder)
	if err != nil {
		writeErr(w, http.StatusNotFound, "paket tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'plan_update', $2, $3)`,
		p.ID, id, map[string]any{
			"price": req.Price, "label": req.Label, "is_active": req.IsActive,
		})
	writeJSON(w, http.StatusOK, map[string]any{"plan": pl})
}
