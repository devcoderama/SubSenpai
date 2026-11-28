<script>
  // Seksi "Dari web" — telusur genre LANGSUNG dari situs sumber
  // (cermin HomeScreen.kt aplikasi, permintaan Rams 23 Nov 2026).
  // Backend /live-browse mengerik halaman genre sumber + menyimpan
  // otomatis judul yang tampil ke katalog (StoreBrowseHits); ketukan
  // pada judul yang belum ada di katalog mengimpornya dulu
  // (POST /titles/import) lalu membuka Detail. Gagal sumber = seksi
  // disembunyikan diam-diam (katalog di atas tetap tampil).
  //
  // Film: filter lengkap ala donkey.to/filters (permintaan Rams
  // 28 Nov 2026) — genre boleh banyak, tahun, negara, urutan.
  // Datanya tetap TMDB lewat discover Hashflix (persis yang
  // ditampilkan donkey.to — hasil Action+2024+Populer terbukti
  // identik), jadi tak perlu mengerik donkey.to; streaming tetap
  // embed Hashflix. Filter ini HANYA menggerakkan seksi live —
  // grid database di atas tetap memakai panel Filter halaman.
  import { goto } from '$app/navigation';
  import { getLiveBrowse, getFilters, importTitle } from '$lib/api.js';
  import { FILM_COUNTRIES, FILM_SORTS } from '$lib/countries.js';
  import { toastError } from '$lib/toast.js';
  import Skeleton from '$lib/components/Skeleton.svelte';

  export let type = ''; // film | anime | donghua | series
  export let genre = '';
  export let year = 0;
  export let country = '';
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

  // State filter film (awal = filter halaman yang membawanya ke sini).
  // country boleh kode ISO ("US") atau nama ("Amerika Serikat" —
  // begitu Beranda menyimpan fCountry); normalisasi ke kode.
  function toCountryCode(v) {
    if (!v) return '';
    const t = String(v).trim();
    if (/^[A-Za-z]{2}$/.test(t)) return t.toUpperCase();
    const hit = FILM_COUNTRIES.find((c) => c.name.toLowerCase() === t.toLowerCase());
    return hit ? hit.code : '';
  }
  let selGenres = genre ? [genre] : [];
  let selYear = year || 0;
  let selCountry = toCountryCode(country);
  let selSort = 'popular';
  let filmGenres = [];
  const years = [];
  for (let y = 2026; y >= 1920; y--) years.push(y);

  $: excludeSlugs = new Set((exclude || []).map((x) => x.slug).filter(Boolean));
  $: excludeIds = new Set((exclude || []).map((x) => x.id).filter(Boolean));
  $: visible = items.filter(
    (it) => !(it.catalog_id && excludeIds.has(it.catalog_id)) && !excludeSlugs.has(it.slug)
  );

  // Daftar genre film/serial TV (19 genre TMDB) untuk chip multi-pilih.
  $: if ((type === 'film' || type === 'series') && !filmGenres.length) {
    getFilters(type)
      .then((f) => (filmGenres = f.genres || []))
      .catch(() => {});
  }

  // Muat ulang halaman 1 setiap kali filter berubah. Film & serial
  // TV selalu muat (tanpa genre = semua judul, urutan Populer —
  // seperti bawaan donkey.to); anime/donghua butuh satu genre.
  $: filterKey = [type, selGenres.join(','), selYear, selCountry, selSort].join('|');
  // filterKey DIBACA di sini supaya Svelte memuat ulang saat salah
  // satu filter berubah — tanpanya (bug 28 Nov 2026) hanya ganti
  // genre yang memuat ulang, sementara tahun/negara/urutan diam
  // (kartu halaman 1 tak berubah; baru "Muat live lainnya" yang
  // memakai filter baru).
  $: if (type && (type === 'film' || type === 'series' || selGenres.length)) {
    void filterKey;
    reloadLive();
  }

  function liveOpts() {
    return { genres: selGenres, year: selYear, country: selCountry, sort: selSort };
  }

  async function reloadLive() {
    const s = ++seq;
    items = [];
    source = '';
    hasMore = false;
    page = 1;
    failed = false;
    loading = true;
    try {
      const res = await getLiveBrowse(type, selGenres[0] || '', 1, liveOpts());
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

  // Muat otomatis saat digulir ke bawah (permintaan Rams 28 Nov
  // 2026 — tombol "Muat live lainnya" dicabut): sentinel di ujung
  // seksi; IntersectionObserver memanggil loadMoreLive begitu
  // sentinel masuk viewport (+600px). Penjaga di loadMoreLive
  // (hasMore + loadingMore) mencegah panggilan ganda; sentinel
  // hilang sendiri saat hasMore habis.
  function sentinelAction(node) {
    const io = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) loadMoreLive();
      },
      { rootMargin: '600px 0px' }
    );
    io.observe(node);
    return {
      destroy() {
        io.disconnect();
      }
    };
  }

  async function loadMoreLive() {
    if (!hasMore || loadingMore) return;
    const s = seq;
    loadingMore = true;
    try {
      const res = await getLiveBrowse(type, selGenres[0] || '', page + 1, liveOpts());
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

  function toggleGenre(g) {
    selGenres = selGenres.includes(g) ? selGenres.filter((x) => x !== g) : [...selGenres, g];
  }

  function resetFilters() {
    selGenres = genre ? [genre] : [];
    selYear = year || 0;
    selCountry = country || '';
    selSort = 'popular';
  }

  $: dirty =
    selGenres.join(',') !== (genre || '') ||
    selYear !== (year || 0) ||
    selCountry !== (country || '') ||
    selSort !== 'popular';

  async function openLive(item) {
    if (item.catalog_id) {
      goto('/judul/' + item.catalog_id);
      return;
    }
    if (importingSlug) return;
    importingSlug = item.slug;
    try {
      const res = await importTitle(type, item, selGenres[0] || genre);
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
      {#if type === 'film' || type === 'series'}
        Filter ala donkey.to — genre, tahun, negara, urutan — langsung dari {source ||
          'situs sumber'}; yang belum ada di katalog tersimpan otomatis saat tampil.
      {:else}
        Langsung dari {source || 'situs sumber'} untuk genre ini — yang belum ada di katalog tersimpan
        otomatis saat tampil.
      {/if}
    </p>
    {#if type === 'film' || type === 'series'}
      <div class="live-filters">
        {#if filmGenres.length}
          <div class="lf-genres">
            {#each filmGenres as g (g.value)}
              <button
                class="chip"
                class:active={selGenres.includes(g.name)}
                on:click={() => toggleGenre(g.name)}>{g.name}</button
              >
            {/each}
          </div>
        {/if}
        <div class="lf-row">
          <select bind:value={selYear} aria-label="Tahun">
            <option value={0}>Semua tahun</option>
            {#each years as y}<option value={y}>{y}</option>{/each}
          </select>
          <select bind:value={selCountry} aria-label="Negara">
            <option value="">Semua negara</option>
            {#each FILM_COUNTRIES as c}<option value={c.code}>{c.name}</option>{/each}
          </select>
          <select bind:value={selSort} aria-label="Urutan">
            {#each FILM_SORTS as s}<option value={s.key}>{s.name}</option>{/each}
          </select>
          {#if dirty}<button class="linklike" on:click={resetFilters}>Reset</button>{/if}
        </div>
      </div>
    {/if}
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
        <div class="more" use:sentinelAction>
          {#if loadingMore}<span class="muted">Memuat…</span>{/if}
        </div>
      {/if}
    {/if}
  </div>
{/if}

<style>
  .live-section { margin-top: 28px; }
  .live-section h2 { font-size: 20px; margin: 0 0 4px; }
  .live-section > .muted { margin: 0 0 14px; font-size: 13px; }
  .live-filters { margin: 0 0 16px; display: flex; flex-direction: column; gap: 10px; }
  .lf-genres { display: flex; flex-wrap: wrap; gap: 8px; }
  .lf-row { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .lf-row select {
    background: var(--panel); color: var(--text); border: 1px solid var(--border);
    border-radius: 8px; padding: 8px 10px; font: inherit; font-size: 14px;
    max-width: 100%;
  }
  .linklike {
    background: none; border: 0; padding: 0; cursor: pointer;
    color: var(--muted); font: inherit; font-size: 14px; text-decoration: underline;
  }
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
  .more { text-align: center; margin: 20px 0 8px; min-height: 24px; }
</style>
