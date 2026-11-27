<script>
  import { onMount, onDestroy } from 'svelte';
  import { getTitles, getTitle, getHome, getHistory } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import { searchQuery } from '$lib/search.js';
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
  // Hero berganti otomatis (27 Nov 2026): daftar putar dari judul
  // paling banyak ditonton (trending 24 jam backend) — 5 teratas.
  let heroItems = []; // [{ title, firstEp }]
  let heroIdx = 0;
  let trending = []; // kartu trending (tanpa sinopsis)
  let heroTimer = null;
  let heroPaused = false;
  let pageHidden = false;
  let reduceMotion = false;
  const HERO_MS = 6000;
  const heroDetailCache = new Map();
  let rails = { anime: [], donghua: [], film: [] };
  let grid = [];
  let nextCursor = '';
  let loadingMore = false;
  let moreError = '';
  let ioSupported = true;
  let continueWatching = [];

  // Pencarian kontekstual navbar (27 Nov 2026): kata kunci dari navbar
  // memfilter Beranda di tempat — hasil cari (API, dalam jenis/chip aktif)
  // menggantikan hero + rel + grid selama kata kunci terisi. Ganti chip
  // saat mencari = cari dalam jenis baru; hapus kata kunci = kembali ke
  // tampilan tab normal.
  let searchMode = false;
  let searchText = '';
  let searchResults = [];
  let searchCursor = '';
  let searchLoading = false;
  let searchLoadingMore = false;
  let searchMoreError = '';
  let searchTimer = null;
  let searchSeq = 0;

  $: if ($searchQuery !== undefined) onSearchQuery($searchQuery);

  function onSearchQuery(q) {
    clearTimeout(searchTimer);
    const query = (q || '').trim();
    if (!query) {
      searchMode = false;
      searchText = '';
      searchResults = [];
      searchCursor = '';
      searchMoreError = '';
      restartHeroTimer();
      return;
    }
    searchText = query;
    if (!searchMode) {
      searchMode = true;
      stopHeroTimer();
    }
    // Bersihkan hasil lama + tampilkan skeleton langsung (bukan sesudah
    // jeda ketik) supaya tidak berkedip.
    searchResults = [];
    searchCursor = '';
    searchLoading = true;
    searchTimer = setTimeout(() => runSearch(query), 300);
  }

  async function runSearch(query) {
    const seq = ++searchSeq;
    searchLoading = true;
    searchMoreError = '';
    try {
      const res = await getTitles({ type: tab, q: query });
      if (seq !== searchSeq) return;
      searchResults = res.items || [];
      searchCursor = res.next_cursor || '';
    } catch (e) {
      if (seq !== searchSeq) return;
      searchResults = [];
      searchCursor = '';
      searchMoreError = e.message || 'Pencarian gagal.';
    } finally {
      if (seq === searchSeq) searchLoading = false;
    }
  }

  async function loadMoreSearch() {
    if (!searchCursor || searchLoadingMore) return;
    searchLoadingMore = true;
    searchMoreError = '';
    try {
      const res = await getTitles({ type: tab, q: searchText, cursor: searchCursor });
      searchResults = [...searchResults, ...(res.items || [])];
      searchCursor = res.next_cursor || '';
    } catch (e) {
      searchMoreError = e.message || 'Gagal memuat lagi.';
    } finally {
      searchLoadingMore = false;
    }
  }

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

  // Hero: ambil detail (sinopsis + episode pertama) untuk maks 5 judul
  // calon, paralel; yang gagal dilewati. Detail di-cache per id supaya
  // ganti tab tidak memuat ulang.
  async function loadHero(candidates) {
    const picks = (candidates || []).slice(0, 5);
    const settled = await Promise.allSettled(
      picks.map(async (c) => {
        if (heroDetailCache.has(c.id)) return heroDetailCache.get(c.id);
        const det = await getTitle(c.id);
        const item = {
          title: { ...(det.title || det), synopsis: det.title?.synopsis || det.synopsis },
          firstEp: det.episodes?.[0]?.id || null
        };
        heroDetailCache.set(c.id, item);
        return item;
      })
    );
    heroItems = settled.filter((r) => r.status === 'fulfilled').map((r) => r.value);
    heroIdx = 0;
    restartHeroTimer();
  }

  function restartHeroTimer() {
    stopHeroTimer();
    if (heroItems.length < 2 || reduceMotion || heroPaused || pageHidden) return;
    heroTimer = setInterval(() => {
      heroIdx = (heroIdx + 1) % heroItems.length;
    }, HERO_MS);
  }
  function stopHeroTimer() {
    if (heroTimer) {
      clearInterval(heroTimer);
      heroTimer = null;
    }
  }
  function goHero(i, manual = false) {
    if (!heroItems.length) return;
    heroIdx = (i + heroItems.length) % heroItems.length;
    if (manual) restartHeroTimer();
  }

  // Geser (swipe) kiri/kanan di hero untuk ganti slide.
  let touchX = null;
  function onTouchStart(e) {
    touchX = e.changedTouches[0].clientX;
  }
  function onTouchEnd(e) {
    if (touchX == null) return;
    const dx = e.changedTouches[0].clientX - touchX;
    touchX = null;
    if (Math.abs(dx) > 50) goHero(heroIdx + (dx < 0 ? 1 : -1), true);
  }

  async function loadTab(t) {
    loading = true;
    error = '';
    stopHeroTimer();
    try {
      if (t === '') {
        // Rel dimuat terpisah: satu jenis yang gagal tidak boleh
        // mengosongkan seluruh Beranda.
        const [a, d, f, h] = await Promise.allSettled([
          getTitles({ type: 'anime' }),
          getTitles({ type: 'donghua' }),
          getTitles({ type: 'film' }),
          getHome()
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
        trending = h.status === 'fulfilled' ? h.value.trending || [] : [];
        grid = [];
        // Hero: 5 judul paling banyak ditonton (trending 24 jam);
        // cadangan bila tren kosong: judul pertama tiap rel.
        const cands = trending.length
          ? trending
          : [rails.anime[0], rails.donghua[0], rails.film[0]].filter(Boolean);
        await loadHero(cands);
      } else {
        const res = await getTitles({ type: t });
        grid = res.items || [];
        nextCursor = res.next_cursor || '';
        rails = { anime: [], donghua: [], film: [] };
        // Hero jenis ini: trending jenis itu; cadangan: 5 pertama grid.
        const cands = trending.filter((x) => x.type === t).slice(0, 5);
        await loadHero(cands.length >= 2 ? cands : grid.slice(0, 5));
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
    // Ganti chip saat sedang mencari = cari dalam jenis baru.
    if (searchMode) runSearch(searchText);
    else loadTab(t);
  }

  onMount(() => {
    ioSupported = 'IntersectionObserver' in window;
    reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const onVis = () => {
      pageHidden = document.hidden;
      if (pageHidden) stopHeroTimer();
      else restartHeroTimer();
    };
    document.addEventListener('visibilitychange', onVis);
    loadTab('');
    loadContinue();
    return () => document.removeEventListener('visibilitychange', onVis);
  });
  onDestroy(() => stopHeroTimer());
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
{:else if searchMode}
  <!-- Hasil cari navbar: menggantikan hero + rel + grid selama mencari. -->
  <div class="container search-results">
    <p class="muted">
      {searchLoading
        ? 'Mencari…'
        : searchResults.length + ' judul ditemukan untuk “' + searchText + '”'}{tab && !searchLoading
        ? ' · ' + (TABS.find((t) => t.id === tab)?.label || '')
        : ''}
    </p>
    {#if searchMoreError}
      <p class="error">{searchMoreError}</p>
    {/if}
    {#if searchLoading}
      <div class="poster-grid"><Skeleton kind="card" count={12} /></div>
    {:else}
      <div class="poster-grid">
        {#each searchResults as item, i (item.id)}
          <div use:reveal={(i % 12) * 45}><TitleCard {item} /></div>
        {/each}
      </div>
      {#if !searchResults.length && !searchMoreError}
        <p class="muted">Tidak ada judul untuk “{searchText}”. Coba kata kunci lain.</p>
      {/if}
      {#if searchCursor}
        <div
          class="sentinel"
          use:infiniteScroll={{ enabled: !!searchCursor && !searchLoadingMore && !searchMoreError, onLoad: loadMoreSearch }}
        ></div>
        {#if searchLoadingMore}
          <p class="load-more-note"><span class="bar skeleton"></span>Memuat lagi…</p>
        {:else if searchMoreError}
          <div class="more">
            <button
              class="btn ghost"
              on:click={() => {
                searchMoreError = '';
                loadMoreSearch();
              }}>Coba lagi</button
            >
          </div>
        {:else if !ioSupported}
          <div class="more">
            <button class="btn ghost" on:click={loadMoreSearch}>Muat lagi</button>
          </div>
        {/if}
      {/if}
    {/if}
  </div>
{:else}
  {#if heroItems.length}
    <div
      class="hero-wrap"
      role="region"
      aria-label="Hero judul populer"
      on:mouseenter={() => {
        heroPaused = true;
        stopHeroTimer();
      }}
      on:mouseleave={() => {
        heroPaused = false;
        restartHeroTimer();
      }}
      on:touchstart={onTouchStart}
      on:touchend={onTouchEnd}
    >
      {#each heroItems as h, i (h.title.id)}
        <div class="hero-slide" class:active={i === heroIdx}>
          <Hero item={h.title} firstEpisodeId={h.firstEp} />
        </div>
      {/each}
      {#if heroItems.length > 1}
        <div class="hero-dots">
          {#each heroItems as _, i}
            <button
              class="dot"
              class:active={i === heroIdx}
              on:click={() => goHero(i, true)}
              aria-label={'Hero ' + (i + 1) + ' dari ' + heroItems.length}
            >
              {#if i === heroIdx && !reduceMotion}
                <span
                  class="dot-fill"
                  style="animation-duration: {HERO_MS}ms; animation-play-state: {heroPaused || pageHidden
                    ? 'paused'
                    : 'running'}"
                ></span>
              {/if}
            </button>
          {/each}
        </div>
        <button class="hero-arrow left" on:click={() => goHero(heroIdx - 1, true)} aria-label="Hero sebelumnya"
          >‹</button
        >
        <button class="hero-arrow right" on:click={() => goHero(heroIdx + 1, true)} aria-label="Hero berikutnya"
          >›</button
        >
      {/if}
    </div>
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

  /* Hero berganti otomatis: slide ditumpuk, crossfade via opacity. */
  .hero-wrap { position: relative; }
  .hero-slide {
    opacity: 0;
    transition: opacity 0.6s ease;
    pointer-events: none;
  }
  .hero-slide.active { opacity: 1; pointer-events: auto; }
  .hero-slide:not(.active) { position: absolute; inset: 0; }
  .hero-dots {
    position: absolute; right: 20px; bottom: 18px; z-index: 5;
    display: flex; gap: 8px;
  }
  .dot {
    width: 24px; height: 4px; border: 0; padding: 0; border-radius: 999px;
    background: rgba(255, 255, 255, 0.35); overflow: hidden;
    position: relative; cursor: pointer;
  }
  .dot-fill {
    position: absolute; left: 0; top: 0; bottom: 0; width: 0;
    background: #fff; animation: hero-fill linear forwards;
  }
  @keyframes hero-fill { from { width: 0; } to { width: 100%; } }
  .hero-arrow {
    position: absolute; top: 50%; transform: translateY(-50%); z-index: 5;
    width: 44px; height: 44px; border-radius: 50%; border: 0;
    background: rgba(0, 0, 0, 0.45); color: #fff; font-size: 26px; line-height: 1;
    cursor: pointer; opacity: 0; transition: opacity 0.2s;
  }
  .hero-wrap:hover .hero-arrow { opacity: 1; }
  .hero-arrow.left { left: 16px; }
  .hero-arrow.right { right: 16px; }
  @media (hover: none) { .hero-arrow { display: none; } }
  @media (prefers-reduced-motion: reduce) {
    .hero-slide { transition: none; }
  }
</style>
