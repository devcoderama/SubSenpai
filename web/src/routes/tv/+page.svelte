<svelte:head><title>TV Langsung — SubSenpai</title></svelte:head>

<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { getTVChannels } from '$lib/api.js';
  import Player from '$lib/components/Player.svelte';
  import TVChannelCard from '$lib/components/TVChannelCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';

  // TV Langsung (28 Nov 2026): kanal HLS dari /api/v1/tv/channels,
  // diputar langsung dari perangkat pengguna ke sumber stream
  // (VPS tidak mem-proxy video). Gratis semua tier.
  // Lencana: "Resmi" (hijau) bila stability 'bagus', "Tidak stabil"
  // (kuning) bila 'tidak stabil' — lihat TVChannelCard.
  const CATEGORIES = ['', 'Nasional', 'Berita', 'Olahraga', 'Daerah', 'Internasional'];

  /** @type {any[]} */
  let channels = [];
  let loading = true;
  let error = '';
  let category = '';

  $: channelSlug = $page.url.searchParams.get('channel') || '';
  /** @type {any} */
  let selected = null;
  $: selected = channelSlug ? channels.find((c) => c.slug === channelSlug) || null : null;
  $: tvSources = selected
    ? [{ url: selected.stream_url, label: selected.name, userAgent: selected.http_user_agent || undefined }]
    : [];
  // Kartu diklik dari grid (navigasi ?channel=) -> gulir ke pemutar.
  let lastSelectedSlug = '';
  $: if (selected && selected.slug !== lastSelectedSlug) {
    lastSelectedSlug = selected.slug;
    if (typeof window !== 'undefined') window.scrollTo({ top: 0, behavior: 'smooth' });
  }
  // Nilai reaktif turunan HARUS deklarasi $:, bukan panggilan fungsi
  // di template (Svelte tidak melacak variabel di badan fungsi).
  $: shown = category ? channels.filter((c) => c.category === category) : channels;
  $: slugNotFound = !loading && !!channelSlug && !selected;

  async function load() {
    loading = true;
    error = '';
    try {
      const res = await getTVChannels();
      channels = res.items || [];
    } catch (e) {
      error = (e && e.message) || 'Gagal memuat kanal TV.';
    } finally {
      loading = false;
    }
  }

  function closePlayer() {
    lastSelectedSlug = '';
    goto('/tv', { replaceState: true, noScroll: true });
  }

  onMount(load);
</script>

<div class="container page">
  <h1>TV Langsung</h1>
  <p class="muted intro">Siaran TV lokal &amp; luar — diputar langsung dari perangkatmu ke sumber stream. Gratis untuk semua.</p>

  {#if selected}
    <div class="tv-player">
      <div class="tv-player-head">
        <div>
          <strong>{selected.name}</strong>
          <span class="muted"> • {selected.category}{selected.quality ? ' • ' + selected.quality : ''}</span>
        </div>
        <button class="btn ghost small" on:click={closePlayer}>
          <i class="fa-solid fa-xmark"></i> Tutup
        </button>
      </div>
      <Player sources={tvSources} title={selected.name} />
      {#if selected.stability !== 'bagus'}
        <p class="note warn"><i class="fa-solid fa-triangle-exclamation"></i> Stream ini tidak stabil — kadang mati.</p>
      {/if}
      {#if selected.http_referer}
        <p class="note warn">
          <i class="fa-solid fa-triangle-exclamation"></i> Kanal ini butuh header Referer khusus — peramban tidak bisa
          menyetelnya, jadi kemungkinan gagal diputar di web (coba di aplikasi).
        </p>
      {/if}
    </div>
  {/if}

  {#if slugNotFound}
    <p class="error">Kanal "{channelSlug}" tidak ditemukan.</p>
  {/if}

  <div class="chips">
    {#each CATEGORIES as c}
      <button
        class="chip"
        class:on={category === c}
        on:click={() => {
          category = c;
        }}
      >
        {c === '' ? 'Semua' : c}
      </button>
    {/each}
  </div>

  {#if loading}
    <div class="tv-grid"><Skeleton kind="card" count={8} /></div>
  {:else if error}
    <p class="error">{error}</p>
    <button class="btn" on:click={load}>Coba lagi</button>
  {:else if !shown.length}
    <p class="muted">Belum ada kanal di kategori ini.</p>
  {:else}
    <div class="tv-grid">
      {#each shown as ch (ch.id)}
        <TVChannelCard channel={ch} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .page {
    padding-top: 24px;
    max-width: 1080px;
  }
  .page h1 {
    font-size: 26px;
    margin-bottom: 4px;
  }
  .intro {
    margin: 0 0 16px;
    font-size: 14px;
  }
  .tv-player {
    margin-bottom: 20px;
  }
  .tv-player-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 8px;
  }
  .tv-player-head strong {
    font-size: 17px;
  }
  .note {
    margin: 10px 0 0;
    font-size: 13px;
    line-height: 1.5;
  }
  .note.warn {
    color: var(--gold);
  }
  .chips {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    margin-bottom: 16px;
  }
  .chip {
    background: var(--panel);
    border: 1px solid var(--border);
    color: var(--text);
    border-radius: 999px;
    padding: 7px 14px;
    font-size: 13px;
    cursor: pointer;
  }
  .chip.on {
    background: var(--red);
    border-color: var(--red);
    color: #fff;
  }
  .tv-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 12px;
  }
  .btn.small {
    padding: 6px 12px;
    font-size: 13px;
  }
</style>
