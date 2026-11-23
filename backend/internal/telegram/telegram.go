// Package telegram adalah klien REST minimal untuk mengirim
// pesan sebagai bot SubSenpai ke topik grup Telegram komunitas
// (permintaan Rams 2026-11-23 malam) — cermin dari notifikasi
// Discord: pengumuman admin, update APK, dan jadwal rilis harian.
// Pengiriman cukup lewat Bot API (sendMessage), tanpa polling.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// BaseURL adalah basis Bot API Telegram. Variabel (bukan
// konstanta) agar pengujian bisa mengarahkannya ke server lokal.
var BaseURL = "https://api.telegram.org"

// Client mengirim pesan dengan token bot dari env server.
type Client struct {
	token string
	http  *http.Client
}

// New membuat klien; token kosong menghasilkan klien nonaktif.
func New(token string) *Client {
	tok := strings.TrimSpace(token)
	return &Client{
		token: tok,
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled melaporkan apakah token tersedia.
func (c *Client) Enabled() bool { return c != nil && c.token != "" }

// Send mengirim satu pesan HTML ke satu topik (message_thread_id)
// di grup. threadID string angka; kosong/tidak valid = kirim ke
// topik General. Konten dipotong ke 4000 karakter (batas Telegram
// 4096). Pemanggil meneruskan teks yang SUDAH diformat HTML
// (lihat ToHTML).
func (c *Client) Send(ctx context.Context, chatID, threadID, text string) error {
	if !c.Enabled() {
		return fmt.Errorf("telegram: token kosong")
	}
	if strings.TrimSpace(chatID) == "" {
		return fmt.Errorf("telegram: grup kosong")
	}
	if r := []rune(text); len(r) > 4000 {
		text = string(r[:4000]) + "…"
	}
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(threadID), 10, 64); err == nil && n > 0 {
		payload["message_thread_id"] = n
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		BaseURL+"/bot"+c.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 ||
		!strings.Contains(string(b), `"ok":true`) {
		return fmt.Errorf("telegram: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// ToHTML mengubah teks gaya Discord (tebal **seperti ini**) ke
// HTML aman Telegram: karakter khusus di-escape lebih dulu, lalu
// pasangan ** menjadi <b>…</b>. Penanda ** ganjil di akhir
// dibiarkan sebagai teks biasa.
func ToHTML(s string) string {
	s = html.EscapeString(s)
	parts := strings.Split(s, "**")
	if len(parts) < 3 {
		return s
	}
	var b strings.Builder
	last := len(parts) - 1
	for i, p := range parts {
		switch {
		case i == 0:
		case i == last && len(parts)%2 == 0:
			// Penanda ** terakhir tak berpasangan (jumlah ganjil):
			// kembalikan sebagai teks biasa, jangan buka <b> yang
			// tidak tertutup (Telegram menolak HTML tak seimbang).
			b.WriteString("**")
		case i%2 == 1:
			b.WriteString("<b>")
		default:
			b.WriteString("</b>")
		}
		b.WriteString(p)
	}
	return b.String()
}
