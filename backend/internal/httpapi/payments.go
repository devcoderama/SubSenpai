package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/billing"
)

// handlePlans: daftar paket premium (publik) — dari database
// (tabel subscription_plans, migrasi 0014).
func (a *API) handlePlans(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"plans": billing.LoadPlans(r.Context(), a.pool)})
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
	Channel       string `json:"channel"` // kode channel Tokopay (bawaan QRIS)
	// Quantity khusus purpose voucher: jumlah kode yang diterbitkan
	// (kuota orang yang bisa dibagi), 1–10, bawaan 1.
	Quantity int `json:"quantity"`
}

// handleCheckout membuat sesi pembayaran. GoPay: nominal unik
// (harga + kode 1–999) untuk pencocokan polling; Midtrans/Xendit:
// transaksi dibuat ke gateway dan URL bayar dikembalikan.
// expireSession menandai sesi pembayaran yang gagal dibuat di
// gateway sebagai kedaluwarsa, agar tidak tertinggal yatim dalam
// status pending (riwayat bersih + tidak menghalangi apa pun).
func (a *API) expireSession(ctx context.Context, sessionID string) {
	_, _ = a.pool.Exec(ctx, `
		update payment_sessions set status = 'expired'
		where id = $1::uuid and status = 'pending'`, sessionID)
}

func (a *API) handleCheckout(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req checkoutReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "isi permintaan tidak valid")
		return
	}
	plan, ok := billing.PlanByIDCtx(r.Context(), a.pool, req.PlanID)
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
	case "tokopay":
		if a.tokopay == nil || !a.tokopay.Configured() {
			writeErr(w, http.StatusServiceUnavailable, "Tokopay belum dikonfigurasi di server")
			return
		}
		if req.Channel == "" {
			req.Channel = "QRIS"
		}
		if !a.tokopayChannelActive(r.Context(), req.Channel) {
			writeErr(w, http.StatusBadRequest, "channel Tokopay tidak dikenal atau sedang nonaktif")
			return
		}
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
	// Voucher: kuota = jumlah kode yang diterbitkan untuk dibagikan
	// (permintaan Rams 2026-11-22). recipient_code kini OPSIONAL —
	// bila diisi, semua kode ditujukan ke akun itu; bila kosong,
	// kode bebas dibagikan ke siapa saja.
	qty := 1
	if req.Purpose == "voucher" {
		if req.Quantity < 0 || req.Quantity > 10 {
			writeErr(w, http.StatusBadRequest, "quantity voucher harus 1–10")
			return
		}
		if req.Quantity > 0 {
			qty = req.Quantity
		}
	}
	var recipientID *string
	if req.Purpose == "voucher" && req.RecipientCode != "" {
		var rid string
		if err := a.pool.QueryRow(ctx, `
			select id from profiles where friend_code = $1`, req.RecipientCode).Scan(&rid); err != nil {
			writeErr(w, http.StatusNotFound, "kode teman penerima tidak ditemukan")
			return
		}
		recipientID = &rid
	}
	// Harga dasar: voucher dikali kuota.
	base := plan.Price
	if req.Purpose == "voucher" {
		base = plan.Price * qty
	}
	// Kode unik untuk pencocokan GoPay (diusahakan tidak bertabrakan
	// dengan sesi gopay pending lain pada nominal yang sama).
	unique := 1
	amount := base
	for attempt := 0; attempt < 20; attempt++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(999))
		unique = int(n.Int64()) + 1
		amount = base + unique
		if req.Provider != "gopay" {
			amount = base
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
	// Voucher: kode terbit sekarang (sebanyak kuota), bisa ditukar
	// setelah sesi lunas. assigned_to terisi hanya bila recipient_code
	// diberikan; kosong = bebas dibagikan ke siapa saja.
	if req.Purpose == "voucher" {
		codes := make([]string, 0, qty)
		for i := 0; i < qty; i++ {
			code := giftCode()
			if _, err := a.pool.Exec(ctx, `
				insert into vouchers (code, source, duration_days, created_by, assigned_to, payment_ref)
				values ($1, 'purchase', $2, $3::uuid, $4::uuid, $5)`,
				code, plan.Days, p.ID, recipientID, sessionID); err != nil {
				a.expireSession(ctx, sessionID)
				writeErr(w, http.StatusInternalServerError, "gagal membuat voucher")
				return
			}
			codes = append(codes, code)
		}
		resp["voucher_code"] = codes[0]
		resp["voucher_codes"] = codes
		resp["quantity"] = qty
	}
	switch req.Provider {
	case "tokopay":
		reffID := strings.ReplaceAll(sessionID, "-", "")
		var displayName *string
		_ = a.pool.QueryRow(ctx, `
			select display_name from profiles where id = $1::uuid`, p.ID).Scan(&displayName)
		name := p.Email
		if displayName != nil && *displayName != "" {
			name = *displayName
		}
		// customer_phone wajib di API Advanced Tokopay; profil belum
		// menyimpan nomor HP — dipakai penanda sampai profil punya HP.
		order, err := a.tokopay.CreateOrder(ctx, req.Channel, reffID, amount, name, p.Email, "080000000000", expires.Unix())
		if err != nil {
			a.expireSession(ctx, sessionID)
			writeErr(w, http.StatusBadGateway, "gateway Tokopay gagal: "+err.Error())
			return
		}
		_, _ = a.pool.Exec(ctx, `
			update payment_sessions set matched_ref = $2 where id = $1::uuid`,
			sessionID, order.TrxID)
		resp["channel"] = req.Channel
		resp["trx_id"] = order.TrxID
		resp["pay_url"] = order.PayURL
		resp["total_bayar"] = order.TotalBayar
		if order.NomorVA != "" {
			resp["nomor_va"] = order.NomorVA
		}
		if order.QRString != "" {
			resp["qr_string"] = order.QRString
		}
		if order.QRLink != "" {
			resp["qr_link"] = order.QRLink
		}
		if order.CheckoutURL != "" {
			resp["checkout_url"] = order.CheckoutURL
		}
		if order.Panduan != "" {
			resp["panduan"] = order.Panduan
		}
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
			a.expireSession(ctx, sessionID)
			writeErr(w, http.StatusBadGateway, "gateway Midtrans gagal: "+err.Error())
			return
		}
		resp["redirect_url"] = redirect
		resp["snap_token"] = token
	case "xendit":
		invoiceURL, err := a.xenditInvoice(ctx, sessionID, amount, p.Email)
		if err != nil {
			a.expireSession(ctx, sessionID)
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

// handleVouchersMine: semua voucher yang berkaitan dengan akun ini —
// yang DIBELI user (created_by, untuk dibagikan; lengkap dengan
// status pembayaran & penukar) dan yang DITUJUKAN ke user
// (assigned_to). Dipakai halaman "Voucher Saya".
func (a *API) handleVouchersMine(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	type voucher struct {
		Code           string     `json:"code"`
		Days           int        `json:"duration_days"`
		Source         string     `json:"source"`
		Status         string     `json:"status"`
		ExpiresAt      *time.Time `json:"expires_at,omitempty"`
		CreatedByMe    bool       `json:"created_by_me"`
		ForMe          bool       `json:"for_me"`
		PaymentStatus  *string    `json:"payment_status,omitempty"`
		RedeemedByName *string    `json:"redeemed_by_name,omitempty"`
	}
	out := []voucher{}
	rows, err := a.pool.Query(r.Context(), `
		select v.code, v.duration_days, v.source, v.status, v.expires_at,
		       (v.created_by = $1::uuid), (v.assigned_to = $1::uuid),
		       ps.status, rp.display_name
		from vouchers v
		left join payment_sessions ps on ps.id::text = v.payment_ref
		left join profiles rp on rp.id = v.redeemed_by
		where v.created_by = $1::uuid or v.assigned_to = $1::uuid
		order by v.created_at desc
		limit 100`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v voucher
			if err := rows.Scan(&v.Code, &v.Days, &v.Source, &v.Status, &v.ExpiresAt,
				&v.CreatedByMe, &v.ForMe, &v.PaymentStatus, &v.RedeemedByName); err == nil {
				out = append(out, v)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"vouchers": out})
}

// handleTransactions: riwayat sesi pembayaran milik user (untuk
// halaman Riwayat Transaksi) — status segar bisa diminta lewat
// GET /api/v1/payments/status/{id} (cek pembayaran).
func (a *API) handleTransactions(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	labels := map[string]string{}
	for _, pl := range billing.LoadPlans(r.Context(), a.pool) {
		labels[pl.ID] = pl.Label
	}
	type transaction struct {
		ID        string    `json:"id"`
		Purpose   string    `json:"purpose"`
		Provider  string    `json:"provider"`
		PlanID    string    `json:"plan_id"`
		PlanLabel string    `json:"plan_label"`
		Amount    int       `json:"amount"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	out := []transaction{}
	rows, err := a.pool.Query(r.Context(), `
		select id, purpose, provider, coalesce(plan,''), amount, status, created_at, expires_at
		from payment_sessions
		where user_id = $1::uuid
		order by created_at desc
		limit 50`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var t transaction
			if err := rows.Scan(&t.ID, &t.Purpose, &t.Provider, &t.PlanID, &t.Amount,
				&t.Status, &t.CreatedAt, &t.ExpiresAt); err == nil {
				t.PlanLabel = labels[t.PlanID]
				if t.PlanLabel == "" {
					t.PlanLabel = t.PlanID
				}
				out = append(out, t)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": out})
}
