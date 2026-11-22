package httpapi

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/billing"
	"subsenpai/backend/internal/payments"
)

var gatewayHTTP = &http.Client{Timeout: 30 * time.Second}

// midtransSnap membuat transaksi Snap Midtrans; order_id = id sesi
// pembayaran agar webhook bisa mengaktifkan sesi yang tepat.
func (a *API) midtransSnap(ctx context.Context, orderID string, amount int, email string) (string, string, error) {
	base := "https://app.sandbox.midtrans.com"
	if a.cfg.MidtransProduction {
		base = "https://app.midtrans.com"
	}
	payload := map[string]any{
		"transaction_details": map[string]any{"order_id": orderID, "gross_amount": amount},
		"customer_details":    map[string]any{"email": email},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/snap/v1/transactions", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.SetBasicAuth(a.cfg.MidtransServerKey, "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := gatewayHTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Token       string `json:"token"`
		RedirectURL string `json:"redirect_url"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", "", err
	}
	return out.RedirectURL, out.Token, nil
}

// xenditInvoice membuat invoice Xendit; external_id = id sesi pembayaran.
func (a *API) xenditInvoice(ctx context.Context, externalID string, amount int, email string) (string, error) {
	payload := map[string]any{
		"external_id": externalID,
		"amount":      amount,
		"payer_email": email,
		"description": "SubSenpai Premium",
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.xendit.co/v2/invoices", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(a.cfg.XenditSecretKey, "")
	req.Header.Set("Content-Type", "application/json")
	resp, err := gatewayHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		InvoiceURL string `json:"invoice_url"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.InvoiceURL, nil
}

// handleMidtransWebhook menerima notifikasi Midtrans (di luar envelope —
// gateway tidak bicara xdata). Tanda tangan SHA512 diverifikasi dulu;
// status settlement/capture mengaktifkan sesi.
func (a *API) handleMidtransWebhook(w http.ResponseWriter, r *http.Request) {
	if a.cfg.MidtransServerKey == "" {
		writeErr(w, http.StatusServiceUnavailable, "Midtrans belum dikonfigurasi")
		return
	}
	var n struct {
		OrderID           string `json:"order_id"`
		StatusCode        string `json:"status_code"`
		GrossAmount       string `json:"gross_amount"`
		SignatureKey      string `json:"signature_key"`
		TransactionStatus string `json:"transaction_status"`
		FraudStatus       string `json:"fraud_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		writeErr(w, http.StatusBadRequest, "isi notifikasi tidak valid")
		return
	}
	sum := sha512.Sum512([]byte(n.OrderID + n.StatusCode + n.GrossAmount + a.cfg.MidtransServerKey))
	if hex.EncodeToString(sum[:]) != n.SignatureKey {
		writeErr(w, http.StatusForbidden, "tanda tangan tidak valid")
		return
	}
	paid := n.TransactionStatus == "settlement" ||
		(n.TransactionStatus == "capture" && (n.FraudStatus == "accept" || n.FraudStatus == ""))
	if paid {
		if err := billing.ActivateSession(r.Context(), a.pool, n.OrderID, n.OrderID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// sessionIDFromReff mengubah reff_id Tokopay (id sesi kita tanpa tanda
// hubung) kembali ke bentuk UUID. String kosong bila formatnya asing.
func sessionIDFromReff(reff string) string {
	if len(reff) != 32 {
		return ""
	}
	for _, c := range reff {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return ""
		}
	}
	return reff[0:8] + "-" + reff[8:12] + "-" + reff[12:16] + "-" + reff[16:20] + "-" + reff[20:32]
}

// handleTokopayWebhook menerima callback Tokopay (gateway utama).
// Keaslian dari signature md5(merchant_id:secret:reff_id) di body;
// status Success/Completed mengaktifkan sesi. Balasan wajib berbentuk
// JSON {"status":true} agar Tokopay berhenti mengirim ulang.
func (a *API) handleTokopayWebhook(w http.ResponseWriter, r *http.Request) {
	fail := func(code int) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"status":false}`))
	}
	if a.tokopay == nil || !a.tokopay.Configured() {
		fail(http.StatusServiceUnavailable)
		return
	}
	var n struct {
		ReffID    string `json:"reff_id"`
		Status    string `json:"status"`
		Signature string `json:"signature"`
		TrxID     string `json:"trx_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil || n.ReffID == "" {
		fail(http.StatusBadRequest)
		return
	}
	if !payments.VerifyTokopayCallback(a.cfg.TokopayMerchantID, a.cfg.TokopaySecret, n.ReffID, n.Signature) {
		fail(http.StatusForbidden)
		return
	}
	if payments.TokopayPaid(n.Status) {
		sessionID := sessionIDFromReff(n.ReffID)
		if sessionID != "" {
			var exists bool
			_ = a.pool.QueryRow(r.Context(), `
				select exists(select 1 from payment_sessions where id = $1::uuid)`,
				sessionID).Scan(&exists)
			if exists {
				if err := billing.ActivateSession(r.Context(), a.pool, sessionID, n.TrxID); err != nil {
					fail(http.StatusInternalServerError)
					return
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":true}`))
}

// handlePaymentStatus: status sesi milik user; untuk Tokopay, status
// segar ditanyakan ke gateway (cek status order) dan sesi diaktifkan
// bila ternyata sudah lunas tapi callback belum sampai.
func (a *API) handlePaymentStatus(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	sessionID := chi.URLParam(r, "sessionId")
	var userID, provider, status string
	var trxRef *string
	err := a.pool.QueryRow(r.Context(), `
		select user_id, provider, status, matched_ref
		from payment_sessions where id = $1::uuid`, sessionID,
	).Scan(&userID, &provider, &status, &trxRef)
	if err != nil || userID != p.ID {
		writeErr(w, http.StatusNotFound, "sesi tidak ditemukan")
		return
	}
	resp := map[string]any{"session_id": sessionID, "provider": provider, "status": status}
	if status == "pending" && provider == "tokopay" && a.tokopay != nil && a.tokopay.Configured() {
		trx := ""
		if trxRef != nil {
			trx = *trxRef
		}
		if fresh, err := a.tokopay.CheckOrderStatus(r.Context(), trx, strings.ReplaceAll(sessionID, "-", "")); err == nil {
			resp["gateway_status"] = fresh
			if payments.TokopayPaid(fresh) {
				if err := billing.ActivateSession(r.Context(), a.pool, sessionID, trx); err == nil {
					resp["status"] = "paid"
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleXenditWebhook menerima callback Xendit; token callback diverifikasi
// dari header, status PAID mengaktifkan sesi (external_id = id sesi).
func (a *API) handleXenditWebhook(w http.ResponseWriter, r *http.Request) {
	if a.cfg.XenditCallbackTok == "" {
		writeErr(w, http.StatusServiceUnavailable, "Xendit belum dikonfigurasi")
		return
	}
	if r.Header.Get("x-callback-token") != a.cfg.XenditCallbackTok {
		writeErr(w, http.StatusForbidden, "token callback tidak valid")
		return
	}
	var n struct {
		ExternalID string `json:"external_id"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		writeErr(w, http.StatusBadRequest, "isi callback tidak valid")
		return
	}
	if n.Status == "PAID" {
		if err := billing.ActivateSession(r.Context(), a.pool, n.ExternalID, n.ExternalID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
