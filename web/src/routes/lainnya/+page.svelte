<script>
  import { onMount } from 'svelte';
  import { user, logout } from '$lib/auth.js';
  import { api } from '$lib/api.js';
  import { goto } from '$app/navigation';

  // Kode Teman: dari profil tersimpan (respons login sudah memuat
  // friend_code); bila profil lama belum punya, ambil via GET /api/v1/me
  // yang mengembalikan {profile: {friend_code, ...}}.
  let friendCode = '';
  let copied = false;
  let copyTimer = null;

  $: if ($user?.friend_code && !friendCode) friendCode = $user.friend_code;

  onMount(async () => {
    if ($user && !$user.friend_code) {
      try {
        const res = await api('/api/v1/me');
        if (res.profile?.friend_code) friendCode = res.profile.friend_code;
      } catch {
        /* abaikan — baris Kode Teman disembunyikan bila gagal dimuat */
      }
    }
  });

  async function copyFriendCode() {
    if (!friendCode) return;
    try {
      await navigator.clipboard.writeText(friendCode);
    } catch {
      window.prompt('Salin kode teman:', friendCode);
      return;
    }
    copied = true;
    clearTimeout(copyTimer);
    copyTimer = setTimeout(() => (copied = false), 1500);
  }
</script>

<div class="container page">
  <h1>Lainnya</h1>

  {#if $user}
    <div class="card profile">
      <strong>{$user.name || $user.email}</strong>
      <span class="muted">{$user.email} • {#if $user.is_premium}<i class="fa-solid fa-crown"></i> Premium{:else}Gratis{/if}</span>
      {#if friendCode}
        <div class="friend-row">
          <span class="muted">Kode Teman</span>
          <code>{friendCode}</code>
          <button class="btn ghost small" on:click={copyFriendCode}>
            <i class="fa-solid fa-copy"></i> {copied ? 'Disalin' : 'Salin'}
          </button>
        </div>
      {/if}
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
    <a href="/premium"><span class="ic"><i class="fa-solid fa-crown"></i></span><span class="lbl">Premium</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a>
    <!-- Tamu diarahkan ke /masuk; pengguna login ke /daftar-saya. -->
    <a href={$user ? '/daftar-saya' : '/masuk'}><span class="ic"><i class="fa-solid fa-bookmark"></i></span><span class="lbl">Daftar Saya</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a>
    <a href={$user ? '/perangkat' : '/masuk'}><span class="ic"><i class="fa-solid fa-mobile-screen"></i></span><span class="lbl">Perangkat</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a>
    <a href="/jadwal"><span class="ic"><i class="fa-solid fa-calendar-days"></i></span><span class="lbl">Jadwal Rilis</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a>
    <a href="/cari"><span class="ic"><i class="fa-solid fa-magnifying-glass"></i></span><span class="lbl">Cari &amp; Genre</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a>
    <a href="https://discord.gg/yJebKEGNh" rel="noreferrer"
      ><span class="ic"><i class="fa-brands fa-discord"></i></span><span class="lbl">Komunitas Discord</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a
    >
    <a href="https://t.me/subsenpaianime" rel="noreferrer"
      ><span class="ic"><i class="fa-brands fa-telegram"></i></span><span class="lbl">Grup Telegram</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a
    >
    <a href="/unduh"
      ><span class="ic"><i class="fa-solid fa-download"></i></span><span class="lbl">Unduh Aplikasi Android</span><span class="chev"><i class="fa-solid fa-chevron-right"></i></span></a
    >
  </div>

  <p class="muted small">SubSenpai Web — nonton anime, donghua &amp; film subtitle Indonesia.</p>
</div>

<style>
  /* Kolom menu ala pengaturan (27 Nov 2026): dibatasi 720px di desktop
     supaya baris menu tidak meregang selebar layar — sama seperti Riwayat. */
  .page { padding-top: 24px; max-width: 720px; }
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
  .ic .fa-crown { color: var(--gold); }
  .ic .fa-bookmark { color: var(--red); }
  .ic .fa-mobile-screen { color: #34d399; }
  .ic .fa-discord { color: #5865f2; }
  .ic .fa-telegram { color: #229ed9; }
  .friend-row { display: flex; align-items: center; gap: 8px; margin-top: 4px; flex-wrap: wrap; }
  .friend-row code {
    background: rgba(255,255,255,.08); padding: 2px 8px; border-radius: 4px;
    font-size: 14px; letter-spacing: 1px;
  }
  .btn.small { padding: 6px 12px; font-size: 13px; }
  .profile .fa-crown { color: var(--gold); }
  .lbl { flex: 1; }
  .chev { color: var(--muted); font-size: 14px; }
  .small { font-size: 13px; }
</style>
