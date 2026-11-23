package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Origin yang boleh memanggil API dari peramban: situs Netlify,
// domain resmi subsenpai.web.id, dan lokal. Sisanya tidak mendapat
// header Access-Control-Allow-Origin.
func TestCORSOrigins(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := CORS("", next)
	kases := []struct {
		origin string
		boleh  bool
	}{
		{"https://app-subsenpai.netlify.app", true},
		{"https://subsenpai-admin.netlify.app", true},
		{"https://subsenpai.web.id", true},
		{"https://www.subsenpai.web.id", true},
		{"http://localhost:8080", true},
		{"http://127.0.0.1:3000", true},
		{"https://evil.example.com", false},
		{"https://subsenpai.web.id.evil.com", false},
		{"http://subsenpai.web.id", false},
		{"https://netlify.app", false},
	}
	for _, k := range kases {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/titles", nil)
		req.Header.Set("Origin", k.origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		ada := rec.Header().Get("Access-Control-Allow-Origin") == k.origin
		if ada != k.boleh {
			t.Errorf("origin %s: allow=%v, want %v", k.origin, ada, k.boleh)
		}
	}
}

// Daftar eksplisit env menonaktifkan pola bawaan sepenuhnya.
func TestCORSExplicitList(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	h := CORS("https://contoh.id", next)
	for _, k := range []struct {
		origin string
		boleh  bool
	}{
		{"https://contoh.id", true},
		{"https://app-subsenpai.netlify.app", false},
		{"https://subsenpai.web.id", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/titles", nil)
		req.Header.Set("Origin", k.origin)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		ada := rec.Header().Get("Access-Control-Allow-Origin") == k.origin
		if ada != k.boleh {
			t.Errorf("origin %s: allow=%v, want %v", k.origin, ada, k.boleh)
		}
	}
}
