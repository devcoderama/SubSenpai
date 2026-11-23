// Package discord adalah klien REST minimal untuk mengirim pesan
// sebagai bot SubSenpai ke channel komunitas (pengumuman admin,
// update APK, jadwal rilis). Pengiriman notifikasi satu arah cukup
// lewat REST API Discord — bot tidak perlu koneksi gateway dan
// memang tampil offline di daftar anggota; itu normal.
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BaseURL adalah basis API Discord. Variabel (bukan konstanta) agar
// pengujian bisa mengarahkannya ke server lokal.
var BaseURL = "https://discord.com/api/v10"

// Client mengirim pesan dengan token bot dari env server.
type Client struct {
	token string
	http  *http.Client
}

// New membuat klien; token kosong menghasilkan klien nonaktif.
func New(token string) *Client {
	return &Client{
		token: strings.TrimSpace(token),
		http:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Enabled melaporkan apakah token tersedia.
func (c *Client) Enabled() bool { return c != nil && c.token != "" }

// Send mengirim satu pesan teks ke channel. Konten dipotong ke
// 1900 karakter (batas Discord 2000, disisakan ruang aman).
func (c *Client) Send(ctx context.Context, channelID, content string) error {
	if !c.Enabled() {
		return fmt.Errorf("discord: token kosong")
	}
	if strings.TrimSpace(channelID) == "" {
		return fmt.Errorf("discord: channel kosong")
	}
	if r := []rune(content); len(r) > 1900 {
		content = string(r[:1900]) + "…"
	}
	body, err := json.Marshal(map[string]string{"content": content})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		BaseURL+"/channels/"+channelID+"/messages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SubSenpai (https://www.subsenpai.web.id, 1.0)")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("discord: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
