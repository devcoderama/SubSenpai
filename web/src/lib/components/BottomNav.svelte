<script>
  import { page } from '$app/stores';

  // Tab aktif mengikuti rute — sama persis dengan bottom nav aplikasi.
  $: path = $page.url.pathname;
  $: active = path === '/'
    ? 'beranda'
    : path.startsWith('/post')
      ? 'post'
      : path.startsWith('/jadwal')
        ? 'jadwal'
        : path.startsWith('/riwayat')
          ? 'riwayat'
          : path.startsWith('/lainnya')
            ? 'lainnya'
            : '';
</script>

<nav class="bottomnav" aria-label="Navigasi utama">
  <a href="/" class:active={active === 'beranda'} aria-current={active === 'beranda' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3l9 8h-3v9h-5v-6h-2v6H6v-9H3l9-8z" /></svg>
    <span>Beranda</span>
  </a>
  <a href="/post" class:active={active === 'post'} aria-current={active === 'post' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 2H4a2 2 0 0 0-2 2v18l4-4h14a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2z" /></svg>
    <span>Post</span>
  </a>
  <a href="/jadwal" class:active={active === 'jadwal'} aria-current={active === 'jadwal' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 2v2H5a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2h-2V2h-2v2H9V2H7zm12 8v9H5v-9h14z" /></svg>
    <span>Jadwal</span>
  </a>
  <a href="/riwayat" class:active={active === 'riwayat'} aria-current={active === 'riwayat' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M13 3a9 9 0 0 0-9 9H3l3.5 3.5L10 12H7a7 7 0 1 1 2.2 5.1l-1.5 1.3A9 9 0 1 0 13 3zm-1 5v5l4.2 2.5.8-1.2-3.5-2.1V8H12z" /></svg>
    <span>Riwayat</span>
  </a>
  <a href="/lainnya" class:active={active === 'lainnya'} aria-current={active === 'lainnya' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16v2H4zm0 5h16v2H4zm0 5h16v2H4z" /></svg>
    <span>Lainnya</span>
  </a>
</nav>

<style>
  /* Desktop: navigasi tetap di navbar atas — bar bawah hanya untuk mobile. */
  .bottomnav {
    display: none;
  }
  @media (max-width: 767px) {
    .bottomnav {
      display: flex;
      position: fixed;
      left: 0;
      right: 0;
      bottom: 0;
      z-index: 60;
      background: rgba(18, 18, 18, 0.97);
      backdrop-filter: blur(10px);
      border-top: 1px solid rgba(255, 255, 255, 0.12);
      padding-bottom: env(safe-area-inset-bottom);
    }
    .bottomnav a {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: 3px;
      padding: 8px 0 7px;
      color: #b3b3b3;
      font-size: 11px;
      font-weight: 600;
      line-height: 1;
    }
    .bottomnav a:hover {
      text-decoration: none;
      color: #fff;
    }
    .bottomnav a svg {
      width: 24px;
      height: 24px;
      fill: currentColor;
    }
    .bottomnav a.active {
      color: #fff;
    }
    .bottomnav a.active svg {
      fill: var(--red);
    }
  }
</style>
