// Package payments berisi klien gateway pembayaran. GoPay di sini adalah
// API merchant pihak ketiga (pola Api-Gopar-Merch): login → token,
// create-payment (QRIS statis menjadi dinamis sesuai nominal), dan
// cek-transaksi untuk pencocokan polling. Bentuk respons diurai longgar
// karena layanan pembungkusnya bukan API resmi GoTo.
package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Tx adalah satu transaksi masuk pada akun merchant.
type Tx struct {
	Amount int
	Time   time.Time
	Ref    string
}

// GoPayClient berbicara ke layanan Api-Gopar-Merch milik merchant.
type GoPayClient struct {
	BaseURL  string
	Username string
	Password string
	http     *http.Client
	token    string
}

func NewGoPayClient(baseURL, username, password string) *GoPayClient {
	return &GoPayClient{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Username: username,
		Password: password,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GoPayClient) Configured() bool {
	return c.BaseURL != "" && c.Username != "" && c.Password != ""
}

func (c *GoPayClient) post(ctx context.Context, path string, payload any, token string) (map[string]any, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GoPay %s: HTTP %d", path, resp.StatusCode)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("respons GoPay tidak terbaca: %w", err)
	}
	return out, nil
}

func (c *GoPayClient) login(ctx context.Context) (string, error) {
	if c.token != "" {
		return c.token, nil
	}
	out, err := c.post(ctx, "/api/login", map[string]string{
		"username": c.Username, "password": c.Password,
	}, "")
	if err != nil {
		return "", err
	}
	for _, k := range []string{"token", "access_token", "accessToken"} {
		if v, ok := out[k].(string); ok && v != "" {
			c.token = v
			return v, nil
		}
	}
	if d, ok := out["data"].(map[string]any); ok {
		if v, ok := d["token"].(string); ok && v != "" {
			c.token = v
			return v, nil
		}
	}
	return "", fmt.Errorf("respons login GoPay tanpa token")
}

// CreatePayment meminta QRIS dinamis untuk nominal sesi (sudah termasuk
// kode unik). Mengembalikan string QRIS bila layanan menyediakannya.
func (c *GoPayClient) CreatePayment(ctx context.Context, amount int) (string, error) {
	token, err := c.login(ctx)
	if err != nil {
		return "", err
	}
	out, err := c.post(ctx, "/api/create-payment", map[string]any{"amount": amount}, token)
	if err != nil {
		return "", err
	}
	for _, k := range []string{"qris", "qr_string", "qrString", "qr", "payment_url"} {
		if v, ok := out[k].(string); ok && v != "" {
			return v, nil
		}
	}
	if d, ok := out["data"].(map[string]any); ok {
		for _, k := range []string{"qris", "qr_string", "qrString", "qr"} {
			if v, ok := d[k].(string); ok && v != "" {
				return v, nil
			}
		}
	}
	return "", nil // sesi tetap sah; QRIS otomatis tidak tersedia
}

// Transactions mengambil daftar transaksi masuk terbaru dari merchant.
func (c *GoPayClient) Transactions(ctx context.Context) ([]Tx, error) {
	token, err := c.login(ctx)
	if err != nil {
		return nil, err
	}
	out, err := c.post(ctx, "/api/cek-transaksi", map[string]any{}, token)
	if err != nil {
		return nil, err
	}
	var list []any
	switch v := out["data"].(type) {
	case []any:
		list = v
	default:
		if v, ok := out["transactions"].([]any); ok {
			list = v
		}
	}
	txs := []Tx{}
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tx := Tx{}
		switch a := m["amount"].(type) {
		case float64:
			tx.Amount = int(a)
		case string:
			tx.Amount, _ = strconv.Atoi(strings.ReplaceAll(a, ".", ""))
		}
		for _, k := range []string{"id", "ref", "reference", "transaction_id"} {
			if v, ok := m[k].(string); ok {
				tx.Ref = v
				break
			}
		}
		for _, k := range []string{"time", "date", "created_at", "timestamp"} {
			if v, ok := m[k].(string); ok && v != "" {
				for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
					if t, err := time.Parse(layout, v); err == nil {
						tx.Time = t
						break
					}
				}
				break
			}
		}
		if tx.Amount > 0 {
			txs = append(txs, tx)
		}
	}
	return txs, nil
}

// Match menemukan transaksi yang cocok untuk satu sesi: nominal persis
// sama dan waktu transaksi di dalam jendela sesi (dibuat −2 menit s.d.
// kedaluwarsa). Mengembalikan ref transaksi bila cocok.
func Match(txs []Tx, amount int, createdAt, expiresAt time.Time) (string, bool) {
	from := createdAt.Add(-2 * time.Minute)
	for _, tx := range txs {
		if tx.Amount != amount {
			continue
		}
		if tx.Time.IsZero() || (tx.Time.After(from) && tx.Time.Before(expiresAt.Add(time.Minute))) {
			return tx.Ref, true
		}
	}
	return "", false
}
