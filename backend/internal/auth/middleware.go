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
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, &p)))
	})
}
