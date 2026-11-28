<svelte:head><title>Cari — SubSenpai</title></svelte:head>

<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { getTitles, getGenres } from '$lib/api.js';
  import TitleCard from '$lib/components/TitleCard.svelte';
  import LiveBrowseSection from '$lib/components/LiveBrowseSection.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal, infiniteScroll } from '$lib/reveal.js';

  const TYPES = [
    { id: '', label: 'Semua' },
    { id: 'film', label: 'Film' },
    { id: 'anime', label: 'Anime' },
    { id: 'donghua', label: 'Donghua' }
  ];

  let q = $page.url.searchParams.get('q') || '';
  let type = $page.url.searchParams.get('type') || '';
  let genre = $page.url.searchParams.get('genre') || '';
  // Filter lanjutan dari tautan "Lihat semua" Beranda (28 Nov 2026):
  // year didukung backend, status disaring di klien (backend belum
  // punya filter status).
  const yearParam = $page.url.searchParams.get('year') || '';
  let year = /^\d{4}$/.test(yearParam) ? yearParam : '';
  const statusParam = $page.url.searchParams.get('status') || '';
  let status = statusParam === 'ongoing' || statusParam === 'completed' ? statusParam : '';
  let genres = [];
  let results = [];
  let nextCursor = '';
  let loading = false;
  let loadingMore = false;
  let moreError = '';
  let ioSupported = true;
  let error = '';
  let searched = false;

  function syncUrl() {
    const params = new URLSearchParams();
    if (q.trim()) params.set('q', q.trim());
    if (type) params.set('type', type);
    if (genre) params.set('genre', genre);
    if (year) params.set('year', year);
    if (status) params.set('status', status);
    goto('/cari' + (params.toString() ? '?' + params.toString() : ''), { replaceState: true });
  }

  // Ambil halaman getTitles lalu saring status di klien — bila hasil
  // saringan < 20 dan masih ada cursor, lanjut ambil halaman berikut
  // (maks 4 tambahan) supaya halaman tidak kosong.
  async function fetchFiltered(params, want) {
    let items = [];
    let cursor = params.cursor || '';
    let extra = 0;
    do {
      const res = await getTitles({ ...params, ...(cursor ? { cursor } : {}) });
      const page = res.items || [];
      items = items.concat(status ? page.filter((it) => it.status === status) : page);
      cursor = res.next_cursor || '';
      extra++;
    } while (status && items.length < want && cursor && extra <= 4);
    return { items, nextCursor: cursor };
  }

  async function search() {
    loading = true;
    error = '';
    moreError = '';
    searched = true;
    syncUrl();
    try {
      const res = await fetchFiltered({ q: q.trim(), type, genre, year }, 20);
      results = res.items;
      nextCursor = res.nextCursor;
    } catch (e) {
      error = e.message || 'Pencarian gagal.';
      results = [];
    } finally {
      loading = false;
    }
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) return;
    loadingMore = true;
    moreError = '';
    try {
      const res = await fetchFiltered({ q: q.trim(), type, genre, year, cursor: nextCursor }, 20);
      results = [...results, ...res.items];
      nextCursor = res.nextCursor;
    } catch (e) {
      moreError = e.message || 'Gagal memuat lagi.';
    } finally {
      loadingMore = false;
    }
  }

  async function loadGenres(t) {
    try {
      const res = await getGenres(t);
      genres = res.genres || [];
    } catch {
      genres = [];
    }
  }

  function setType(t) {
    type = t;
    genre = '';
    loadGenres(t);
    search();
  }

  function setGenre(g) {
    genre = g === genre ? '' : g;
    search();
  }

  onMount(async () => {
    ioSupported = 'IntersectionObserver' in window;
    await loadGenres(type);
    if (q || type || genre || year || status) search();
    else {
      // Tanpa kata kunci: tampilkan katalog terbaru sebagai penjelajah.
      loading = true;
      try {
        const res = await fetchFiltered({ type, year }, 20);
        results = res.items;
        nextCursor = res.nextCursor;
        searched = true;
      } catch (e) {
        error = e.message || 'Gagal memuat katalog.';
      } finally {
        loading = false;
      }
    }
  });
</script>

<div class="container">
  <h1>Cari &amp; Genre</h1>
  <form class="searchbar" on:submit|preventDefault={search}>
    <!-- id dipakai navbar: ikon cari di /cari fokus ke kolom ini -->
    <input id="cari-input" type="search" placeholder="Cari anime, donghua, film…" bind:value={q} />
    <button class="btn" type="submit">Cari</button>
  </form>

  <div class="tabs">
    {#each TYPES as t}
      <button class="chip" class:active={type === t.id} on:click={() => setType(t.id)}>{t.label}</button>
    {/each}
  </div>

  {#if year || status}
    <div class="active-filters">
      {#if year}
        <button
          class="chip active"
          on:click={() => {
            year = '';
            search();
          }}>Tahun {year} ✕</button
        >
      {/if}
      {#if status}
        <button
          class="chip active"
          on:click={() => {
            status = '';
            search();
          }}>{status === 'ongoing' ? 'Ongoing' : 'Selesai'} ✕</button
        >
      {/if}
    </div>
  {/if}

  {#if genres.length}
    <div class="genres">
      {#each genres as g}
        <button class="chip" class:active={genre === g.name} on:click={() => setGenre(g.name)}>
          {g.name} ({g.count})
        </button>
      {/each}
    </div>
  {/if}

  {#if loading}
    <div class="poster-grid"><Skeleton kind="card" count={12} /></div>
  {:else if error}
    <p class="error">{error}</p>
  {:else if searched}
    <p class="muted">{results.length} judul ditemukan{q ? ` untuk “${q}”` : ''}</p>
    <div class="poster-grid">
      {#each results as item, i (item.id)}
        <div use:reveal={(i % 12) * 45}><TitleCard {item} /></div>
      {/each}
      {#if loadingMore}
        <Skeleton kind="card" count={6} />
      {/if}
    </div>
    {#if !results.length}
      <p class="muted">Tidak ada hasil. Coba kata kunci lain — pencarian juga mengambil langsung dari sumber (Anichin/Kuronime/Hashflix) bila katalog kosong.</p>
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
    {#if type && !q.trim() && (genre || type === 'film')}
      {#key type + '|' + genre + '|' + year}
        <LiveBrowseSection {type} {genre} year={year ? +year : 0} exclude={results} />
      {/key}
    {/if}
  {/if}
</div>

<style>
  h1 { margin: 24px 0 12px; font-size: 28px; }
  .searchbar { display: flex; gap: 8px; margin-bottom: 16px; max-width: 720px; }
  .searchbar input {
    flex: 1; background: var(--panel); border: 1px solid var(--border);
    color: #fff; border-radius: 4px; padding: 12px 14px; font-size: 15px;
  }
  .searchbar input:focus { outline: none; border-color: #fff; }
  .tabs { display: flex; gap: 8px; margin-bottom: 12px; overflow-x: auto; }
  .active-filters { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
  .genres { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 20px; }
  .more { text-align: center; margin: 24px 0; }
</style>
