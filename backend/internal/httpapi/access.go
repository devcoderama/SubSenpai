package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/billing"
)

// Tier akses penonton pada satu request.
type tier string

const (
	tierGuest   tier = "guest"
	tierFree    tier = "free"
	tierPremium tier = "premium"
)

// isPremiumAktif: status premium bersumber dari langganan aktif
// (subscriptions) — profiles.premium_until adalah cerminnya.
func (a *API) isPremiumAktif(userID string) bool {
	var ok bool
	err := a.pool.QueryRow(context.Background(), `
		select exists(
			select 1 from subscriptions
			where user_id = $1::uuid and status = 'active'
			  and (current_period_end is null or current_period_end > now())
		) or exists(
			select 1 from profiles
			where id = $1::uuid and premium_until is not null and premium_until > now()
		)`, userID).Scan(&ok)
	return err == nil && ok
}

// viewerTier menentukan tingkat akses: premium / free (login) / tamu.
func (a *API) viewerTier(r *http.Request) (tier, *auth.Profile) {
	p := auth.FromContext(r.Context())
	if p == nil {
		return tierGuest, nil
	}
	if a.isPremiumAktif(p.ID) {
		return tierPremium, p
	}
	return tierFree, p
}

// deviceKey menormalkan identitas perangkat tamu: header X-Device-Id
// di-hash; tanpa header, dipakai hash alamat IP (cadangan kasar).
func deviceKey(r *http.Request) string {
	id := r.Header.Get("X-Device-Id")
	if id == "" {
		id = "ip:" + r.RemoteAddr
	}
	sum := sha256.Sum256([]byte("subsenpai-guest:" + id))
	return hex.EncodeToString(sum[:])
}

// gateEpisode menegakkan batas tamu untuk satu episode: tamu hanya boleh
// membuka SATU episode (yang pertama ia buka, tercatat di guest_access);
// membuka episode lain ditolak. User login selalu lolos.
func (a *API) gateEpisode(w http.ResponseWriter, r *http.Request, episodeID, titleID string) bool {
	t, _ := a.viewerTier(r)
	if t != tierGuest {
		return true
	}
	ctx := r.Context()
	dk := deviceKey(r)
	var usedEpisode string
	err := a.pool.QueryRow(ctx, `
		select episode_id from guest_access
		where device_key = $1 and episode_id is not null limit 1`, dk).Scan(&usedEpisode)
	if err == nil {
		if usedEpisode == episodeID {
			return true
		}
		writeErr(w, http.StatusForbidden, "batas_tamu: tamu hanya bisa membuka 1 episode — masuk untuk melanjutkan")
		return false
	}
	if _, err := a.pool.Exec(ctx, `
		insert into guest_access (device_key, title_id, episode_id)
		values ($1, $2::uuid, $3::uuid)`, dk, titleID, episodeID); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return false
	}
	return true
}

// gateChapter: pasangan gateEpisode untuk chapter (batas 1 chapter tamu).
func (a *API) gateChapter(w http.ResponseWriter, r *http.Request, chapterID, titleID string) bool {
	t, _ := a.viewerTier(r)
	if t != tierGuest {
		return true
	}
	ctx := r.Context()
	dk := deviceKey(r)
	var usedChapter string
	err := a.pool.QueryRow(ctx, `
		select chapter_id from guest_access
		where device_key = $1 and chapter_id is not null limit 1`, dk).Scan(&usedChapter)
	if err == nil {
		if usedChapter == chapterID {
			return true
		}
		writeErr(w, http.StatusForbidden, "batas_tamu: tamu hanya bisa membuka 1 chapter — masuk untuk melanjutkan")
		return false
	}
	if _, err := a.pool.Exec(ctx, `
		insert into guest_access (device_key, title_id, chapter_id)
		values ($1, $2::uuid, $3::uuid)`, dk, titleID, chapterID); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return false
	}
	return true
}

// extendPremium adalah pembungkus tipis ke billing.ExtendPremium —
// satu-satunya jalan aktivasi premium (webhook, polling GoPay, voucher,
// trial) agar pencatatannya konsisten.
func (a *API) extendPremium(userID string, days int, provider, ref string) (time.Time, error) {
	return billing.ExtendPremium(context.Background(), a.pool, userID, days, provider, ref)
}
