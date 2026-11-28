<script>
  import { onMount } from 'svelte';
  import '../app.css';
  import Navbar from '$lib/components/Navbar.svelte';
  import BottomNav from '$lib/components/BottomNav.svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { logout } from '$lib/auth.js';
  import { maybeLoadAds } from '$lib/ads.js';

  // Banner satu kali: login ini menendang perangkat lain (flag
  // ssweb.revoked_device dari auth.js, dibaca tiap navigasi).
  let kickedNotice = '';

  // Token tersimpan mati (kedaluwarsa 30 hari / tidak valid): hapus
  // token, lalu arahkan ke /masuk — di sana One Tap bisa masuk lagi
  // otomatis tanpa klik bila sesi Google di peramban ini masih ada.
  function onSessionExpired() {
    logout();
    goto('/masuk');
  }

  // Sesi dicabut di perangkat ini (login di perangkat lain melewati
  // batas, atau dicabut manual di halaman Perangkat): hapus token,
  // simpan nama perangkat untuk pesan di /masuk, lalu arahkan ke login.
  function onSessionRevoked(e) {
    const name = (e && e.detail) || '';
    logout();
    try {
      sessionStorage.setItem('ssweb.session_revoked', name);
    } catch {
      /* abaikan — mode privat */
    }
    goto('/masuk');
  }

  // Iklan Monetag hanya untuk non-premium (gate di $lib/ads.js) — tag
  // dimuat sekali di sini sehingga berlaku di semua halaman, termasuk
  // Detail (/judul/*) dan Nonton (/nonton/*).
  onMount(() => {
    maybeLoadAds();
    window.addEventListener('session-revoked', onSessionRevoked);
    window.addEventListener('session-expired', onSessionExpired);
    return () => {
      window.removeEventListener('session-revoked', onSessionRevoked);
      window.removeEventListener('session-expired', onSessionExpired);
    };
  });

  // Halaman Detail & Nonton ala aplikasi: layar penuh tanpa navbar
  // atas, footer, maupun navigasi bawah — kembali lewat tombol ←.
  $: path = $page.url.pathname;
  $: immersive = path.startsWith('/judul/') || path.startsWith('/nonton/');

  // Baca flag banner tiap navigasi: tampilkan sekali, lalu hapus.
  $: if (path) {
    try {
      const name = sessionStorage.getItem('ssweb.revoked_device');
      if (name) {
        kickedNotice = name;
        sessionStorage.removeItem('ssweb.revoked_device');
      }
    } catch {
      /* abaikan — mode privat */
    }
  }
</script>

{#if !immersive}
  <Navbar />
{/if}

{#if kickedNotice}
  <div class="container">
    <div class="kicked-banner" role="status">
      <span
        >Perangkat <strong>{kickedNotice}</strong> dikeluarkan karena akun ini login di perangkat
        lain.</span
      >
      <button class="x" aria-label="Tutup pemberitahuan" on:click={() => (kickedNotice = '')}
        >✕</button
      >
    </div>
  </div>
{/if}

<main>
  <slot />
</main>

{#if !immersive}
  <footer>
    <div class="container">
      <div class="foot-logo">SUB<span>SENPAI</span></div>
      <p class="muted">
        Nonton anime, donghua &amp; film subtitle Indonesia. Subtitle dan video diambil dari situs
        sumber masing-masing; SubSenpai tidak menyimpan video di server sendiri.
      </p>
      <p class="muted small">
        Unduh aplikasi Android di
        <a href="/unduh">subsenpai.web.id/unduh</a>
        • Komunitas:
        <a href="https://discord.gg/yJebKEGNh" rel="noreferrer"><i class="fa-brands fa-discord"></i> Discord</a>
        •
        <a href="https://t.me/subsenpaianime" rel="noreferrer"><i class="fa-brands fa-telegram"></i> Telegram</a>
      </p>
    </div>
  </footer>
{/if}

<style>
  main { min-height: calc(100vh - 220px); }
  /* Banner satu kali sesudah login menendang perangkat lain. */
  .kicked-banner {
    display: flex; align-items: center; justify-content: space-between; gap: 12px;
    margin-top: 12px; padding: 10px 14px; border-radius: 4px;
    background: rgba(245, 197, 24, 0.12); border: 1px solid rgba(245, 197, 24, 0.45);
    color: #fde68a; font-size: 14px; line-height: 1.5;
  }
  .kicked-banner strong { color: #fff; }
  .kicked-banner .x {
    flex-shrink: 0; background: transparent; border: 0; color: #fde68a;
    font-size: 16px; cursor: pointer; padding: 2px 6px; line-height: 1;
  }
  footer { border-top: 1px solid var(--border); margin-top: 48px; padding: 28px 0 36px; }
  .foot-logo { color: var(--red); font-weight: 900; font-size: 18px; }
  .foot-logo span { color: #fff; }
  footer p { font-size: 13px; line-height: 1.6; max-width: 760px; }
  .small a { color: #fff; }
  .small i[class*="fa-"] { margin-right: 4px; }
  @media (max-width: 767px) {
    /* Ruang untuk navigasi bawah yang menempel di layar mobile. */
    footer { padding-bottom: 92px; }
  }
</style>

{#if !immersive}
  <BottomNav />
{/if}
