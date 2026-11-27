package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey struct{}

// Profile adalah identitas user yang terotentikasi pada satu request.
type Profile struct {
	ID        string
	Email     string
	Role      string
	Banned    bool
	Status    string
	AdminNote string
}

// blockedMessage menyusun pesan penolakan akun nonaktif yang
// dipahami aplikasi: "akun_nonaktif:<status>:<catatan admin>".
func blockedMessage(status, note string) string {
	note = strings.ReplaceAll(note, "\n", " ")
	if r := []rune(note); len(r) > 300 {
		note = string(r[:300])
	}
	return "akun_nonaktif:" + status + ":" + note
}

// Authenticator memverifikasi JWT backend dan memuat profil dari database.
type Authenticator struct {
	Pool      *pgxpool.Pool
	JWTSecret string
}

func writeAuthErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// revokedDevice: nama perangkat bila X-Device-Id pada request ini
// sudah dicabut untuk akun tersebut (dikeluarkan karena login di
// perangkat lain melewati batas, atau dicabut manual di halaman
// perangkat); ok=false bila tidak dicabut. Tanpa header atau tanpa
// baris (login lama / klien lama yang belum mencatat perangkat) →
// tidak diblokir, supaya peluncuran tidak mengusir sesi lama yang sah.
func (a *Authenticator) revokedDevice(ctx context.Context, profileID string, r *http.Request) (string, bool) {
	deviceID := r.Header.Get("X-Device-Id")
	if deviceID == "" {
		return "", false
	}
	var name *string
	err := a.Pool.QueryRow(ctx, `
		select device_name from user_devices
		where user_id = $1::uuid and device_id = $2 and revoked_at is not null`,
		profileID, deviceID).Scan(&name)
	if err != nil {
		return "", false
	}
	if name == nil || *name == "" {
		return "perangkat ini", true
	}
	return *name, true
}

// Require adalah middleware untuk endpoint khusus user login: token
// "Authorization: Bearer <JWT backend>" wajib valid dan akun tidak banned.
func (a *Authenticator) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeAuthErr(w, http.StatusUnauthorized, "login diperlukan")
			return
		}
		profileID, _, err := ParseJWT(a.JWTSecret, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			writeAuthErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		var p Profile
		err = a.Pool.QueryRow(r.Context(), `
			select id, coalesce(email,''), role, is_banned,
			       coalesce(status, 'active'), coalesce(admin_note, '')
			from profiles where id = $1::uuid`, profileID,
		).Scan(&p.ID, &p.Email, &p.Role, &p.Banned, &p.Status, &p.AdminNote)
		if err != nil {
			writeAuthErr(w, http.StatusUnauthorized, "profil tidak ditemukan")
			return
		}
		if p.Banned || p.Status != "active" {
			st := p.Status
			if p.Banned && st == "active" {
				st = "banned"
			}
			writeAuthErr(w, http.StatusForbidden, blockedMessage(st, p.AdminNote))
			return
		}
		// Perangkat yang dicabut tidak lagi bisa memakai endpoint login:
		// 401 "perangkat_dicabut:<nama>" — klien menghapus token dan
		// menampilkan "Sesi berakhir — akun ini login di perangkat lain".
		if name, revoked := a.revokedDevice(r.Context(), p.ID, r); revoked {
			writeAuthErr(w, http.StatusUnauthorized, "perangkat_dicabut:"+name)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, &p)))
	})
}

// FromContext mengambil profil hasil middleware Require.
func FromContext(ctx context.Context) *Profile {
	p, _ := ctx.Value(ctxKey{}).(*Profile)
	return p
}

// Optional: bila header Authorization berisi JWT yang valid, profil
// dimuat ke context; tanpa header, request lanjut sebagai tamu. Header
// ada tapi tidak valid → 401 (jangan diam-diam turun jadi tamu).
func (a *Authenticator) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(h, "Bearer ") {
			writeAuthErr(w, http.StatusUnauthorized, "format Authorization tidak valid")
			return
		}
		profileID, _, err := ParseJWT(a.JWTSecret, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			writeAuthErr(w, http.StatusUnauthorized, err.Error())
			return
		}
		var p Profile
		err = a.Pool.QueryRow(r.Context(), `
			select id, coalesce(email,''), role, is_banned,
			       coalesce(status, 'active'), coalesce(admin_note, '')
			from profiles where id = $1::uuid`, profileID,
		).Scan(&p.ID, &p.Email, &p.Role, &p.Banned, &p.Status, &p.AdminNote)
		if err != nil {
			writeAuthErr(w, http.StatusUnauthorized, "profil tidak ditemukan")
			return
		}
		if p.Banned || p.Status != "active" {
			st := p.Status
			if p.Banned && st == "active" {
				st = "banned"
			}
			writeAuthErr(w, http.StatusForbidden, blockedMessage(st, p.AdminNote))
			return
		}
		// Perangkat yang dicabut turun jadi tamu (bukan 401): rute
		// opsional tetap melayani konten tamu; 401-nya muncul di
		// endpoint Require berikutnya (mis. /api/v1/me yang dipoll
		// aplikasi/web tiap siklus).
		if _, revoked := a.revokedDevice(r.Context(), p.ID, r); revoked {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, &p)))
	})
}
