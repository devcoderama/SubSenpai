// Package httpapi berisi router (chi) dan handler API SubSenpai.
//
// Semua endpoint JSON di /api/v1/* (kecuali handshake) dibungkus envelope
// terenkripsi xdata oleh middleware. Katalog bersifat bacaan publik;
// endpoint user (watchlist, riwayat, /me) wajib JWT backend sejak Fase 3;
// gerbang freemium ditegakkan mulai Fase 4.
package httpapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/chat"
	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/middleware"
	"subsenpai/backend/internal/payments"
)

type API struct {
	pool      *pgxpool.Pool
	sessions  *middleware.Sessions
	cache     *ttlCache
	verifier  *auth.GoogleVerifier
	jwtSecret string
	jwtTTL    time.Duration
	authn     *auth.Authenticator
	cfg       config.Config
	gopay     *payments.GoPayClient
	tokopay   *payments.TokopayClient
	hub       *chat.Hub
	guard     *fillGuard // kunci + cooldown jalur on-demand (ondemand.go)
	// Cache jadwal rilis per kategori (schedule.go).
	schedAnime   schedCache
	schedDonghua schedCache
}

func New(pool *pgxpool.Pool, sessions *middleware.Sessions, cfg config.Config) *API {
	a := &API{
		pool:      pool,
		sessions:  sessions,
		cache:     newTTLCache(5 * time.Minute),
		jwtSecret: cfg.JWTSecret,
		jwtTTL:    time.Duration(cfg.JWTTTLHours) * time.Hour,
		authn:     &auth.Authenticator{Pool: pool, JWTSecret: cfg.JWTSecret},
		cfg:       cfg,
		hub:       chat.NewHub(pool),
		guard:     newFillGuard(onDemandCooldown),
	}
	if cfg.GoogleClientID != "" {
		a.verifier = auth.NewGoogleVerifier(cfg.GoogleClientID, cfg.GoogleCertsURL)
	}
	if cfg.GoPayBaseURL != "" {
		a.gopay = payments.NewGoPayClient(cfg.GoPayBaseURL, cfg.GoPayUsername, cfg.GoPayPassword)
	}
	if cfg.TokopayMerchantID != "" {
		a.tokopay = payments.NewTokopayClient(cfg.TokopayAPIURL, cfg.TokopayMerchantID, cfg.TokopaySecret)
	}
	return a
}

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	// CORS untuk klien web (dashboard admin) — aplikasi Android
	// tidak terpengaruh; preflight OPTIONS dijawab di middleware.
	r.Use(func(next http.Handler) http.Handler {
		return middleware.CORS(a.cfg.WebOrigins, next)
	})
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.With(middleware.RateLimit(30)).Post("/api/v1/session", a.handleSession)
	// Webhook gateway pembayaran: polos, tanpa envelope & tanpa JWT —
	// keasliannya dari tanda tangan/token callback gateway (Fase 4).
	r.Post("/api/v1/payments/webhook/midtrans", a.handleMidtransWebhook)
	r.Post("/api/v1/payments/webhook/xendit", a.handleXenditWebhook)
	r.Post("/api/v1/payments/webhook/tokopay", a.handleTokopayWebhook)
	// WebSocket chat & watch party (Fase 4): auth via header Authorization.
	r.Get("/ws", a.handleWS)
	// Gambar feed komunitas: publik, tanpa envelope — nama berkas
	// acak 128-bit; dimuat langsung oleh Coil/peramban.
	r.Get("/uploads/feed/{name}", a.handleFeedFile)
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middleware.Envelope(a.sessions, next)
		})
		r.Get("/api/v1/home", a.handleHome)
		r.Get("/api/v1/titles", a.handleTitles)
		// Detail memakai auth OPSIONAL: tanpa ini FromContext selalu
		// nil → semua penonton dianggap tamu → episode pengguna
		// premium ikut terkunci (bug dilaporkan Rams 2026-11-23).
		r.With(a.authn.Optional).Get("/api/v1/titles/{id}", a.handleTitleDetail)
		// Playback & chapter melewati gerbang freemium (Fase 4): JWT
		// opsional membedakan tamu / user gratis / premium.
		r.With(a.authn.Optional).Get("/api/v1/episodes/{id}/playback", a.handlePlayback)
		r.With(a.authn.Optional).Get("/api/v1/chapters/{id}", a.handleChapter)
		r.With(middleware.RateLimit(30)).Post("/api/v1/auth/google", a.handleGoogleLogin)
		r.Get("/api/v1/schedule", a.handleSchedule)
		r.Get("/api/v1/genres", a.handleGenres)
		r.Get("/api/v1/filters", a.handleFilters)
		r.Get("/api/v1/plans", a.handlePlans)
		r.Get("/api/v1/payment-methods", a.handlePaymentMethods)
		// Endpoint khusus user login (Fase 3–4).
		r.Group(func(r chi.Router) {
			r.Use(a.authn.Require)
			r.Get("/api/v1/admin/payment-methods", a.handleAdminPaymentMethods)
			r.Patch("/api/v1/admin/payment-methods/{code}", a.handleAdminPaymentMethodUpdate)
			r.Get("/api/v1/admin/stats", a.handleAdminStats)
			r.Get("/api/v1/admin/users", a.handleAdminUsers)
			r.Patch("/api/v1/admin/users/{id}", a.handleAdminUserUpdate)
			r.Get("/api/v1/admin/transactions", a.handleAdminTransactions)
			r.Get("/api/v1/admin/plans", a.handleAdminPlans)
			r.Patch("/api/v1/admin/plans/{id}", a.handleAdminPlanUpdate)
			r.Get("/api/v1/me", a.handleMe)
			r.Get("/api/v1/watchlist", a.handleWatchlistList)
			r.Get("/api/v1/watchlist/{titleId}", a.handleWatchlistCheck)
			r.Post("/api/v1/watchlist/{titleId}", a.handleWatchlistAdd)
			r.Delete("/api/v1/watchlist/{titleId}", a.handleWatchlistRemove)
			r.Post("/api/v1/history/watch", a.handleHistoryWatchReport)
			r.Get("/api/v1/history/watch", a.handleHistoryWatchList)
			r.Post("/api/v1/history/read", a.handleHistoryReadReport)
			r.Get("/api/v1/history/read", a.handleHistoryReadList)
			// Premium & pembayaran (Fase 4).
			r.Get("/api/v1/premium/status", a.handlePremiumStatus)
			r.Post("/api/v1/premium/trial", a.handleTrial)
			r.Post("/api/v1/payments/checkout", a.handleCheckout)
			r.Get("/api/v1/payments/status/{sessionId}", a.handlePaymentStatus)
			r.Get("/api/v1/transactions", a.handleTransactions)
			r.Post("/api/v1/vouchers/redeem", a.handleVoucherRedeem)
			r.Get("/api/v1/vouchers/mine", a.handleVouchersMine)
			r.Get("/api/v1/devices", a.handleDevicesList)
			r.Post("/api/v1/devices/revoke", a.handleDeviceRevoke)
			// Sosial: chat, teman, grup (Fase 4).
			r.Get("/api/v1/chat/channels", a.handleChatChannels)
			r.Get("/api/v1/chat/channels/{id}/messages", a.handleChatHistory)
			r.Get("/api/v1/friends", a.handleFriendsList)
			r.Post("/api/v1/friends/request", a.handleFriendRequest)
			r.Post("/api/v1/friends/accept", a.handleFriendAccept)
			r.Post("/api/v1/friends/block", a.handleFriendBlock)
			r.Post("/api/v1/dm/{userId}", a.handleDMOpen)
			r.Get("/api/v1/groups", a.handleGroupsList)
			r.Post("/api/v1/groups", a.handleGroupCreate)
			r.Post("/api/v1/groups/join", a.handleGroupJoinByCode)
			r.Post("/api/v1/groups/{id}/join", a.handleGroupJoin)
			// Feed komunitas (redesain Chat 2026-11-22).
			r.Get("/api/v1/feed", a.handleFeedList)
			r.Post("/api/v1/feed/posts", a.handleFeedCreate)
			r.Get("/api/v1/feed/posts/{id}", a.handleFeedGetPost)
			r.Delete("/api/v1/feed/posts/{id}", a.handleFeedDelete)
			r.Post("/api/v1/feed/posts/{id}/like", a.handleFeedLike)
			r.Post("/api/v1/feed/posts/{id}/repost", a.handleFeedRepost)
			r.Post("/api/v1/feed/posts/{id}/bookmark", a.handleFeedBookmark)
			r.Post("/api/v1/feed/posts/{id}/share", a.handleFeedShare)
			r.Get("/api/v1/feed/posts/{id}/comments", a.handleFeedComments)
			r.Post("/api/v1/feed/posts/{id}/comments", a.handleFeedCommentCreate)
			r.Post("/api/v1/feed/comments/{id}/like", a.handleFeedCommentLike)
			r.Delete("/api/v1/feed/comments/{id}", a.handleFeedCommentDelete)
			r.Post("/api/v1/feed/upload", a.handleFeedUpload)
			r.Get("/api/v1/feed/notifications", a.handleFeedNotifications)
			r.Post("/api/v1/feed/notifications/read", a.handleFeedNotificationsRead)
		})
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
