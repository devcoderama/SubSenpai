<script>
  import { onMount } from 'svelte';
  import '../app.css';
  import Navbar from '$lib/components/Navbar.svelte';
  import BottomNav from '$lib/components/BottomNav.svelte';
  import { page } from '$app/stores';
  import { maybeLoadAds } from '$lib/ads.js';

  // Iklan Monetag hanya untuk non-premium (gate di $lib/ads.js) — tag
  // dimuat sekali di sini sehingga berlaku di semua halaman, termasuk
  // Detail (/judul/*) dan Nonton (/nonton/*).
  onMount(() => {
    maybeLoadAds();
  });

  // Halaman Detail & Nonton ala aplikasi: layar penuh tanpa navbar
  // atas, footer, maupun navigasi bawah — kembali lewat tombol ←.
  $: path = $page.url.pathname;
  $: immersive = path.startsWith('/judul/') || path.startsWith('/nonton/');
</script>

{#if !immersive}
  <Navbar />
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
        <a href="https://discord.gg/yJebKEGNh" target="_blank" rel="noreferrer"><i class="fa-brands fa-discord"></i> Discord</a>
        •
        <a href="https://t.me/subsenpaianime" target="_blank" rel="noreferrer"><i class="fa-brands fa-telegram"></i> Telegram</a>
      </p>
    </div>
  </footer>
{/if}

<style>
  main { min-height: calc(100vh - 220px); }
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
