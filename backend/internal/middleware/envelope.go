package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Envelope membungkus response JSON handler menjadi {xdata, xtime} memakai
// kunci sesi dari header X-Session-Id. Request tanpa sesi yang berlaku
// ditolak 401 (klien selalu handshake lebih dulu — tamu pun anonim).
// Endpoint handshake sendiri dan /healthz tidak melewati middleware ini.
func Envelope(sessions *Sessions, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := sessions.Key(r.Header.Get("X-Session-Id"))
		if !ok {
			writePlainError(w, http.StatusUnauthorized, "sesi tidak ada atau kedaluwarsa — lakukan handshake POST /api/v1/session")
			return
		}
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		body := rec.buf.Bytes()
		if len(body) == 0 {
			rec.flushHeader()
			return
		}
		xdata, xtime, err := Seal(key, body)
		if err != nil {
			rec.flushHeader()
			_, _ = w.Write(body)
			return
		}
		env, _ := json.Marshal(map[string]any{"xdata": xdata, "xtime": xtime})
		rec.flushHeader()
		_, _ = w.Write(env)
	})
}

type recorder struct {
	http.ResponseWriter
	buf         bytes.Buffer
	status      int
	wroteHeader bool
}

func (r *recorder) WriteHeader(status int) { r.status = status }

func (r *recorder) Write(b []byte) (int, error) { return r.buf.Write(b) }

func (r *recorder) flushHeader() {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.ResponseWriter.Header().Set("Content-Type", "application/json")
	r.ResponseWriter.WriteHeader(r.status)
}

func writePlainError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// RateLimit sederhana per IP (token bucket isi ulang per menit) — dipakai
// membatasi handshake sesi (KEAMANAN.md: handshake dibatasi laju).
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	type bucket struct {
		tokens int
		reset  int64
	}
	var mu sync.Mutex
	buckets := make(map[string]*bucket)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now().Unix() / 60
			mu.Lock()
			b, ok := buckets[ip]
			if !ok || b.reset != now {
				b = &bucket{tokens: perMinute, reset: now}
				buckets[ip] = b
			}
			if b.tokens <= 0 {
				mu.Unlock()
				writePlainError(w, http.StatusTooManyRequests, "terlalu banyak percobaan — coba lagi sebentar")
				return
			}
			b.tokens--
			mu.Unlock()
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
