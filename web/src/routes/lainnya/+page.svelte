<script>
  import { user, logout } from '$lib/auth.js';
  import { goto } from '$app/navigation';
</script>

<div class="container page">
  <h1>Lainnya</h1>

  {#if $user}
    <div class="card profile">
      <strong>{$user.name || $user.email}</strong>
      <span class="muted">{$user.email} • {$user.is_premium ? '👑 Premium' : 'Gratis'}</span>
      <div class="row">
        <a class="btn" href="/premium">Premium</a>
        <button
          class="btn ghost"
          on:click={() => {
            logout();
            goto('/');
          }}>Keluar</button
        >
      </div>
    </div>
  {:else}
    <div class="card profile">
      <strong>Belum masuk</strong>
      <span class="muted">Masuk untuk Premium, voucher, dan lanjut menonton di semua perangkat.</span>
      <div class="row">
        <a class="btn" href="/masuk">Masuk dengan Google</a>
      </div>
    </div>
  {/if}

  <div class="menu">
    <a href="/premium"><span class="ic">👑</span><span class="lbl">Premium</span><span class="chev">›</span></a>
    <a href="/jadwal"><span class="ic">🗓️</span><span class="lbl">Jadwal Rilis</span><span class="chev">›</span></a>
    <a href="/cari"><span class="ic">🔍</span><span class="lbl">Cari &amp; Genre</span><span class="chev">›</span></a>
    <a href="https://discord.gg/yJebKEGNh" target="_blank" rel="noreferrer"
      ><span class="ic">💬</span><span class="lbl">Komunitas Discord</span><span class="chev">›</span></a
    >
    <a href="https://t.me/subsenpaianime" target="_blank" rel="noreferrer"
      ><span class="ic">✈️</span><span class="lbl">Grup Telegram</span><span class="chev">›</span></a
    >
    <a href="/unduh"
      ><span class="ic">📱</span><span class="lbl">Unduh Aplikasi Android</span><span class="chev">›</span></a
    >
  </div>

  <p class="muted small">SubSenpai Web — nonton anime, donghua &amp; film subtitle Indonesia.</p>
</div>

<style>
  .page { padding-top: 24px; }
  .page h1 { font-size: 26px; margin-bottom: 16px; }
  .profile {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 16px;
  }
  .menu {
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 10px;
    overflow: hidden;
    margin-bottom: 16px;
  }
  .menu a {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 14px 16px;
    color: #fff;
    font-size: 15px;
    border-bottom: 1px solid var(--border);
  }
  .menu a:last-child { border-bottom: 0; }
  .menu a:hover { background: var(--panel-2); text-decoration: none; }
  .ic { width: 24px; text-align: center; }
  .lbl { flex: 1; }
  .chev { color: var(--muted); font-size: 18px; }
  .small { font-size: 13px; }
</style>
