// Package chat adalah hub WebSocket chat SubSenpai (Fase 4): kanal topik
// global, kanal grup, DM antar teman, kehadiran sederhana, dan watch
// party efemer (sinyal sinkronisasi putar). Pesan teks persisten di
// chat_messages; tautan luar ditolak server sebelum disimpan (aturan
// komunitas). Kustomisasi nama premium (warna/emoji) hanya tampil selama
// premium user aktif.
package chat

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User adalah identitas penampil satu koneksi.
type User struct {
	ID         string
	Email      string
	Name       string
	NameColor  *string
	NameEmoji  *string
	Premium    bool
	MutedUntil *time.Time
}

// Client adalah satu koneksi WebSocket.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	user     User
	channels map[string]bool
	parties  map[string]bool
}

type Hub struct {
	pool    *pgxpool.Pool
	mu      sync.Mutex
	rooms   map[string]map[*Client]bool
	parties map[string]map[*Client]bool
}

func NewHub(pool *pgxpool.Pool) *Hub {
	return &Hub{
		pool:    pool,
		rooms:   map[string]map[*Client]bool{},
		parties: map[string]map[*Client]bool{},
	}
}

var linkRe = regexp.MustCompile(`(?i)(https?://|www\.)|([a-z0-9-]+\.(com|net|org|id|xyz|top|info|me|co|io|tv|app|dev|site|online|link|club|fun)\b)`)

// ContainsLink menerapkan aturan komunitas: tautan luar dilarang.
func ContainsLink(s string) bool { return linkRe.MatchString(s) }

// CanAccess: global = semua user login; grup = anggota; DM = peserta.
func (h *Hub) CanAccess(ctx context.Context, userID, channelID string) bool {
	var ok bool
	err := h.pool.QueryRow(ctx, `
		select case
			when c.kind = 'global' then true
			when c.kind = 'group' then exists(
				select 1 from group_members gm
				where gm.group_id = c.group_id and gm.user_id = $1::uuid)
			when c.kind = 'dm' then (c.dm_user_a_id = $1::uuid or c.dm_user_b_id = $1::uuid)
			else false end
		from chat_channels c where c.id = $2::uuid`, userID, channelID).Scan(&ok)
	return err == nil && ok
}

type inbound struct {
	Type      string  `json:"type"`
	ChannelID string  `json:"channel_id"`
	Body      string  `json:"body"`
	ReplyToID *string `json:"reply_to_id"`
	Code      string  `json:"code"`
	EpisodeID string  `json:"episode_id"`
	Action    string  `json:"action"`
	Position  int     `json:"position_sec"`
}

type outbound struct {
	Type    string `json:"type"`
	Channel string `json:"channel_id,omitempty"`
	Code    string `json:"code,omitempty"`
	Error   string `json:"error,omitempty"`
	Message any    `json:"message,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Online  bool   `json:"online,omitempty"`
	Action  string `json:"action,omitempty"`
	Position int   `json:"position_sec,omitempty"`
	From    string `json:"from,omitempty"`
}

func (c *Client) emit(v outbound) {
	b, _ := json.Marshal(v)
	select {
	case c.send <- b:
	default: // antrean penuh — buang, jangan blokir hub
	}
}

// Serve menjalankan pumps baca/tulis untuk satu koneksi (blokir sampai
// koneksi putus).
func (h *Hub) Serve(conn *websocket.Conn, user User) {
	c := &Client{
		hub: h, conn: conn, send: make(chan []byte, 64),
		user: user, channels: map[string]bool{}, parties: map[string]bool{},
	}
	go c.writePump()
	c.readPump()
	h.disconnect(c)
}

func (h *Hub) disconnect(c *Client) {
	h.mu.Lock()
	for ch := range c.channels {
		delete(h.rooms[ch], c)
		h.broadcastLocked(ch, outbound{Type: "presence", Channel: ch, UserID: c.user.ID, Online: false}, c)
	}
	for code := range c.parties {
		delete(h.parties[code], c)
	}
	h.mu.Unlock()
	close(c.send)
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() { _ = c.conn.Close() }()
	c.conn.SetReadLimit(16 << 10)
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var in inbound
		if err := json.Unmarshal(raw, &in); err != nil {
			c.emit(outbound{Type: "error", Error: "pesan tidak valid"})
			continue
		}
		switch in.Type {
		case "join":
			c.join(in.ChannelID)
		case "leave":
			c.leave(in.ChannelID)
		case "message":
			c.message(in)
		case "party_create":
			c.partyCreate(in)
		case "party_join":
			c.partyJoin(in.Code)
		case "party_sync":
			c.partySync(in)
		case "party_leave":
			c.partyLeave(in.Code)
		default:
			c.emit(outbound{Type: "error", Error: "tipe pesan tidak dikenal"})
		}
	}
}

func (h *Hub) broadcastLocked(channelID string, v outbound, except *Client) {
	for c := range h.rooms[channelID] {
		if c != except {
			c.emit(v)
		}
	}
}

func (c *Client) join(channelID string) {
	ctx := context.Background()
	if !c.hub.CanAccess(ctx, c.user.ID, channelID) {
		c.emit(outbound{Type: "error", Channel: channelID, Error: "tidak punya akses ke kanal ini"})
		return
	}
	c.hub.mu.Lock()
	if c.hub.rooms[channelID] == nil {
		c.hub.rooms[channelID] = map[*Client]bool{}
	}
	c.hub.rooms[channelID][c] = true
	c.channels[channelID] = true
	c.hub.broadcastLocked(channelID, outbound{Type: "presence", Channel: channelID, UserID: c.user.ID, Online: true}, c)
	c.hub.mu.Unlock()
	c.emit(outbound{Type: "joined", Channel: channelID})
}

func (c *Client) leave(channelID string) {
	c.hub.mu.Lock()
	delete(c.hub.rooms[channelID], c)
	delete(c.channels, channelID)
	c.hub.broadcastLocked(channelID, outbound{Type: "presence", Channel: channelID, UserID: c.user.ID, Online: false}, c)
	c.hub.mu.Unlock()
}

type messageOut struct {
	ID           string    `json:"id"`
	ChannelID    string    `json:"channel_id"`
	UserID       string    `json:"user_id"`
	DisplayName  string    `json:"display_name"`
	NameColor    *string   `json:"name_color,omitempty"`
	NameEmoji    *string   `json:"name_emoji,omitempty"`
	Body         string    `json:"body"`
	ReplyToID    *string   `json:"reply_to_id,omitempty"`
	ReplyPreview *string   `json:"reply_preview,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

func (c *Client) message(in inbound) {
	if !c.channels[in.ChannelID] {
		c.join(in.ChannelID)
		if !c.channels[in.ChannelID] {
			return
		}
	}
	body := strings.TrimSpace(in.Body)
	if body == "" || len([]rune(body)) > 500 {
		c.emit(outbound{Type: "error", Channel: in.ChannelID, Error: "pesan harus 1–500 karakter"})
		return
	}
	if ContainsLink(body) {
		c.emit(outbound{Type: "error", Channel: in.ChannelID, Error: "tautan luar dilarang di chat — bagikan lewat kartu berbagi aplikasi"})
		return
	}
	if c.user.MutedUntil != nil && c.user.MutedUntil.After(time.Now()) {
		c.emit(outbound{Type: "error", Channel: in.ChannelID, Error: "kamu sedang dibisukan sementara oleh admin"})
		return
	}
	ctx := context.Background()
	var replyPreview *string
	if in.ReplyToID != nil && *in.ReplyToID == "" {
		in.ReplyToID = nil
	}
	if in.ReplyToID != nil {
		var origBody *string
		var deleted *time.Time
		err := c.hub.pool.QueryRow(ctx, `
			select body, deleted_at from chat_messages
			where id = $1::uuid and channel_id = $2::uuid`, *in.ReplyToID, in.ChannelID,
		).Scan(&origBody, &deleted)
		if err != nil {
			in.ReplyToID = nil
		} else if deleted != nil {
			s := "pesan telah dihapus"
			replyPreview = &s
		} else {
			replyPreview = origBody
		}
	}
	msg := messageOut{
		ChannelID: in.ChannelID, UserID: c.user.ID, DisplayName: c.user.Name,
		Body: body, ReplyToID: in.ReplyToID, ReplyPreview: replyPreview,
	}
	if c.user.Premium {
		msg.NameColor = c.user.NameColor
		msg.NameEmoji = c.user.NameEmoji
	}
	if err := c.hub.pool.QueryRow(ctx, `
		insert into chat_messages (channel_id, user_id, kind, body, reply_to_id)
		values ($1::uuid, $2::uuid, 'text', $3, $4::uuid)
		returning id, created_at`,
		in.ChannelID, c.user.ID, body, in.ReplyToID,
	).Scan(&msg.ID, &msg.CreatedAt); err != nil {
		log.Printf("simpan pesan chat: %v", err)
		c.emit(outbound{Type: "error", Channel: in.ChannelID, Error: "pesan gagal disimpan"})
		return
	}
	out := outbound{Type: "message", Channel: in.ChannelID, Message: msg}
	c.hub.mu.Lock()
	for peer := range c.hub.rooms[in.ChannelID] {
		peer.emit(out)
	}
	c.hub.mu.Unlock()
}

// ---------- Watch party (efemer, hanya sinyal) ----------

const partyAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

func (c *Client) partyCreate(in inbound) {
	b := make([]byte, 6)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(partyAlphabet))))
		if err != nil {
			c.emit(outbound{Type: "error", Error: "gagal membuat ruang nonton bareng"})
			return
		}
		b[i] = partyAlphabet[n.Int64()]
	}
	code := string(b)
	c.hub.mu.Lock()
	if c.hub.parties[code] == nil {
		c.hub.parties[code] = map[*Client]bool{}
	}
	c.hub.parties[code][c] = true
	c.parties[code] = true
	c.hub.mu.Unlock()
	c.emit(outbound{Type: "party_created", Code: code})
	// Pengumuman ke kanal yang sedang dibuka pembuat (bila ada) agar
	// teman bisa ikut — isi episode dibawa klien.
	_ = in.EpisodeID
}

func (c *Client) partyJoin(code string) {
	c.hub.mu.Lock()
	members, ok := c.hub.parties[code]
	if ok {
		members[c] = true
		c.parties[code] = true
	}
	c.hub.mu.Unlock()
	if !ok {
		c.emit(outbound{Type: "error", Error: "kode nonton bareng tidak ditemukan"})
		return
	}
	c.emit(outbound{Type: "party_joined", Code: code})
}

func (c *Client) partySync(in inbound) {
	if !c.parties[in.Code] {
		return
	}
	out := outbound{
		Type: "party_sync", Code: in.Code, Action: in.Action,
		Position: in.Position, From: c.user.ID,
	}
	c.hub.mu.Lock()
	for peer := range c.hub.parties[in.Code] {
		if peer != c {
			peer.emit(out)
		}
	}
	c.hub.mu.Unlock()
}

func (c *Client) partyLeave(code string) {
	c.hub.mu.Lock()
	delete(c.hub.parties[code], c)
	delete(c.parties, code)
	c.hub.mu.Unlock()
}
