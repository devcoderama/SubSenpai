<script>
  import { onMount } from 'svelte';
  import { getTitles, getTitle } from '$lib/api.js';
  import Hero from '$lib/components/Hero.svelte';
  import Rail from '$lib/components/Rail.svelte';
  import TitleCard from '$lib/components/TitleCard.svelte';

  const TABS = [
    { id: '', label: 'Semua' },
    { id: 'film', label: 'Film' },
    { id: 'anime', label: 'Anime' },
    { id: 'donghua', label: 'Donghua' }
  ];

  let tab = '';
  let loading = true;
  let error = '';
  let hero = null;
  let heroFirstEp = null;
  let rails = { anime: [], donghua: [], film: [] };
  let grid = [];
  let nextCursor = '';
  let loadingMore = false;

  async function loadTab(t) {
    loading = true;
    error = '';
    try {
      if (t === '') {
        const [a, d, f] = await Promise.all([
          getTitles({ type: 'anime' }),
          getTitles({ type: 'donghua' }),
          getTitles({ type: 'film' })
        ]);
        rails = { anime: a.items || [], donghua: d.items || [], film: f.items || [] };
        grid = [];
        const pick = rails.anime[0] || rails.donghua[0] || rails.film[0];
        if (pick) {
          const det = await getTitle(pick.id);
          hero = { ...(det.title || det), synopsis: det.title?.synopsis || det.synopsis };
          heroFirstEp = det.episodes?.[0]?.id || null;
        }
      } else {
        const res = await getTitles({ type: t });
        grid = res.items || [];
        nextCursor = res.next_cursor || '';
        rails = { anime: [], donghua: [], film: [] };
      }
    } catch (e) {
      error = e.message || 'Gagal memuat beranda.';
    } finally {
      loading = false;
    }
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) return;
    loadingMore = true;
    try {
      const res = await getTitles({ type: tab, cursor: nextCursor });
      grid = [...grid, ...(res.items || [])];
      nextCursor = res.next_cursor || '';
    } catch (e) {
      error = e.message || 'Gagal memuat lagi.';
    } finally {
      loadingMore = false;
    }
  }

  function setTab(t) {
    tab = t;
    loadTab(t);
  }

  onMount(() => loadTab(''));
</script>

{#if hero && tab === ''}
  <Hero item={hero} firstEpisodeId={heroFirstEp} />
{/if}

<div class="container">
  <div class="tabs">
    {#each TABS as t}
      <button class="chip" class:active={tab === t.id} on:click={() => setTab(t.id)}>{t.label}</button>
    {/each}
  </div>

  {#if loading}
    <p class="loading">Memuat katalog…</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if tab === ''}
    <Rail title="Anime" items={rails.anime.slice(0, 12)} moreHref="/cari?type=anime" />
    <Rail title="Donghua" items={rails.donghua.slice(0, 12)} moreHref="/cari?type=donghua" />
    <Rail title="Film" items={rails.film.slice(0, 12)} moreHref="/cari?type=film" />
  {:else}
    <div class="poster-grid">
      {#each grid as item (item.id)}
        <TitleCard {item} />
      {/each}
    </div>
    {#if !grid.length}
      <p class="muted">Belum ada judul untuk jenis ini.</p>
    {/if}
    {#if nextCursor}
      <div class="more">
        <button class="btn ghost" on:click={loadMore} disabled={loadingMore}>
          {loadingMore ? 'Memuat…' : 'Muat lagi'}
        </button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .tabs { display: flex; gap: 8px; margin: 20px 0 4px; overflow-x: auto; }
  .more { text-align: center; margin: 24px 0; }
</style>
