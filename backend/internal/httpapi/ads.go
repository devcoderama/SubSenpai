package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
)

// Unlock episode lewat iklan (permintaan Rams 2026-11-23, Monetag):
// akun gratis & tamu menonton iklan ±15 detik di aplikasi, lalu
// episode itu terbuka permanen untuk mereka. Alurnya dua langkah
// agar waktu tonton ditegakkan SERVER, bukan sekadar timer klien:
//   POST /episodes/{id}/ad-unlock/start    -> token bertanda tangan
//   POST /episodes/{id}/ad-unlock/complete -> token yang sama,
//        diterima hanya bila sudah >= adUnlockWait sejak start.
// Token = base64url(episode|subjek|unixStart) + "." + HMAC-SHA256
// (kunci JWT_SECRET) — tanpa tabel tambahan, dan token milik subjek
// lain (user/perangkat lain) otomatis tidak berlaku.

// adUnlockWait: lama minimal iklan "ditonton" sebelum unlock sah.
// Variabel (bukan konstanta) agar pengujian bisa memendekkannya.
var adUnlockWait = 15 * time.Second

// adSubject: identitas pemilik unlock — "u:<user_id>" untuk user
// login, "d:<device_key>" untuk tamu.
func adSubject(r *http.Request) string {
	if p := auth.FromContext(r.Context()); p != nil {
		return "u:" + p.ID
	}
	return "d:" + deviceKey(r)
}

func (a *API) adTokenSign(payload string) string {
	mac := hmac.New(sha256.New, []byte(a.cfg.JWTSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *API) adTokenMake(episodeID, subject string, start time.Time) string {
	payload := episodeID + "|" + subject + "|" + strconv.FormatInt(start.Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + a.adTokenSign(payload)
}

// adTokenVerify: token sah untuk episode+subjek ini dan sudah cukup
// lama sejak diterbitkan (belum kedaluwarsa 10 menit).
func (a *API) adTokenVerify(token, episodeID, subject string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	payload := string(raw)
	if !hmac.Equal([]byte(a.adTokenSign(payload)), []byte(parts[1])) {
		return false
	}
	f := strings.Split(payload, "|")
	if len(f) != 3 || f[0] != episodeID || f[1] != subject {
		return false
	}
	startUnix, err := strconv.ParseInt(f[2], 10, 64)
	if err != nil {
		return false
	}
	elapsed := time.Since(time.Unix(startUnix, 0))
	return elapsed >= adUnlockWait && elapsed <= 10*time.Minute
}

// hasAdUnlock: episode ini sudah dibuka lewat iklan oleh subjek
// request (user login atau perangkat tamu).
func (a *API) hasAdUnlock(r *http.Request, episodeID string) bool {
	ctx := r.Context()
	var ok bool
	var err error
	if p := auth.FromContext(ctx); p != nil {
		err = a.pool.QueryRow(ctx, `
			select exists(select 1 from ad_unlocks
			              where episode_id = $1::uuid and user_id = $2::uuid)`,
			episodeID, p.ID).Scan(&ok)
	} else {
		err = a.pool.QueryRow(ctx, `
			select exists(select 1 from ad_unlocks
			              where episode_id = $1::uuid and device_key = $2)`,
			episodeID, deviceKey(r)).Scan(&ok)
	}
	return err == nil && ok
}

// adUnlockedEpisodes: himpunan episode_id pada satu judul yang sudah
// dibuka lewat iklan oleh subjek request (untuk stempel locked di
// halaman detail).
func (a *API) adUnlockedEpisodes(r *http.Request, titleID string) map[string]bool {
	out := map[string]bool{}
	ctx := r.Context()
	var userID any
	var devKey any
	if p := auth.FromContext(ctx); p != nil {
		userID = p.ID
	} else {
		devKey = deviceKey(r)
	}
	rows, err := a.pool.Query(ctx, `
		select au.episode_id from ad_unlocks au
		join episodes e on e.id = au.episode_id
		where e.title_id = $1::uuid
		  and (($2::uuid is not null and au.user_id = $2::uuid)
		    or ($3::text is not null and au.device_key = $3::text))`,
		titleID, userID, devKey)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			out[id] = true
		}
	}
	return out
}

// handleAdUnlockStart: terbitkan token sesi iklan untuk satu episode.
func (a *API) handleAdUnlockStart(w http.ResponseWriter, r *http.Request) {
	episodeID := chi.URLParam(r, "id")
	var exists bool
	err := a.pool.QueryRow(r.Context(), `
		select exists(select 1 from episodes e
		              join titles t on t.id = e.title_id
		              where e.id = $1::uuid and not t.is_hidden)`, episodeID).Scan(&exists)
	if err != nil || !exists {
		writeErr(w, http.StatusNotFound, "episode tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":        a.adTokenMake(episodeID, adSubject(r), time.Now()),
		"wait_seconds": int(adUnlockWait.Seconds()),
	})
}

// handleAdUnlockComplete: tukar token yang sudah cukup umur menjadi
// baris ad_unlocks (idempoten — mengulang complete tidak menggandakan).
func (a *API) handleAdUnlockComplete(w http.ResponseWriter, r *http.Request) {
	episodeID := chi.URLParam(r, "id")
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		writeErr(w, http.StatusBadRequest, "token wajib diisi")
		return
	}
	if !a.adTokenVerify(req.Token, episodeID, adSubject(r)) {
		writeErr(w, http.StatusBadRequest, "sesi iklan belum selesai — tonton iklannya sampai selesai dulu")
		return
	}
	ctx := r.Context()
	var err error
	if p := auth.FromContext(ctx); p != nil {
		_, err = a.pool.Exec(ctx, `
			insert into ad_unlocks (user_id, episode_id) values ($1::uuid, $2::uuid)
			on conflict (user_id, episode_id) where user_id is not null do nothing`,
			p.ID, episodeID)
	} else {
		_, err = a.pool.Exec(ctx, `
			insert into ad_unlocks (device_key, episode_id) values ($1, $2::uuid)
			on conflict (device_key, episode_id) where device_key is not null do nothing`,
			deviceKey(r), episodeID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unlocked": true})
}
