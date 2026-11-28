<script>
  import { posterUrl } from '$lib/api.js';

  export let item;
  export let firstEpisodeId = null;
  // Toggle Daftar Saya (28 Nov 2026, permintaan Rams): tombol hidup di
  // baris aksi, di kiri tombol Nonton — bukan lagi overlay kanan atas
  // (posisi lama menutupi backdrop dan jauh dari Nonton/Detail).
  // onToggleWatchlist null = tombol tak tampil (dipakai halaman lain).
  export let inWatchlist = false;
  export let wlBusy = false;
  export let onToggleWatchlist = null;
  // Hero menjorok ke bawah navbar transparan (28 Nov 2026): padding
  // atas ditambah setinggi navbar + baris chip supaya teks tetap di
  // tempat yang sama, sementara backdrop naik ke balik navbar.
  export let underNav = false;

  $: typeLabel = item.type === 'donghua' ? 'Donghua' : item.type === 'anime' ? 'Anime' : 'Film';
</script>

<section class="hero">
  <div class="backdrop">
    {#if item.poster_url}
      <img src={posterUrl(item.poster_url)} alt="" />
    {/if}
    <div class="shade"></div>
  </div>
  <div class="container hero-body" class:under-nav={underNav}>
    <div class="eyebrow">SUBSENPAI • {typeLabel.toUpperCase()}</div>
    <h1>{item.title}</h1>
    <div class="alt">{item.title_alt || ''}</div>
    <div class="meta">
      <span class="meta-text">
        {#if item.rating}<span class="rate"><i class="fa-solid fa-star"></i> {Number(item.rating).toFixed(1)}</span>{/if}
        {item.year || ''}
        {#if item.status}• {item.status}{/if}
        {#if item.genres?.length}• {item.genres.join(' • ')}{/if}
      </span>
    </div>
    <p class="synopsis">{item.synopsis || ''}</p>
    <div class="actions">
      {#if onToggleWatchlist}
        <button
          class="btn ghost hero-wl"
          class:active={inWatchlist}
          on:click={onToggleWatchlist}
          disabled={wlBusy}
          aria-pressed={inWatchlist}
          aria-label={inWatchlist ? 'Hapus dari Daftar Saya' : 'Simpan ke Daftar Saya'}
        >
          <i class={inWatchlist ? 'fa-solid fa-bookmark' : 'fa-regular fa-bookmark'}></i>
          {wlBusy ? 'Menyimpan…' : inWatchlist ? 'Tersimpan' : 'Daftar Saya'}
        </button>
      {/if}
      {#if firstEpisodeId}
        <a class="btn" href={'/nonton/' + firstEpisodeId}><i class="fa-solid fa-play"></i> Nonton</a>
      {/if}
      <a class="btn ghost" href={'/judul/' + item.id}><i class="fa-solid fa-circle-info"></i> Detail</a>
    </div>
  </div>
</section>

<style>
  .hero { position: relative; margin-bottom: 8px; overflow: hidden; }
  .backdrop { position: absolute; inset: 0; }
  .backdrop img {
    width: 100%; height: 100%; object-fit: cover; object-position: center 20%;
    filter: blur(2px) brightness(.55); transform: scale(1.05);
  }
  .shade {
    position: absolute; inset: 0;
    background: linear-gradient(to right, rgba(20,20,20,.92) 0%, rgba(20,20,20,.55) 55%, rgba(20,20,20,.15) 100%),
      linear-gradient(to top, var(--bg) 0%, transparent 40%);
  }
  /* Teks hero rata dengan garis pinggir konten (27 Nov 2026): .container
     di atas sudah memberi padding 4vw di desktop, jadi tidak perlu lagi
     trik margin-left yang mengikuti lebar 1280px lama. */
  .hero-body { position: relative; padding-top: 72px; padding-bottom: 64px; max-width: 640px; margin-left: 0; }
  .hero-body.under-nav { padding-top: 192px; }
  .eyebrow { color: var(--red); font-weight: 800; letter-spacing: 2px; font-size: 13px; }
  /* Ukuran hero SAMA di semua slide (28 Nov 2026, permintaan Rams):
     tiap blok teks punya tinggi tetap — judul 2 baris, judul lain 1,
     meta 1, sinopsis 3 — jadi ganti slide tidak mengubah tinggi hero
     dan halaman di bawahnya tidak meloncat. Blok kosong/short tetap
     memakan tempat (min-height), bukan runtuh. */
  h1 {
    margin: 10px 0 4px; font-size: clamp(28px, 5vw, 48px); line-height: 1.05;
    min-height: calc(2 * 1.05em);
    display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2;
    -webkit-box-orient: vertical; overflow: hidden;
  }
  .alt {
    color: var(--muted); font-size: 15px; min-height: 1.3em;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .meta { margin-top: 12px; font-size: 14px; color: #ddd; min-height: 1.4em; }
  .meta-text { display: block; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .rate { color: var(--gold); font-weight: 700; }
  .synopsis {
    margin: 14px 0 20px; font-size: 15px; line-height: 1.55; color: #e5e5e5;
    min-height: calc(3 * 1.55em);
    display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3;
    -webkit-box-orient: vertical; overflow: hidden;
  }
  /* Tiga tombol sebaris di desktop; di HP sempit boleh membungkus ke
     baris berikut daripada terpotong. */
  .actions { display: flex; gap: 10px; flex-wrap: wrap; }
  .hero-wl.active { background: var(--red); }
  .hero-wl.active:hover { background: var(--red-dark); }
</style>
