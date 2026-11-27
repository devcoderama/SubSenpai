<script>
  import { API_BASE, likePost, repostPost, bookmarkPost, sharePost, deletePost, getComments, createComment } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import { fmtWhen } from '$lib/history.js';

  export let post;
  export let onDelete = null;

  let liked = post.liked;
  let likeCount = post.like_count;
  let reposted = post.reposted;
  let repostCount = post.repost_count;
  let bookmarked = post.bookmarked;
  let shareCount = post.share_count;
  let showComments = false;
  let comments = post.comments_preview || [];
  let commentsLoaded = false;
  let commentBody = '';
  let commentBusy = false;
  let actionError = '';

  $: author = post.author;
  $: mediaUrl = post.image_url
    ? post.image_url.startsWith('http')
      ? post.image_url
      : API_BASE + post.image_url
    : '';

  function esc(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  // Tautan di feed memang dibolehkan (keputusan Rams) — jadikan klik.
  function linkify(text) {
    return esc(text || '').replace(
      /(https?:\/\/[^\s<]+)/g,
      '<a href="$1" target="_blank" rel="noreferrer">$1</a>'
    );
  }

  async function toggleLike() {
    if (!$user) return;
    const was = liked;
    liked = !was;
    likeCount += liked ? 1 : -1;
    try {
      const res = await likePost(post.post_id);
      liked = res.liked;
      likeCount = res.like_count;
    } catch (e) {
      liked = was;
      likeCount += was ? 1 : -1;
      actionError = e.message || 'Gagal menyukai.';
    }
  }

  async function toggleRepost() {
    if (!$user) return;
    try {
      const res = await repostPost(post.post_id);
      reposted = res.reposted ?? !reposted;
      repostCount = res.repost_count ?? repostCount + (reposted ? 1 : -1);
    } catch (e) {
      actionError = e.message || 'Gagal repost.';
    }
  }

  async function toggleBookmark() {
    if (!$user) return;
    try {
      const res = await bookmarkPost(post.post_id);
      bookmarked = res.bookmarked ?? !bookmarked;
    } catch (e) {
      actionError = e.message || 'Gagal bookmark.';
    }
  }

  async function doShare() {
    try {
      const res = await sharePost(post.post_id);
      shareCount = res.share_count ?? shareCount;
    } catch {
      /* bagikan tetap jalan walau hitungannya gagal */
    }
    try {
      await navigator.clipboard.writeText(window.location.origin + '/post');
      actionError = 'Tautan disalin.';
      setTimeout(() => (actionError = ''), 2000);
    } catch {
      /* clipboard tak tersedia */
    }
  }

  async function toggleComments() {
    showComments = !showComments;
    if (showComments && !commentsLoaded) {
      try {
        const res = await getComments(post.post_id);
        comments = res.items || res.comments || [];
        commentsLoaded = true;
      } catch {
        comments = post.comments_preview || [];
      }
    }
  }

  async function sendComment() {
    const body = commentBody.trim();
    if (!body || commentBusy) return;
    commentBusy = true;
    try {
      const res = await createComment(post.post_id, body);
      if (res.comment) comments = [...comments, res.comment];
      commentBody = '';
    } catch (e) {
      actionError = e.message || 'Gagal mengirim komentar.';
    } finally {
      commentBusy = false;
    }
  }

  async function remove() {
    if (!confirm('Hapus postingan ini?')) return;
    try {
      await deletePost(post.post_id);
      onDelete?.(post.id);
    } catch (e) {
      actionError = e.message || 'Gagal menghapus.';
    }
  }
</script>

<article class="post card">
  {#if post.reposted_by}
    <p class="repost-line muted">🔁 {post.reposted_by.display_name} me-repost</p>
  {/if}
  <div class="head">
    <span class="avatar">{author.display_name ? author.display_name[0].toUpperCase() : 'S'}</span>
    <div class="who">
      <strong>{author.display_name}{author.is_premium ? ' 👑' : ''}</strong>
      <span class="muted">{fmtWhen(post.created_at)}{post.category ? ' • ' + post.category : ''}{post.group_name ? ' • ' + post.group_name : ''}</span>
    </div>
    {#if post.mine}
      <button class="icon-btn" on:click={remove} aria-label="Hapus postingan">🗑️</button>
    {/if}
  </div>

  {#if post.body}
    <p class="body">{@html linkify(post.body)}</p>
  {/if}
  {#if mediaUrl}
    <div class="media">
      {#if /\.(mp4|webm)(\?|$)/i.test(mediaUrl)}
        <!-- svelte-ignore a11y_media_has_caption -->
        <video src={mediaUrl} controls preload="metadata" playsinline></video>
      {:else}
        <img src={mediaUrl} alt="" loading="lazy" />
      {/if}
    </div>
  {/if}
  {#if post.title_id}
    <p><a class="btn ghost small" href={'/judul/' + post.title_id}>Lihat judul terkait ›</a></p>
  {/if}

  <div class="actions">
    <button class:on={liked} on:click={toggleLike} disabled={!$user} aria-label="Suka">
      {liked ? '❤️' : '🤍'} {likeCount}
    </button>
    <button on:click={toggleComments} aria-label="Komentar">💬 {post.comment_count}</button>
    <button class:on={reposted} on:click={toggleRepost} disabled={!$user} aria-label="Repost">
      🔁 {repostCount}
    </button>
    <button class:on={bookmarked} on:click={toggleBookmark} disabled={!$user} aria-label="Bookmark">
      {bookmarked ? '🔖' : '📑'}
    </button>
    <button on:click={doShare} aria-label="Bagikan">↗️ {shareCount}</button>
  </div>
  {#if actionError}<p class="error">{actionError}</p>{/if}

  {#if showComments}
    <div class="comments">
      {#each comments as c (c.id)}
        <div class="comment">
          <strong>{c.author?.display_name || 'Pengguna'}</strong>
          <span>{c.body}</span>
        </div>
      {/each}
      {#if !comments.length}<p class="muted">Belum ada komentar.</p>{/if}
      {#if $user}
        <div class="comment-form">
          <input type="text" placeholder="Tulis komentar…" bind:value={commentBody} maxlength="1000" />
          <button class="btn" on:click={sendComment} disabled={commentBusy || !commentBody.trim()}>Kirim</button>
        </div>
      {/if}
    </div>
  {/if}
</article>

<style>
  .post { margin-bottom: 16px; padding: 16px; }
  .repost-line { font-size: 13px; margin: 0 0 8px; }
  .head { display: flex; align-items: center; gap: 10px; }
  .who { flex: 1; display: flex; flex-direction: column; gap: 1px; font-size: 14px; }
  .who span { font-size: 12px; }
  .avatar {
    width: 38px; height: 38px; border-radius: 50%;
    background: var(--red); color: #fff; font-weight: 700;
    display: flex; align-items: center; justify-content: center; font-size: 16px;
  }
  .icon-btn { background: none; border: 0; font-size: 16px; cursor: pointer; }
  .body { white-space: pre-wrap; word-break: break-word; line-height: 1.55; margin: 12px 0; }
  .media { margin: 4px 0 8px; border-radius: 10px; overflow: hidden; }
  .media img, .media video { width: 100%; display: block; max-height: 520px; object-fit: contain; background: #000; }
  .actions { display: flex; gap: 4px; margin-top: 10px; border-top: 1px solid var(--border); padding-top: 10px; }
  .actions button {
    background: none; border: 0; color: var(--muted); font-size: 14px;
    padding: 6px 10px; border-radius: 999px; cursor: pointer;
  }
  .actions button:hover:not(:disabled) { background: var(--panel-2); color: #fff; }
  .actions button:disabled { opacity: 0.45; cursor: default; }
  .actions button.on { color: var(--red); }
  .comments { margin-top: 12px; border-top: 1px solid var(--border); padding-top: 12px; }
  .comment { display: flex; flex-direction: column; gap: 2px; font-size: 14px; margin-bottom: 10px; }
  .comment-form { display: flex; gap: 8px; margin-top: 8px; }
  .comment-form input {
    flex: 1; background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 999px; color: #fff; padding: 9px 14px; font-size: 14px;
  }
  .small { font-size: 13px; padding: 6px 12px; }
  .error { color: #ff8a8a; font-size: 13px; }
</style>
