package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"time"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/billing"
)

// handlePlans: daftar paket premium (publik).
func (a *API) handlePlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"plans": billing.Plans})
}

// handlePremiumStatus: tingkat akses akun + sisa masa premium + trial.
func (a *API) handlePremiumStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var until *time.Time
	var trialUsed *time.Time
	err := a.pool.QueryRow(r.Context(), `
		select premium_until, trial_used_at from profiles where id = $1::uuid`, p.ID,
	).Scan(&until, &trialUsed)
	if err != nil {
		writeErr(w, http.StatusNotFound, "profil tidak ditemukan")
		return
	}
	premium := until != nil && until.After(time.Now())
	t := string(tierFree)
	if premium {
		t = string(tierPremium)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tier":            t,
		"is_premium":      premium,
		"premium_until":   until,
		"trial_available": trialUsed == nil,
	})
}

// handleTrial: klaim uji coba premium 3 hari, sekali per akun.
func (a *API) handleTrial(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var trialUsed *time.Time
	if err := a.pool.QueryRow(ctx, `
		select trial_used_at from profiles where id = $1::uuid`, p.ID).Scan(&trialUsed); err != nil {
		writeErr(w, http.StatusNotFound, "profil tidak ditemukan")
		return
	}
	if trialUsed != nil {
		writeErr(w, http.StatusConflict, "uji coba premium sudah pernah dipakai")
		return
	}
	if _, err := a.pool.Exec(ctx, `
		update profiles set trial_used_at = now() where id = $1::uuid`, p.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	until, err := a.extendPremium(p.ID, 3, "trial", "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"premium_until": until})
}

// checkoutReq adalah isi permintaan pembuatan sesi pembayaran.
type checkoutReq struct {
	PlanID        string `json:"plan_id"`
	Provider      string `json:"provider"`
	Purpose       string `json:"purpose"` // subscription | voucher
	RecipientCode string `json:"recipient_code"`
}

// handleCheckout membuat sesi pembayaran. GoPay: nominal unik
// (harga + kode 1–999) untuk pencocokan polling; Midtrans/Xendit:
// transaksi dibuat ke gateway dan URL bayar dikembalikan.
func (a *API) handleCheckout(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req checkoutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "isi permintaan tidak valid")
		return
	}
	plan, ok := billing.PlanByID(req.PlanID)
	if !ok {
		writeErr(w, http.StatusBadRequest, "paket tidak dikenal")
		return
	}
	if req.Purpose == "" {
		req.Purpose = "subscription"
	}
	if req.Purpose != "subscription" && req.Purpose != "voucher" {
		writeErr(w, http.StatusBadRequest, "purpose tidak dikenal")
		return
	}
	switch req.Provider {
	case "gopay":
	case "midtrans":
		if a.cfg.MidtransServerKey == "" {
			writeErr(w, http.StatusServiceUnavailable, "Midtrans belum dikonfigurasi di server")
			return
		}
	case "xendit":
		if a.cfg.XenditSecretKey == "" {
			writeErr(w, http.StatusServiceUnavailable, "Xendit belum dikonfigurasi di server")
			return
		}
	default:
		writeErr(w, http.StatusBadRequest, "provider tidak dikenal")
		return
	}
	ctx := r.Context()
	// Penerima hadiah voucher harus ada sebelum sesi dibuat.
	var recipientID *string
	if req.Purpose == "voucher" {
		if req.RecipientCode == "" {
			writeErr(w, http.StatusBadRequest, "recipient_code wajib untuk hadiah voucher")
			return
		}
		var rid string
		if err := a.pool.QueryRow(ctx, `
			select id from profiles where friend_code = $1`, req.RecipientCode).Scan(&rid); err != nil {
			writeErr(w, http.StatusNotFound, "kode teman penerima tidak ditemukan")
			return
		}
		recipientID = &rid
	}
	// Kode unik untuk pencocokan GoPay (diusahakan tidak bertabrakan
	// dengan sesi gopay pending lain pada nominal yang sama).
	unique := 1
	amount := plan.Price
	for attempt := 0; attempt < 20; attempt++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(999))
		unique = int(n.Int64()) + 1
		amount = plan.Price + unique
		if req.Provider != "gopay" {
			amount = plan.Price
			break
		}
		var clash bool
		_ = a.pool.QueryRow(ctx, `
			select exists(select 1 from payment_sessions
				where provider = 'gopay' and status = 'pending' and amount = $1)`, amount).Scan(&clash)
		if !clash {
			break
		}
	}
	expires := time.Now().Add(15 * time.Minute)
	var sessionID string
	if err := a.pool.QueryRow(ctx, `
		insert into payment_sessions (user_id, purpose, provider, amount, unique_code, plan, expires_at)
		values ($1::uuid, $2, $3, $4, $5, $6, $7)
		returning id`, p.ID, req.Purpose, req.Provider, amount, unique, plan.ID, expires).Scan(&sessionID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat sesi pembayaran")
		return
	}
	resp := map[string]any{
		"session_id":  sessionID,
		"provider":    req.Provider,
		"purpose":     req.Purpose,
		"plan_id":     plan.ID,
		"amount":      amount,
		"unique_code": unique,
		"expires_at":  expires,
	}
	// Hadiah voucher: voucher terbit sekarang, bisa ditukar setelah sesi lunas.
	if req.Purpose == "voucher" {
		code := giftCode()
		if _, err := a.pool.Exec(ctx, `
			insert into vouchers (code, source, duration_days, created_by, assigned_to, payment_ref)
			values ($1, 'purchase', $2, $3::uuid, $4::uuid, $5)`,
			code, plan.Days, p.ID, *recipientID, sessionID); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal membuat voucher hadiah")
			return
		}
		resp["voucher_code"] = code
	}
	switch req.Provider {
	case "gopay":
		if a.gopay != nil && a.gopay.Configured() {
			if qris, err := a.gopay.CreatePayment(ctx, amount); err == nil && qris != "" {
				resp["qris"] = qris
			}
		} else {
			resp["note"] = "QRIS otomatis belum dikonfigurasi — transfer tepat nominal unik ini ke QRIS merchant; aktivasi lewat pencocokan polling."
		}
	case "midtrans":
		redirect, token, err := a.midtransSnap(ctx, sessionID, amount, p.Email)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "gateway Midtrans gagal: "+err.Error())
			return
		}
		resp["redirect_url"] = redirect
		resp["snap_token"] = token
	case "xendit":
		invoiceURL, err := a.xenditInvoice(ctx, sessionID, amount, p.Email)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "gateway Xendit gagal: "+err.Error())
			return
		}
		resp["invoice_url"] = invoiceURL
	}
	writeJSON(w, http.StatusOK, resp)
}

func giftCode() string {
	const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, 10)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[n.Int64()]
	}
	return "SS-" + string(b)
}

// handleVoucherRedeem: tukar kode voucher menjadi hari premium.
func (a *API) handleVoucherRedeem(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeErr(w, http.StatusBadRequest, "code wajib diisi")
		return
	}
	ctx := r.Context()
	var v struct {
		ID        string
		Days      int
		Source    string
		Status    string
		Assigned  *string
		ExpiresAt *time.Time
		PaymentRef *string
	}
	err := a.pool.QueryRow(ctx, `
		select id, duration_days, source, status, assigned_to, expires_at, payment_ref
		from vouchers where code = $1`, req.Code,
	).Scan(&v.ID, &v.Days, &v.Source, &v.Status, &v.Assigned, &v.ExpiresAt, &v.PaymentRef)
	if err != nil {
		writeErr(w, http.StatusNotFound, "voucher tidak ditemukan")
		return
	}
	if v.Status != "active" {
		writeErr(w, http.StatusConflict, "voucher tidak aktif (status: "+v.Status+")")
		return
	}
	if v.ExpiresAt != nil && v.ExpiresAt.Before(time.Now()) {
		writeErr(w, http.StatusConflict, "voucher sudah kedaluwarsa")
		return
	}
	if v.Assigned != nil && *v.Assigned != p.ID {
		writeErr(w, http.StatusForbidden, "voucher ini ditujukan untuk akun lain")
		return
	}
	if v.Source == "purchase" {
		// Voucher beli/hadiah hanya bisa ditukar setelah pembayarannya lunas.
		var paid bool
		if v.PaymentRef != nil {
			_ = a.pool.QueryRow(ctx, `
				select exists(select 1 from payment_sessions
					where id = $1::uuid and status = 'paid')`, *v.PaymentRef).Scan(&paid)
		}
		if !paid {
			writeErr(w, http.StatusConflict, "pembayaran voucher belum selesai")
			return
		}
	}
	until, err := a.extendPremium(p.ID, v.Days, "voucher", v.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	if _, err := a.pool.Exec(ctx, `
		update vouchers
		set status = 'redeemed', redeemed_by = $2::uuid, redeemed_at = now()
		where id = $1::uuid`, v.ID, p.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"premium_until": until, "days_added": v.Days})
}

// handleVouchersMine: voucher aktif yang ditujukan ke akun ini.
func (a *API) handleVouchersMine(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	type voucher struct {
		Code      string     `json:"code"`
		Days      int        `json:"duration_days"`
		Source    string     `json:"source"`
		ExpiresAt *time.Time `json:"expires_at,omitempty"`
	}
	out := []voucher{}
	rows, err := a.pool.Query(r.Context(), `
		select code, duration_days, source, expires_at from vouchers
		where assigned_to = $1::uuid and status = 'active'
		order by created_at desc`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v voucher
			if err := rows.Scan(&v.Code, &v.Days, &v.Source, &v.ExpiresAt); err == nil {
				out = append(out, v)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"vouchers": out})
}
