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
    <i class="fa-solid fa-house"></i>
    <span>Beranda</span>
  </a>
  <a href="/post" class:active={active === 'post'} aria-current={active === 'post' ? 'page' : undefined}>
    <i class="fa-solid fa-message"></i>
    <span>Post</span>
  </a>
  <a href="/jadwal" class:active={active === 'jadwal'} aria-current={active === 'jadwal' ? 'page' : undefined}>
    <i class="fa-solid fa-calendar-days"></i>
    <span>Jadwal</span>
  </a>
  <a href="/riwayat" class:active={active === 'riwayat'} aria-current={active === 'riwayat' ? 'page' : undefined}>
    <i class="fa-solid fa-clock-rotate-left"></i>
    <span>Riwayat</span>
  </a>
  <a href="/lainnya" class:active={active === 'lainnya'} aria-current={active === 'lainnya' ? 'page' : undefined}>
    <i class="fa-solid fa-bars"></i>
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
    .bottomnav a i {
      font-size: 22px;
    }
    .bottomnav a.active {
      color: #fff;
    }
    .bottomnav a.active i {
      color: var(--red);
    }
  }
</style>
