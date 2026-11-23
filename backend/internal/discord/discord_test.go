package discord

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessage(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()

	c := New("token-uji")
	if !c.Enabled() {
		t.Fatal("klien dengan token harus aktif")
	}
	if err := c.Send(context.Background(), "123456", "halo komunitas"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotAuth != "Bot token-uji" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotPath != "/channels/123456/messages" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["content"] != "halo komunitas" {
		t.Errorf("content = %q", gotBody["content"])
	}
}

func TestSendErrors(t *testing.T) {
	if err := New("").Send(context.Background(), "1", "x"); err == nil {
		t.Error("token kosong harus error")
	}
	if err := New("t").Send(context.Background(), "", "x"); err == nil {
		t.Error("channel kosong harus error")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Missing Access"}`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = old }()
	err := New("t").Send(context.Background(), "1", "x")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("status 403 harus jadi error berisi kode: %v", err)
	}
}
