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
	_, err := c.send(ctx, chatID, threadID, text, false)
	return err
}

// SendID sama seperti Send, tetapi mengembalikan message_id pesan
// yang terkirim (0 bila respons tidak memuatnya) — dipakai jadwal
// rilis untuk mem-pin pesan header hari itu.
func (c *Client) SendID(ctx context.Context, chatID, threadID, text string) (int64, error) {
	return c.send(ctx, chatID, threadID, text, false)
}

// SendPreview sama seperti Send, tetapi pratinjau tautan (WebPage)
// diizinkan: Telegram menampilkan kartu berisi og:image dari
// tautan PERTAMA di pesan. Hanya cocok untuk pesan berisi satu
// tautan — jadwal per judul, update APK, pengumuman (permintaan
// Rams 2026-11-24: 1 pesan 1 judul agar poster tiap judul muncul).
func (c *Client) SendPreview(ctx context.Context, chatID, threadID, text string) error {
	_, err := c.send(ctx, chatID, threadID, text, true)
	return err
}

// Pin menyematkan satu pesan di grup (pinChatMessage) tanpa
// pemberitahuan ke anggota. Bot harus admin grup dengan hak
// can_pin_messages; bila haknya kurang, Bot API menolak (403).
func (c *Client) Pin(ctx context.Context, chatID string, messageID int64) error {
	if messageID <= 0 {
		return fmt.Errorf("telegram: message_id pin tidak valid")
	}
	_, err := c.do(ctx, "pinChatMessage", map[string]any{
		"chat_id":              chatID,
		"message_id":           messageID,
		"disable_notification": true,
	})
	return err
}

// Unpin melepas sematan satu pesan di grup. Gagal (pesan sudah
// tidak tersemat / terhapus) dikembalikan sebagai error agar
// pemanggil bisa mencatatnya dan tetap lanjut.
func (c *Client) Unpin(ctx context.Context, chatID string, messageID int64) error {
	if messageID <= 0 {
		return fmt.Errorf("telegram: message_id unpin tidak valid")
	}
	_, err := c.do(ctx, "unpinChatMessage", map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
	})
	return err
}

// do memanggil satu metode Bot API dan mengembalikan badan respons
// mentahnya. Sukses mensyaratkan HTTP 2xx DAN badan memuat
// `"ok":true` (pola yang sama dipakai semua pengiriman selama ini).
func (c *Client) do(ctx context.Context, method string, payload map[string]any) ([]byte, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("telegram: token kosong")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		BaseURL+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 5000))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 ||
		!strings.Contains(string(b), `"ok":true`) {
		return nil, fmt.Errorf("telegram: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

func (c *Client) send(ctx context.Context, chatID, threadID, text string, preview bool) (int64, error) {
	if strings.TrimSpace(chatID) == "" {
		return 0, fmt.Errorf("telegram: grup kosong")
	}
	if r := []rune(text); len(r) > 4000 {
		text = string(r[:4000]) + "…"
	}
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "HTML",
		"disable_web_page_preview": !preview,
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(threadID), 10, 64); err == nil && n > 0 {
		payload["message_thread_id"] = n
	}
	b, err := c.do(ctx, "sendMessage", payload)
	if err != nil {
		return 0, err
	}
	var out struct {
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, nil // id tak terbaca — pengiriman sendiri sudah sukses
	}
	return out.Result.MessageID, nil
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
