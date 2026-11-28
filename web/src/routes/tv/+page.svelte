<svelte:head>
  <title>TV Langsung — SubSenpai</title>
  <!-- 28 Nov 2026: no-referrer di halaman ini saja. Origin Trans Media
       (live.cnbcindonesia.com) & detik (video.detik.com) menolak
       Referer asing (403) tapi melayani tanpa Referer + CORS * — tanpa
       meta ini Trans TV/Trans7/CNN/CNBC gagal diputar di web (hls.js
       mengirim Referer halaman secara bawaan). Peramban tetap mengirim
       Origin (tidak bisa dihapus), dan itu mereka izinkan. -->
  <meta name="referrer" content="no-referrer" />
</svelte:head>

<script>
  import { onMount, tick } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { getTVChannels, posterUrl } from '$lib/api.js';
  import Player from '$lib/components/Player.svelte';
  import TVChannelCard from '$lib/components/TVChannelCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';

  // TV Langsung (28 Nov 2026): kanal HLS dari /api/v1/tv/channels,
  // diputar langsung dari perangkat pengguna ke sumber stream
  // (VPS tidak mem-proxy video). Gratis semua tier.
  // Lencana: "Resmi" (hijau) bila stability 'bagus', "Tidak stabil"
  // (kuning) bila 'tidak stabil' — lihat TVChannelCard.
  //
  // Tata letak mode tonton ala YouTube (28 Nov 2026, permintaan Rams):
  // pemutar NEMPEL (sticky) — di mobile nempel di bawah navbar, di
  // desktop nempel di kolom kiri — sementara daftar channel di
  // sebelah kanan (desktop) / di bawah (mobile) bisa di-scroll
  // sendiri tanpa menggeser atau menutupi channel yang sedang diputar.
  // Ganti channel = ganti ?channel= saja, pemutar tetap di tempatnya.
  const CATEGORIES = ['', 'Nasional', 'Berita', 'Olahraga', 'Daerah', 'Internasional', 'Anak'];

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
  // Nilai reaktif turunan HARUS deklarasi $:, bukan panggilan fungsi
  // di template (Svelte tidak melacak variabel di badan fungsi).
  $: shown = category ? channels.filter((c) => c.category === category) : channels;
  // Daftar di panel kanan: kanal yang sedang diputar SELALU ada,
  // walau filternya tidak cocok — biar tidak hilang dari daftar.
  $: listShown =
    selected && !shown.some((c) => c.slug === selected.slug) ? [selected, ...shown] : shown;
  $: selIdx = selected ? listShown.findIndex((c) => c.slug === selected.slug) : -1;
  $: prevCh =
    selIdx > -1 && listShown.length > 1
      ? listShown[(selIdx - 1 + listShown.length) % listShown.length]
      : null;
  $: nextCh =
    selIdx > -1 && listShown.length > 1 ? listShown[(selIdx + 1) % listShown.length] : null;
  $: slugNotFound = !loading && !!channelSlug && !selected;

  // Masuk mode tonton (dari grid / deep link) -> gulir ke pemutar.
  // Ganti channel di dalam mode tonton -> JANGAN gulir: pemutar sudah
  // nempel di tempatnya. Selalu gulir baris aktif ke dalam panel list.
  let lastSelectedSlug = '';
  $: if (selected && selected.slug !== lastSelectedSlug) {
    const entering = !lastSelectedSlug;
    lastSelectedSlug = selected.slug;
    if (entering && typeof window !== 'undefined') window.scrollTo({ top: 0, behavior: 'smooth' });
    tick().then(() => {
      if (typeof document !== 'undefined') {
        document.querySelector('.tv-row.active')?.scrollIntoView({ block: 'nearest' });
      }
    });
  }

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

<div class="container page" class:watch={!!selected}>
  {#if !selected}
    <h1>TV Langsung</h1>
    <p class="muted intro">
      Siaran TV lokal &amp; luar — diputar langsung dari perangkatmu ke sumber stream. Gratis untuk semua.
    </p>
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

  {#if selected}
    <div class="watch">
      <div class="watch-player">
        <Player sources={tvSources} title={selected.name} />
        <div class="player-title">
          <div class="pt-info">
            <span class="live"><i class="fa-solid fa-circle"></i> LIVE</span>
            <strong>{selected.name}</strong>
            <span class="muted"> • {selected.category}{selected.quality ? ' • ' + selected.quality : ''}</span>
          </div>
          <button class="btn ghost small" on:click={closePlayer}>
            <i class="fa-solid fa-xmark"></i> Tutup
          </button>
        </div>
      </div>

      <div class="watch-info">
        {#if selected.web_ok === false}
          <p class="note warn">
            <i class="fa-solid fa-mobile-screen"></i> Channel ini hanya bisa diputar di aplikasi Android SubSenpai
            (stream http:// atau MPEG-DASH — peramban tidak bisa memutarnya).
          </p>
        {/if}
        {#if selected.stability !== 'bagus'}
          <p class="note warn">
            <i class="fa-solid fa-triangle-exclamation"></i> Stream ini tidak stabil — kadang mati.
          </p>
        {/if}
        {#if selected.http_referer}
          <p class="note warn">
            <i class="fa-solid fa-triangle-exclamation"></i> Kanal ini butuh header Referer khusus — peramban tidak
            bisa menyetelnya, jadi kemungkinan gagal diputar di web (coba di aplikasi).
          </p>
        {/if}
        {#if prevCh || nextCh}
          <div class="zap">
            {#if prevCh}
              <a class="btn ghost small" href={'/tv?channel=' + prevCh.slug}>
                <i class="fa-solid fa-chevron-left"></i> {prevCh.name}
              </a>
            {/if}
            <span class="muted zap-pos">{selIdx + 1} / {listShown.length}</span>
            {#if nextCh}
              <a class="btn ghost small" href={'/tv?channel=' + nextCh.slug}>
                {nextCh.name} <i class="fa-solid fa-chevron-right"></i>
              </a>
            {/if}
          </div>
        {/if}
      </div>

      <aside class="watch-list">
        <div class="list-head">
          <strong>Channel lainnya</strong>
          <span class="muted">{listShown.length} channel</span>
        </div>
        <div class="list-rows">
          {#each listShown as ch (ch.id)}
            <a class="tv-row" class:active={ch.slug === selected.slug} href={'/tv?channel=' + ch.slug}>
              <span class="thumb">
                {#if ch.logo_url}
                  <img src={posterUrl(ch.logo_url)} alt={ch.name} loading="lazy" />
                {:else}
                  <span class="no-logo"><i class="fa-solid fa-tv"></i></span>
                {/if}
                <span class="live-tag">LIVE</span>
                {#if ch.quality}
                  <span class="quality">{ch.quality}</span>
                {/if}
              </span>
              <span class="row-meta">
                <span class="row-name">{ch.name}</span>
                <span class="row-sub">{ch.category}{ch.quality ? ' • ' + ch.quality : ''}</span>
                <span class="row-badges">
                  {#if ch.stability === 'bagus'}
                    <span class="badge ok"><i class="fa-solid fa-circle-check"></i> Resmi</span>
                  {:else}
                    <span class="badge warn"><i class="fa-solid fa-triangle-exclamation"></i> Tidak stabil</span>
                  {/if}
                  {#if ch.web_ok === false}
                    <span class="badge app"><i class="fa-solid fa-mobile-screen"></i> Aplikasi saja</span>
                  {/if}
                  {#if ch.slug === selected.slug}
                    <span class="now">
                      <span class="eq"><i></i><i></i><i></i></span> Sedang diputar
                    </span>
                  {/if}
                </span>
              </span>
            </a>
          {/each}
        </div>
      </aside>
    </div>
  {:else if loading}
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
  .page.watch {
    max-width: 1440px;
  }
  .page h1 {
    font-size: 26px;
    margin-bottom: 4px;
  }
  .intro {
    margin: 0 0 16px;
    font-size: 14px;
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

  /* --- Mode tonton ala YouTube --- */
  /* Mobile (default): pemutar nempel di bawah navbar (60px) sementara
     daftar di bawahnya di-scroll. Desktop (>=1024px): dua kolom —
     pemutar kiri nempel, daftar kanan nempel + scroll sendiri. */
  .watch-player {
    position: sticky;
    top: 60px;
    z-index: 30;
    background: var(--bg);
    padding-bottom: 10px;
  }
  .player-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-top: 10px;
  }
  .pt-info {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex-wrap: wrap;
  }
  .pt-info strong {
    font-size: 16px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .live {
    color: #ff4e45;
    font-size: 12px;
    font-weight: 800;
    display: inline-flex;
    align-items: center;
    gap: 5px;
    flex: none;
  }
  .live i {
    font-size: 7px;
  }
  .watch-info {
    margin-top: 2px;
  }
  .zap {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-top: 12px;
    flex-wrap: wrap;
  }
  .zap a {
    text-decoration: none;
    max-width: 45%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .zap-pos {
    font-size: 12px;
    flex: none;
  }
  .watch-list {
    margin-top: 16px;
    padding-bottom: 84px; /* ruang BottomNav mobile */
  }
  .list-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    margin-bottom: 8px;
  }
  .list-head strong {
    font-size: 15px;
  }
  .list-head .muted {
    font-size: 12px;
  }
  .list-rows {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .tv-row {
    display: flex;
    gap: 10px;
    padding: 8px;
    border-radius: 8px;
    text-decoration: none;
    color: var(--text);
    border-left: 3px solid transparent;
  }
  .tv-row:hover {
    background: var(--panel);
  }
  .tv-row.active {
    background: var(--panel-2);
    border-left-color: var(--red);
  }
  .thumb {
    position: relative;
    flex: 0 0 112px;
    width: 112px;
    aspect-ratio: 16 / 9;
    background: #000;
    border-radius: 6px;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .thumb img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }
  .no-logo {
    color: var(--muted);
    font-size: 20px;
  }
  .live-tag {
    position: absolute;
    left: 4px;
    bottom: 4px;
    background: #e50914;
    color: #fff;
    font-size: 10px;
    font-weight: 800;
    padding: 1px 5px;
    border-radius: 3px;
  }
  .quality {
    position: absolute;
    top: 4px;
    right: 4px;
    background: rgba(0, 0, 0, 0.75);
    color: #fff;
    font-size: 10px;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 3px;
  }
  .row-meta {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
  }
  .row-name {
    font-size: 14px;
    font-weight: 600;
    line-height: 1.3;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .row-sub {
    font-size: 12px;
    color: var(--muted);
  }
  .row-badges {
    display: flex;
    gap: 5px;
    flex-wrap: wrap;
    align-items: center;
  }
  .badge {
    font-size: 11px;
    font-weight: 700;
    padding: 2px 7px;
    border-radius: 4px;
  }
  .badge.ok {
    background: rgba(52, 211, 153, 0.15);
    color: #34d399;
  }
  .badge.warn {
    background: rgba(245, 197, 24, 0.15);
    color: var(--gold);
  }
  .badge.app {
    background: rgba(139, 92, 246, 0.15);
    color: #a78bfa;
  }
  .now {
    color: var(--red);
    font-size: 11px;
    font-weight: 700;
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .eq {
    display: inline-flex;
    align-items: flex-end;
    gap: 2px;
    height: 12px;
  }
  .eq i {
    width: 3px;
    background: var(--red);
    transform-origin: bottom;
    animation: eq 0.9s ease-in-out infinite;
  }
  .eq i:nth-child(1) {
    height: 40%;
  }
  .eq i:nth-child(2) {
    height: 100%;
    animation-delay: 0.2s;
  }
  .eq i:nth-child(3) {
    height: 60%;
    animation-delay: 0.4s;
  }
  @keyframes eq {
    0%,
    100% {
      transform: scaleY(0.4);
    }
    50% {
      transform: scaleY(1);
    }
  }

  @media (min-width: 1024px) {
    .watch {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 360px;
      column-gap: 24px;
      align-items: start;
    }
    .watch-player {
      grid-column: 1;
      grid-row: 1;
      top: 76px; /* navbar 60px + celah */
      padding-bottom: 0;
    }
    .watch-info {
      grid-column: 1;
      grid-row: 2;
    }
    .watch-list {
      grid-column: 2;
      grid-row: 1 / span 2;
      position: sticky;
      top: 76px;
      max-height: calc(100vh - 92px);
      overflow-y: auto;
      margin-top: 0;
      padding-bottom: 12px;
      scrollbar-width: thin;
    }
  }
</style>
