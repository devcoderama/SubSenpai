package payments

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Tokopay adalah gateway utama SubSenpai (dipilih Rams 2026-11-22
// karena biaya admin & fee lebih murah dari Midtrans/Xendit).
// Dokumentasi: https://docs.tokopay.id — API Order Advanced (POST JSON),
// signature md5(merchant_id:secret:reff_id), callback diverifikasi
// dengan rumus yang sama, status lunas = "Success" atau "Completed".

// Channels: kode channel pembayaran Tokopay (docs: Metode Pembayaran).
var TokopayChannels = map[string]string{
	"BRIVA": "BRI Virtual Akun", "BCAVA": "BCA Virtual Akun",
	"BNIVA": "BNI Virtual Akun", "MANDIRIVA": "Mandiri Virtual Akun",
	"PERMATAVA": "Permata Virtual Akun", "PERMATAVAA": "Permata VA 2",
	"CIMBVA": "CIMB Virtual Akun", "DANAMONVA": "Danamon Virtual Akun",
	"BSIVA": "BSI Virtual Akun", "BNCVA": "BNC Virtual Akun",
	"SHOPEEPAY": "ShopeePay", "GOPAY": "GoPay", "DANA": "DANA",
	"LINKAJA": "LinkAja",
	"QRIS": "QRIS", "QRISREALTIME": "QRIS Realtime",
	"QRIS_REALTIME_NOBU": "QRIS Realtime Nobu",
	"ALFAMART": "Alfamart", "INDOMARET": "Indomaret",
	"TELKOMSEL": "Pulsa Telkomsel", "AXIS": "Pulsa AXIS",
	"XL": "Pulsa XL", "TRI": "Pulsa Tri",
}

// TokopaySignature: md5 dari "merchant_id:secret:reff_id".
func TokopaySignature(merchantID, secret, reffID string) string {
	sum := md5.Sum([]byte(merchantID + ":" + secret + ":" + reffID))
	return hex.EncodeToString(sum[:])
}

// VerifyTokopayCallback membandingkan signature callback secara
// waktu-konstan dengan signature yang dihitung dari reff_id payload.
func VerifyTokopayCallback(merchantID, secret, reffID, given string) bool {
	want := TokopaySignature(merchantID, secret, reffID)
	return subtle.ConstantTimeCompare([]byte(want), []byte(given)) == 1
}

// TokopayClient berbicara ke API Tokopay.
type TokopayClient struct {
	BaseURL    string // bawaan https://api.tokopay.id
	MerchantID string
	Secret     string
	http       *http.Client
}

func NewTokopayClient(baseURL, merchantID, secret string) *TokopayClient {
	if baseURL == "" {
		baseURL = "https://api.tokopay.id"
	}
	return &TokopayClient{
		BaseURL: baseURL, MerchantID: merchantID, Secret: secret,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *TokopayClient) Configured() bool {
	return c.MerchantID != "" && c.Secret != ""
}

// OrderResult adalah bagian berguna dari respons buat order.
type OrderResult struct {
	TrxID       string
	PayURL      string
	NomorVA     string
	QRString    string
	QRLink      string
	CheckoutURL string
	Panduan     string
	TotalBayar  int
	TotalTerima int
}

// CreateOrder membuat order Advanced. ReffID adalah id sesi pembayaran
// kita (tanpa tanda hubung agar aman sebagai ref alfanumerik).
func (c *TokopayClient) CreateOrder(ctx context.Context, channel, reffID string, amount int, name, email, phone string, expiredUnix int64) (*OrderResult, error) {
	payload := map[string]any{
		"merchant_id":    c.MerchantID,
		"kode_channel":   channel,
		"reff_id":        reffID,
		"amount":         amount,
		"customer_name":  name,
		"customer_email": email,
		"customer_phone": phone,
		"expired_ts":     expiredUnix,
		"signature":      TokopaySignature(c.MerchantID, c.Secret, reffID),
		"items": []map[string]any{
			{"product_code": "PREMIUM", "name": "SubSenpai Premium", "price": amount},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/order", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error_msg"`
		Data   struct {
			TrxID       string `json:"trx_id"`
			PayURL      string `json:"pay_url"`
			NomorVA     string `json:"nomor_va"`
			QRString    string `json:"qr_string"`
			QRLink      string `json:"qr_link"`
			CheckoutURL string `json:"checkout_url"`
			Panduan     string `json:"panduan_pembayaran"`
			TotalBayar  int    `json:"total_bayar"`
			TotalTerima int    `json:"total_diterima"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("respons Tokopay tidak terbaca: %s", string(raw))
	}
	if out.Status != "Success" || out.Data.TrxID == "" {
		msg := out.Error
		if msg == "" {
			msg = string(raw)
		}
		return nil, fmt.Errorf("Tokopay menolak order: %s", msg)
	}
	d := out.Data
	return &OrderResult{
		TrxID: d.TrxID, PayURL: d.PayURL, NomorVA: d.NomorVA,
		QRString: d.QRString, QRLink: d.QRLink, CheckoutURL: d.CheckoutURL,
		Panduan: d.Panduan, TotalBayar: d.TotalBayar, TotalTerima: d.TotalTerima,
	}, nil
}

// CheckOrderStatus mengecek status via GET /v1/check-order.
// Mengembalikan status Tokopay (Unpaid/Success/Completed/Failed/...).
func (c *TokopayClient) CheckOrderStatus(ctx context.Context, trxID, refID string) (string, error) {
	q := url.Values{
		"merchant_id": {c.MerchantID},
		"secret":      {c.Secret},
	}
	if trxID != "" {
		q.Set("trx_id", trxID)
	} else {
		q.Set("ref_id", refID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/check-order?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Status int    `json:"status"`
		Error  string `json:"error_msg"`
		Data   struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("respons cek status tidak terbaca: %s", string(raw))
	}
	if out.Status != 1 {
		return "", fmt.Errorf("cek status Tokopay: %s", out.Error)
	}
	return out.Data.Status, nil
}

// TokopayPaid: status yang dianggap lunas menurut dokumentasi.
func TokopayPaid(status string) bool {
	return status == "Success" || status == "Completed"
}
