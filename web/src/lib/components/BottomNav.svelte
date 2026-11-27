<script>
  import { page } from '$app/stores';

  // Tab aktif mengikuti rute — sama seperti bottom nav aplikasi.
  $: path = $page.url.pathname;
  $: active = path === '/'
    ? 'beranda'
    : path.startsWith('/jadwal')
      ? 'jadwal'
      : path.startsWith('/cari')
        ? 'cari'
        : path.startsWith('/premium')
          ? 'premium'
          : path.startsWith('/lainnya')
            ? 'lainnya'
            : '';
</script>

<nav class="bottomnav" aria-label="Navigasi utama">
  <a href="/" class:active={active === 'beranda'} aria-current={active === 'beranda' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3l9 8h-3v9h-5v-6h-2v6H6v-9H3l9-8z" /></svg>
    <span>Beranda</span>
  </a>
  <a href="/jadwal" class:active={active === 'jadwal'} aria-current={active === 'jadwal' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 2v2H5a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2h-2V2h-2v2H9V2H7zm12 8v9H5v-9h14z" /></svg>
    <span>Jadwal</span>
  </a>
  <a href="/cari" class:active={active === 'cari'} aria-current={active === 'cari' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15.5 14h-.79l-.28-.27a6.5 6.5 0 1 0-.7.7l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0A4.5 4.5 0 1 1 14 9.5 4.5 4.5 0 0 1 9.5 14z" /></svg>
    <span>Cari</span>
  </a>
  <a href="/premium" class:active={active === 'premium'} aria-current={active === 'premium' ? 'page' : undefined}>
    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 2 9.5 6.5 4 5l1.5 12h13L20 5l-5.5 1.5L12 2zm-2.5 12.5v-2h5v2h-5z" /></svg>
    <span>Premium</span>
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
