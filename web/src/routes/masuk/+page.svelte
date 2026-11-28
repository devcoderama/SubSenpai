<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { GOOGLE_CLIENT_ID } from '$lib/api.js';
  import { loginWithGoogle, user } from '$lib/auth.js';

  let error = '';
  let busy = false;
  let gisReady = false;
  // null = tidak ada flag; '' = flag ada tapi nama perangkat tak diketahui.
  let revokedName = null;

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
        callback: onCredential,
        // Masuk otomatis tanpa klik (28 Nov 2026): bila sesi Google di
        // peramban ini masih ada dan akun ini pernah login, One Tap
        // mengembalikan kredensial sendiri — jadi tutup web lalu buka
        // lagi tidak perlu pencet tombol. use_fedcm_for_prompt supaya
        // tetap jalan walau cookie pihak ketiga diblokir (Brave).
        // Bila tidak bisa (belum pernah login / sesi Google habis /
        // peramban memblokir), prompt diam-diam tidak tampil dan tombol
        // Google di bawah tetap bisa dipakai seperti biasa.
        auto_select: true,
        use_fedcm_for_prompt: true
      });
      window.google.accounts.id.renderButton(document.getElementById('googleBtn'), {
        theme: 'filled_black',
        size: 'large',
        text: 'signin_with',
        width: 320
      });
      gisReady = true;
      // Jangan masuk otomatis bila sesi ini baru dicabut perangkat
      // lain: pengguna harus membaca dulu pemberitahuan kuning di atas
      // (login otomatis di sini akan menendang balik perangkat itu
      // tanpa ia sadari). Untuk sesi yang sekadar habis / token hilang,
      // One Tap boleh masuk sendiri.
      if (revokedName === null) {
        try {
          window.google.accounts.id.prompt(() => {
            /* diam bila One Tap tidak tampil — tombol tetap ada */
          });
        } catch {
          /* abaikan */
        }
      }
    } catch (e) {
      error = 'Gagal memuat tombol Google: ' + (e.message || 'kesalahan');
    }
  }

  onMount(() => {
    // Sesi dicabut di perangkat ini (login di perangkat lain melewati
    // batas): tampilkan pemberitahuan sekali, lalu hapus flag.
    try {
      const name = sessionStorage.getItem('ssweb.session_revoked');
      if (name !== null) {
        revokedName = name;
        sessionStorage.removeItem('ssweb.session_revoked');
      }
    } catch {
      /* abaikan — mode privat */
    }
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
    {#if revokedName !== null}
      <p class="notice">
        <i class="fa-solid fa-triangle-exclamation"></i>
        Sesi berakhir — akun ini login di perangkat lain.{#if revokedName}
          Perangkat {revokedName} telah dikeluarkan.{/if}
      </p>
    {/if}
    <div id="googleBtn" class="gbtn"></div>
    {#if busy}<p class="muted">Memproses login…</p>{/if}
    {#if error}<p class="error">{error}</p>{/if}
    {#if !gisReady && !error}
      <p class="muted">Memuat tombol Google…</p>
    {/if}
    <p class="muted small">
      Dengan masuk, kamu menyetujui
      <a href="https://www.subsenpai.web.id/terms.html" rel="noreferrer">Ketentuan</a>
      dan
      <a href="https://www.subsenpai.web.id/privacy.html" rel="noreferrer">Kebijakan Privasi</a>
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
  .notice {
    background: rgba(245, 197, 24, 0.12); border: 1px solid rgba(245, 197, 24, 0.45);
    color: #fde68a; border-radius: 4px; padding: 12px 14px; font-size: 14px;
    line-height: 1.5; text-align: left; margin: 16px 0 0;
  }
  .notice i { margin-right: 6px; }
  .small { font-size: 12px; }
</style>
