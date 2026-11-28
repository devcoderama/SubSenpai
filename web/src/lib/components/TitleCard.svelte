<script>
  import { goto } from '$app/navigation';
  import { posterUrl, getTitle } from '$lib/api.js';
  import { wlMap, wlPending, ensureWl, toggleWl } from '$lib/watchlist.js';
  import { user } from '$lib/auth.js';
  import { toastError } from '$lib/toast.js';

  export let item;
  export let href = null;
  export let sub = null;

  $: link = href || '/judul/' + item.id;
  $: detailHref = '/judul/' + item.id;
  $: typeLabel = item.type === 'donghua' ? 'Donghua' : item.type === 'anime' ? 'Anime' : 'Film';

  // Status Daftar Saya mengikuti store bersama ($lib/watchlist.js):
  // satu judul di beberapa rel memakai state yang sama — toggle di satu
  // kartu langsung tercermin di duplikatnya (28 Nov 2026). Tamu melihat
  // tombol sebagai belum tersimpan (tombol -> /masuk).
  $: wlState = $user ? $wlMap[item.id] : undefined;
  $: wlBusy = !!$wlPending[item.id];
  let playBusy = false;

  async function toggleWatchlist() {
    if (!$user) { goto('/masuk'); return; }
    if (wlBusy) return;
    try {
      await toggleWl(item.id);
    } catch {
      // Gagal: store tidak berubah.
      toastError('Gagal mengubah Daftar Saya. Coba lagi.');
    }
  }

  // Putar (mirip tombol Putar di Detail): href khusus (Lanjutkan
  // Menonton) -> episode lanjutannya; kalau tidak, episode pertama judul
  // ini. Terkunci -> buka Detail (dialog kunci + iklan/premium ada di sana).
  async function playFirst() {
    if (playBusy) return;
    if (href) { goto(href); return; }
    playBusy = true;
    try {
      const d = await getTitle(item.id);
      const eps = d?.episodes || [];
      if (!eps.length || eps[0].locked) { goto(detailHref); return; }
      goto('/nonton/' + eps[0].id);
    } catch {
      goto(detailHref);
    } finally {
      playBusy = false;
    }
  }
</script>

<div class="card">
  <div class="poster-wrap">
    <a class="poster" href={link} on:mouseenter={() => ensureWl(item.id)}>
      {#if item.poster_url}
        <img src={posterUrl(item.poster_url)} alt={item.title} loading="lazy" />
      {:else}
        <div class="no-poster">SUBSENPAI</div>
      {/if}
      {#if item.rating}
        <span class="rating"><i class="fa-solid fa-star"></i> {Number(item.rating).toFixed(1)}</span>
      {/if}
    </a>
    <!-- Aksi hover (desktop saja): tombol hidup di ATAS poster tapi di
         luar <a> supaya klik tombol tidak ikut menavigasi; klik poster
         di luar tombol tetap tembus ke tautan via pointer-events. -->
    <div class="hover-actions">
      <button class="play" on:click={playFirst} disabled={playBusy} aria-label={'Putar ' + item.title}>
        <i class={playBusy ? 'fa-solid fa-circle-notch fa-spin' : 'fa-solid fa-play'}></i>
      </button>
      <div class="hover-row">
        <button
          class="mini"
          class:active={wlState}
          on:click={toggleWatchlist}
          disabled={wlBusy}
          aria-pressed={wlState}
          aria-label={wlState ? 'Hapus dari Daftar Saya' : 'Simpan ke Daftar Saya'}
        >
          <i class={wlState ? 'fa-solid fa-bookmark' : 'fa-regular fa-bookmark'}></i>
        </button>
        <a class="mini" href={detailHref} aria-label={'Detail ' + item.title}>
          <i class="fa-solid fa-circle-info"></i>
        </a>
      </div>
    </div>
  </div>
  <div class="meta">
    <a class="title" href={link}>{item.title}</a>
    <div class="sub muted">
      {#if sub}{sub}{:else}{typeLabel}{item.year ? ' • ' + item.year : ''}{item.status ? ' • ' + item.status : ''}{/if}
    </div>
    {#if item.genres?.length}
      <div class="genres muted">{item.genres.slice(0, 3).join(' • ')}</div>
    {/if}
  </div>
</div>

<style>
  .card {
    display: block; padding: 0; overflow: hidden; position: relative;
    transition: transform .15s, box-shadow .15s;
  }
  .card:hover {
    transform: scale(1.05); text-decoration: none; border-color: #555;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5); z-index: 5;
  }
  .poster-wrap { position: relative; }
  .poster {
    display: block; position: relative; aspect-ratio: 2 / 3;
    background: #000; overflow: hidden;
  }
  .poster img { width: 100%; height: 100%; object-fit: cover; }
  .no-poster {
    width: 100%; height: 100%; display: flex; align-items: center; justify-content: center;
    color: var(--red); font-weight: 900; font-size: 13px; letter-spacing: 1px;
  }
  .rating {
    position: absolute; top: 6px; left: 6px;
    background: rgba(0,0,0,.75); color: var(--gold);
    font-size: 12px; font-weight: 700; padding: 2px 6px; border-radius: 4px;
  }
  /* Panel aksi hover: hanya perangkat ber-hover (desktop). Di layar
     sentuh panel tidak pernah tampil (tap tetap -> Detail seperti biasa). */
  .hover-actions {
    position: absolute; inset: 0; z-index: 2;
    display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 14px;
    background: linear-gradient(rgba(0,0,0,.15), rgba(0,0,0,.55) 55%, rgba(0,0,0,.75));
    opacity: 0; transition: opacity .15s; pointer-events: none;
  }
  @media (hover: hover) {
    .card:hover .hover-actions { opacity: 1; }
  }
  @media (hover: none) {
    .hover-actions { display: none; }
  }
  .hover-actions button, .hover-actions a { pointer-events: auto; }
  .play {
    width: 52px; height: 52px; border-radius: 50%; border: 0; cursor: pointer;
    background: #fff; color: #000; font-size: 20px;
    display: flex; align-items: center; justify-content: center;
    box-shadow: 0 4px 16px rgba(0,0,0,.5); transition: transform .15s;
  }
  .play:hover { transform: scale(1.08); }
  .play:disabled { opacity: .7; cursor: default; }
  .play .fa-play { margin-left: 2px; }
  .hover-row { display: flex; gap: 10px; }
  .mini {
    width: 38px; height: 38px; border-radius: 50%; border: 1px solid rgba(255,255,255,.6);
    background: rgba(0,0,0,.55); color: #fff; font-size: 15px; cursor: pointer;
    display: inline-flex; align-items: center; justify-content: center; text-decoration: none;
  }
  .mini:hover { background: rgba(255,255,255,.25); text-decoration: none; }
  .mini.active { background: var(--red); border-color: var(--red); }
  .mini:disabled { opacity: .6; cursor: default; }
  .meta { padding: 10px; }
  .title {
    color: inherit; text-decoration: none;
    font-size: 14px; font-weight: 600; line-height: 1.3;
    display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
  }
  .title:hover { text-decoration: none; }
  .sub { font-size: 12px; margin-top: 4px; }
  .genres {
    font-size: 11px; margin-top: 2px;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
</style>
