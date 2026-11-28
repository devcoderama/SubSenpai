<script>
  // Seksi "Dari web" — telusur genre LANGSUNG dari situs sumber
  // (cermin HomeScreen.kt aplikasi, permintaan Rams 23 Nov 2026).
  // Backend /live-browse mengerik halaman genre sumber + menyimpan
  // otomatis judul yang tampil ke katalog (StoreBrowseHits); ketukan
  // pada judul yang belum ada di katalog mengimpornya dulu
  // (POST /titles/import) lalu membuka Detail. Gagal sumber = seksi
  // disembunyikan diam-diam (katalog di atas tetap tampil).
  import { goto } from '$app/navigation';
  import { getLiveBrowse, importTitle } from '$lib/api.js';
  import { toastError } from '$lib/toast.js';
  import Skeleton from '$lib/components/Skeleton.svelte';

  export let type = ''; // film | anime | donghua
  export let genre = '';
  export let exclude = []; // item DB yang sudah tampil di atas (dedup)

  let items = [];
  let source = '';
  let hasMore = false;
  let page = 1;
  let loading = false;
  let loadingMore = false;
  let failed = false;
  let importingSlug = null;
  let seq = 0;

  $: excludeSlugs = new Set((exclude || []).map((x) => x.slug).filter(Boolean));
  $: excludeIds = new Set((exclude || []).map((x) => x.id).filter(Boolean));
  $: visible = items.filter(
    (it) => !(it.catalog_id && excludeIds.has(it.catalog_id)) && !excludeSlugs.has(it.slug)
  );

  // Muat ulang halaman 1 setiap kali pasangan type|genre berubah.
  $: if (type && genre) reloadLive(type, genre);

  async function reloadLive(t, g) {
    const s = ++seq;
    items = [];
    source = '';
    hasMore = false;
    page = 1;
    failed = false;
    loading = true;
    try {
      const res = await getLiveBrowse(t, g, 1);
      if (s !== seq) return;
      source = res.source || '';
      hasMore = !!res.has_more;
      items = res.items || [];
    } catch {
      if (s === seq) failed = true;
    } finally {
      if (s === seq) loading = false;
    }
  }

  async function loadMoreLive() {
    if (!hasMore || loadingMore) return;
    const s = seq;
    loadingMore = true;
    try {
      const res = await getLiveBrowse(type, genre, page + 1);
      if (s !== seq) return;
      hasMore = !!res.has_more;
      const seen = new Set(items.map((x) => x.slug));
      items = [...items, ...(res.items || []).filter((x) => !seen.has(x.slug))];
      page += 1;
    } catch {
      /* halaman tambahan gagal — tombol tetap ada untuk coba lagi */
    } finally {
      if (s === seq) loadingMore = false;
    }
  }

  async function openLive(item) {
    if (item.catalog_id) {
      goto('/judul/' + item.catalog_id);
      return;
    }
    if (importingSlug) return;
    importingSlug = item.slug;
    try {
      const res = await importTitle(type, item, genre);
      goto('/judul/' + res.id);
    } catch (e) {
      toastError(e.message || 'Gagal menambahkan judul.');
    } finally {
      importingSlug = null;
    }
  }
</script>

{#if !failed && (loading || visible.length)}
  <div class="live-section">
    <h2>Dari web — {source || 'situs sumber'}</h2>
    <p class="muted">
      Langsung dari {source || 'situs sumber'} untuk genre ini — yang belum ada di katalog tersimpan
      otomatis saat tampil.
    </p>
    {#if loading}
      <div class="poster-grid"><Skeleton kind="card" count={6} /></div>
    {:else}
      <div class="poster-grid">
        {#each visible as item (item.slug)}
          <button class="live-card" on:click={() => openLive(item)} disabled={importingSlug === item.slug}>
            {#if item.poster_url}
              <img src={item.poster_url} alt={item.title} loading="lazy" />
            {:else}
              <span class="live-noimg">Tanpa poster</span>
            {/if}
            <span class="live-title">{item.title}</span>
            {#if item.year}<span class="muted">{item.year}</span>{/if}
            {#if importingSlug === item.slug}<span class="muted">Menambahkan…</span>{/if}
          </button>
        {/each}
      </div>
      {#if hasMore}
        <div class="more">
          <button class="btn ghost" on:click={loadMoreLive} disabled={loadingMore}>
            {loadingMore ? 'Memuat…' : 'Muat live lainnya'}
          </button>
        </div>
      {/if}
    {/if}
  </div>
{/if}

<style>
  .live-section { margin-top: 28px; }
  .live-section h2 { font-size: 20px; margin: 0 0 4px; }
  .live-section > .muted { margin: 0 0 14px; font-size: 13px; }
  .live-card {
    display: block; text-align: left; background: none; border: 0; padding: 0;
    cursor: pointer; color: inherit; font: inherit;
  }
  .live-card:disabled { opacity: 0.6; cursor: default; }
  .live-card img {
    display: block; width: 100%; aspect-ratio: 2 / 3; object-fit: cover;
    border-radius: 4px; background: var(--panel);
  }
  .live-noimg {
    display: flex; align-items: center; justify-content: center;
    width: 100%; aspect-ratio: 2 / 3; border-radius: 4px;
    background: var(--panel); color: var(--muted); font-size: 13px;
  }
  .live-title { display: block; font-size: 14px; line-height: 1.35; margin-top: 6px; }
  .live-card .muted { display: block; font-size: 12px; margin-top: 2px; }
  .more { text-align: center; margin: 20px 0 8px; }
</style>
