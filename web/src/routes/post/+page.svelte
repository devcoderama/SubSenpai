<script>
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { user } from '$lib/auth.js';
  import { searchQuery } from '$lib/search.js';
  import { getFeed, getGroups, getTitle, createPost, uploadFeedImage } from '$lib/api.js';
  import PostCard from '$lib/components/PostCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal, infiniteScroll } from '$lib/reveal.js';

  const TABS = [
    { id: 'teman', label: 'Teman' },
    { id: 'grup', label: 'Grup' }
  ];
  const CATS = ['semua', 'umum', 'rekomendasi', 'anime', 'donghua', 'film'];
  const VISIBILITIES = [
    { id: 'publik', label: 'Publik' },
    { id: 'teman', label: 'Teman' },
    { id: 'pribadi', label: 'Pribadi' }
  ];
  // Post yang disembunyikan lokal (tidak ada endpoint hide di server;
  // aplikasi juga menyembunyikan lokal — FeedScreens.kt hiddenIds).
  const HIDDEN_KEY = 'subsenpai_hidden_posts';

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
  let vis = 'publik';
  let file = null;
  let filePreview = '';
  let posting = false;
  let postError = '';
  let uploadError = '';
  let failedFile = null;
  let composerTa = null;

  // FAB kembali ke atas (item 1).
  let showFab = false;
  function onScroll() {
    showFab = window.scrollY > 600;
  }
  function fabToTop() {
    window.scrollTo({ top: 0, behavior: 'smooth' });
    composerTa?.focus({ preventScroll: true });
  }

  // Grup (item 5): chip filter hanya di tab Grup.
  let groups = [];
  let groupsLoaded = false;
  let groupId = '';
  async function loadGroups() {
    if (groupsLoaded) return;
    groupsLoaded = true;
    try {
      const res = await getGroups();
      // Hanya grup yang diikuti — feed grup server mensyaratkan keanggotaan.
      groups = (res.groups || []).filter((g) => g.is_member);
    } catch {
      groups = [];
    }
  }
  function setGroup(g) {
    groupId = g;
    load(true);
  }

  // Post tersembunyi (item 2).
  function loadHiddenIds() {
    try {
      const raw = JSON.parse(localStorage.getItem(HIDDEN_KEY) || '[]');
      return new Set(Array.isArray(raw) ? raw.filter((x) => typeof x === 'string') : []);
    } catch {
      return new Set();
    }
  }
  let hiddenIds = loadHiddenIds();
  function hidePost(id) {
    hiddenIds = new Set([...hiddenIds, id]);
    try {
      localStorage.setItem(HIDDEN_KEY, JSON.stringify([...hiddenIds]));
    } catch {
      /* penyimpanan penuh/diblokir — sembunyikan tetap berlaku sesi ini */
    }
    posts = posts.filter((p) => p.id !== id);
  }

  // "Bagikan ke Post" dari halaman Detail/Nonton (?title={id}):
  // judul terlampir di postingan (title_id), seperti di aplikasi.
  let shareTitleId = '';
  let shareTitle = null;

  async function loadShareTitle() {
    const tid = $page.url.searchParams.get('title');
    if (!tid) return;
    try {
      const res = await getTitle(tid);
      shareTitle = res.title || res;
      shareTitleId = shareTitle?.id || tid;
    } catch {
      /* abaikan — komposer tetap bisa dipakai tanpa lampiran */
    }
  }

  // Pencarian kontekstual navbar (27 Nov 2026): kata kunci dari navbar
  // memuat ulang feed dengan q (API feed mendukung q). Ganti tab/kategori
  // saat mencari = cari dalam tab/kategori baru (load memakai searchQ).
  let searchQ = '';
  let searchTimer = null;
  $: if ($searchQuery !== undefined) onSearchQuery($searchQuery);
  function onSearchQuery(q) {
    const query = (q || '').trim();
    if (query === searchQ) return;
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => {
      searchQ = query;
      load(true);
    }, 300);
  }

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
        q: searchQ,
        cursor: reset ? '' : nextCursor,
        group_id: tab === 'grup' ? groupId : ''
      });
      // Post tersembunyi lokal difilter di setiap halaman yang dimuat
      // (awal + infinite scroll) agar tetap tersembunyi sesudah reload.
      const items = (res.items || []).filter((p) => !hiddenIds.has(p.id));
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
    if (t === 'grup') loadGroups();
    else groupId = '';
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
    // File baru menggantikan yang gagal — bersihkan spanduk error.
    uploadError = '';
    failedFile = null;
  }

  function discardFailedMedia() {
    file = null;
    failedFile = null;
    uploadError = '';
    if (filePreview) URL.revokeObjectURL(filePreview);
    filePreview = '';
  }

  async function submitPost() {
    const text = body.trim();
    if ((!text && !file && !shareTitleId) || posting) return;
    posting = true;
    postError = '';
    try {
      let imageUrl = '';
      if (file) {
        try {
          const up = await uploadFeedImage(file);
          imageUrl = up.url || '';
          uploadError = '';
          failedFile = null;
        } catch (ue) {
          // Unggah gagal: tampilkan spanduk (item 4) — file disimpan
          // agar "Coba lagi" mengulang dengan file yang sama.
          uploadError = ue.message || 'Gagal mengunggah media.';
          failedFile = file;
          return;
        }
      }
      const res = await createPost({
        body: text,
        image_url: imageUrl,
        category: cat,
        visibility: vis,
        title_id: shareTitleId || null
      });
      const created = res.post || res;
      if (created?.id || created?.post_id) posts = [created, ...posts];
      else await load(true);
      body = '';
      file = null;
      if (filePreview) URL.revokeObjectURL(filePreview);
      filePreview = '';
      shareTitleId = '';
      shareTitle = null;
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
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    loadShareTitle();
    load(true);
  });

  onDestroy(() => {
    window.removeEventListener('scroll', onScroll);
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
    {#if uploadError}
      <div class="card upload-banner" role="alert">
        <span><i class="fa-solid fa-triangle-exclamation"></i> {uploadError}</span>
        <div class="banner-actions">
          <button class="btn small" on:click={submitPost} disabled={posting}>Coba lagi</button>
          <button class="btn ghost small" on:click={discardFailedMedia}>Buang</button>
        </div>
      </div>
    {/if}
    <div class="card composer">
      {#if shareTitle}
        <div class="share-attach">
          <i class="fa-solid fa-tv"></i>
          <span>Membagikan: <strong>{shareTitle.title}</strong></span>
          <button
            class="attach-x"
            aria-label="Lepas lampiran judul"
            on:click={() => { shareTitleId = ''; shareTitle = null; }}
          ><i class="fa-solid fa-xmark"></i></button>
        </div>
      {/if}
      <textarea
        bind:this={composerTa}
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
        <select bind:value={vis} aria-label="Visibilitas">
          {#each VISIBILITIES as v}<option value={v.id}>{v.label}</option>{/each}
        </select>
        <label class="attach" title="Lampirkan gambar/video">
          <i class="fa-solid fa-paperclip"></i> <input type="file" accept="image/jpeg,image/png,image/webp,video/mp4,video/webm" on:change={onFile} hidden />
        </label>
        <span class="muted count">{body.length}/1000</span>
        <button class="btn" on:click={submitPost} disabled={posting || (!body.trim() && !file && !shareTitleId)}>
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

    {#if searchQ && !loading}
      <p class="muted search-note">{posts.length} postingan untuk “{searchQ}”</p>
    {/if}

    {#if tab === 'grup' && groups.length}
      <div class="group-chips" aria-label="Filter grup">
        <button class="chip" class:active={!groupId} on:click={() => setGroup('')}>Semua</button>
        {#each groups as g (g.id)}
          <button class="chip" class:active={groupId === g.id} on:click={() => setGroup(g.id)}>{g.name}</button>
        {/each}
      </div>
    {/if}

    {#if loading}
      <Skeleton kind="post" count={3} />
    {:else if error}
      <p class="error">{error}</p>
    {:else if !posts.length}
      <p class="muted">
        {searchQ ? 'Tidak ada postingan untuk “' + searchQ + '”. Coba kata kunci lain.' : 'Belum ada postingan di sini.'}
      </p>
    {:else}
      {#each posts as post, i (post.id)}
        <div use:reveal={(i % 10) * 40}><PostCard {post} onDelete={removePost} onHide={hidePost} /></div>
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

  {#if $user && showFab}
    <button class="fab" on:click={fabToTop} aria-label="Kembali ke atas dan tulis postingan">
      <i class="fa-solid fa-arrow-up"></i>
    </button>
  {/if}
</div>

<style>
  .page { padding-top: 24px; max-width: 680px; }
  .page h1 { font-size: 26px; margin-bottom: 16px; }
  .login-card { display: flex; flex-direction: column; gap: 8px; margin-bottom: 16px; }
  .composer { margin-bottom: 16px; }
  .upload-banner {
    display: flex; flex-direction: column; gap: 10px; margin-bottom: 16px;
    border-color: #7a2e2e; background: #2a1616; font-size: 14px;
  }
  .upload-banner i { color: #ff8a8a; margin-right: 6px; }
  .banner-actions { display: flex; gap: 8px; }
  .small { font-size: 13px; padding: 6px 12px; }
  .group-chips { display: flex; gap: 8px; margin: 0 0 16px; overflow-x: auto; align-items: center; }
  .fab {
    position: fixed; right: 16px; bottom: 100px; z-index: 50;
    width: 48px; height: 48px; border-radius: 50%; border: 0;
    background: var(--red); color: #fff; font-size: 18px; cursor: pointer;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.5);
  }
  .fab:hover { filter: brightness(1.15); }
  .share-attach {
    display: flex; align-items: center; gap: 8px;
    background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 4px; padding: 10px 12px; margin-bottom: 10px; font-size: 14px;
  }
  .share-attach i.fa-tv { color: var(--purple); }
  .share-attach span { flex: 1; min-width: 0; }
  .attach-x { background: none; border: 0; color: var(--muted); font-size: 15px; padding: 2px 6px; }
  .composer textarea {
    width: 100%; background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 4px; color: #fff; padding: 12px; font-size: 15px; resize: vertical;
    font-family: inherit;
  }
  .preview { margin-top: 10px; }
  .preview img, .preview video { width: 100%; max-height: 360px; object-fit: contain; background: #000; border-radius: 4px; }
  .composer-foot { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 10px; }
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
