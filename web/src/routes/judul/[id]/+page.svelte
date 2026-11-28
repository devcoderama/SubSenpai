<script>
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    getTitle, getTitles, posterUrl,
    checkWatchlist, addWatchlist, removeWatchlist
  } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import EpisodeList from '$lib/components/EpisodeList.svelte';
  import TitleCard from '$lib/components/TitleCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal } from '$lib/reveal.js';

  let loading = true;
  let error = '';
  let title = null;
  let episodes = [];
  let serupa = [];
  let copied = false;
  let shareOpen = false;
  let lockedEp = null; // episode terkunci yang dialognya sedang tampil
  let inWatchlist = false;
  let wlBusy = false;
  let wlError = '';

  $: id = $page.params.id;
  $: firstLocked = episodes.length > 0 && episodes[0].locked;
  // Baris meta ala aplikasi (DetailHeading): rating · tahun · durasi ·
  // jenis · status (Tamat/Berlanjut).
  $: durationLabel = (() => {
    const m = title?.duration_min;
    if (!m) return '';
    if (title.type === 'film') return m >= 60 ? `${Math.floor(m / 60)}j ${m % 60}mnt` : `${m} mnt`;
    return `${m} mnt/ep`;
  })();
  $: statusLabel =
    title?.status === 'completed' ? 'Tamat' : title?.status === 'ongoing' ? 'Berlanjut' : title?.status || '';
  $: metaLine = [
    title?.rating ? `★ ${Number(title.rating).toFixed(1)}` : null,
    title?.year || null,
    durationLabel || null,
    title?.type ? title.type[0].toUpperCase() + title.type.slice(1) : null,
    statusLabel || null
  ]
    .filter(Boolean)
    .join(' · ');
  $: seasonLabel = title?.season
    ? title.season[0].toUpperCase() + title.season.slice(1) + (title.year ? ' ' + title.year : '')
    : '';
  $: infoRows = [
    title?.studio ? ['Studio', title.studio] : null,
    seasonLabel ? ['Musim', seasonLabel] : null,
    title?.title_alt && title.title_alt !== title?.title ? ['Judul Lain', title.title_alt] : null
  ].filter(Boolean);

  function goBack() {
    if (window.history.length > 1) window.history.back();
    else goto('/');
  }

  // Tombol utama (Putar / Buka dengan Iklan): episode terkunci → tampilkan
  // dialog "Episode Terkunci" dulu (Tonton Iklan / Premium / Nanti Saja),
  // seperti ketuk episode terkunci di daftar. Sebelum perbaikan ini
  // (27 Nov 20:13, verifikasi film Elsinore) tombol ini langsung loncat
  // ke Nonton sehingga dialog tidak pernah muncul di mana pun: film tidak
  // menampilkan daftar episode (jadi onEpisodeClick tak pernah terpanggil)
  // dan anime/donghua kini selalu terbuka untuk semua tier.
  function playFirst() {
    if (!episodes.length) return;
    if (episodes[0].locked) {
      lockedEp = episodes[0];
      return;
    }
    goto('/nonton/' + episodes[0].id);
  }

  async function shareCopy() {
    const url = window.location.href;
    try {
      await navigator.clipboard.writeText(url);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      window.prompt('Salin tautan ini:', url);
    }
  }

  // Bagikan native (Web Share API): bila tidak tersedia, jatuh kembali
  // ke Salin tautan. Judul/teks = judul konten, URL = halaman ini.
  async function shareNative() {
    const url = window.location.href;
    if (navigator.share) {
      try {
        await navigator.share({ title: title?.title || 'SubSenpai', text: title?.title || '', url });
        return;
      } catch (e) {
        // Pengguna menutup dialog share — jangan fallback ke salin.
        if (e?.name === 'AbortError') return;
      }
    }
    shareCopy();
  }

  // Ketuk episode terkunci di daftar: tahan navigasi, tampilkan dialog
  // kunci dulu (seperti aplikasi). Episode terbuka tetap langsung jalan.
  // Daftar episode di-render EpisodeList (tautan <a> polos), jadi klik
  // dicegat di pembungkus ini lewat bubbling.
  function onEpisodeClick(e) {
    const a = e.target?.closest?.('a');
    if (!a) return;
    const m = (a.getAttribute('href') || '').match(/^\/nonton\/([^?]+)/);
    if (!m) return;
    const ep = episodes.find((x) => x.id === m[1]);
    if (ep?.locked) {
      e.preventDefault();
      lockedEp = ep;
    }
  }

  async function toggleWatchlist() {
    if (!$user) {
      goto('/masuk');
      return;
    }
    if (wlBusy) return;
    wlBusy = true;
    wlError = '';
    try {
      if (inWatchlist) {
        await removeWatchlist(id);
        inWatchlist = false;
      } else {
        await addWatchlist(id);
        inWatchlist = true;
      }
    } catch (e) {
      wlError = e.message || 'Gagal mengubah Daftar Saya.';
    } finally {
      wlBusy = false;
    }
  }

  async function loadDetail() {
    loading = true;
    error = '';
    serupa = [];
    try {
      const res = await getTitle(id);
      title = res.title || res;
      episodes = res.episodes || [];
      if ($user) {
        try {
          const c = await checkWatchlist(id);
          inWatchlist = !!c.in_watchlist;
        } catch {
          /* abaikan — Daftar Saya tetap bisa diketuk */
        }
      }
      const g = title.genres?.[0];
      if (g) {
        try {
          const r = await getTitles({ genre: g, type: title.type || '', limit: 10 });
          serupa = (r.items || []).filter((t) => t.id !== id).slice(0, 9);
        } catch {
          /* abaikan — Judul Serupa opsional */
        }
      }
    } catch (e) {
      error = e.message || 'Judul tidak ditemukan.';
    } finally {
      loading = false;
    }
  }

  // Pindah antar Detail (ketuk Judul Serupa) TIDAK me-remount komponen
  // — SvelteKit memakai ulang instance untuk rute yang sama — jadi
  // onMount saja tidak cukup: muat ulang tiap id berubah. Pola yang
  // sama dengan loadedEp di halaman Nonton (yang sudah benar).
  let loadedId = '';
  $: if (id && id !== loadedId) {
    loadedId = id;
    loadDetail();
  }
</script>

{#if loading}
  <Skeleton kind="detail" />
{:else if error}
  <div class="container">
    <p class="error">{error}</p>
    <p><a href="/"><i class="fa-solid fa-arrow-left"></i> Kembali ke Beranda</a></p>
  </div>
{:else if title}
  <!-- Kepala ala aplikasi: latar buram dari poster yang sama, poster
       tajam di tengah, tombol kembali melayang. -->
  <div class="backdrop">
    {#if title.poster_url}
      <img class="bg" src={posterUrl(title.poster_url)} alt="" aria-hidden="true" />
      <div class="shade"></div>
      <img class="fg" src={posterUrl(title.poster_url)} alt={title.title} />
      <div class="fade"></div>
    {/if}
    <button class="back" on:click={goBack} aria-label="Kembali">
      <i class="fa-solid fa-arrow-left"></i>
    </button>
  </div>

  <div class="detail-wrap">
    <h1>{title.title}</h1>
    {#if title.title_alt}<div class="alt muted">{title.title_alt}</div>{/if}
    {#if metaLine}<div class="meta">{metaLine}</div>{/if}
    {#if title.genres?.length}
      <div class="genres">{title.genres.join(' • ')}</div>
    {/if}

    {#if episodes.length}
      <button class="play-btn" on:click={playFirst}>
        <i class={'fa-solid ' + (firstLocked ? 'fa-lock' : 'fa-play')}></i>
        {firstLocked ? 'Buka dengan Iklan' : 'Putar'}
      </button>
    {/if}

    <div class="actions-row">
      <button class="detail-action" on:click={toggleWatchlist} disabled={wlBusy}>
        <i class={'fa-solid ' + (inWatchlist ? 'fa-check' : 'fa-plus')}></i>
        <span>Daftar Saya</span>
      </button>
      <div class="share-wrap">
        <button class="detail-action" on:click={() => (shareOpen = !shareOpen)}>
          <i class="fa-solid fa-share"></i>
          <span>Bagikan</span>
        </button>
        {#if shareOpen}
          <button class="menu-backdrop" on:click={() => (shareOpen = false)} aria-label="Tutup menu"></button>
          <div class="menu">
            <button on:click={() => { shareOpen = false; shareNative(); }}>
              Bagikan…
            </button>
            <button on:click={() => { shareOpen = false; goto('/post?title=' + id); }}>
              Bagikan ke Post
            </button>
            <button on:click={() => { shareOpen = false; shareCopy(); }}>
              {copied ? 'Tautan disalin' : 'Salin tautan'}
            </button>
          </div>
        {/if}
      </div>
    </div>
    {#if wlError}<p class="error">{wlError}</p>{/if}

    {#if title.synopsis}
      <h2 class="sec">Sinopsis</h2>
      <p class="synopsis">{title.synopsis}</p>
    {/if}

    {#if infoRows.length}
      <h2 class="sec">Detail</h2>
      {#each infoRows as [label, value]}
        <div class="info-row"><span class="info-label">{label}</span><span>{value}</span></div>
      {/each}
    {/if}

    {#if title.type !== 'film'}
      {#if episodes.length}
        <div on:click={onEpisodeClick}>
          <EpisodeList {episodes} titleId={title.id} />
        </div>
      {:else}
        <p class="muted">Episode belum tersedia di sumber untuk judul ini.</p>
        <button class="btn ghost" on:click={loadDetail}>Coba Lagi</button>
      {/if}
    {/if}

    {#if serupa.length}
      <section class="similar">
        <h2 class="sec-big">Judul Serupa</h2>
        <div class="similar-grid">
          {#each serupa as t, i (t.id)}
            <div use:reveal={(i % 9) * 40}><TitleCard item={t} /></div>
          {/each}
        </div>
      </section>
    {/if}
  </div>

  {#if lockedEp}
    <div class="modal-backdrop" on:click={() => (lockedEp = null)}></div>
    <div class="modal" role="dialog" aria-label="Episode Terkunci">
      <h2><i class="fa-solid fa-lock"></i> Episode Terkunci</h2>
      <p class="muted">
        Episode {lockedEp.episode_no}{lockedEp.title ? ' — ' + lockedEp.title : ''} khusus Premium. Buka
        episode ini dengan menonton iklan, atau upgrade ke Premium.
      </p>
      <div class="modal-actions">
        <button class="btn" on:click={() => goto('/nonton/' + lockedEp.id + '?title=' + id)}>
          <i class="fa-solid fa-tv"></i> Tonton Iklan
        </button>
        <button class="btn ghost" on:click={() => goto('/premium')}>Premium</button>
        <button class="btn ghost" on:click={() => (lockedEp = null)}>Nanti Saja</button>
      </div>
    </div>
  {/if}
{/if}

<style>
  /* Backdrop: poster buram memenuhi lebar, poster tajam di tengah,
     gradasi ke latar di bawah (seperti DetailBackdrop aplikasi).
     Lebar 100% (bukan 100vw): 100vw ikut menghitung lebar scrollbar,
     sehingga halaman mendapat scroll horizontal yang tidak perlu. */
  .backdrop { position: relative; height: 340px; width: 100%; overflow: hidden; }
  .backdrop .bg { position: absolute; inset: -5%; width: 110%; height: 110%; object-fit: cover; filter: blur(30px); }
  .backdrop .shade { position: absolute; inset: 0; background: rgba(0, 0, 0, 0.35); }
  .backdrop .fg {
    position: absolute; left: 50%; top: 0; transform: translateX(-50%);
    height: 100%; aspect-ratio: 2 / 3; object-fit: cover;
  }
  .backdrop .fade {
    position: absolute; inset: 0;
    background: linear-gradient(to bottom, rgba(0, 0, 0, 0.3), transparent 45%, var(--bg));
  }
  .back {
    position: absolute; top: 12px; left: 12px; z-index: 2;
    width: 36px; height: 36px; border-radius: 50%; border: 0;
    background: rgba(0, 0, 0, 0.6); color: #fff; font-size: 16px;
    display: flex; align-items: center; justify-content: center;
  }

  .detail-wrap { max-width: 720px; margin: 0 auto; padding: 0 16px 48px; }
  @media (min-width: 1280px) {
    /* Desktop ala Netflix (27 Nov 2026): backdrop lebih tinggi, kolom
       info + episode + serupa lebih lega (1024px), bukan lagi 720px. */
    .backdrop { height: 420px; }
    .detail-wrap { max-width: 1024px; padding: 0 48px 48px; }
  }
  h1 { margin: 16px 0 4px; font-size: 28px; line-height: 1.15; font-weight: 900; }
  .alt { font-size: 15px; }
  .meta { margin-top: 6px; color: #ddd; font-size: 15px; }
  .meta .fa-star, .meta .rate { color: var(--gold); }
  .genres { margin-top: 4px; color: var(--muted); font-size: 15px; }
  .play-btn {
    width: 100%; height: 48px; margin-top: 14px; border: 0; border-radius: 4px;
    background: #fff; color: #000; font-size: 17px; font-weight: 700;
    display: flex; align-items: center; justify-content: center; gap: 6px;
  }
  .play-btn i { font-size: 20px; }
  .actions-row { display: flex; gap: 28px; margin-top: 10px; }
  .detail-action {
    background: none; border: 0; color: #fff; padding: 0;
    display: flex; flex-direction: column; align-items: center; gap: 6px;
    font-size: 12px;
  }
  .detail-action:disabled { opacity: 0.5; }
  .detail-action i { font-size: 22px; }
  .share-wrap { position: relative; }
  .menu-backdrop { position: fixed; inset: 0; background: transparent; border: 0; z-index: 40; }
  .menu {
    position: absolute; top: calc(100% + 8px); left: 50%; transform: translateX(-50%);
    z-index: 41; min-width: 190px; background: var(--panel-2);
    border: 1px solid var(--border); border-radius: 4px; padding: 6px;
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.5);
  }
  .menu button {
    display: block; width: 100%; text-align: left; background: none; border: 0;
    color: #fff; font-size: 14px; padding: 10px 12px; border-radius: 4px;
  }
  .menu button:hover { background: rgba(255, 255, 255, 0.08); }
  .sec { font-size: 15px; font-weight: 700; margin: 18px 0 6px; }
  .synopsis { margin: 0; line-height: 1.65; color: #e5e5e5; font-size: 15px; }
  .info-row { display: flex; padding: 2px 0; font-size: 15px; }
  .info-label { flex: 0 0 28%; color: var(--muted); }
  .sec-big { font-size: 19px; font-weight: 700; margin: 28px 0 12px; }
  .similar-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
  /* Aturan 4 kolom HARUS sesudah aturan dasar di atas (27 Nov 2026):
     spesifisitasnya sama, jadi yang ditulis belakangan yang menang —
     sebelumnya aturan ini ada di blok media awal dan kalah dengan
     aturan dasar 3 kolom di bawahnya. */
  @media (min-width: 1280px) { .similar-grid { grid-template-columns: repeat(4, 1fr); } }
  /* Dialog kunci episode — pola modal yang sama dengan halaman Nonton. */
  .modal-backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.7); z-index: 70; }
  .modal {
    position: fixed; z-index: 71; left: 50%; top: 50%; transform: translate(-50%, -50%);
    width: min(420px, calc(100vw - 32px));
    background: var(--panel); border: 1px solid var(--border); border-radius: 14px; padding: 20px;
  }
  .modal h2 { margin: 0 0 6px; font-size: 19px; }
  .modal h2 i { color: var(--gold); margin-right: 6px; }
  .modal p { margin: 0 0 12px; font-size: 14px; line-height: 1.55; }
  .modal-actions { display: flex; gap: 10px; margin-top: 14px; flex-wrap: wrap; }
</style>
