<script>
  import { page } from '$app/stores';

  $: is404 = $page.status === 404;
</script>

<svelte:head><title>{$page.status} — SubSenpai</title></svelte:head>

<div class="container error-page">
  <div class="code">{$page.status}</div>
  <h1>{is404 ? 'Halaman tidak ditemukan' : 'Terjadi kesalahan'}</h1>
  <p class="muted">
    {#if is404}
      Tautan yang kamu buka sudah tidak ada atau salah ketik.
    {:else}
      {$page.error?.message || 'Coba lagi nanti.'}
    {/if}
  </p>
  <div class="actions">
    <a class="btn" href="/">← Beranda</a>
    <a class="btn ghost" href="/cari">Cari judul</a>
  </div>
</div>

<style>
  .error-page {
    max-width: 560px; padding-top: 72px; padding-bottom: 72px;
    text-align: center;
  }
  .code {
    font-size: 88px; font-weight: 900; line-height: 1;
    color: transparent; -webkit-text-stroke: 2px var(--red);
    letter-spacing: 2px;
  }
  h1 { font-size: 24px; margin: 16px 0 8px; }
  p { font-size: 15px; margin: 0 0 24px; }
  .actions { display: flex; gap: 12px; justify-content: center; flex-wrap: wrap; }
</style>
