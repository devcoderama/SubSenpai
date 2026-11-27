<script>
  import { tick } from 'svelte';
  import { user, logout } from '$lib/auth.js';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { searchOpen, searchQuery, closeSearch, hideSearchRow, toggleSearch } from '$lib/search.js';

  let showMenu = false;

  // Cakupan pencarian kontekstual (27 Nov 2026): di 4 halaman daftar,
  // tombol cari memfilter halaman itu sendiri; di halaman lain, cari
  // tetap membuka /cari seperti dulu.
  $: path = $page.url.pathname;
  $: scope =
    path === '/'
      ? 'beranda'
      : path === '/post'
        ? 'post'
        : path === '/jadwal'
          ? 'jadwal'
          : path === '/riwayat'
            ? 'riwayat'
            : null;
  $: placeholder =
    scope === 'beranda'
      ? 'Cari judul di Beranda…'
      : scope === 'post'
        ? 'Cari postingan…'
        : scope === 'jadwal'
          ? 'Cari jadwal…'
          : scope === 'riwayat'
            ? 'Cari riwayat…'
            : 'Cari judul…';

  // Pindah halaman -> bersihkan pencarian supaya halaman baru tampil normal.
  let lastPath = '';
  $: if (path !== lastPath) {
    lastPath = path;
    closeSearch();
  }

  // Enter di kolom cari navbar:
  // - halaman kontekstual: hasil sudah difilter live saat mengetik — Enter
  //   hanya menutup bilah (kata kunci tetap, hasil tetap terfilter);
  // - halaman lain: buka /cari dengan kata kunci itu.
  function search(e) {
    e.preventDefault();
    const q = ($searchQuery || '').trim();
    if (scope) {
      hideSearchRow();
      if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
      return;
    }
    goto('/cari' + (q ? '?q=' + encodeURIComponent(q) : ''));
  }

  // Ikon kaca pembesar (mobile): di /cari fokus ke kolom cari halaman itu
  // (sudah ada di sana); di halaman lain buka/tutup bilah cari navbar.
  async function onSearchIcon() {
    if (path === '/cari') {
      document.getElementById('cari-input')?.focus();
      window.scrollTo({ top: 0 });
      return;
    }
    toggleSearch();
    if ($searchOpen) {
      await tick();
      document.getElementById('nav-search-input')?.focus();
    }
  }
</script>

<header class="nav">
  <div class="container nav-inner">
    <a href="/" class="logo">SUB<span>SENPAI</span></a>
    <nav class="links">
      <a href="/">Beranda</a>
      <a href="/jadwal">Jadwal Rilis</a>
      <a href="/cari">Genre</a>
      <a href="/premium">Premium</a>
      <a href="/unduh">Unduh</a>
    </nav>
    <form class="search" on:submit={search}>
      <input
        type="search"
        placeholder={placeholder}
        bind:value={$searchQuery}
        aria-label="Cari"
      />
    </form>
    <button class="icon-btn search-ic" on:click={onSearchIcon} aria-label="Cari">
      <i class="fa-solid fa-magnifying-glass"></i>
    </button>
    {#if $user}
      <div class="userbox">
        <button class="avatar" on:click={() => (showMenu = !showMenu)} aria-label="Menu pengguna">
          {#if $user.avatar_url}
            <img src={$user.avatar_url} alt="" />
          {:else}
            <span>{$user.name ? $user.name[0].toUpperCase() : 'S'}</span>
          {/if}
        </button>
        {#if showMenu}
          <div class="menu">
            <div class="menu-head">
              <strong>{$user.name || $user.email}</strong>
              <span class="muted">{#if $user.is_premium}<i class="fa-solid fa-crown"></i> Premium{:else}Gratis{/if}</span>
            </div>
            <a href="/premium" on:click={() => (showMenu = false)}>Premium</a>
            <button
              on:click={() => {
                logout();
                showMenu = false;
                goto('/');
              }}>Keluar</button
            >
          </div>
        {/if}
      </div>
    {:else}
      <a href="/masuk" class="btn">Masuk</a>
    {/if}
  </div>
  {#if $searchOpen}
    <div class="container search-row">
      <form on:submit={search}>
        <i class="fa-solid fa-magnifying-glass"></i>
        <input
          id="nav-search-input"
          type="search"
          placeholder={placeholder}
          bind:value={$searchQuery}
          aria-label="Cari"
        />
        {#if $searchQuery}
          <button
            type="button"
            class="row-clear"
            on:click={() => searchQuery.set('')}
            aria-label="Bersihkan pencarian"><i class="fa-solid fa-xmark"></i></button
          >
        {/if}
      </form>
      <button class="row-cancel" on:click={closeSearch}>Batal</button>
    </div>
  {/if}
</header>

<style>
  .nav {
    position: sticky; top: 0; z-index: 50;
    background: linear-gradient(to bottom, rgba(0,0,0,.9), rgba(20,20,20,.85));
    backdrop-filter: blur(8px);
    border-bottom: 1px solid rgba(255,255,255,.08);
  }
  .nav-inner {
    display: flex; align-items: center; gap: 20px;
    height: 60px;
  }
  .logo {
    font-size: 22px; font-weight: 900; letter-spacing: .5px;
    color: var(--red); white-space: nowrap;
  }
  .logo span { color: #fff; }
  .logo:hover { text-decoration: none; }
  .links { display: flex; gap: 18px; overflow-x: auto; scrollbar-width: none; }
  .links a { font-size: 14px; color: #e5e5e5; white-space: nowrap; }
  .links a:hover { color: #fff; }
  .search { margin-left: auto; }
  .search input {
    background: rgba(0,0,0,.6); border: 1px solid #555; color: #fff;
    border-radius: 6px; padding: 8px 12px; font-size: 14px; width: 180px;
  }
  .search input:focus { outline: none; border-color: #fff; }
  .userbox { position: relative; }
  .avatar {
    width: 34px; height: 34px; border-radius: 6px; overflow: hidden;
    border: 0; padding: 0; background: var(--red);
    color: #fff; font-weight: 700; font-size: 16px;
  }
  .avatar img { width: 100%; height: 100%; object-fit: cover; }
  .menu {
    position: absolute; right: 0; top: 44px; min-width: 220px;
    background: var(--panel); border: 1px solid var(--border);
    border-radius: 8px; padding: 8px; box-shadow: 0 8px 24px rgba(0,0,0,.6);
  }
  .menu-head {
    display: flex; flex-direction: column; gap: 2px;
    padding: 8px; border-bottom: 1px solid var(--border); margin-bottom: 4px;
    font-size: 13px;
  }
  .menu a, .menu button {
    display: block; width: 100%; text-align: left;
    padding: 8px; background: none; border: 0; color: #fff; font-size: 14px;
  }
  .menu a:hover, .menu button:hover { background: var(--panel-2); text-decoration: none; }
  .search-ic { display: none; }
  /* Bilah cari yang muncul saat ikon kaca pembesar diketuk (27 Nov 2026). */
  .search-row {
    display: flex; align-items: center; gap: 10px;
    padding-top: 0; padding-bottom: 10px;
  }
  .search-row form {
    flex: 1; display: flex; align-items: center; gap: 8px;
    background: rgba(0,0,0,.6); border: 1px solid #555; border-radius: 8px;
    padding: 0 12px;
  }
  .search-row form:focus-within { border-color: #fff; }
  .search-row form i { color: var(--muted); font-size: 15px; }
  .search-row input {
    flex: 1; min-width: 0; background: none; border: 0; color: #fff;
    padding: 10px 0; font-size: 15px;
  }
  .search-row input:focus { outline: none; }
  .row-clear { background: none; border: 0; color: var(--muted); font-size: 15px; padding: 4px; }
  .row-cancel { background: none; border: 0; color: #fff; font-size: 14px; padding: 6px 2px; }
  @media (max-width: 767px) {
    /* Mobile: tautan & kolom cari pindah ke navigasi bawah (BottomNav);
       yang tersisa di bar atas hanya logo, ikon cari, dan akun. */
    .links { display: none; }
    .search { display: none; }
    .search-ic {
      display: flex; align-items: center; justify-content: center;
      margin-left: auto; width: 36px; height: 36px; color: #fff;
    }
    .search-ic i { font-size: 22px; }
  }
  .menu-head .fa-crown { color: var(--gold); }
</style>
