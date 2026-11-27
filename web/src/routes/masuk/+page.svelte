<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { GOOGLE_CLIENT_ID } from '$lib/api.js';
  import { loginWithGoogle, user } from '$lib/auth.js';

  let error = '';
  let busy = false;
  let gisReady = false;

  async function onCredential(resp) {
    busy = true;
    error = '';
    try {
      await loginWithGoogle(resp.credential);
      goto('/');
    } catch (e) {
      error = e.message || 'Login gagal.';
      busy = false;
    }
  }

  function renderButton() {
    if (!window.google?.accounts?.id) return;
    try {
      window.google.accounts.id.initialize({
        client_id: GOOGLE_CLIENT_ID,
        callback: onCredential
      });
      window.google.accounts.id.renderButton(document.getElementById('googleBtn'), {
        theme: 'filled_black',
        size: 'large',
        text: 'signin_with',
        width: 320
      });
      gisReady = true;
    } catch (e) {
      error = 'Gagal memuat tombol Google: ' + (e.message || 'kesalahan');
    }
  }

  onMount(() => {
    if ($user) {
      goto('/');
      return;
    }
    if (window.google?.accounts?.id) {
      renderButton();
    } else {
      // Skrip GIS dimuat async di app.html — tunggu sebentar.
      const t = setInterval(() => {
        if (window.google?.accounts?.id) {
          clearInterval(t);
          renderButton();
        }
      }, 300);
      setTimeout(() => clearInterval(t), 10000);
    }
  });
</script>

<div class="container narrow">
  <div class="card login">
    <div class="logo">SUB<span>SENPAI</span></div>
    <h1>Masuk</h1>
    <p class="muted">
      Masuk dengan Google untuk menyimpan riwayat, membuka Premium, dan menonton episode lanjutan.
      Sama seperti aplikasi: hanya Google, tanpa email/password — untuk menghindari bot.
    </p>
    <div id="googleBtn" class="gbtn"></div>
    {#if busy}<p class="muted">Memproses login…</p>{/if}
    {#if error}<p class="error">{error}</p>{/if}
    {#if !gisReady && !error}
      <p class="muted">Memuat tombol Google…</p>
    {/if}
    <p class="muted small">
      Dengan masuk, kamu menyetujui
      <a href="https://www.subsenpai.web.id/terms.html" target="_blank" rel="noreferrer">Ketentuan</a>
      dan
      <a href="https://www.subsenpai.web.id/privacy.html" target="_blank" rel="noreferrer">Kebijakan Privasi</a>
      SubSenpai.
    </p>
  </div>
</div>

<style>
  .narrow { max-width: 480px; padding-top: 48px; }
  .login { text-align: center; padding: 32px 24px; }
  .logo { color: var(--red); font-weight: 900; font-size: 26px; }
  .logo span { color: #fff; }
  h1 { margin: 12px 0 8px; font-size: 24px; }
  .gbtn { display: flex; justify-content: center; margin: 20px 0; min-height: 44px; }
  .small { font-size: 12px; }
</style>
