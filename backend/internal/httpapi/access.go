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

// watchAllowed: aturan nonton (diubah Rams 2026-11-27) —
// anime & donghua BEBAS untuk semua tier: semua episode tanpa
// premium dan tanpa iklan. Film khusus premium (atau dibuka lewat
// iklan — ad-unlock diperiksa di gateEpisode, bukan di sini).
// isFirst dipertahankan demi kompatibilitas pemanggil.
func watchAllowed(t tier, titleType string, isFirst bool) bool {
	if t == tierPremium {
		return true
	}
	return titleType == "anime" || titleType == "donghua"
}

// episodeAccessInfo: tipe judul + apakah episode ini episode
// pertamanya (episode_no terkecil pada judul tersebut).
func (a *API) episodeAccessInfo(ctx context.Context, titleID string, episodeNo int) (string, bool, error) {
	var titleType string
	var isFirst bool
	err := a.pool.QueryRow(ctx, `
		select t.type,
		       not exists(select 1 from episodes e
		                  where e.title_id = t.id and e.episode_no < $2 and not e.is_hidden)
		from titles t where t.id = $1::uuid and not t.is_hidden`, titleID, episodeNo).Scan(&titleType, &isFirst)
	return titleType, isFirst, err
}

// gateEpisode menegakkan aturan nonton untuk satu episode
// (diubah Rams 2026-11-27): anime & donghua BEBAS untuk semua —
// premium, akun gratis, maupun tamu — tanpa premium, tanpa iklan,
// dan tanpa batas 1 episode tamu (batas tamu lama ikut dicabut
// karena aturan "hanya episode pertama" sudah tidak ada).
// Film khusus premium, atau dibuka lewat iklan (Monetag) untuk
// pemilik unlock-nya (hasAdUnlock diperiksa sebelum aturan ini).
func (a *API) gateEpisode(w http.ResponseWriter, r *http.Request, episodeID, titleID string, episodeNo int) bool {
	// Info akses dibaca lebih dulu: judul yang disembunyikan admin
	// (atau tidak ada) tidak bisa diputar siapa pun, termasuk premium.
	titleType, isFirst, err := a.episodeAccessInfo(r.Context(), titleID, episodeNo)
	if err != nil {
		writeErr(w, http.StatusNotFound, "judul tidak ditemukan")
		return false
	}
	t, _ := a.viewerTier(r)
	if t == tierPremium {
		return true
	}
	// Episode yang sudah dibuka lewat iklan (Monetag) lolos untuk
	// pemilik unlock-nya.
	if a.hasAdUnlock(r, episodeID) {
		return true
	}
	if !watchAllowed(t, titleType, isFirst) {
		writeErr(w, http.StatusForbidden, "premium_required: film khusus Premium — atau buka dengan menonton iklan")
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
