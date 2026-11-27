<script>
  import { user, logout } from '$lib/auth.js';
  import { goto } from '$app/navigation';

  let q = '';
  let showMenu = false;

  function search(e) {
    e.preventDefault();
    goto('/cari' + (q.trim() ? '?q=' + encodeURIComponent(q.trim()) : ''));
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
    </nav>
    <form class="search" on:submit={search}>
      <input type="search" placeholder="Cari judul…" bind:value={q} aria-label="Cari judul" />
    </form>
    <a class="icon-btn search-ic" href="/cari" aria-label="Cari judul">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15.5 14h-.79l-.28-.27a6.5 6.5 0 1 0-.7.7l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0A4.5 4.5 0 1 1 14 9.5 4.5 4.5 0 0 1 9.5 14z" /></svg>
    </a>
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
              <span class="muted">{$user.is_premium ? '👑 Premium' : 'Gratis'}</span>
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
  @media (max-width: 767px) {
    /* Mobile: tautan & kolom cari pindah ke navigasi bawah (BottomNav);
       yang tersisa di bar atas hanya logo, ikon cari, dan akun. */
    .links { display: none; }
    .search { display: none; }
    .search-ic {
      display: flex; align-items: center; justify-content: center;
      margin-left: auto; width: 36px; height: 36px; color: #fff;
    }
    .search-ic svg { width: 24px; height: 24px; fill: currentColor; }
  }
</style>
