<script>
  import { API_BASE, likePost, repostPost, bookmarkPost, sharePost, deletePost, getComments, createComment } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import { fmtWhen } from '$lib/history.js';
  import { toastError, toastSuccess } from '$lib/toast.js';

  export let post;
  export let onDelete = null;
  export let onHide = null;

  let menuOpen = false;
  let lightbox = '';
  let zoomed = false;

  // Embed YouTube (hanya YouTube — TikTok/Instagram/Facebook tetap tautan
  // biasa: embed script mereka berat dan tidak andal di web).
  // Pola ID disamakan dengan aplikasi (FeedScreens.kt socialEmbedUrl).
  $: ytId = (() => {
    const m = (post.body || '').match(
      /(?:youtube\.com\/(?:watch\?v=|shorts\/|embed\/)|youtu\.be\/)([A-Za-z0-9_-]{6,})/i
    );
    return m ? m[1] : '';
  })();

  function openLightbox(url) {
    lightbox = url;
    zoomed = false;
  }
  function closeLightbox() {
    lightbox = '';
    zoomed = false;
  }
  function onKey(e) {
    if (e.key === 'Escape' && lightbox) closeLightbox();
  }

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
  // Tanpa tab baru (27 Nov 2026): tautan dibuka di tab yang sama.
  function linkify(text) {
    return esc(text || '').replace(
      /(https?:\/\/[^\s<]+)/g,
      '<a href="$1" rel="noreferrer">$1</a>'
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
      toastError(e.message || 'Gagal menyukai.');
    }
  }

  async function toggleRepost() {
    if (!$user) return;
    try {
      const res = await repostPost(post.post_id);
      reposted = res.reposted ?? !reposted;
      repostCount = res.repost_count ?? repostCount + (reposted ? 1 : -1);
    } catch (e) {
      toastError(e.message || 'Gagal repost.');
    }
  }

  async function toggleBookmark() {
    if (!$user) return;
    try {
      const res = await bookmarkPost(post.post_id);
      bookmarked = res.bookmarked ?? !bookmarked;
    } catch (e) {
      toastError(e.message || 'Gagal bookmark.');
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
      toastSuccess('Tautan disalin.');
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
      toastError(e.message || 'Gagal mengirim komentar.');
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
      toastError(e.message || 'Gagal menghapus.');
    }
  }

  // Sembunyikan lokal saja (aplikasi juga begitu: FeedScreens.kt
  // hiddenIds — tidak ada endpoint hide di server). Induk menyimpan
  // ID-nya ke localStorage dan memfilter feed.
  function hide() {
    menuOpen = false;
    onHide?.(post.id);
  }
</script>

<svelte:window on:keydown={onKey} />

<article class="post card">
  {#if post.reposted_by}
    <p class="repost-line muted"><i class="fa-solid fa-retweet"></i> {post.reposted_by.display_name} me-repost</p>
  {/if}
  <div class="head">
    <span class="avatar">{author.display_name ? author.display_name[0].toUpperCase() : 'S'}</span>
    <div class="who">
      <strong>{author.display_name}{#if author.is_premium} <i class="fa-solid fa-crown"></i>{/if}</strong>
      <span class="muted">{fmtWhen(post.created_at)}{post.category ? ' • ' + post.category : ''}{post.group_name ? ' • ' + post.group_name : ''}</span>
    </div>
    <div class="menu-wrap">
      <button class="icon-btn" on:click={() => (menuOpen = !menuOpen)} aria-label="Menu postingan" aria-expanded={menuOpen}>
        <i class="fa-solid fa-ellipsis-vertical"></i>
      </button>
      {#if menuOpen}
        <div class="menu" role="menu">
          {#if post.mine}
            <button
              role="menuitem"
              on:click={() => {
                menuOpen = false;
                remove();
              }}><i class="fa-solid fa-trash"></i> Hapus postingan</button
            >
          {/if}
          <button role="menuitem" on:click={hide}><i class="fa-solid fa-eye-slash"></i> Sembunyikan</button>
        </div>
      {/if}
    </div>
  </div>

  {#if post.body}
    <p class="body">{@html linkify(post.body)}</p>
  {/if}
  {#if ytId}
    <div class="yt-embed">
      <!-- svelte-ignore a11y_missing_attribute -->
      <iframe
        src={'https://www.youtube-nocookie.com/embed/' + ytId}
        title="Video YouTube"
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
        allowfullscreen
        loading="lazy"
      ></iframe>
    </div>
  {/if}
  {#if mediaUrl}
    <div class="media">
      {#if /\.(mp4|webm)(\?|$)/i.test(mediaUrl)}
        <!-- svelte-ignore a11y_media_has_caption -->
        <video src={mediaUrl} controls preload="metadata" playsinline></video>
      {:else}
        <img src={mediaUrl} alt="" loading="lazy" on:click={() => openLightbox(mediaUrl)} />
      {/if}
    </div>
  {/if}
  {#if post.title_id}
    <p><a class="btn ghost small" href={'/judul/' + post.title_id}>Lihat judul terkait <i class="fa-solid fa-chevron-right"></i></a></p>
  {/if}

  <div class="actions">
    <button class:on={liked} on:click={toggleLike} disabled={!$user} aria-label="Suka">
      <i class={liked ? 'fa-solid fa-heart' : 'fa-regular fa-heart'}></i> {likeCount}
    </button>
    <button on:click={toggleComments} aria-label="Komentar"><i class="fa-regular fa-comment"></i> {post.comment_count}</button>
    <button class:on={reposted} on:click={toggleRepost} disabled={!$user} aria-label="Repost">
      <i class="fa-solid fa-retweet"></i> {repostCount}
    </button>
    <button class:on={bookmarked} on:click={toggleBookmark} disabled={!$user} aria-label="Bookmark">
      <i class={bookmarked ? 'fa-solid fa-bookmark' : 'fa-regular fa-bookmark'}></i>
    </button>
    <button on:click={doShare} aria-label="Bagikan"><i class="fa-solid fa-share-nodes"></i> {shareCount}</button>
  </div>

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

{#if lightbox}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="lightbox" on:click={closeLightbox}>
    <button class="lb-close" aria-label="Tutup gambar" on:click|stopPropagation={closeLightbox}>
      <i class="fa-solid fa-xmark"></i>
    </button>
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
    <img
      src={lightbox}
      alt="Gambar postingan ukuran penuh"
      class:zoomed
      on:click|stopPropagation={() => (zoomed = !zoomed)}
    />
    <p class="lb-hint muted">Ketuk gambar untuk {zoomed ? 'perkecil' : 'perbesar'} • Esc / ketuk latar untuk menutup</p>
  </div>
{/if}

<style>
  .post { margin-bottom: 16px; padding: 16px; }
  .repost-line { font-size: 13px; margin: 0 0 8px; }
  .head { display: flex; align-items: center; gap: 10px; }
  .who { flex: 1; display: flex; flex-direction: column; gap: 1px; font-size: 14px; }
  .who span { font-size: 12px; }
  .who .fa-crown { color: var(--gold); }
  .avatar {
    width: 38px; height: 38px; border-radius: 50%;
    background: var(--red); color: #fff; font-weight: 700;
    display: flex; align-items: center; justify-content: center; font-size: 16px;
  }
  .icon-btn { background: none; border: 0; font-size: 16px; cursor: pointer; }
  .menu-wrap { position: relative; }
  .menu {
    position: absolute; right: 0; top: 100%; z-index: 20; min-width: 180px;
    background: var(--panel); border: 1px solid var(--border); border-radius: 4px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5); padding: 4px; display: flex; flex-direction: column;
  }
  .menu button {
    display: flex; align-items: center; gap: 8px; background: none; border: 0;
    color: #fff; font-size: 14px; text-align: left; padding: 9px 12px;
    border-radius: 4px; cursor: pointer;
  }
  .menu button:hover { background: var(--panel-2); }
  .menu button i { color: var(--muted); width: 16px; text-align: center; }
  .yt-embed { margin: 4px 0 8px; border-radius: 4px; overflow: hidden; aspect-ratio: 16 / 9; background: #000; }
  .yt-embed iframe { width: 100%; height: 100%; border: 0; display: block; }
  .lightbox {
    position: fixed; inset: 0; z-index: 100; background: rgba(0, 0, 0, 0.92);
    display: flex; flex-direction: column; align-items: center; justify-content: center;
    padding: 16px; cursor: zoom-out; overflow: auto;
  }
  .lightbox img { max-width: 100%; max-height: calc(100vh - 70px); object-fit: contain; cursor: zoom-in; }
  .lightbox img.zoomed { max-width: none; max-height: none; width: auto; height: auto; cursor: zoom-out; }
  .lb-close {
    position: absolute; top: 14px; right: 14px; width: 40px; height: 40px; border-radius: 50%;
    background: rgba(255, 255, 255, 0.12); border: 0; color: #fff; font-size: 18px; cursor: pointer;
  }
  .lb-hint { margin: 10px 0 0; font-size: 12px; text-align: center; }
  .media img { cursor: zoom-in; }
  .body { white-space: pre-wrap; word-break: break-word; line-height: 1.55; margin: 12px 0; }
  .media { margin: 4px 0 8px; border-radius: 4px; overflow: hidden; }
  .media img, .media video { width: 100%; display: block; max-height: 520px; object-fit: contain; background: #000; }
  .actions { display: flex; gap: 4px; margin-top: 10px; border-top: 1px solid var(--border); padding-top: 10px; }
  .actions button {
    background: none; border: 0; color: var(--muted); font-size: 14px;
    padding: 6px 10px; border-radius: 999px; cursor: pointer;
  }
  .actions button:hover:not(:disabled) { background: var(--panel-2); color: #fff; }
  .actions button:disabled { opacity: 0.45; cursor: default; }
  .actions button.on { color: var(--red); }
  .actions button i { margin-right: 5px; }
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
