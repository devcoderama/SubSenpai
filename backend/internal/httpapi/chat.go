package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"subsenpai/backend/internal/auth"
	"subsenpai/backend/internal/chat"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// handleWS meningkatkan koneksi ke WebSocket chat. Auth: header
// Authorization Bearer (dipakai aplikasi) atau ?token= (cadangan).
func (a *API) handleWS(w http.ResponseWriter, r *http.Request) {
	token := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token = strings.TrimPrefix(h, "Bearer ")
	} else if q := r.URL.Query().Get("token"); q != "" {
		token = q
	}
	if token == "" {
		writeErr(w, http.StatusUnauthorized, "login diperlukan")
		return
	}
	profileID, _, err := auth.ParseJWT(a.jwtSecret, token)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	var u chat.User
	var banned bool
	var premiumUntil *time.Time
	err = a.pool.QueryRow(r.Context(), `
		select id, coalesce(email,''), coalesce(display_name, ''),
		       name_color, name_emoji, is_banned, chat_muted_until, premium_until
		from profiles where id = $1::uuid`, profileID,
	).Scan(&u.ID, &u.Email, &u.Name, &u.NameColor, &u.NameEmoji, &banned, &u.MutedUntil, &premiumUntil)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "profil tidak ditemukan")
		return
	}
	if banned {
		writeErr(w, http.StatusForbidden, "akun diblokir")
		return
	}
	if u.Name == "" {
		u.Name = u.Email
	}
	u.Premium = premiumUntil != nil && premiumUntil.After(time.Now())
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	a.hub.Serve(conn, u)
}
