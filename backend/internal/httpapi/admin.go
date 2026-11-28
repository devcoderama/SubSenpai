package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/billing"
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
	Status       string     `json:"status"`
	AdminNote    string     `json:"admin_note"`
	PremiumAktif bool       `json:"premium_aktif"`
	PremiumUntil *time.Time `json:"premium_until,omitempty"`
}

const adminUserCols = `
	id, coalesce(email, ''), coalesce(display_name, ''),
	coalesce(avatar_url, ''), role, is_banned,
	coalesce(status, 'active'), coalesce(admin_note, ''),
	coalesce(premium_until > now(), false), premium_until`

func scanAdminUser(row interface {
	Scan(dest ...any) error
}, u *adminUser) error {
	return row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.AvatarURL,
		&u.Role, &u.IsBanned, &u.Status, &u.AdminNote, &u.PremiumAktif, &u.PremiumUntil)
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
		select `+adminUserCols+`
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
		if err := scanAdminUser(rows, &u); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca pengguna")
			return
		}
		out = append(out, u)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

// handleAdminUserGet: satu pengguna lengkap untuk halaman
// detail Daftar Pengguna.
func (a *API) handleAdminUserGet(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	var u adminUser
	err := scanAdminUser(a.pool.QueryRow(r.Context(), `
		select `+adminUserCols+`
		from profiles where id = $1::uuid`, chi.URLParam(r, "id")), &u)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pengguna tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

// handleAdminUserStatus: ubah status akun (active|banned|
// suspended) + catatan admin. Akun nonaktif langsung kehilangan
// akses: login ditolak dan middleware menolak sesi berjalannya.
// Admin tidak bisa mengubah status akunnya sendiri (cegah
// terkunci keluar). is_banned dijaga selaras dengan status.
func (a *API) handleAdminUserStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	targetID := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		(req.Status != "active" && req.Status != "banned" && req.Status != "suspended") {
		writeErr(w, http.StatusBadRequest, "status harus 'active', 'banned', atau 'suspended'")
		return
	}
	if len([]rune(req.Note)) > 1000 {
		writeErr(w, http.StatusBadRequest, "catatan maksimal 1000 karakter")
		return
	}
	if targetID == p.ID {
		writeErr(w, http.StatusBadRequest, "tidak bisa mengubah status akun kamu sendiri")
		return
	}
	ctx := r.Context()
	var u adminUser
	err := scanAdminUser(a.pool.QueryRow(ctx, `
		update profiles
		set status = $2, admin_note = $3, status_updated_at = now(),
		    is_banned = ($2 = 'banned')
		where id = $1::uuid
		returning `+adminUserCols,
		targetID, req.Status, req.Note), &u)
	if err != nil {
		writeErr(w, http.StatusNotFound, "pengguna tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'user_status_change', $2, $3)`,
		p.ID, targetID, map[string]any{"status": req.Status, "note": req.Note})
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
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
	err := scanAdminUser(a.pool.QueryRow(ctx, `
		update profiles set role = $2 where id = $1::uuid
		returning `+adminUserCols,
		targetID, req.Role), &u)
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

// adminTransactionByID membaca ulang satu transaksi (dengan pemiliknya).
func (a *API) adminTransactionByID(w http.ResponseWriter, r *http.Request, id string) (adminTransaction, bool) {
	var t adminTransaction
	err := a.pool.QueryRow(r.Context(), `
		select s.id, coalesce(p.email, ''), coalesce(p.display_name, ''),
		       s.purpose, coalesce(s.plan, ''), s.provider, s.amount,
		       s.status, s.created_at
		from payment_sessions s
		join profiles p on p.id = s.user_id
		where s.id = $1::uuid`, id,
	).Scan(&t.ID, &t.Email, &t.Name, &t.Purpose, &t.Plan,
		&t.Provider, &t.Amount, &t.Status, &t.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "transaksi tidak ditemukan")
		return t, false
	}
	return t, true
}

// handleAdminTransactionUpdate: edit status/jumlah/paket transaksi.
// Mengubah status menjadi 'paid' dari status lain menjalankan jalur
// aktivasi resmi (billing.ActivateSession — premium diperpanjang /
// voucher bisa ditukar), sama seperti callback gateway. Mengubah
// status KELUAR dari 'paid' tidak mencabut premium yang sudah aktif.
func (a *API) handleAdminTransactionUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		Status *string `json:"status"`
		Amount *int    `json:"amount"`
		Plan   *string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	if req.Status != nil && *req.Status != "pending" && *req.Status != "paid" && *req.Status != "expired" {
		writeErr(w, http.StatusBadRequest, "status harus pending, paid, atau expired")
		return
	}
	if req.Amount != nil && *req.Amount <= 0 {
		writeErr(w, http.StatusBadRequest, "jumlah harus lebih dari 0")
		return
	}
	ctx := r.Context()
	if req.Plan != nil && *req.Plan != "" {
		if _, ok := billing.PlanByIDCtx(ctx, a.pool, *req.Plan); !ok {
			writeErr(w, http.StatusBadRequest, "paket tidak dikenal: "+*req.Plan)
			return
		}
	}
	cur, ok := a.adminTransactionByID(w, r, id)
	if !ok {
		return
	}
	// Koreksi jumlah/paket diterapkan lebih dulu agar aktivasi
	// memakai paket yang benar.
	if req.Amount != nil || req.Plan != nil {
		if _, err := a.pool.Exec(ctx, `
			update payment_sessions
			set amount = coalesce($2, amount), plan = coalesce($3, plan)
			where id = $1::uuid`, id, req.Amount, req.Plan); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal mengubah transaksi")
			return
		}
	}
	note := ""
	if req.Status != nil && *req.Status != cur.Status {
		if *req.Status == "paid" {
			// Jalur resmi: menandai lunas + mengaktifkan haknya.
			if err := billing.ActivateSession(ctx, a.pool, id, "admin-manual"); err != nil {
				writeErr(w, http.StatusInternalServerError, "gagal mengaktifkan transaksi: "+err.Error())
				return
			}
			note = "Status diubah menjadi paid dan hak pengguna diaktifkan."
		} else {
			if _, err := a.pool.Exec(ctx, `
				update payment_sessions set status = $2 where id = $1::uuid`,
				id, *req.Status); err != nil {
				writeErr(w, http.StatusInternalServerError, "gagal mengubah status")
				return
			}
			if cur.Status == "paid" {
				note = "Status diubah, tetapi premium/voucher yang sudah terlanjur aktif TIDAK dicabut."
			}
		}
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'transaction_update', $2, $3)`,
		p.ID, id, map[string]any{
			"status": req.Status, "amount": req.Amount, "plan": req.Plan,
			"status_before": cur.Status,
		})
	updated, ok := a.adminTransactionByID(w, r, id)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transaction": updated, "note": note})
}

// handleAdminTransactionDelete: hapus permanen satu transaksi.
// Premium/voucher yang sudah terlanjur aktif dari transaksi ini
// TIDAK ikut dicabut (riwayat aktivasi berdiri sendiri).
func (a *API) handleAdminTransactionDelete(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	cur, ok := a.adminTransactionByID(w, r, id)
	if !ok {
		return
	}
	ctx := r.Context()
	if _, err := a.pool.Exec(ctx, `delete from payment_sessions where id = $1::uuid`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus transaksi")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'transaction_delete', $2, $3)`,
		p.ID, id, map[string]any{
			"email": cur.Email, "amount": cur.Amount,
			"status": cur.Status, "purpose": cur.Purpose, "plan": cur.Plan,
		})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
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

type adminVoucher struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	Source       string     `json:"source"`
	DurationDays int        `json:"duration_days"`
	Status       string     `json:"status"`
	CreatedBy    string     `json:"created_by_email"`
	AssignedTo   string     `json:"assigned_to_email,omitempty"`
	RedeemedBy   string     `json:"redeemed_by_email,omitempty"`
	RedeemedAt   *time.Time `json:"redeemed_at,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// handleAdminVouchers: daftar voucher semua sumber (terbaru dulu).
// Status 'active' yang masa berlakunya lewat ditampilkan 'expired'.
func (a *API) handleAdminVouchers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := a.pool.Query(r.Context(), `
		select v.id, v.code, v.source, v.duration_days,
		       case when v.status = 'active' and v.expires_at < now()
		            then 'expired' else v.status end,
		       coalesce(cb.email, ''), coalesce(at.email, ''),
		       coalesce(rb.email, ''), v.redeemed_at, v.expires_at, v.created_at
		from vouchers v
		left join profiles cb on cb.id = v.created_by
		left join profiles at on at.id = v.assigned_to
		left join profiles rb on rb.id = v.redeemed_by
		order by v.created_at desc
		limit $1`, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memuat voucher")
		return
	}
	defer rows.Close()
	out := []adminVoucher{}
	for rows.Next() {
		var v adminVoucher
		if err := rows.Scan(&v.ID, &v.Code, &v.Source, &v.DurationDays, &v.Status,
			&v.CreatedBy, &v.AssignedTo, &v.RedeemedBy,
			&v.RedeemedAt, &v.ExpiresAt, &v.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca voucher")
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"vouchers": out})
}

// handleAdminVoucherCreate: buat voucher hadiah (source 'admin')
// untuk dibagikan ke pengguna — ditukar lewat menu Premium di
// aplikasi dan langsung menambah hari premium penukar.
func (a *API) handleAdminVoucherCreate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		DurationDays  int    `json:"duration_days"`
		Quantity      int    `json:"quantity"`
		AssignedEmail string `json:"assigned_email"`
		ValidDays     int    `json:"valid_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	if req.DurationDays < 1 || req.DurationDays > 3650 {
		writeErr(w, http.StatusBadRequest, "durasi harus 1–3650 hari")
		return
	}
	if req.Quantity == 0 {
		req.Quantity = 1
	}
	if req.Quantity < 1 || req.Quantity > 50 {
		writeErr(w, http.StatusBadRequest, "jumlah harus 1–50 kode")
		return
	}
	if req.ValidDays < 0 {
		writeErr(w, http.StatusBadRequest, "masa berlaku tidak boleh negatif")
		return
	}
	ctx := r.Context()
	var assignedID, assignedEmail string
	if req.AssignedEmail != "" {
		err := a.pool.QueryRow(ctx, `
			select id, email from profiles where lower(email) = lower($1)`,
			req.AssignedEmail).Scan(&assignedID, &assignedEmail)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "pengguna dengan email itu tidak ditemukan")
			return
		}
	}
	var expiresAt *time.Time
	if req.ValidDays > 0 {
		t := time.Now().Add(time.Duration(req.ValidDays) * 24 * time.Hour)
		expiresAt = &t
	}
	created := []adminVoucher{}
	for i := 0; i < req.Quantity; i++ {
		var v adminVoucher
		var err error
		// Kode unik — tabrakan sangat jarang; coba ulang beberapa kali.
		for attempt := 0; attempt < 5; attempt++ {
			err = a.pool.QueryRow(ctx, `
				insert into vouchers (code, source, duration_days, created_by, assigned_to, expires_at)
				values ($1, 'admin', $2, $3::uuid, $4::uuid, $5)
				returning id, code, source, duration_days, status, created_at`,
				giftCode(), req.DurationDays, p.ID, nullStr(assignedID), expiresAt,
			).Scan(&v.ID, &v.Code, &v.Source, &v.DurationDays, &v.Status, &v.CreatedAt)
			if err == nil {
				break
			}
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membuat voucher")
			return
		}
		v.CreatedBy = p.Email
		v.AssignedTo = assignedEmail
		v.ExpiresAt = expiresAt
		created = append(created, v)
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'voucher_create', $2, $3)`,
		p.ID, created[0].ID, map[string]any{
			"duration_days": req.DurationDays, "quantity": req.Quantity,
			"assigned_email": assignedEmail, "valid_days": req.ValidDays,
		})
	writeJSON(w, http.StatusOK, map[string]any{"vouchers": created})
}

// handleAdminVoucherDelete: hapus voucher yang belum ditukar
// (yang sudah ditukar adalah riwayat — tidak bisa dihapus).
func (a *API) handleAdminVoucherDelete(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var code, status string
	err := a.pool.QueryRow(ctx, `
		select code, status from vouchers where id = $1::uuid`, id).Scan(&code, &status)
	if err != nil {
		writeErr(w, http.StatusNotFound, "voucher tidak ditemukan")
		return
	}
	if status == "redeemed" {
		writeErr(w, http.StatusBadRequest, "voucher sudah ditukar — tidak bisa dihapus")
		return
	}
	if _, err := a.pool.Exec(ctx, `delete from vouchers where id = $1::uuid`, id); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus voucher")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'voucher_delete', $2, $3)`,
		p.ID, id, map[string]any{"code": code, "status": status})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type adminPlan struct {
	ID              string `json:"id"`
	Days            int    `json:"days"`
	Price           int    `json:"price"`
	Label           string `json:"label"`
	IsActive        bool   `json:"is_active"`
	SortOrder       int    `json:"sort_order"`
	OriginalPrice   int    `json:"original_price"`
	DiscountPercent int    `json:"discount_percent"`
}

// handleAdminPlans: daftar paket premium (tabel subscription_plans).
func (a *API) handleAdminPlans(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		select id, days, price, label, is_active, sort_order, coalesce(original_price, 0), coalesce(discount_percent, 0)
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
			&pl.IsActive, &pl.SortOrder, &pl.OriginalPrice, &pl.DiscountPercent); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membaca paket")
			return
		}
		out = append(out, pl)
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": out})
}

// handleAdminPlanCreate: tambah paket premium baru (untuk promo event,
// mis. paket khusus film GJLS). id unik, pola [a-z0-9_-].
func (a *API) handleAdminPlanCreate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		ID              string `json:"id"`
		Label           string `json:"label"`
		Days            int    `json:"days"`
		Price           int    `json:"price"`
		OriginalPrice   int    `json:"original_price"`
		DiscountPercent int    `json:"discount_percent"`
		IsActive        *bool  `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	req.ID = strings.TrimSpace(strings.ToLower(req.ID))
	req.Label = strings.TrimSpace(req.Label)
	if req.ID == "" || len(req.ID) > 64 || !validPlanID(req.ID) {
		writeErr(w, http.StatusBadRequest, "id paket tidak valid (huruf kecil, angka, - atau _)")
		return
	}
	if req.Label == "" || req.Days <= 0 || req.Price < 0 || req.OriginalPrice < 0 ||
		req.DiscountPercent < 0 || req.DiscountPercent > 100 {
		writeErr(w, http.StatusBadRequest, "label/durasi/harga/diskon tidak valid")
		return
	}
	ctx := r.Context()
	var exists bool
	if err := a.pool.QueryRow(ctx, `select exists(select 1 from subscription_plans where id = $1)`, req.ID).Scan(&exists); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memeriksa paket")
		return
	}
	if exists {
		writeErr(w, http.StatusConflict, "id paket sudah dipakai")
		return
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	var pl adminPlan
	err := a.pool.QueryRow(ctx, `
		insert into subscription_plans (id, days, price, label, is_active, sort_order, original_price, discount_percent)
		values ($1, $2, $3, $4, $5, coalesce((select max(sort_order) from subscription_plans), 0) + 1, $6, $7)
		returning id, days, price, label, is_active, sort_order, original_price, discount_percent`,
		req.ID, req.Days, req.Price, req.Label, active, req.OriginalPrice, req.DiscountPercent,
	).Scan(&pl.ID, &pl.Days, &pl.Price, &pl.Label, &pl.IsActive, &pl.SortOrder, &pl.OriginalPrice, &pl.DiscountPercent)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat paket")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'plan_create', $2, $3)`,
		p.ID, req.ID, map[string]any{
			"days": req.Days, "price": req.Price, "label": req.Label,
			"original_price": req.OriginalPrice, "discount_percent": req.DiscountPercent,
		})
	writeJSON(w, http.StatusCreated, map[string]any{"plan": pl})
}

func validPlanID(id string) bool {
	for i, c := range id {
		ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok || (i == 0 && (c == '-' || c == '_')) {
			return false
		}
	}
	return true
}

// handleAdminPlanDelete: hapus paket premium. Ditolak bila masih ada
// pelanggan dengan langganan aktif di paket itu — nonaktifkan saja
// (is_active=false) supaya paket hilang dari toko tanpa mengganggu
// pelanggan yang sudah bayar. Premium yang sudah aktif tidak dicabut.
func (a *API) handleAdminPlanDelete(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var aktif int
	if err := a.pool.QueryRow(ctx, `select count(*) from subscriptions where plan = $1 and status = 'active'`, id).Scan(&aktif); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal memeriksa paket")
		return
	}
	if aktif > 0 {
		writeErr(w, http.StatusConflict, fmt.Sprintf("paket masih dipakai %d pelanggan aktif — nonaktifkan saja", aktif))
		return
	}
	tag, err := a.pool.Exec(ctx, `delete from subscription_plans where id = $1`, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menghapus paket")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "paket tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'plan_delete', $2, $3)`,
		p.ID, id, map[string]any{"pelanggan_aktif": aktif})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleAdminPlanUpdate: ubah harga/label/status aktif/harga asli/
// diskon satu paket. price tetap harga yang ditagih; dashboard
// menghitungnya dari harga asli + diskon sebelum mengirim.
func (a *API) handleAdminPlanUpdate(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		Price           *int    `json:"price"`
		Label           *string `json:"label"`
		IsActive        *bool   `json:"is_active"`
		OriginalPrice   *int    `json:"original_price"`
		DiscountPercent *int    `json:"discount_percent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	if req.Price != nil && *req.Price < 0 {
		writeErr(w, http.StatusBadRequest, "harga tidak boleh negatif")
		return
	}
	if req.OriginalPrice != nil && *req.OriginalPrice < 0 {
		writeErr(w, http.StatusBadRequest, "harga asli tidak boleh negatif")
		return
	}
	if req.DiscountPercent != nil && (*req.DiscountPercent < 0 || *req.DiscountPercent > 100) {
		writeErr(w, http.StatusBadRequest, "diskon harus 0-100%")
		return
	}
	ctx := r.Context()
	var pl adminPlan
	err := a.pool.QueryRow(ctx, `
		update subscription_plans
		set price = coalesce($2, price),
		    label = coalesce($3, label),
		    is_active = coalesce($4, is_active),
		    original_price = coalesce($5, original_price),
		    discount_percent = coalesce($6, discount_percent),
		    updated_at = now()
		where id = $1
		returning id, days, price, label, is_active, sort_order, original_price, discount_percent`,
		id, req.Price, req.Label, req.IsActive, req.OriginalPrice, req.DiscountPercent,
	).Scan(&pl.ID, &pl.Days, &pl.Price, &pl.Label, &pl.IsActive, &pl.SortOrder, &pl.OriginalPrice, &pl.DiscountPercent)
	if err != nil {
		writeErr(w, http.StatusNotFound, "paket tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'plan_update', $2, $3)`,
		p.ID, id, map[string]any{
			"price": req.Price, "label": req.Label, "is_active": req.IsActive,
			"original_price": req.OriginalPrice, "discount_percent": req.DiscountPercent,
		})
	writeJSON(w, http.StatusOK, map[string]any{"plan": pl})
}
