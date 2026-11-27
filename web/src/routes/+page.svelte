<script>
  import { onMount } from 'svelte';
  import { getTitles, getTitle, getHistory } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import { getLocalHistory, fmtWhen } from '$lib/history.js';
  import Hero from '$lib/components/Hero.svelte';
  import Rail from '$lib/components/Rail.svelte';
  import TitleCard from '$lib/components/TitleCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal, infiniteScroll } from '$lib/reveal.js';

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
  let moreError = '';
  let ioSupported = true;
  let continueWatching = [];

  // Lanjutkan Menonton (seperti aplikasi): server bila masuk, lokal bila tamu.
  async function loadContinue() {
    try {
      const raw = $user ? (await getHistory(12)).items || [] : getLocalHistory();
      const seen = new Set();
      continueWatching = raw
        .filter((it) => {
          if (seen.has(it.title_id)) return false;
          seen.add(it.title_id);
          return true;
        })
        .slice(0, 12)
        .map((it) => ({
          id: it.title_id,
          poster_url: it.poster_url,
          title: it.title,
          sub: 'Episode ' + it.episode_no + ' • ' + fmtWhen(it.updated_at),
          href: '/nonton/' + it.episode_id
        }));
    } catch {
      continueWatching = [];
    }
  }

  async function loadTab(t) {
    loading = true;
    error = '';
    try {
      if (t === '') {
        // Rel dimuat terpisah: satu jenis yang gagal tidak boleh
        // mengosongkan seluruh Beranda.
        const [a, d, f] = await Promise.allSettled([
          getTitles({ type: 'anime' }),
          getTitles({ type: 'donghua' }),
          getTitles({ type: 'film' })
        ]);
        rails = {
          anime: a.status === 'fulfilled' ? a.value.items || [] : [],
          donghua: d.status === 'fulfilled' ? d.value.items || [] : [],
          film: f.status === 'fulfilled' ? f.value.items || [] : []
        };
        if (!rails.anime.length && !rails.donghua.length && !rails.film.length) {
          const failed = [a, d, f].find((r) => r.status === 'rejected');
          if (failed && failed.status === 'rejected') throw failed.reason;
        }
        grid = [];
        const pick = rails.anime[0] || rails.donghua[0] || rails.film[0];
        if (pick) {
          try {
            const det = await getTitle(pick.id);
            hero = { ...(det.title || det), synopsis: det.title?.synopsis || det.synopsis };
            heroFirstEp = det.episodes?.[0]?.id || null;
          } catch {
            // Hero gagal tidak boleh mengosongkan Beranda — rel tetap tampil.
            hero = null;
          }
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
    moreError = '';
    try {
      const res = await getTitles({ type: tab, cursor: nextCursor });
      grid = [...grid, ...(res.items || [])];
      nextCursor = res.next_cursor || '';
    } catch (e) {
      // Gagal memuat halaman berikut tidak boleh mengosongkan grid —
      // tampilkan retry di bawah daftar.
      moreError = e.message || 'Gagal memuat lagi.';
    } finally {
      loadingMore = false;
    }
  }

  function setTab(t) {
    tab = t;
    loadTab(t);
  }

  onMount(() => {
    ioSupported = 'IntersectionObserver' in window;
    loadTab('');
    loadContinue();
  });
</script>

<div class="container tabs-top">
  <div class="tabs">
    {#each TABS as t}
      <button class="chip" class:active={tab === t.id} on:click={() => setTab(t.id)}>{t.label}</button>
    {/each}
  </div>
</div>

{#if loading}
  {#if tab === ''}
    <Skeleton kind="hero" />
    <div class="container">
      <div class="rail" aria-hidden="true"><Skeleton kind="card" count={6} /></div>
      <div class="rail" aria-hidden="true"><Skeleton kind="card" count={6} /></div>
    </div>
  {:else}
    <div class="container">
      <div class="poster-grid"><Skeleton kind="card" count={12} /></div>
    </div>
  {/if}
{:else}
  {#if hero && tab === ''}
    <Hero item={hero} firstEpisodeId={heroFirstEp} />
  {/if}
  <div class="container">
    {#if error}
      <p class="error">{error}</p>
    {:else if tab === ''}
      {#if continueWatching.length}
        <Rail title="Lanjutkan Menonton" items={continueWatching} moreHref="/riwayat" />
      {/if}
      <Rail title="Anime" items={rails.anime.slice(0, 12)} moreHref="/cari?type=anime" />
      <Rail title="Donghua" items={rails.donghua.slice(0, 12)} moreHref="/cari?type=donghua" />
      <Rail title="Film" items={rails.film.slice(0, 12)} moreHref="/cari?type=film" />
    {:else}
      <div class="poster-grid">
        {#each grid as item, i (item.id)}
          <div use:reveal={(i % 12) * 45}><TitleCard {item} /></div>
        {/each}
        {#if loadingMore}
          <Skeleton kind="card" count={6} />
        {/if}
      </div>
      {#if !grid.length}
        <p class="muted">Belum ada judul untuk jenis ini.</p>
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
  </div>
{/if}

<style>
  .tabs-top { padding-top: 16px; }
  .tabs { display: flex; gap: 8px; margin: 0 0 16px; overflow-x: auto; justify-content: safe center; }
  .more { text-align: center; margin: 24px 0; }
</style>
