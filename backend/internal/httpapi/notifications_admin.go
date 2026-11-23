package httpapi

// Notifikasi siaran admin (permintaan Rams 2026-11-23): admin
// menulis judul + isi dari dashboard, backend membuat satu baris
// broadcasts lalu menyebarkannya (fan-out) ke tabel notifications
// semua pengguna (kind='broadcast') — tampil di halaman Notifikasi
// aplikasi bersama notifikasi sosial, lengkap dengan lencana
// belum-dibaca dan notifikasi sistem Android dari polling berkala.

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type broadcastOut struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	Audience   string    `json:"audience"`
	SentCount  int       `json:"sent_count"`
	ReadCount  int       `json:"read_count"`
	AdminEmail string    `json:"admin_email"`
	CreatedAt  time.Time `json:"created_at"`
}

func (a *API) handleAdminNotificationSend(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	var req struct {
		Title    string `json:"title"`
		Body     string `json:"body"`
		Audience string `json:"audience"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	req.Title = truncate(req.Title, 140)
	req.Body = truncate(req.Body, 1000)
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "judul notifikasi wajib diisi")
		return
	}
	if req.Audience != "premium" && req.Audience != "free" {
		req.Audience = "all"
	}
	ctx := r.Context()
	var bid string
	if err := a.pool.QueryRow(ctx, `
		insert into broadcasts (admin_id, audience, title, body)
		values ($1::uuid, $2, $3, $4) returning id`,
		p.ID, req.Audience, req.Title, req.Body).Scan(&bid); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat siaran")
		return
	}
	aud := ""
	switch req.Audience {
	case "premium":
		aud = " and premium_until > now()"
	case "free":
		aud = " and (premium_until is null or premium_until <= now())"
	}
	tag, err := a.pool.Exec(ctx, `
		insert into notifications (user_id, kind, title, body, actor_id, broadcast_id)
		select id, 'broadcast', $2, $3, $1::uuid, $4::uuid from profiles
		where true`+aud, p.ID, req.Title, req.Body, bid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyebarkan notifikasi")
		return
	}
	sent := int(tag.RowsAffected())
	_, _ = a.pool.Exec(ctx, `update broadcasts set sent_count = $2 where id = $1::uuid`, bid, sent)
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'notification_send', $2, $3)`,
		p.ID, bid, map[string]any{"title": req.Title, "audience": req.Audience, "recipients": sent})
	// Cermin ke Discord komunitas (#pengumuman) — fire-and-forget.
	a.discordNotify(a.cfg.DiscordChannelPengumuman,
		"📣 **"+req.Title+"**\n\n"+req.Body)
	// Cermin ke topik Pengumuman grup Telegram — fire-and-forget.
	a.telegramNotify(a.cfg.TelegramTopicPengumuman,
		"📣 **"+req.Title+"**\n\n"+req.Body)
	writeJSON(w, http.StatusOK, map[string]any{"id": bid, "recipients": sent})
}

func (a *API) handleAdminNotifications(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		select b.id, b.title, b.body, b.audience, b.sent_count,
		       (select count(*) from notifications n
		        where n.broadcast_id = b.id and n.read_at is not null),
		       coalesce(pr.email, ''), b.created_at
		from broadcasts b
		left join profiles pr on pr.id = b.admin_id
		order by b.created_at desc limit 50`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	defer rows.Close()
	out := []broadcastOut{}
	for rows.Next() {
		var b broadcastOut
		if err := rows.Scan(&b.ID, &b.Title, &b.Body, &b.Audience, &b.SentCount,
			&b.ReadCount, &b.AdminEmail, &b.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		out = append(out, b)
	}
	writeJSON(w, http.StatusOK, map[string]any{"broadcasts": out})
}

func (a *API) handleAdminNotificationDelete(w http.ResponseWriter, r *http.Request) {
	p, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	// Hapus salinan di kotak masuk pengguna dulu (FK-nya SET NULL,
	// jadi tanpa ini notifikasi yatim akan tetap tampil).
	_, _ = a.pool.Exec(ctx, `delete from notifications where broadcast_id = $1::uuid`, id)
	tag, err := a.pool.Exec(ctx, `delete from broadcasts where id = $1::uuid`, id)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "siaran tidak ditemukan")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into admin_logs (admin_id, action, target, detail)
		values ($1::uuid, 'notification_delete', $2, '{}'::jsonb)`, p.ID, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
