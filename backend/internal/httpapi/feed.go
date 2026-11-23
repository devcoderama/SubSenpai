package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"subsenpai/backend/internal/auth"
)

// ---------- Feed komunitas (redesain Chat 2026-11-22) ----------
//
// Postingan adalah model baru (feed_posts) di samping chat kanal:
// kanal tetap untuk ruang obrolan grup/DM, feed untuk postingan
// sosial bergaya Threads/Facebook dengan like, komentar berjenjang,
// bookmark, repost, dan notifikasi sosial. Berbeda dari ruang chat,
// feed MEMBOLEHKAN tautan di postingan/komentar (keputusan Rams
// 2026-11-22 mengikuti desain referensi); larangan tautan tetap
// berlaku di ruang chat.

var feedCategories = map[string]bool{
	"umum": true, "rekomendasi": true, "anime": true,
	"donghua": true, "film": true,
}

type feedAuthor struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	IsPremium   bool    `json:"is_premium"`
}

type feedPost struct {
	ID string `json:"id"` // id baris feed (baris repost bila ini repost)
	PostID string `json:"post_id"` // id postingan konten (asli)
	Author feedAuthor `json:"author"`
	RepostedBy *feedAuthor `json:"reposted_by,omitempty"`
	Body         *string    `json:"body,omitempty"`
	ImageURL     *string    `json:"image_url,omitempty"`
	Category     string     `json:"category"`
	Visibility   string     `json:"visibility"`
	GroupID      *string    `json:"group_id,omitempty"`
	GroupName    *string    `json:"group_name,omitempty"`
	TitleID      *string    `json:"title_id,omitempty"`
	LikeCount    int        `json:"like_count"`
	CommentCount int        `json:"comment_count"`
	RepostCount  int        `json:"repost_count"`
	ShareCount   int        `json:"share_count"`
	Liked        bool       `json:"liked"`
	Bookmarked   bool       `json:"bookmarked"`
	Reposted     bool       `json:"reposted"`
	Mine         bool       `json:"mine"`
	CreatedAt    time.Time   `json:"created_at"`
	CommentsPreview []feedComment `json:"comments_preview"`
}

// feedPostCols: fp = baris feed, op = postingan konten (asli). Like,
// komentar, bookmark, dan repost selalu menunjuk op agar baris repost
// menampilkan & mengubah konten yang sama dengan aslinya.
const feedPostCols = `
	fp.id, op.id, op.body, op.image_url, op.category, op.visibility, op.group_id, g.name,
	op.share_count, fp.created_at,
	op.user_id, coalesce(pr.display_name, pr.email, 'Pengguna'), pr.avatar_url, pr.is_premium,
	fp.user_id,
	fp.repost_of_id is not null,
	coalesce(rp.display_name, rp.email, ''), rp.avatar_url, coalesce(rp.is_premium, false),
	(select count(*) from feed_post_likes l where l.post_id = op.id),
	(select count(*) from feed_comments c where c.post_id = op.id and c.deleted_at is null),
	(select count(*) from feed_posts x where x.repost_of_id = op.id and x.deleted_at is null),
	exists(select 1 from feed_post_likes l where l.post_id = op.id and l.user_id = $ME),
	exists(select 1 from feed_bookmarks b where b.post_id = op.id and b.user_id = $ME),
	exists(select 1 from feed_posts x where x.repost_of_id = op.id and x.user_id = $ME and x.deleted_at is null),
	op.title_id`

const feedPostJoins = ` from feed_posts fp
	join feed_posts op on op.id = coalesce(fp.repost_of_id, fp.id)
	join profiles pr on pr.id = op.user_id
	left join profiles rp on rp.id = fp.user_id and fp.repost_of_id is not null
	left join groups g on g.id = op.group_id`

func scanFeedPost(rows interface {
	Scan(dest ...any) error
}, me string) (feedPost, time.Time, string, error) {
	var p feedPost
	var authorID, rowAuthorID string
	var isRepost bool
	var rpName string
	var rpAvatar *string
	var rpPremium bool
	err := rows.Scan(
		&p.ID, &p.PostID, &p.Body, &p.ImageURL, &p.Category, &p.Visibility, &p.GroupID, &p.GroupName,
		&p.ShareCount, &p.CreatedAt,
		&authorID, &p.Author.DisplayName, &p.Author.AvatarURL, &p.Author.IsPremium,
		&rowAuthorID, &isRepost, &rpName, &rpAvatar, &rpPremium,
		&p.LikeCount, &p.CommentCount, &p.RepostCount, &p.Liked, &p.Bookmarked, &p.Reposted,
		&p.TitleID,
	)
	p.Author.ID = authorID
	p.Mine = rowAuthorID == me
	if isRepost {
		p.RepostedBy = &feedAuthor{
			ID: rowAuthorID, DisplayName: rpName,
			AvatarURL: rpAvatar, IsPremium: rpPremium,
		}
	}
	if p.CommentsPreview == nil {
		p.CommentsPreview = []feedComment{}
	}
	return p, p.CreatedAt, p.ID, err
}

// feedWhere menyusun saringan tab/kategori/pencarian untuk daftar
// postingan. $1 selalu id pemanggil.
func feedWhere(tab, category, q, groupID string) (string, []any) {
	args := []any{nil} // $1 diisi pemanggil
	where := `fp.deleted_at is null and op.deleted_at is null
		and not exists (
			select 1 from user_blocks b
			where (b.blocker_id = $1::uuid and b.blocked_id in (fp.user_id, op.user_id))
			   or (b.blocker_id in (fp.user_id, op.user_id) and b.blocked_id = $1::uuid))`
	if tab == "grup" {
		where += ` and fp.group_id is not null and exists (
			select 1 from group_members gm
			where gm.group_id = fp.group_id and gm.user_id = $1::uuid)`
		if groupID != "" {
			args = append(args, groupID)
			where += " and fp.group_id = $" + strconv.Itoa(len(args)) + "::uuid"
		}
	} else { // teman (bawaan): baris saya/teman, atau konten publik siapa pun
		where += ` and (fp.user_id = $1::uuid or op.visibility = 'publik' or exists (
			select 1 from friendships f
			where f.status = 'accepted'
			  and ((f.requester_id = $1::uuid and f.addressee_id = fp.user_id)
			    or (f.addressee_id = $1::uuid and f.requester_id = fp.user_id))))`
	}
	// Gerbang visibilitas konten (kedua tab): menilai penulis ASLI —
	// publik untuk semua, teman hanya untuk teman penulis, pribadi
	// hanya penulis. Repost tidak bisa membocorkan konten terbatas.
	where += ` and (op.user_id = $1::uuid or op.visibility = 'publik' or (
		op.visibility = 'teman' and exists (
			select 1 from friendships f
			where f.status = 'accepted'
			  and ((f.requester_id = $1::uuid and f.addressee_id = op.user_id)
			    or (f.addressee_id = $1::uuid and f.requester_id = op.user_id)))))`
	if category != "" && category != "semua" {
		args = append(args, category)
		where += " and op.category = $" + strconv.Itoa(len(args))
	}
	if q != "" {
		args = append(args, "%"+q+"%")
		where += " and op.body ilike $" + strconv.Itoa(len(args))
	}
	return where, args
}

func (a *API) queryFeedPosts(ctx context.Context, me, tab, category, q, groupID string, cur *cursor, limit int) ([]feedPost, *cursor, error) {
	where, args := feedWhere(tab, category, q, groupID)
	args[0] = me
	cols := strings.ReplaceAll(feedPostCols, "$ME", "$1")
	if cur != nil {
		args = append(args, cur.C, cur.I)
		where += " and (fp.created_at, fp.id) < ($" + strconv.Itoa(len(args)-1) + ", $" + strconv.Itoa(len(args)) + "::uuid)"
	}
	args = append(args, limit+1)
	sql := "select " + cols + feedPostJoins + `
		where ` + where + `
		order by fp.created_at desc, fp.id desc limit $` + strconv.Itoa(len(args))
	rows, err := a.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	out := []feedPost{}
	for rows.Next() {
		p, _, _, err := scanFeedPost(rows, me)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	// Satu item ekstra diambil untuk tahu ada halaman berikutnya;
	// kursor menunjuk item terakhir yang benar-benar dikembalikan.
	var next *cursor
	if len(out) > limit {
		out = out[:limit]
		tail := out[len(out)-1]
		next = &cursor{C: tail.CreatedAt, I: tail.ID}
	}
	return out, next, nil
}

func (a *API) handleFeedList(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	q := r.URL.Query()
	tab := q.Get("tab")
	if tab != "grup" {
		tab = "teman"
	}
	category := strings.ToLower(q.Get("category"))
	if category != "" && category != "semua" && !feedCategories[category] {
		writeErr(w, http.StatusBadRequest, "kategori tidak dikenal")
		return
	}
	var cur *cursor
	if s := q.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "kursor tidak valid")
			return
		}
		cur = &c
	}
	posts, next, err := a.queryFeedPosts(r.Context(), p.ID, tab, category, q.Get("q"), q.Get("group_id"), cur, 20)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	a.attachCommentPreviews(r.Context(), p.ID, posts)
	nextStr := ""
	if next != nil {
		nextStr = encodeCursor(*next)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": posts, "next_cursor": nextStr})
}

func (a *API) feedPostByID(ctx context.Context, me, id string) (feedPost, error) {
	cols := strings.ReplaceAll(feedPostCols, "$ME", "$1")
	row := a.pool.QueryRow(ctx, "select "+cols+feedPostJoins+`
		where fp.id = $2::uuid and fp.deleted_at is null`, me, id)
	p, _, _, err := scanFeedPost(row, me)
	return p, err
}

// attachCommentPreviews mengisi sampai 3 komentar pertama tiap
// postingan agar bisa dirender sebaris di kartu feed (gaya
// referensi: komentar tampil di dalam kartu, bukan halaman lain).
func (a *API) attachCommentPreviews(ctx context.Context, me string, posts []feedPost) {
	if len(posts) == 0 {
		return
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, p := range posts {
		if !seen[p.PostID] {
			seen[p.PostID] = true
			ids = append(ids, p.PostID)
		}
	}
	args := []any{me}
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		args = append(args, id)
		placeholders[i] = "$" + strconv.Itoa(len(args)) + "::uuid"
	}
	rows, err := a.pool.Query(ctx, `
		select c.id, c.post_id, c.parent_id, c.body, c.created_at,
		       c.user_id, coalesce(pr.display_name, pr.email, 'Pengguna'), pr.avatar_url, pr.is_premium,
		       (select count(*) from feed_comment_likes l where l.comment_id = c.id),
		       exists(select 1 from feed_comment_likes l where l.comment_id = c.id and l.user_id = $1::uuid)
		from (
			select *, row_number() over (partition by post_id order by created_at asc) as rn
			from feed_comments
			where post_id in (`+strings.Join(placeholders, ",")+`) and deleted_at is null
		) c
		join profiles pr on pr.id = c.user_id
		where c.rn <= 3
		order by c.post_id, c.created_at asc`, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	byPost := map[string][]feedComment{}
	for rows.Next() {
		var c feedComment
		var authorID string
		if err := rows.Scan(&c.ID, &c.PostID, &c.ParentID, &c.Body, &c.CreatedAt,
			&authorID, &c.Author.DisplayName, &c.Author.AvatarURL, &c.Author.IsPremium,
			&c.LikeCount, &c.Liked); err != nil {
			return
		}
		c.Author.ID = authorID
		c.Mine = authorID == me
		byPost[c.PostID] = append(byPost[c.PostID], c)
	}
	for i := range posts {
		if prev, ok := byPost[posts[i].PostID]; ok {
			posts[i].CommentsPreview = prev
		}
	}
}

// handleFeedRepost: posting ulang (🔁). Toggle — repost kedua
// membatalkan. Repost selalu menunjuk postingan asli, tidak bisa
// memposting ulang postingan sendiri.
func (a *API) handleFeedRepost(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var origID, authorID, category string
	var groupID *string
	if err := a.pool.QueryRow(ctx, `
		select op.id, op.user_id, op.category, op.group_id
		from feed_posts fp
		join feed_posts op on op.id = coalesce(fp.repost_of_id, fp.id)
		where fp.id = $1::uuid and fp.deleted_at is null and op.deleted_at is null`,
		id).Scan(&origID, &authorID, &category, &groupID); err != nil {
		writeErr(w, http.StatusNotFound, "postingan tidak ditemukan")
		return
	}
	tag, _ := a.pool.Exec(ctx, `
		delete from feed_posts
		where repost_of_id = $1::uuid and user_id = $2::uuid and deleted_at is null`,
		origID, p.ID)
	reposted := false
	if tag.RowsAffected() == 0 {
		if authorID == p.ID {
			writeErr(w, http.StatusBadRequest, "tidak bisa memposting ulang postingan sendiri")
			return
		}
		if _, err := a.pool.Exec(ctx, `
			insert into feed_posts (user_id, category, group_id, repost_of_id)
			values ($1::uuid, $2, $3::uuid, $4::uuid)`,
			p.ID, category, groupID, origID); err == nil {
			reposted = true
			a.notifySocial(ctx, authorID, p.ID, "memposting ulang postinganmu", &origID)
		}
	}
	var count int
	_ = a.pool.QueryRow(ctx, `
		select count(*) from feed_posts
		where repost_of_id = $1::uuid and deleted_at is null`, origID).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]any{"reposted": reposted, "repost_count": count})
}

func (a *API) handleFeedGetPost(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	post, err := a.feedPostByID(r.Context(), p.ID, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "postingan tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": post})
}

func (a *API) handleFeedCreate(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	var req struct {
		Body       string  `json:"body"`
		ImageURL   *string `json:"image_url"`
		Category   string  `json:"category"`
		Visibility string  `json:"visibility"`
		GroupID    *string `json:"group_id"`
		TitleID    *string `json:"title_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	body := strings.TrimSpace(req.Body)
	if len(body) > 1000 {
		writeErr(w, http.StatusBadRequest, "postingan maksimal 1000 karakter")
		return
	}
	if body == "" && (req.ImageURL == nil || *req.ImageURL == "") {
		writeErr(w, http.StatusBadRequest, "postingan kosong — tulis sesuatu atau lampirkan gambar")
		return
	}

	category := strings.ToLower(req.Category)
	if category == "" {
		category = "umum"
	}
	if !feedCategories[category] {
		writeErr(w, http.StatusBadRequest, "kategori tidak dikenal")
		return
	}
	visibility := strings.ToLower(req.Visibility)
	if visibility == "" {
		visibility = "publik"
	}
	if visibility != "publik" && visibility != "teman" && visibility != "pribadi" {
		writeErr(w, http.StatusBadRequest, "visibilitas tidak dikenal")
		return
	}
	if req.ImageURL != nil && *req.ImageURL != "" &&
		!strings.Contains(*req.ImageURL, "/uploads/feed/") &&
		!strings.HasPrefix(*req.ImageURL, "https://i.ibb.co/") &&
		!strings.HasPrefix(*req.ImageURL, "https://www.image2url.com/r2/") {
		writeErr(w, http.StatusBadRequest, "media harus diunggah lewat aplikasi")
		return
	}
	ctx := r.Context()
	if req.TitleID != nil {
		var exists bool
		err := a.pool.QueryRow(ctx, `
			select exists(select 1 from titles where id = $1::uuid)`, *req.TitleID).Scan(&exists)
		if err != nil || !exists {
			writeErr(w, http.StatusBadRequest, "judul tidak ditemukan")
			return
		}
	}
	if req.GroupID != nil {
		var member bool
		err := a.pool.QueryRow(ctx, `
			select exists(select 1 from group_members where group_id = $1::uuid and user_id = $2::uuid)`,
			*req.GroupID, p.ID).Scan(&member)
		if err != nil || !member {
			writeErr(w, http.StatusForbidden, "kamu bukan anggota grup ini")
			return
		}
	}
	var bodyPtr *string
	if body != "" {
		bodyPtr = &body
	}
	var id string
	err := a.pool.QueryRow(ctx, `
		insert into feed_posts (user_id, body, image_url, category, visibility, group_id, title_id)
		values ($1::uuid, $2, $3, $4, $5, $6::uuid, $7::uuid) returning id`,
		p.ID, bodyPtr, req.ImageURL, category, visibility, req.GroupID, req.TitleID).Scan(&id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan postingan")
		return
	}
	post, err := a.feedPostByID(ctx, p.ID, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"post": post})
}

func (a *API) handleFeedDelete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	tag, err := a.pool.Exec(r.Context(), `
		update feed_posts set deleted_at = now()
		where id = $1::uuid and user_id = $2::uuid and deleted_at is null`, id, p.ID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "postingan tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// notifySocial mencatat notifikasi sosial untuk pemilik konten.
func (a *API) notifySocial(ctx context.Context, ownerID, actorID, verb string, postID *string) {
	if ownerID == actorID {
		return
	}
	var actorName string
	if err := a.pool.QueryRow(ctx, `
		select coalesce(display_name, email, 'Pengguna') from profiles where id = $1::uuid`,
		actorID).Scan(&actorName); err != nil {
		return
	}
	_, _ = a.pool.Exec(ctx, `
		insert into notifications (user_id, kind, title, body, ref_post_id, actor_id)
		values ($1::uuid, 'social', $2, $3, $4::uuid, $5::uuid)`,
		ownerID, actorName, verb, postID, actorID)
}

func (a *API) handleFeedLike(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var authorID string
	if err := a.pool.QueryRow(ctx, `
		select user_id from feed_posts where id = $1::uuid and deleted_at is null`, id).Scan(&authorID); err != nil {
		writeErr(w, http.StatusNotFound, "postingan tidak ditemukan")
		return
	}
	tag, _ := a.pool.Exec(ctx, `delete from feed_post_likes where post_id = $1::uuid and user_id = $2::uuid`, id, p.ID)
	liked := false
	if tag.RowsAffected() == 0 {
		if _, err := a.pool.Exec(ctx, `
			insert into feed_post_likes (post_id, user_id) values ($1::uuid, $2::uuid)
			on conflict do nothing`, id, p.ID); err == nil {
			liked = true
			a.notifySocial(ctx, authorID, p.ID, "menyukai postinganmu", &id)
		}
	}
	var count int
	_ = a.pool.QueryRow(ctx, `select count(*) from feed_post_likes where post_id = $1::uuid`, id).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]any{"liked": liked, "like_count": count})
}

func (a *API) handleFeedBookmark(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	tag, _ := a.pool.Exec(ctx, `delete from feed_bookmarks where post_id = $1::uuid and user_id = $2::uuid`, id, p.ID)
	bookmarked := false
	if tag.RowsAffected() == 0 {
		if _, err := a.pool.Exec(ctx, `
			insert into feed_bookmarks (post_id, user_id) values ($1::uuid, $2::uuid)
			on conflict do nothing`, id, p.ID); err == nil {
			bookmarked = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"bookmarked": bookmarked})
}

func (a *API) handleFeedShare(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var count int
	err := a.pool.QueryRow(r.Context(), `
		update feed_posts set share_count = share_count + 1
		where id = $1::uuid and deleted_at is null returning share_count`, id).Scan(&count)
	if err != nil {
		writeErr(w, http.StatusNotFound, "postingan tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"share_count": count})
}

// ---------- Komentar ----------

type feedComment struct {
	ID        string     `json:"id"`
	PostID    string     `json:"post_id"`
	ParentID  *string    `json:"parent_id,omitempty"`
	Author    feedAuthor `json:"author"`
	Body      string     `json:"body"`
	LikeCount int        `json:"like_count"`
	Liked     bool       `json:"liked"`
	Mine      bool       `json:"mine"`
	CreatedAt time.Time  `json:"created_at"`
}

func (a *API) handleFeedComments(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	postID := chi.URLParam(r, "id")
	ctx := r.Context()
	var exists bool
	if err := a.pool.QueryRow(ctx, `
		select exists(select 1 from feed_posts where id = $1::uuid and deleted_at is null)`, postID).Scan(&exists); err != nil || !exists {
		writeErr(w, http.StatusNotFound, "postingan tidak ditemukan")
		return
	}
	rows, err := a.pool.Query(ctx, `
		select c.id, c.post_id, c.parent_id, c.body, c.created_at,
		       c.user_id, coalesce(pr.display_name, pr.email, 'Pengguna'), pr.avatar_url, pr.is_premium,
		       (select count(*) from feed_comment_likes l where l.comment_id = c.id),
		       exists(select 1 from feed_comment_likes l where l.comment_id = c.id and l.user_id = $2::uuid)
		from feed_comments c
		join profiles pr on pr.id = c.user_id
		where c.post_id = $1::uuid and c.deleted_at is null
		order by c.created_at asc`, postID, p.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	defer rows.Close()
	out := []feedComment{}
	for rows.Next() {
		var c feedComment
		var authorID string
		if err := rows.Scan(&c.ID, &c.PostID, &c.ParentID, &c.Body, &c.CreatedAt,
			&authorID, &c.Author.DisplayName, &c.Author.AvatarURL, &c.Author.IsPremium,
			&c.LikeCount, &c.Liked); err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		c.Author.ID = authorID
		c.Mine = authorID == p.ID
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (a *API) handleFeedCommentCreate(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	postID := chi.URLParam(r, "id")
	var req struct {
		Body     string  `json:"body"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "format permintaan tidak valid")
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" {
		writeErr(w, http.StatusBadRequest, "komentar kosong")
		return
	}
	if len(body) > 500 {
		writeErr(w, http.StatusBadRequest, "komentar maksimal 500 karakter")
		return
	}
	ctx := r.Context()
	var postAuthor string
	if err := a.pool.QueryRow(ctx, `
		select user_id from feed_posts where id = $1::uuid and deleted_at is null`, postID).Scan(&postAuthor); err != nil {
		writeErr(w, http.StatusNotFound, "postingan tidak ditemukan")
		return
	}
	if req.ParentID != nil {
		var ok bool
		if err := a.pool.QueryRow(ctx, `
			select exists(select 1 from feed_comments
				where id = $1::uuid and post_id = $2::uuid and deleted_at is null)`,
			*req.ParentID, postID).Scan(&ok); err != nil || !ok {
			writeErr(w, http.StatusBadRequest, "komentar induk tidak ditemukan")
			return
		}
	}
	var id string
	if err := a.pool.QueryRow(ctx, `
		insert into feed_comments (post_id, user_id, parent_id, body)
		values ($1::uuid, $2::uuid, $3::uuid, $4) returning id`,
		postID, p.ID, req.ParentID, body).Scan(&id); err != nil {
		writeErr(w, http.StatusInternalServerError, "gagal menyimpan komentar")
		return
	}
	if req.ParentID != nil {
		var parentAuthor string
		if err := a.pool.QueryRow(ctx, `select user_id from feed_comments where id = $1::uuid`, *req.ParentID).Scan(&parentAuthor); err == nil {
			a.notifySocial(ctx, parentAuthor, p.ID, "membalas komentarmu", &postID)
		}
	} else {
		a.notifySocial(ctx, postAuthor, p.ID, "mengomentari postinganmu", &postID)
	}
	var c feedComment
	var authorID string
	err := a.pool.QueryRow(ctx, `
		select c.id, c.post_id, c.parent_id, c.body, c.created_at,
		       c.user_id, coalesce(pr.display_name, pr.email, 'Pengguna'), pr.avatar_url, pr.is_premium
		from feed_comments c join profiles pr on pr.id = c.user_id
		where c.id = $1::uuid`, id).Scan(&c.ID, &c.PostID, &c.ParentID, &c.Body, &c.CreatedAt,
		&authorID, &c.Author.DisplayName, &c.Author.AvatarURL, &c.Author.IsPremium)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	c.Author.ID = authorID
	c.Mine = true
	writeJSON(w, http.StatusCreated, map[string]any{"comment": c})
}

func (a *API) handleFeedCommentLike(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	var authorID, postID string
	if err := a.pool.QueryRow(ctx, `
		select user_id, post_id from feed_comments where id = $1::uuid and deleted_at is null`, id).Scan(&authorID, &postID); err != nil {
		writeErr(w, http.StatusNotFound, "komentar tidak ditemukan")
		return
	}
	tag, _ := a.pool.Exec(ctx, `delete from feed_comment_likes where comment_id = $1::uuid and user_id = $2::uuid`, id, p.ID)
	liked := false
	if tag.RowsAffected() == 0 {
		if _, err := a.pool.Exec(ctx, `
			insert into feed_comment_likes (comment_id, user_id) values ($1::uuid, $2::uuid)
			on conflict do nothing`, id, p.ID); err == nil {
			liked = true
			a.notifySocial(ctx, authorID, p.ID, "menyukai komentarmu", &postID)
		}
	}
	var count int
	_ = a.pool.QueryRow(ctx, `select count(*) from feed_comment_likes where comment_id = $1::uuid`, id).Scan(&count)
	writeJSON(w, http.StatusOK, map[string]any{"liked": liked, "like_count": count})
}

func (a *API) handleFeedCommentDelete(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	tag, err := a.pool.Exec(r.Context(), `
		update feed_comments set deleted_at = now()
		where id = $1::uuid and user_id = $2::uuid and deleted_at is null`, id, p.ID)
	if err != nil || tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "komentar tidak ditemukan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ---------- Unggah & sajikan gambar ----------

var feedFileRe = regexp.MustCompile(`^[a-f0-9]{32}\.(jpg|png|webp|mp4|webm)$`)

// handleFeedUpload menerima gambar atau video postingan.
// Gambar: bila IMGBB_API_KEY terisi, diunggah ke ImgBB dan URL
// publiknya yang disimpan (permintaan Rams 2026-11-22); bila ImgBB
// gagal/tidak terkonfigurasi, disimpan lokal di FeedUploadDir.
// Video (MP4/WebM, maks 50 MB): selalu lokal — ImgBB khusus gambar.
func (a *API) handleFeedUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 52<<20)
	file, _, err := r.FormFile("image")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "berkas media wajib dilampirkan")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 50<<20+1))
	if err != nil || len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "berkas media tidak terbaca")
		return
	}
	if len(data) > 50<<20 {
		writeErr(w, http.StatusBadRequest, "media maksimal 50 MB")
		return
	}
	ctype := http.DetectContentType(data)
	var ext string
	isImage := true
	switch ctype {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "video/mp4":
		ext = ".mp4"
		isImage = false
	case "video/webm":
		ext = ".webm"
		isImage = false
	default:
		writeErr(w, http.StatusBadRequest, "format media harus JPG, PNG, WebP, MP4, atau WebM")
		return
	}
	if isImage {
		if len(data) > 5<<20 {
			writeErr(w, http.StatusBadRequest, "gambar maksimal 5 MB")
			return
		}
		if a.cfg.ImgBBAPIKey != "" {
			if url, err := a.uploadImgBB(r.Context(), data); err == nil {
				writeJSON(w, http.StatusCreated, map[string]any{"url": url})
				return
			}
			// ImgBB gagal → jatuh ke penyimpanan lokal di bawah.
		}
	} else {
		// Video: utamakan image2url (URL CDN publik, teruji dari
		// server 2026-11-22). Kuota tamu per-IP terbatas — kegagalan
		// apa pun jatuh ke penyimpanan lokal agar posting tak gagal.
		if url, err := a.uploadImage2URL(r.Context(), data, ext); err == nil {
			writeJSON(w, http.StatusCreated, map[string]any{"url": url})
			return
		}
	}
	url, err := a.saveFeedFile(r, data, ext)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "penyimpanan media tidak tersedia")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"url": url})
}

// uploadImgBB mengunggah gambar ke ImgBB (API 1/upload, base64) dan
// mengembalikan URL publik gambarnya (i.ibb.co).
func (a *API) uploadImgBB(ctx context.Context, data []byte) (string, error) {
	form := url.Values{
		"key":   {a.cfg.ImgBBAPIKey},
		"image": {base64.StdEncoding.EncodeToString(data)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.imgbb.com/1/upload", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.Success || out.Data.URL == "" {
		return "", errImgBBUpload
	}
	return out.Data.URL, nil
}

// uploadImage2URL meneruskan video ke image2url.com
// (POST /api/upload-video, field "file") dan mengembalikan URL
// publiknya di R2 mereka. Teruji dari server agen 2026-11-22:
// tanpa akun/cookie, balasan {success:true, url:...}.
func (a *API) uploadImage2URL(ctx context.Context, data []byte, ext string) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// Content-Type bagian wajib video/* — bawaan Go
	// (application/octet-stream) ditolak layanan ("Only video
	// files are allowed"), terbukti di uji hidup 2026-11-22.
	partCT := "video/mp4"
	if ext == ".webm" {
		partCT = "video/webm"
	}
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="video`+ext+`"`)
	hdr.Set("Content-Type", partCT)
	part, err := mw.CreatePart(hdr)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.image2url.com/api/upload-video", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Success bool   `json:"success"`
		URL     string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || !out.Success || out.URL == "" {
		return "", errImgBBUpload
	}
	return out.URL, nil
}

var errImgBBUpload = &feedError{"layanan unggah menolak berkas"}

type feedError struct{ s string }

func (e *feedError) Error() string { return e.s }

// saveFeedFile menyimpan media ke folder unggahan lokal dan
// mengembalikan URL publiknya.
func (a *API) saveFeedFile(r *http.Request, data []byte, ext string) (string, error) {
	if err := os.MkdirAll(a.cfg.FeedUploadDir, 0o755); err != nil {
		return "", err
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	name := hex.EncodeToString(raw) + ext
	if err := os.WriteFile(filepath.Join(a.cfg.FeedUploadDir, name), data, 0o644); err != nil {
		return "", err
	}
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "https"
	}
	return proto + "://" + r.Host + "/uploads/feed/" + name, nil
}

// handleFeedFile menyajikan gambar feed secara publik (nama berkas
// acak 128-bit, tidak bisa ditebak) di luar envelope xdata agar bisa
// dimuat Coil/peramban tanpa header sesi.
func (a *API) handleFeedFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if !feedFileRe.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(filepath.Join(a.cfg.FeedUploadDir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}

// ---------- Notifikasi sosial ----------

// notifKindClause: pengumuman admin (broadcast) dan aktivitas
// sosial adalah DUA hal terpisah (permintaan Rams) — klien dapat
// meminta satu jenis saja lewat ?kind=social|broadcast. Tanpa
// parameter, keduanya dikembalikan (kompatibilitas aplikasi lama).
func notifKindClause(kind string) string {
	switch kind {
	case "social":
		return "kind = 'social'"
	case "broadcast":
		return "kind = 'broadcast'"
	case "release":
		return "kind = 'release'"
	default:
		return "kind in ('social','broadcast','release')"
	}
}

func (a *API) handleFeedNotifications(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	ctx := r.Context()
	kc := notifKindClause(r.URL.Query().Get("kind"))
	var unread int
	_ = a.pool.QueryRow(ctx, `
		select count(*) from notifications
		where user_id = $1::uuid and `+kc+` and read_at is null`, p.ID).Scan(&unread)
	rows, err := a.pool.Query(ctx, `
		select n.id, n.kind, n.title, n.body, n.ref_post_id, n.actor_id, n.read_at, n.created_at,
		       coalesce(pr.avatar_url, ''), n.ref_title_id
		from notifications n
		left join profiles pr on pr.id = n.actor_id
		where n.user_id = $1::uuid and n.`+kc+`
		order by n.created_at desc limit 50`, p.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	defer rows.Close()
	type notif struct {
		ID         string     `json:"id"`
		Kind       string     `json:"kind"`
		Title      string     `json:"title"`
		Body       string     `json:"body"`
		RefPostID  *string    `json:"ref_post_id,omitempty"`
		RefTitleID *string    `json:"ref_title_id,omitempty"`
		ActorID    *string    `json:"actor_id,omitempty"`
		AvatarURL  string     `json:"actor_avatar_url"`
		ReadAt     *time.Time `json:"read_at,omitempty"`
		CreatedAt  time.Time  `json:"created_at"`
	}
	out := []notif{}
	for rows.Next() {
		var n notif
		if err := rows.Scan(&n.ID, &n.Kind, &n.Title, &n.Body, &n.RefPostID, &n.ActorID, &n.ReadAt, &n.CreatedAt, &n.AvatarURL, &n.RefTitleID); err != nil {
			writeErr(w, http.StatusInternalServerError, "kesalahan server")
			return
		}
		out = append(out, n)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "unread": unread})
}

func (a *API) handleFeedNotificationsRead(w http.ResponseWriter, r *http.Request) {
	p := auth.FromContext(r.Context())
	// Body opsional {"kind":"broadcast"|"social"}: tandai dibaca
	// hanya jenis itu; tanpa body = semua (perilaku lama).
	var req struct {
		Kind string `json:"kind"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	kc := notifKindClause(req.Kind)
	_, err := a.pool.Exec(r.Context(), `
		update notifications set read_at = now()
		where user_id = $1::uuid and `+kc+` and read_at is null`, p.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kesalahan server")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
