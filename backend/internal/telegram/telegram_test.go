package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestToHTML(t *testing.T) {
	cases := map[string]string{
		"📣 **Halo Uji**\n\nIsi <b>mentah</b> & aman": "📣 <b>Halo Uji</b>\n\nIsi &lt;b&gt;mentah&lt;/b&gt; &amp; aman",
		"tanpa tebal":          "tanpa tebal",
		"**satu** dan **dua**": "<b>satu</b> dan <b>dua</b>",
		"penanda ** ganjil":    "penanda ** ganjil",
		"**a** tengah **b":     "<b>a</b> tengah **b",
	}
	for in, want := range cases {
		if got := ToHTML(in); got != want {
			t.Errorf("ToHTML(%q) = %q, mau %q", in, got, want)
		}
	}
}

func TestSendKeTopik(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sendMessage") {
			t.Errorf("path janggal: %s", r.URL.Path)
		}
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()

	c := New("token-uji")
	if err := c.Send(context.Background(), "-100123", "20729", "<b>halo</b>"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload bukan JSON: %v", err)
	}
	if payload["chat_id"] != "-100123" || payload["message_thread_id"] != float64(20729) ||
		payload["text"] != "<b>halo</b>" || payload["parse_mode"] != "HTML" {
		t.Fatalf("payload janggal: %v", payload)
	}
	// Send biasa: pratinjau tautan MATI.
	if payload["disable_web_page_preview"] != true {
		t.Fatalf("Send harus mematikan pratinjau: %v", payload)
	}
	// SendPreview: pratinjau tautan MENYALA (kartu og:image tampil).
	body = nil
	if err := c.SendPreview(context.Background(), "-100123", "20729", "satu tautan"); err != nil {
		t.Fatalf("SendPreview: %v", err)
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload SendPreview bukan JSON: %v", err)
	}
	if payload["disable_web_page_preview"] != false {
		t.Fatalf("SendPreview harus menyalakan pratinjau: %v", payload)
	}
	// Topik kosong -> tanpa message_thread_id.
	body = nil
	if err := c.Send(context.Background(), "-100123", "", "halo"); err != nil {
		t.Fatalf("Send tanpa topik: %v", err)
	}
	if strings.Contains(string(body), "message_thread_id") {
		t.Fatalf("topik kosong tidak boleh membawa thread id: %s", body)
	}
}

// SendID mengembalikan message_id dari respons; Pin/Unpin memanggil
// metode Bot API yang tepat dengan muatan yang benar (dipakai pin
// header jadwal harian, permintaan Rams 2026-11-25).
func TestSendIDPinUnpin(t *testing.T) {
	var calls []struct {
		path string
		body map[string]any
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var p map[string]any
		_ = json.Unmarshal(raw, &p)
		calls = append(calls, struct {
			path string
			body map[string]any
		}{r.URL.Path, p})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()

	c := New("token-uji")
	id, err := c.SendID(context.Background(), "-100123", "20730", "<b>halo</b>")
	if err != nil || id != 42 {
		t.Fatalf("SendID: id=%d err=%v", id, err)
	}
	if err := c.Pin(context.Background(), "-100123", 42); err != nil {
		t.Fatalf("Pin: %v", err)
	}
	if err := c.Unpin(context.Background(), "-100123", 41); err != nil {
		t.Fatalf("Unpin: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("harus 3 panggilan, dapat %d", len(calls))
	}
	if !strings.HasSuffix(calls[1].path, "/pinChatMessage") ||
		calls[1].body["chat_id"] != "-100123" || calls[1].body["message_id"] != float64(42) ||
		calls[1].body["disable_notification"] != true {
		t.Errorf("Pin janggal: %+v", calls[1])
	}
	if !strings.HasSuffix(calls[2].path, "/unpinChatMessage") ||
		calls[2].body["chat_id"] != "-100123" || calls[2].body["message_id"] != float64(41) {
		t.Errorf("Unpin janggal: %+v", calls[2])
	}
	if err := c.Pin(context.Background(), "-100123", 0); err == nil {
		t.Errorf("Pin id 0 harus ditolak")
	}
}
