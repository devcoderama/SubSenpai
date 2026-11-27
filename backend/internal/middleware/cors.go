package middleware

import (
	"net/http"
	"strings"
)

// CORS membuka API untuk klien web (dashboard admin di Netlify) —
// aplikasi Android tidak membutuhkannya. Origin yang diizinkan:
// daftar eksplisit dari env WEB_ORIGINS (pisah koma); bila kosong,
// pola bawaan https://*.netlify.app dan http://localhost(:port).
// Keamanan tetap bertumpu pada JWT + gerbang role admin; CORS di
// sini hanya gerbang peramban, bukan pengganti autentikasi.
func CORS(webOrigins string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range strings.Split(webOrigins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			allowed[o] = true
		}
	}
	originOK := func(origin string) bool {
		if allowed[origin] {
			return true
		}
		if len(allowed) > 0 {
			return false
		}
		// Pola bawaan (env kosong): situs Netlify mana pun + lokal.
		if strings.HasPrefix(origin, "https://") && strings.HasSuffix(origin, ".netlify.app") {
			return true
		}
		// Domain resmi (halaman download & bagikan, dibeli Rams
		// 2026-11-23): tanpa ini katalog di subsenpai.web.id kosong
		// karena peramban memblokir panggilan API (CORS).
		if origin == "https://subsenpai.web.id" || origin == "https://www.subsenpai.web.id" {
			return true
		}
		return strings.HasPrefix(origin, "http://localhost") ||
			strings.HasPrefix(origin, "http://127.0.0.1")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && originOK(origin) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Session-Id, X-Device-Id")
			h.Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
