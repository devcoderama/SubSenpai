<script>
  import { onMount } from 'svelte';
  import { user } from '$lib/auth.js';
  import { api, getMyVouchers, redeemVoucher } from '$lib/api.js';
  import { refreshPremium } from '$lib/ads.js';
  import { toastSuccess, toastError } from '$lib/toast.js';
  import Skeleton from '$lib/components/Skeleton.svelte';

  /** Halaman Undang Teman (referral): kode teman = kode referral.
      Backend sudah memberi voucher 3 hari ke pengundang + pendaftar
      saat akun baru login dengan referral_code (auth.go); web hanya
      menyediakan tautan undangan, statistik, dan penukaran voucher.
      Statistik dihitung dari GET /api/v1/vouchers/mine (source
      'referral'): satu voucher yang dibuat orang lain untukku = satu
      teman yang berhasil daftar lewat kodeku. */

  let friendCode = '';
  let loading = true;
  let error = '';
  let vouchers = [];
  let redeemBusy = '';

  $: inviteLink = friendCode
    ? `https://subsenpai.web.id/masuk?ref=${encodeURIComponent(friendCode)}`
    : '';
  $: referralVouchers = vouchers.filter((v) => v.source === 'referral');
  $: joinedCount = referralVouchers.filter((v) => !v.created_by_me && v.for_me).length;
  $: daysEarned = referralVouchers
    .filter((v) => !v.created_by_me && v.for_me && v.status === 'redeemed')
    .reduce((s, v) => s + (v.duration_days || 0), 0);

  $: if ($user?.friend_code && !friendCode) friendCode = $user.friend_code;

  onMount(async () => {
    if (!$user) {
      loading = false;
      return;
    }
    if (!friendCode) {
      try {
        const res = await api('/api/v1/me');
        if (res.profile?.friend_code) friendCode = res.profile.friend_code;
      } catch {
        /* abaikan — kode disembunyikan bila gagal dimuat */
      }
    }
    await loadVouchers();
  });

  async function loadVouchers() {
    try {
      const res = await getMyVouchers();
      vouchers = res.vouchers || [];
    } catch (e) {
      error = e.message || 'Gagal memuat voucher.';
    } finally {
      loading = false;
    }
  }

  async function copyText(text, what) {
    try {
      await navigator.clipboard.writeText(text);
      toastSuccess(what + ' disalin.');
    } catch {
      window.prompt('Salin:', text);
    }
  }

  async function shareInvite() {
    const text =
      'Nonton anime, donghua & film subtitle Indonesia di SubSenpai — daftar pakai tautanku dan kita berdua dapat Premium 3 hari gratis!';
    if (navigator.share) {
      try {
        await navigator.share({ title: 'SubSenpai — Undang Teman', text, url: inviteLink });
        return;
      } catch {
        /* dibatalkan — diam */
      }
    }
    copyText(inviteLink, 'Tautan undangan');
  }

  async function redeem(v) {
    if (redeemBusy) return;
    redeemBusy = v.code;
    try {
      const res = await redeemVoucher(v.code);
      toastSuccess('Voucher ditukar! Premium +' + (res.days_added || v.duration_days) + ' hari.');
      refreshPremium();
      await loadVouchers();
    } catch (e) {
      toastError(e.message || 'Gagal menukar voucher.');
    } finally {
      redeemBusy = '';
    }
  }

  const statusLabel = (s) =>
    s === 'active' ? 'Aktif' : s === 'redeemed' ? 'Ditukar' : s === 'expired' ? 'Kedaluwarsa' : s;
</script>

<div class="container page">
  <h1>Undang Teman</h1>

  {#if !$user}
    <div class="card">
      <p>Masuk dulu untuk melihat kode referral kamu.</p>
      <a class="btn" href="/masuk">Masuk dengan Google</a>
    </div>
  {:else if loading}
    <Skeleton />
    <Skeleton />
  {:else if error}
    <div class="card">
      <p class="error">{error}</p>
      <button class="btn ghost" on:click={() => { loading = true; error = ''; loadVouchers(); }}>Coba Lagi</button>
    </div>
  {:else}
    <div class="card hero">
      <div class="hero-ic"><i class="fa-solid fa-gift"></i></div>
      <h2>Undang teman, berdua dapat Premium 3 hari</h2>
      <p class="muted">
        Bagikan tautan undanganmu. Teman daftar pakai Google lewat tautan itu,
        kalian berdua langsung dapat voucher Premium 3 hari — tukar di bawah.
      </p>
      <ol class="steps">
        <li>Salin tautan atau bagikan ke temanmu</li>
        <li>Teman daftar dengan Google lewat tautan itu</li>
        <li>Voucher 3 hari masuk ke akun kalian berdua</li>
      </ol>
    </div>

    {#if friendCode}
      <div class="card">
        <span class="muted small">Kode referral kamu</span>
        <div class="code-row">
          <code>{friendCode}</code>
          <button class="btn ghost small" on:click={() => copyText(friendCode, 'Kode referral')}>
            <i class="fa-solid fa-copy"></i> Salin Kode
          </button>
        </div>
        <span class="muted small">Tautan undangan</span>
        <div class="link-row">
          <span class="link">{inviteLink}</span>
        </div>
        <div class="row">
          <button class="btn ghost small" on:click={() => copyText(inviteLink, 'Tautan undangan')}>
            <i class="fa-solid fa-link"></i> Salin Tautan
          </button>
          <button class="btn small" on:click={shareInvite}>
            <i class="fa-solid fa-share-nodes"></i> Bagikan
          </button>
        </div>
      </div>

      <div class="stats">
        <div class="stat">
          <strong>{joinedCount}</strong>
          <span class="muted small">teman bergabung</span>
        </div>
        <div class="stat">
          <strong>{daysEarned} hari</strong>
          <span class="muted small">Premium dari undangan</span>
        </div>
      </div>
    {/if}

    <h2 class="sec">Voucher referral kamu</h2>
    {#if referralVouchers.length === 0}
      <div class="card">
        <p class="muted">
          Belum ada voucher referral. Bagikan tautanmu — tiap teman yang daftar
          lewat tautan itu memberimu satu voucher Premium 3 hari.
        </p>
      </div>
    {:else}
      <div class="vlist">
        {#each referralVouchers as v (v.code)}
          <div class="vrow">
            <div class="vinfo">
              <code>{v.code}</code>
              <span class="muted small">
                +{v.duration_days} hari Premium •
                {v.created_by_me && v.for_me
                  ? 'Voucher sambutanmu (kamu daftar pakai kode teman)'
                  : 'Dari teman yang daftar lewat tautanmu'}
              </span>
            </div>
            <span class="chip {v.status}">{statusLabel(v.status)}</span>
            {#if v.status === 'active' && v.for_me}
              <button class="btn small" disabled={redeemBusy === v.code} on:click={() => redeem(v)}>
                {redeemBusy === v.code ? 'Menukar…' : 'Tukar'}
              </button>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
    <p class="muted small">
      Voucher referral juga bisa ditukar di halaman Premium. Satu teman = satu voucher;
      kode tidak bisa dipakai untuk akun sendiri.
    </p>
  {/if}
</div>

<style>
  .page { padding-top: 24px; max-width: 720px; }
  .page h1 { font-size: 26px; margin-bottom: 16px; }
  .card { margin-bottom: 16px; }
  .hero { text-align: center; }
  .hero-ic {
    font-size: 32px; color: var(--gold); margin-bottom: 8px;
  }
  .hero h2 { font-size: 20px; margin: 0 0 8px; }
  .hero p { margin: 0 0 12px; line-height: 1.6; }
  .steps {
    text-align: left; max-width: 420px; margin: 0 auto;
    padding-left: 20px; line-height: 1.8; font-size: 14px;
  }
  .small { font-size: 13px; }
  .code-row {
    display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
    margin: 8px 0 16px;
  }
  .code-row code {
    background: rgba(255,255,255,.08); padding: 8px 16px; border-radius: 4px;
    font-size: 20px; letter-spacing: 3px;
  }
  .link-row { margin: 8px 0 12px; }
  .link {
    display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
    background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 4px; padding: 10px 12px; font-size: 13px; color: var(--muted);
  }
  .row { display: flex; gap: 8px; flex-wrap: wrap; }
  .btn.small { padding: 8px 14px; font-size: 13px; }
  .stats {
    display: flex; gap: 12px; margin-bottom: 16px;
  }
  .stat {
    flex: 1; background: var(--panel); border: 1px solid var(--border);
    border-radius: 4px; padding: 16px; text-align: center;
    display: flex; flex-direction: column; gap: 4px;
  }
  .stat strong { font-size: 24px; }
  .sec { font-size: 18px; margin: 20px 0 12px; }
  .vlist {
    background: var(--panel); border: 1px solid var(--border);
    border-radius: 4px; overflow: hidden; margin-bottom: 16px;
  }
  .vrow {
    display: flex; align-items: center; gap: 12px; flex-wrap: wrap;
    padding: 12px 16px; border-bottom: 1px solid var(--border);
  }
  .vrow:last-child { border-bottom: 0; }
  .vinfo { flex: 1; display: flex; flex-direction: column; gap: 2px; min-width: 180px; }
  .vinfo code { font-size: 14px; letter-spacing: 1px; }
  .chip {
    font-size: 12px; padding: 3px 10px; border-radius: 20px; font-weight: 700;
  }
  .chip.active { background: rgba(52, 211, 153, 0.15); color: #34d399; }
  .chip.redeemed { background: rgba(255,255,255,0.08); color: var(--muted); }
  .chip.expired { background: rgba(229, 9, 20, 0.15); color: #f87171; }
  .error { color: #f87171; }
</style>
