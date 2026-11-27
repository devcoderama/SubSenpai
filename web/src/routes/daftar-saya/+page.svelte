<script>
  import { onMount } from 'svelte';
  import { user } from '$lib/auth.js';
  import { getWatchlist, removeWatchlist } from '$lib/api.js';
  import TitleCard from '$lib/components/TitleCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal } from '$lib/reveal.js';

  let items = [];
  let loading = true;
  let error = '';
  let removeError = '';
  let removingId = null;

  async function load() {
    // Tamu: jangan panggil API watchlist — tampilkan kartu ajakan login.
    if (!$user) {
      loading = false;
      return;
    }
    loading = true;
    error = '';
    try {
      // GET /api/v1/watchlist -> {items: [{id, slug, type, title,
      // poster_url, year, status, genres, rating, ...}]} (queryCards).
      const res = await getWatchlist();
      items = res.items || [];
    } catch (e) {
      error = e.message || 'Gagal memuat Daftar Saya.';
    } finally {
      loading = false;
    }
  }

  async function removeItem(id) {
    if (removingId) return;
    removingId = id;
    removeError = '';
    try {
      await removeWatchlist(id);
      // Refresh daftar dari server sesudah hapus.
      const res = await getWatchlist();
      items = res.items || [];
    } catch (e) {
      removeError = e.message || 'Gagal menghapus dari Daftar Saya.';
    } finally {
      removingId = null;
    }
  }

  onMount(load);
</script>

<svelte:head><title>Daftar Saya — SubSenpai</title></svelte:head>

<div class="container page">
  <h1>Daftar Saya</h1>

  {#if loading}
    <div class="poster-grid"><Skeleton kind="card" count={8} /></div>
  {:else if !$user}
    <div class="card empty">
      <strong>Masuk untuk melihat Daftar Saya.</strong>
      <span class="muted">Simpan judul favoritmu dan lanjutkan di semua perangkat.</span>
      <div class="row"><a class="btn" href="/masuk">Masuk dengan Google</a></div>
    </div>
  {:else if error}
    <p class="error">{error}</p>
    <button class="btn" on:click={load}>Coba Lagi</button>
  {:else if !items.length}
    <div class="card empty">
      <strong>Daftar Saya masih kosong.</strong>
      <span class="muted">Ketuk ikon bookmark di Detail atau hero Beranda untuk menyimpan judul.</span>
    </div>
  {:else}
    {#if removeError}<p class="error">{removeError}</p>{/if}
    <div class="poster-grid">
      {#each items as item, i (item.id)}
        <div class="cell" use:reveal={(i % 12) * 45}>
          <TitleCard {item} />
          <button
            class="del"
            aria-label="Hapus dari Daftar Saya"
            disabled={removingId === item.id}
            on:click={() => removeItem(item.id)}
            ><i class="fa-solid fa-trash"></i></button
          >
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page { padding-top: 24px; }
  .page h1 { font-size: 26px; margin-bottom: 16px; }
  .empty { display: flex; flex-direction: column; gap: 8px; }
  .row { display: flex; gap: 12px; align-items: center; margin-top: 4px; flex-wrap: wrap; }
  /* Tombol hapus menumpang di sudut kartu (di luar <a> TitleCard
     supaya ketuk hapus tidak ikut menavigasi ke /judul/{id}). */
  .cell { position: relative; }
  .del {
    position: absolute; top: 8px; right: 8px; z-index: 2;
    width: 32px; height: 32px; border-radius: 50%; border: 0; cursor: pointer;
    background: rgba(0,0,0,.7); color: #fff; font-size: 13px;
    display: flex; align-items: center; justify-content: center;
  }
  .del:hover { background: var(--red); }
  .del:disabled { opacity: .5; cursor: default; }
</style>
