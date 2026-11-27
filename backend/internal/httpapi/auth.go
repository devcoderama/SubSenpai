package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"time"

	"subsenpai/backend/internal/auth"
)

type profileOut struct {
	ID           string     `json:"id"`
	Email        string     `json:"email"`
	DisplayName  *string    `json:"display_name,omitempty"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	Role         string     `json:"role"`
	IsPremium    bool       `json:"is_premium"`
	PremiumUntil *time.Time `json:"premium_until,omitempty"`
	AppLanguage  string     `json:"app_language"`
	FriendCode   *string    `json:"friend_code,omitempty"`
}

const friendAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func newFriendCode() string {
	var sb strings.Builder
	for i := 0; i < 8; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(friendAlphabet))))
		sb.WriteByte(friendAlphabet[n.Int64()])
	}
	return sb.String()
}

// handleGoogleLogin: verifikasi token ID Google → buat/perbarui profil →
// terbitkan JWT backend. Satu-satunya jalan masuk akun (Google-only).
func (a *API) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if a.verifier == nil || a.jwtSecret == "" {
		writeErr(w, http.StatusServiceUnavailable, "login Google belum dikonfigurasi di server")
		return
	}
	var req struct {
		IDToken      string `json:"id_token"`
		ReferralCode string `json:"referral_code"`
		DeviceID     string `json:"device_id"`
		DeviceName   string `json:"device_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IDToken == "" {
		writeErr(w, http.StatusBadRequest, "id_token wajib diisi")
		return
	}
	claims, err := a.verifier.Verify(r.Context(), req.IDToken)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "token Google tidak valid: "+err.Error())
		return
	}
	ctx := r.Context()
	var prof profileOut
	var banned bool
	var accStatus, accNote string
	var isNew bool
	upsert := func(code string) error {
		return a.pool.QueryRow(ctx, `
			insert into profiles (email, google_sub, display_name, avatar_url, friend_code)
			values ($1, $2, $3, $4, $5)
			on conflict (google_sub) do update
			  set email = excluded.email,
			      display_name = excluded.display_name,
			      avatar_url = excluded.avatar_url
			returning id, email, display_name, avatar_url, role, is_premium,
			          premium_until, app_language, friend_code, is_banned,
			          coalesce(status, 'active'), coalesce(admin_note, ''), (xmax = 0)`,
			claims.Email, claims.Sub, nullStr(claims.Name), nullStr(claims.Picture), code,
		).Scan(&prof.ID, &prof.Email, &prof.DisplayName, &prof.AvatarURL, &prof.Role,
			&prof.IsPremium, &prof.PremiumUntil, &prof.AppLanguage, &prof.FriendCode, &banned,
			&accStatus, &accNote, &isNew)
	}
	// Kode teman acak bisa bertabrakan (unik) — coba ulang beberapa kali.
	err = upsert(newFriendCode())
	for attempt := 0; err != nil && strings.Contains(err.Error(), "friend_code") && attempt < 5; attempt++ {
		err = upsert(newFriendCode())
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat profil")
		return
	}
	if banned || accStatus != "active" {
		st := accStatus
		if banned && st == "active" {
			st = "banned"
		}
		note := strings.ReplaceAll(accNote, "\n", " ")
		writeErr(w, http.StatusForbidden, "akun_nonaktif:"+st+":"+note)
		return
	}
	// Perangkat dicatat saat login; kelebihan batas (premium maks 2,
	// gratis maks 1) mencabut perangkat yang paling lama tidak aktif
	// — perangkat yang dicabut dikembalikan ke klien supaya bisa
	// ditampilkan ("Perangkat X dikeluarkan").
	kicked, ok := a.registerDevice(w, r, prof.ID, req.DeviceID, req.DeviceName)
	if !ok {
		return
	}
	// Referral: pendaftar baru yang membawa kode teman memicu voucher
	// 3 hari untuk pengundang DAN pendaftar (STRUKTUR Bagian 4).
	if isNew && req.ReferralCode != "" {
		var inviterID string
		if err := a.pool.QueryRow(ctx, `
			select id from profiles where friend_code = $1 and id <> $2::uuid`,
			req.ReferralCode, prof.ID).Scan(&inviterID); err == nil {
			for _, target := range []string{inviterID, prof.ID} {
				_, _ = a.pool.Exec(ctx, `
					insert into vouchers (code, source, duration_days, created_by, assigned_to)
					values ($1, 'referral', 3, $2::uuid, $3::uuid)`,
					"REF-"+giftCode()[3:], prof.ID, target)
			}
		}
	}
	// Profil tontonan bawaan "Utama" dibuat sekali untuk tiap akun (F26).
	var vpID string
	if err := a.pool.QueryRow(ctx, `
		insert into viewing_profiles (account_id, name, app_language)
		select $1::uuid, 'Utama', $2
		where not exists (select 1 from viewing_profiles where account_id = $1::uuid)
		returning id`, prof.ID, prof.AppLanguage).Scan(&vpID); err != nil {
		// Sudah ada dari login sebelumnya — ambil yang paling awal dibuat.
		if err := a.pool.QueryRow(ctx, `
			select id from viewing_profiles
			where account_id = $1::uuid order by created_at limit 1`, prof.ID).Scan(&vpID); err != nil {
			writeErr(w, http.StatusInternalServerError, "gagal menyiapkan profil tontonan")
			return
		}
	}
	token, err := auth.IssueJWT(a.jwtSecret, prof.ID, prof.Role, a.jwtTTL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menerbitkan sesi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":              token,
		"profile":            prof,
		"viewing_profile_id": vpID,
		"revoked_device":     kicked,
	})
}

// handleMe: profil akun yang sedang login + daftar profil tontonannya.
func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	var prof profileOut
	err := a.pool.QueryRow(ctx, `
		select id, email, display_name, avatar_url, role, is_premium,
		       premium_until, app_language, friend_code
		from profiles where id = $1::uuid`, p.ID,
	).Scan(&prof.ID, &prof.Email, &prof.DisplayName, &prof.AvatarURL, &prof.Role,
		&prof.IsPremium, &prof.PremiumUntil, &prof.AppLanguage, &prof.FriendCode)
	if err != nil {
		writeErr(w, http.StatusNotFound, "profil tidak ditemukan")
		return
	}
	type vp struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		IsKids      bool   `json:"is_kids"`
		AppLanguage string `json:"app_language"`
	}
	vps := []vp{}
	rows, err := a.pool.Query(ctx, `
		select id, name, is_kids, app_language from viewing_profiles
		where account_id = $1::uuid order by created_at`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var v vp
			if err := rows.Scan(&v.ID, &v.Name, &v.IsKids, &v.AppLanguage); err == nil {
				vps = append(vps, v)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": prof, "viewing_profiles": vps})
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
