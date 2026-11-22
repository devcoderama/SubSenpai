package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/payments"
)

// paymentMethod: satu channel Tokopay dari tabel payment_methods
// (migrasi 0015) — daftarnya dikelola lewat database agar admin
// bisa mengaktifkan/menonaktifkan channel tanpa rilis ulang.
type paymentMethod struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Group    string `json:"group"`
	IconURL  string `json:"icon_url"`
	IsActive bool   `json:"is_active"`
}

// paymentGroup menebak grup channel untuk daftar cadangan.
func paymentGroup(code string) string {
	switch {
	case strings.HasPrefix(code, "QRIS"):
		return "qris"
	case code == "ALFAMART" || code == "INDOMARET":
		return "retail"
	case code == "TELKOMSEL" || code == "AXIS" || code == "XL" || code == "TRI":
		return "pulsa"
	case strings.Contains(code, "VA"):
		return "va"
	default:
		return "ewallet"
	}
}

// fallbackPaymentMethods: daftar cadangan dari peta channel di
// kode, dipakai hanya bila tabel payment_methods belum ada/kosong.
func fallbackPaymentMethods() []paymentMethod {
	out := make([]paymentMethod, 0, len(payments.TokopayChannels))
	for code, name := range payments.TokopayChannels {
		out = append(out, paymentMethod{
			Code: code, Name: name, Group: paymentGroup(code), IsActive: true,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// loadPaymentMethods membaca metode dari database; activeOnly=true
// menyaring yang aktif saja (untuk publik & validasi checkout).
func (a *API) loadPaymentMethods(ctx context.Context, activeOnly bool) []paymentMethod {
	q := `select code, name, grup, icon_url, is_active from payment_methods`
	if activeOnly {
		q += ` where is_active`
	}
	q += ` order by sort_order, code`
	rows, err := a.pool.Query(ctx, q)
	if err != nil {
		return fallbackPaymentMethods()
	}
	defer rows.Close()
	var out []paymentMethod
	for rows.Next() {
		var m paymentMethod
		if err := rows.Scan(&m.Code, &m.Name, &m.Group, &m.IconURL, &m.IsActive); err != nil {
			return fallbackPaymentMethods()
		}
		out = append(out, m)
	}
	if rows.Err() != nil || len(out) == 0 {
		return fallbackPaymentMethods()
	}
	return out
}

// tokopayChannelActive: channel dikenal DAN sedang aktif. Bila
// tabel belum ada (migrasi 0015 belum jalan), jatuh ke peta kode.
func (a *API) tokopayChannelActive(ctx context.Context, code string) bool {
	var active bool
	err := a.pool.QueryRow(ctx, `
		select is_active from payment_methods where code = $1`, code).Scan(&active)
	if err != nil {
		_, known := payments.TokopayChannels[code]
		return known
	}
	return active
}

// handlePaymentMethods: daftar metode pembayaran aktif (publik).
func (a *API) handlePaymentMethods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"methods": a.loadPaymentMethods(r.Context(), true),
	})
}

// handleAdminPaymentMethods: semua metode termasuk nonaktif (admin).
func (a *API) handleAdminPaymentMethods(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if p == nil || p.Role != "admin" {
		writeErr(w, http.StatusForbidden, "khusus admin")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"methods": a.loadPaymentMethods(r.Context(), false),
	})
}

// handleAdminPaymentMethodUpdate: aktif/nonaktifkan satu metode.
func (a *API) handleAdminPaymentMethodUpdate(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	if p == nil || p.Role != "admin" {
		writeErr(w, http.StatusForbidden, "khusus admin")
		return
	}
	code := strings.ToUpper(chi.URLParam(r, "code"))
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	ctx := r.Context()
	var m paymentMethod
	err := a.pool.QueryRow(ctx, `
		update payment_methods
		set is_active = $2, updated_at = now()
		where code = $1
		returning code, name, grup, icon_url, is_active`,
		code, req.IsActive).Scan(&m.Code, &m.Name, &m.Group, &m.IconURL, &m.IsActive)
	if err != nil {
		writeErr(w, http.StatusNotFound, "metode tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'payment_method_toggle', $2, $3)`,
		p.ID, code, map[string]any{"is_active": req.IsActive})
	writeJSON(w, http.StatusOK, map[string]any{"method": m})
}
