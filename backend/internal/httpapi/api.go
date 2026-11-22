// Package httpapi berisi router (chi) dan handler API SubSenpai.
//
// Semua endpoint JSON di /api/v1/* (kecuali handshake) dibungkus envelope
// terenkripsi xdata oleh middleware. Katalog bersifat bacaan publik;
// gerbang freemium & kepemilikan user ditegakkan mulai Fase 3–4.
package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/middleware"
)

type API struct {
	pool     *pgxpool.Pool
	sessions *middleware.Sessions
	cache    *ttlCache
}

func New(pool *pgxpool.Pool, sessions *middleware.Sessions) *API {
	return &API{pool: pool, sessions: sessions, cache: newTTLCache(5 * time.Minute)}
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.With(middleware.RateLimit(30)).Post("/api/v1/session", a.handleSession)
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middleware.Envelope(a.sessions, next)
		})
		r.Get("/api/v1/home", a.handleHome)
		r.Get("/api/v1/titles", a.handleTitles)
		r.Get("/api/v1/titles/{id}", a.handleTitleDetail)
		r.Get("/api/v1/episodes/{id}/playback", a.handlePlayback)
		r.Get("/api/v1/chapters/{id}", a.handleChapter)
	})
	return r
}

// handleSession = handshake ECDH (satu-satunya endpoint API tanpa envelope).
func (a *API) handleSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientPublicKey string `json:"client_public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ClientPublicKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "client_public_key wajib diisi"})
		return
	}
	id, serverPub, expires, err := a.sessions.Handshake(req.ClientPublicKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":        id,
		"server_public_key": serverPub,
		"expires_at":        expires.Unix(),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ttlCache: cache memori sederhana untuk bacaan katalog (STRUKTUR.md:
// katalog di-cache di memori backend, TTL 1–5 menit).
type ttlCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]cacheEntry
}

type cacheEntry struct {
	body []byte
	exp  time.Time
}

func newTTLCache(ttl time.Duration) *ttlCache {
	return &ttlCache{ttl: ttl, m: make(map[string]cacheEntry)}
}

func (c *ttlCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.exp) {
		return nil, false
	}
	return e.body, true
}

func (c *ttlCache) put(key string, body []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{body: body, exp: time.Now().Add(c.ttl)}
}

// serveCached menulis body JSON dari cache bila ada; bila tidak, panggil
// build untuk membuatnya lalu simpan.
func (a *API) serveCached(w http.ResponseWriter, key string, build func() (any, error)) {
	if body, ok := a.cache.get(key); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
		return
	}
	v, err := build()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	body, err := json.Marshal(v)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	a.cache.put(key, body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
