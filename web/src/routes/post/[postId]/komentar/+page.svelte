<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { getPost, getComments, createComment, likeComment, deleteComment } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import { fmtWhen } from '$lib/history.js';
  import { toastError, toastSuccess } from '$lib/toast.js';
  import PostCard from '$lib/components/PostCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';

  // Halaman Komentar khusus (28 Nov 2026, rekomendasi no. 7) — cermin
  // CommentsScreen aplikasi (FeedScreens.kt): postingan di atas, daftar
  // komentar beruntun di bawah, bilah tulis menempel di bawah layar.
  const postId = $page.params.postId;

  let post = null;
  let comments = [];
  let collapsed = new Set();
  let loading = true;
  let error = '';
  let input = '';
  let replyTo = null;
  let sending = false;

  // Cerminkan buildCommentRows aplikasi: anak dikelompokkan per parent_id,
  // kedalaman dibatasi 3, tiap baris membawa jumlah keturunan.
  function buildRows(list, collapsedSet) {
    const children = new Map();
    for (const c of list) {
      const k = c.parent_id || null;
      if (!children.has(k)) children.set(k, []);
      children.get(k).push(c);
    }
    const countDesc = (id) =>
      (children.get(id) || []).reduce((n, k) => n + 1 + countDesc(k.id), 0);
    const out = [];
    (function walk(items, depth) {
      for (const c of items) {
        const kids = children.get(c.id) || [];
        out.push({ comment: c, depth, descendantCount: countDesc(c.id), hasChildren: kids.length > 0 });
        if (!collapsedSet.has(c.id)) walk(kids, Math.min(depth + 1, 3));
      }
    })(children.get(null) || [], 0);
    return out;
  }

  $: rows = buildRows(comments, collapsed);
  $: myInitial = ($user?.display_name || $user?.email || 'S')[0].toUpperCase();

  async function load() {
    loading = true;
    error = '';
    try {
      const [pr, cr] = await Promise.all([getPost(postId), getComments(postId)]);
      post = pr.post || pr;
      comments = cr.items || cr.comments || [];
    } catch (e) {
      error = e.message || 'Gagal memuat komentar.';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    if ($user) load();
    else loading = false;
  });

  function toggleCollapse(id) {
    const next = new Set(collapsed);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    collapsed = next;
  }

  async function send() {
    const body = input.trim();
    if (!body || sending) return;
    sending = true;
    try {
      const res = await createComment(postId, body, replyTo?.id || null);
      if (res.comment) comments = [...comments, res.comment];
      if (post) post = { ...post, comment_count: (post.comment_count || 0) + 1 };
      input = '';
      replyTo = null;
    } catch (e) {
      toastError(e.message || 'Gagal mengirim komentar.');
    } finally {
      sending = false;
    }
  }

  async function toggleLike(c) {
    const before = c;
    comments = comments.map((x) =>
      x.id === c.id ? { ...x, liked: !c.liked, like_count: c.like_count + (c.liked ? -1 : 1) } : x
    );
    try {
      const res = await likeComment(c.id);
      comments = comments.map((x) =>
        x.id === c.id ? { ...x, liked: res.liked, like_count: res.like_count } : x
      );
    } catch (e) {
      comments = comments.map((x) => (x.id === c.id ? before : x));
      toastError(e.message || 'Gagal menyukai komentar.');
    }
  }

  async function removeComment(c) {
    try {
      await deleteComment(c.id);
      // Komentar terhapus + balasannya yang jadi yatim ikut hilang dari
      // tampilan (induknya tak ada — sama seperti aplikasi sesudah reload).
      const gone = new Set([c.id]);
      let changed = true;
      while (changed) {
        changed = false;
        for (const x of comments) {
          if (x.parent_id && gone.has(x.parent_id) && !gone.has(x.id)) {
            gone.add(x.id);
            changed = true;
          }
        }
      }
      const removed = comments.filter((x) => gone.has(x.id)).length;
      comments = comments.filter((x) => !gone.has(x.id));
      if (post) post = { ...post, comment_count: Math.max(0, (post.comment_count || 0) - removed) };
      toastSuccess('Komentar dihapus.');
    } catch (e) {
      toastError(e.message || 'Gagal menghapus komentar.');
    }
  }
</script>

<svelte:head><title>Komentar — SubSenpai</title></svelte:head>

<div class="container page">
  <a class="back" href="/post" on:click|preventDefault={() => history.back()}>← Kembali</a>

  {#if !$user}
    <h1>Komentar</h1>
    <p class="muted">Komentar hanya untuk anggota yang masuk.</p>
    <a class="btn" href="/masuk">Masuk dengan Google</a>
  {:else if loading}
    <Skeleton kind="post" />
  {:else if error}
    <p class="error">{error}</p>
    <button class="btn ghost" on:click={load}>Coba lagi</button>
  {:else if post}
    <PostCard {post} hideComments onDelete={() => goto('/post')} onHide={() => goto('/post')} />
    <h2 class="count">{post.comment_count} komentar</h2>
    {#if !comments.length}
      <p class="muted empty">Belum ada komentar. Jadilah yang pertama berkomentar.</p>
    {/if}
    <div class="list">
      {#each rows as row (row.comment.id)}
        {@const c = row.comment}
        <div class="comment" style="padding-left: {row.depth * 22}px">
          <span class="avatar">{c.author?.display_name ? c.author.display_name[0].toUpperCase() : 'S'}</span>
          <div class="c-body">
            <div class="c-head">
              <strong>{c.author?.display_name || 'Pengguna'}</strong>
              {#if c.author?.is_premium} <i class="fa-solid fa-crown"></i>{/if}
              <span class="muted">{fmtWhen(c.created_at)}</span>
            </div>
            <p class="c-text">{c.body}</p>
            <div class="c-actions">
              <button class:on={c.liked} on:click={() => toggleLike(c)} aria-label="Suka komentar">
                <i class={c.liked ? 'fa-solid fa-thumbs-up' : 'fa-regular fa-thumbs-up'}></i>
                {c.like_count}
              </button>
              <button on:click={() => (replyTo = c)}>Balas</button>
              {#if c.mine}
                <button on:click={() => removeComment(c)}>Hapus</button>
              {/if}
            </div>
            {#if row.hasChildren}
              <button class="toggle-replies" on:click={() => toggleCollapse(c.id)}>
                {collapsed.has(c.id) ? `Lihat ${row.descendantCount} balasan` : 'Sembunyikan balasan'}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

{#if $user && post}
  <div class="composer">
    <div class="composer-inner">
      {#if replyTo}
        <div class="replying">
          <span>Membalas {replyTo.author?.display_name || 'Pengguna'}</span>
          <button type="button" on:click={() => (replyTo = null)} aria-label="Batal membalas">
            <i class="fa-solid fa-xmark"></i>
          </button>
        </div>
      {/if}
      <div class="composer-row">
        <span class="avatar">{myInitial}</span>
        <textarea
          rows="1"
          bind:value={input}
          maxlength="500"
          placeholder={replyTo ? 'Tulis balasan…' : 'Tulis komentar…'}
        ></textarea>
        <button class="btn" on:click={send} disabled={sending || !input.trim()}>Kirim</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .page { max-width: 680px; padding-top: 24px; padding-bottom: 180px; }
  .back { display: inline-block; margin-bottom: 16px; color: var(--muted); font-size: 14px; }
  .back:hover { color: #fff; }
  h1 { font-size: 22px; margin: 0 0 8px; }
  .count { font-size: 15px; margin: 16px 0 4px; }
  .empty { margin: 8px 0; }
  .error { color: #ff8a8a; }
  .comment { display: flex; gap: 10px; padding: 10px 0; }
  .avatar {
    width: 32px; height: 32px; border-radius: 50%; flex-shrink: 0;
    background: var(--red); color: #fff; font-weight: 700;
    display: flex; align-items: center; justify-content: center; font-size: 14px;
  }
  .c-body { flex: 1; min-width: 0; }
  .c-head { display: flex; align-items: baseline; gap: 6px; flex-wrap: wrap; font-size: 14px; }
  .c-head .fa-crown { color: var(--gold); font-size: 12px; }
  .c-head .muted { font-size: 12px; }
  .c-text { margin: 2px 0 4px; font-size: 15px; line-height: 1.5; overflow-wrap: anywhere; }
  .c-actions { display: flex; gap: 16px; align-items: center; }
  .c-actions button {
    background: none; border: 0; padding: 4px 0; cursor: pointer;
    color: var(--muted); font-size: 13px; font-weight: 600;
  }
  .c-actions button.on { color: #8b5cf6; }
  .toggle-replies {
    background: none; border: 0; padding: 2px 0; cursor: pointer;
    color: #8b5cf6; font-size: 13px; font-weight: 700;
  }
  .composer {
    position: fixed; left: 0; right: 0; bottom: 0; z-index: 70;
    background: #1f1f1f; border-top: 1px solid var(--border);
  }
  @media (max-width: 767px) {
    .composer { bottom: calc(60px + env(safe-area-inset-bottom)); }
  }
  .composer-inner { max-width: 680px; margin: 0 auto; padding: 10px 16px calc(10px + env(safe-area-inset-bottom)); }
  .replying {
    display: flex; justify-content: space-between; align-items: center;
    font-size: 13px; color: #8b5cf6; padding-bottom: 8px;
  }
  .replying button { background: none; border: 0; color: #fff; cursor: pointer; padding: 2px 6px; }
  .composer-row { display: flex; gap: 8px; align-items: center; }
  .composer-row .avatar { width: 34px; height: 34px; }
  .composer-row textarea {
    flex: 1; min-width: 0; resize: none;
    background: var(--panel); border: 1px solid var(--border); border-radius: 4px;
    color: var(--text); font: inherit; font-size: 14px; padding: 9px 12px;
  }
</style>
