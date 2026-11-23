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
	// Topik kosong -> tanpa message_thread_id.
	body = nil
	if err := c.Send(context.Background(), "-100123", "", "halo"); err != nil {
		t.Fatalf("Send tanpa topik: %v", err)
	}
	if strings.Contains(string(body), "message_thread_id") {
		t.Fatalf("topik kosong tidak boleh membawa thread id: %s", body)
	}
}
