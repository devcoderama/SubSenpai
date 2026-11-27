<script>
  import { onMount } from 'svelte';
  import { user } from '$lib/auth.js';
  import { getFeed, createPost, uploadFeedImage } from '$lib/api.js';
  import PostCard from '$lib/components/PostCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal, infiniteScroll } from '$lib/reveal.js';

  const TABS = [
    { id: 'teman', label: 'Teman' },
    { id: 'grup', label: 'Grup' }
  ];
  const CATS = ['semua', 'umum', 'rekomendasi', 'anime', 'donghua', 'film'];

  let tab = 'teman';
  let category = 'semua';
  let posts = [];
  let nextCursor = '';
  let loading = true;
  let loadingMore = false;
  let moreError = '';
  let ioSupported = true;
  let error = '';

  // Komposer.
  let body = '';
  let cat = 'umum';
  let file = null;
  let filePreview = '';
  let posting = false;
  let postError = '';

  async function load(reset = true) {
    if (reset) {
      loading = true;
      error = '';
      moreError = '';
      posts = [];
      nextCursor = '';
    }
    try {
      const res = await getFeed({
        tab,
        category: category === 'semua' ? '' : category,
        cursor: reset ? '' : nextCursor
      });
      const items = res.items || [];
      posts = reset ? items : [...posts, ...items];
      nextCursor = res.next_cursor || '';
    } catch (e) {
      // Gagal memuat halaman berikut tidak boleh mengosongkan feed.
      if (reset) error = e.message || 'Gagal memuat Post.';
      else moreError = e.message || 'Gagal memuat lagi.';
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  function setTab(t) {
    tab = t;
    load(true);
  }
  function setCat(c) {
    category = c;
    load(true);
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) return;
    loadingMore = true;
    await load(false);
  }

  function onFile(e) {
    file = e.target.files?.[0] || null;
    if (filePreview) URL.revokeObjectURL(filePreview);
    filePreview = file ? URL.createObjectURL(file) : '';
  }

  async function submitPost() {
    const text = body.trim();
    if ((!text && !file) || posting) return;
    posting = true;
    postError = '';
    try {
      let imageUrl = '';
      if (file) {
        const up = await uploadFeedImage(file);
        imageUrl = up.url || '';
      }
      const res = await createPost({ body: text, image_url: imageUrl, category: cat });
      const created = res.post || res;
      if (created?.id || created?.post_id) posts = [created, ...posts];
      else await load(true);
      body = '';
      file = null;
      if (filePreview) URL.revokeObjectURL(filePreview);
      filePreview = '';
    } catch (e) {
      postError = e.message || 'Gagal mengirim postingan.';
    } finally {
      posting = false;
    }
  }

  function removePost(id) {
    posts = posts.filter((p) => p.id !== id);
  }

  onMount(() => {
    ioSupported = 'IntersectionObserver' in window;
    load(true);
  });
</script>

<div class="container page">
  <h1>Post</h1>

  {#if !$user}
    <div class="card login-card">
      <strong>Masuk untuk melihat Post.</strong>
      <span class="muted">Feed komunitas hanya untuk anggota yang masuk.</span>
      <div class="row"><a class="btn" href="/masuk">Masuk dengan Google</a></div>
    </div>
  {:else}
    <div class="card composer">
      <textarea
        placeholder="Bagikan rekomendasi anime, donghua, atau filmmu…"
        bind:value={body}
        maxlength="1000"
        rows="3"
      ></textarea>
      {#if filePreview}
        <div class="preview">
          {#if file?.type?.startsWith('video')}
            <!-- svelte-ignore a11y_media_has_caption -->
            <video src={filePreview} controls preload="metadata"></video>
          {:else}
            <img src={filePreview} alt="Lampiran" />
          {/if}
          <button
            class="btn ghost small"
            on:click={() => {
              file = null;
              URL.revokeObjectURL(filePreview);
              filePreview = '';
            }}>Hapus media</button
          >
        </div>
      {/if}
      <div class="composer-foot">
        <select bind:value={cat} aria-label="Kategori">
          {#each CATS.slice(1) as c}<option value={c}>{c[0].toUpperCase() + c.slice(1)}</option>{/each}
        </select>
        <label class="attach" title="Lampirkan gambar/video">
          <i class="fa-solid fa-paperclip"></i> <input type="file" accept="image/jpeg,image/png,image/webp,video/mp4,video/webm" on:change={onFile} hidden />
        </label>
        <span class="muted count">{body.length}/1000</span>
        <button class="btn" on:click={submitPost} disabled={posting || (!body.trim() && !file)}>
          {posting ? 'Mengirim…' : 'Kirim'}
        </button>
      </div>
      {#if postError}<p class="error">{postError}</p>{/if}
    </div>

    <div class="tabs">
      {#each TABS as t}
        <button class="chip" class:active={tab === t.id} on:click={() => setTab(t.id)}>{t.label}</button>
      {/each}
      <span class="sep"></span>
      {#each CATS as c}
        <button class="chip" class:active={category === c} on:click={() => setCat(c)}
          >{c === 'semua' ? 'Semua' : c[0].toUpperCase() + c.slice(1)}</button
        >
      {/each}
    </div>

    {#if loading}
      <Skeleton kind="post" count={3} />
    {:else if error}
      <p class="error">{error}</p>
    {:else if !posts.length}
      <p class="muted">Belum ada postingan di sini.</p>
    {:else}
      {#each posts as post, i (post.id)}
        <div use:reveal={(i % 10) * 40}><PostCard {post} onDelete={removePost} /></div>
      {/each}
      {#if loadingMore}
        <Skeleton kind="post" count={2} />
      {/if}
      {#if nextCursor}
        <div
          class="sentinel"
          use:infiniteScroll={{ enabled: !!nextCursor && !loadingMore && !moreError, onLoad: loadMore }}
        ></div>
        {#if loadingMore}
          <p class="load-more-note"><span class="bar skeleton"></span>Memuat lagi…</p>
        {:else if moreError}
          <p class="error">{moreError}</p>
          <div class="more">
            <button
              class="btn ghost"
              on:click={() => {
                moreError = '';
                loadMore();
              }}>Coba lagi</button
            >
          </div>
        {:else if !ioSupported}
          <div class="more">
            <button class="btn ghost" on:click={loadMore}>Muat lagi</button>
          </div>
        {/if}
      {/if}
    {/if}
  {/if}
</div>

<style>
  .page { padding-top: 24px; max-width: 680px; }
  .page h1 { font-size: 26px; margin-bottom: 16px; }
  .login-card { display: flex; flex-direction: column; gap: 8px; margin-bottom: 16px; }
  .composer { margin-bottom: 16px; }
  .composer textarea {
    width: 100%; background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 10px; color: #fff; padding: 12px; font-size: 15px; resize: vertical;
    font-family: inherit;
  }
  .preview { margin-top: 10px; }
  .preview img, .preview video { width: 100%; max-height: 360px; object-fit: contain; background: #000; border-radius: 10px; }
  .composer-foot { display: flex; align-items: center; gap: 10px; margin-top: 10px; }
  .composer-foot select {
    background: var(--panel-2); border: 1px solid var(--border); color: #fff;
    border-radius: 999px; padding: 8px 12px; font-size: 13px;
  }
  .attach { font-size: 20px; cursor: pointer; padding: 4px; }
  .count { margin-left: auto; font-size: 12px; }
  .tabs { display: flex; gap: 8px; margin: 4px 0 16px; overflow-x: auto; align-items: center; }
  .sep { width: 1px; height: 22px; background: var(--border); flex-shrink: 0; }
  .more { text-align: center; margin: 24px 0; }
  .error { color: #ff8a8a; font-size: 13px; }
</style>
