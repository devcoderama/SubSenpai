package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
)

// ---------- Kanal & riwayat chat ----------

type channelOut struct {
	ID      string  `json:"id"`
	Slug    string  `json:"slug"`
	Name    string  `json:"name"`
	Kind    string  `json:"kind"`
	GroupID *string `json:"group_id,omitempty"`
	OtherID *string `json:"other_user_id,omitempty"`
}

// handleChatChannels: kanal global + kanal grup saya + DM saya.
func (a *API) handleChatChannels(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	out := []channelOut{}
	rows, err := a.pool.Query(r.Context(), `
		select c.id, c.slug, c.name, c.kind, c.group_id,
		       case when c.kind = 'dm' and c.dm_user_a_id = $1::uuid then c.dm_user_b_id
		            when c.kind = 'dm' then c.dm_user_a_id end
		from chat_channels c
		where c.kind = 'global'
		   or (c.kind = 'group' and exists(
				select 1 from group_members gm
				where gm.group_id = c.group_id and gm.user_id = $1::uuid))
		   or (c.kind = 'dm' and (c.dm_user_a_id = $1::uuid or c.dm_user_b_id = $1::uuid))
		order by c.kind, c.sort_order, c.name`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var c channelOut
			if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.Kind, &c.GroupID, &c.OtherID); err == nil {
				out = append(out, c)
			}
		}
	}
	// Nama DM diganti nama tampil lawan bicara.
	for i, c := range out {
		if c.Kind == "dm" && c.OtherID != nil {
			var name string
			if err := a.pool.QueryRow(r.Context(), `
				select coalesce(display_name, email, 'Pengguna') from profiles where id = $1::uuid`,
				*c.OtherID).Scan(&name); err == nil {
				out[i].Name = name
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": out})
}

type historyMsg struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	DisplayName  string    `json:"display_name"`
	NameColor    *string   `json:"name_color,omitempty"`
	NameEmoji    *string   `json:"name_emoji,omitempty"`
	Body         *string   `json:"body,omitempty"`
	ReplyToID    *string   `json:"reply_to_id,omitempty"`
	ReplyPreview *string   `json:"reply_preview,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	Deleted      bool      `json:"deleted"`
}

// handleChatHistory: 50 pesan terakhir kanal (akses dicek seperti WS).
func (a *API) handleChatHistory(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	channelID := chi.URLParam(r, "id")
	if !a.hub.CanAccess(r.Context(), p.ID, channelID) {
		writeErr(w, http.StatusForbidden, "tidak punya akses ke kanal ini")
		return
	}
	rows, err := a.pool.Query(r.Context(), `
		select m.id, m.user_id, coalesce(pr.display_name, pr.email, 'Pengguna'),
		       case when pr.premium_until > now() then pr.name_color end,
		       case when pr.premium_until > now() then pr.name_emoji end,
		       m.body, m.reply_to_id, m.created_at, m.deleted_at
		from chat_messages m
		join profiles pr on pr.id = m.user_id
		where m.channel_id = $1::uuid
		order by m.created_at desc limit 50`, channelID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	defer rows.Close()
	msgs := []historyMsg{}
	for rows.Next() {
		var m historyMsg
		var deletedAt *time.Time
		if err := rows.Scan(&m.ID, &m.UserID, &m.DisplayName, &m.NameColor, &m.NameEmoji,
			&m.Body, &m.ReplyToID, &m.CreatedAt, &deletedAt); err == nil {
			m.Deleted = deletedAt != nil
			if m.Deleted {
				m.Body = nil
			}
			msgs = append(msgs, m)
		}
	}
	// Urutan tampil: tertua → terbaru; isi pratinjau balasan.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	byID := map[string]*historyMsg{}
	for i := range msgs {
		byID[msgs[i].ID] = &msgs[i]
	}
	for i := range msgs {
		if msgs[i].ReplyToID != nil {
			if orig, ok := byID[*msgs[i].ReplyToID]; ok {
				if orig.Deleted {
					s := "pesan telah dihapus"
					msgs[i].ReplyPreview = &s
				} else {
					msgs[i].ReplyPreview = orig.Body
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// ---------- Teman ----------

type friendOut struct {
	UserID      string  `json:"user_id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	Status      string  `json:"status"` // accepted | pending_in | pending_out
}

// handleFriendsList: teman + permintaan masuk/keluar.
func (a *API) handleFriendsList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	out := []friendOut{}
	rows, err := a.pool.Query(r.Context(), `
		select pr.id, pr.email, pr.display_name, pr.avatar_url,
		       case when f.status = 'accepted' then 'accepted'
		            when f.requester_id = $1::uuid then 'pending_out'
		            else 'pending_in' end
		from friendships f
		join profiles pr on pr.id = case when f.requester_id = $1::uuid then f.addressee_id else f.requester_id end
		where f.requester_id = $1::uuid or f.addressee_id = $1::uuid
		order by pr.display_name nulls last, pr.email`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var f friendOut
			if err := rows.Scan(&f.UserID, &f.Email, &f.DisplayName, &f.AvatarURL, &f.Status); err == nil {
				out = append(out, f)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"friends": out})
}

func (a *API) blockedEitherWay(userA, userB string) bool {
	var blocked bool
	_ = a.pool.QueryRow(context.Background(), `
		select exists(select 1 from user_blocks
			where (blocker_id = $1::uuid and blocked_id = $2::uuid)
			   or (blocker_id = $2::uuid and blocked_id = $1::uuid))`,
		userA, userB).Scan(&blocked)
	return blocked
}

// handleFriendRequest: kirim permintaan pertemanan via kode teman.
func (a *API) handleFriendRequest(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		FriendCode string `json:"friend_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FriendCode == "" {
		writeErr(w, http.StatusBadRequest, "friend_code wajib diisi")
		return
	}
	ctx := r.Context()
	var target string
	if err := a.pool.QueryRow(ctx, `
		select id from profiles where friend_code = $1`, req.FriendCode).Scan(&target); err != nil {
		writeErr(w, http.StatusNotFound, "kode teman tidak ditemukan")
		return
	}
	if target == p.ID {
		writeErr(w, http.StatusBadRequest, "tidak bisa menambahkan diri sendiri")
		return
	}
	if a.blockedEitherWay(p.ID, target) {
		writeErr(w, http.StatusForbidden, "pertemanan terblokir")
		return
	}
	// Permintaan balik yang masih pending → langsung diterima.
	var reverseStatus string
	err := a.pool.QueryRow(ctx, `
		select status from friendships
		where requester_id = $1::uuid and addressee_id = $2::uuid`, target, p.ID).Scan(&reverseStatus)
	if err == nil && reverseStatus == "pending" {
		_, _ = a.pool.Exec(ctx, `
			update friendships set status = 'accepted'
			where requester_id = $1::uuid and addressee_id = $2::uuid`, target, p.ID)
		writeJSON(w, http.StatusOK, map[string]any{"status": "accepted"})
		return
	}
	if _, err := a.pool.Exec(ctx, `
		insert into friendships (requester_id, addressee_id, status)
		values ($1::uuid, $2::uuid, 'pending')
		on conflict (requester_id, addressee_id) do nothing`, p.ID, target); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "pending"})
}

// handleFriendAccept menerima permintaan dari user_id pengirim.
func (a *API) handleFriendAccept(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "user_id wajib diisi")
		return
	}
	tag, err := a.pool.Exec(r.Context(), `
		update friendships set status = 'accepted'
		where requester_id = $1::uuid and addressee_id = $2::uuid and status = 'pending'`,
		req.UserID, p.ID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "permintaan pertemanan tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "accepted"})
}

// handleFriendBlock memblokir user + menghapus pertemanan.
func (a *API) handleFriendBlock(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeErr(w, http.StatusBadRequest, "user_id wajib diisi")
		return
	}
	ctx := r.Context()
	if _, err := a.pool.Exec(ctx, `
		insert into user_blocks (blocker_id, blocked_id)
		values ($1::uuid, $2::uuid)
		on conflict do nothing`, p.ID, req.UserID); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	_, _ = a.pool.Exec(ctx, `
		delete from friendships
		where (requester_id = $1::uuid and addressee_id = $2::uuid)
		   or (requester_id = $2::uuid and addressee_id = $1::uuid)`, p.ID, req.UserID)
	writeJSON(w, http.StatusOK, map[string]any{"blocked": true})
}

// handleDMOpen membuka (atau membuat) kanal DM dengan satu user.
func (a *API) handleDMOpen(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	other := chi.URLParam(r, "userId")
	if other == p.ID {
		writeErr(w, http.StatusBadRequest, "tidak bisa DM diri sendiri")
		return
	}
	if a.blockedEitherWay(p.ID, other) {
		writeErr(w, http.StatusForbidden, "percakapan terblokir")
		return
	}
	aID, bID := p.ID, other
	if bID < aID {
		aID, bID = bID, aID
	}
	slug := "dm-" + aID + "-" + bID
	var channelID string
	err := a.pool.QueryRow(r.Context(), `
		insert into chat_channels (slug, name, kind, dm_user_a_id, dm_user_b_id)
		values ($1, 'DM', 'dm', $2::uuid, $3::uuid)
		on conflict (slug) do update set name = excluded.name
		returning id`, slug, aID, bID).Scan(&channelID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "user tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channel_id": channelID})
}

// ---------- Grup ----------

type groupOut struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	IsPublic    bool    `json:"is_public"`
	OwnerID     string  `json:"owner_id"`
	InviteCode  *string `json:"invite_code,omitempty"`
	IsMember    bool    `json:"is_member"`
	ChannelID   *string `json:"channel_id,omitempty"`
}

// handleGroupsList: grup saya + grup publik untuk ditemukan.
func (a *API) handleGroupsList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	out := []groupOut{}
	rows, err := a.pool.Query(r.Context(), `
		select g.id, g.name, g.description, g.is_public, g.owner_id,
		       case when gm.user_id is not null then g.invite_code end,
		       gm.user_id is not null, c.id
		from groups g
		left join group_members gm on gm.group_id = g.id and gm.user_id = $1::uuid
		left join chat_channels c on c.group_id = g.id
		where gm.user_id is not null or g.is_public
		order by (gm.user_id is not null) desc, g.name`, p.ID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var g groupOut
			if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.OwnerID,
				&g.InviteCode, &g.IsMember, &g.ChannelID); err == nil {
				out = append(out, g)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out})
}

// handleGroupCreate membuat grup + keanggotaan owner + kanal chat-nya.
func (a *API) handleGroupCreate(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		IsPublic    *bool  `json:"is_public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name wajib diisi")
		return
	}
	isPublic := true
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}
	ctx := r.Context()
	code := giftCode()[3:] // 10 karakter acak untuk tautan undangan
	var groupID string
	if err := a.pool.QueryRow(ctx, `
		insert into groups (name, description, owner_id, is_public, invite_code)
		values ($1, $2, $3::uuid, $4, $5)
		returning id`, req.Name, nullStr(req.Description), p.ID, isPublic, code).Scan(&groupID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat grup")
		return
	}
	if _, err := a.pool.Exec(ctx, `
		insert into group_members (group_id, user_id, role)
		values ($1::uuid, $2::uuid, 'owner')`, groupID, p.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat grup")
		return
	}
	var channelID string
	if err := a.pool.QueryRow(ctx, `
		insert into chat_channels (slug, name, kind, group_id)
		values ($1, $2, 'group', $3::uuid)
		returning id`, "grup-"+groupID, req.Name, groupID).Scan(&channelID); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal membuat kanal grup")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"group_id": groupID, "channel_id": channelID, "invite_code": code,
	})
}

// handleGroupJoin bergabung ke grup publik lewat id.
func (a *API) handleGroupJoin(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	groupID := chi.URLParam(r, "id")
	var isPublic bool
	if err := a.pool.QueryRow(r.Context(), `
		select is_public from groups where id = $1::uuid`, groupID).Scan(&isPublic); err != nil {
		writeErr(w, http.StatusNotFound, "grup tidak ditemukan")
		return
	}
	if !isPublic {
		writeErr(w, http.StatusForbidden, "grup privat — bergabung lewat tautan undangan")
		return
	}
	if _, err := a.pool.Exec(r.Context(), `
		insert into group_members (group_id, user_id, role)
		values ($1::uuid, $2::uuid, 'member')
		on conflict do nothing`, groupID, p.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"joined": true})
}

// handleGroupJoinByCode bergabung lewat kode undangan (tautan resmi
// aplikasi — pengecualian aturan tanpa-tautan; berlaku juga grup privat).
func (a *API) handleGroupJoinByCode(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InviteCode == "" {
		writeErr(w, http.StatusBadRequest, "invite_code wajib diisi")
		return
	}
	ctx := r.Context()
	var groupID, name string
	if err := a.pool.QueryRow(ctx, `
		select id, name from groups
		where invite_code = $1 and invite_enabled`, req.InviteCode).Scan(&groupID, &name); err != nil {
		writeErr(w, http.StatusNotFound, "kode undangan tidak valid atau nonaktif")
		return
	}
	if _, err := a.pool.Exec(ctx, `
		insert into group_members (group_id, user_id, role)
		values ($1::uuid, $2::uuid, 'member')
		on conflict do nothing`, groupID, p.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"joined": true, "group_id": groupID, "group_name": name})
}
