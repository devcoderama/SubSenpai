<script>
  import '../app.css';
  import Navbar from '$lib/components/Navbar.svelte';
  import BottomNav from '$lib/components/BottomNav.svelte';
  import { page } from '$app/stores';
</script>

<Navbar />

<main>
  <slot />
</main>

<footer>
  <div class="container">
    <div class="foot-logo">SUB<span>SENPAI</span></div>
    <p class="muted">
      Nonton anime, donghua &amp; film subtitle Indonesia. Subtitle dan video diambil dari situs
      sumber masing-masing; SubSenpai tidak menyimpan video di server sendiri.
    </p>
    <p class="muted small">
      Unduh aplikasi Android di
      <a href="https://www.subsenpai.web.id" target="_blank" rel="noreferrer">subsenpai.web.id</a>
      • Komunitas:
      <a href="https://discord.gg/yJebKEGNh" target="_blank" rel="noreferrer">Discord</a>
      •
      <a href="https://t.me/subsenpaianime" target="_blank" rel="noreferrer">Telegram</a>
    </p>
  </div>
</footer>

<style>
  main { min-height: calc(100vh - 220px); }
  footer { border-top: 1px solid var(--border); margin-top: 48px; padding: 28px 0 36px; }
  .foot-logo { color: var(--red); font-weight: 900; font-size: 18px; }
  .foot-logo span { color: #fff; }
  footer p { font-size: 13px; line-height: 1.6; }
  .small a { color: #fff; }
  @media (max-width: 767px) {
    /* Ruang untuk navigasi bawah yang menempel di layar mobile. */
    footer { padding-bottom: 92px; }
  }
</style>

{#if !$page.url.pathname.startsWith('/nonton')}
  <BottomNav />
{/if}
