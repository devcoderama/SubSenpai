<script>
  import { onMount } from 'svelte';
  import { getPlans, getPaymentMethods, checkout, paymentStatus, api } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import Skeleton from '$lib/components/Skeleton.svelte';

  let loading = true;
  let error = '';
  let plans = [];
  let methods = [];
  let premium = null;
  let selectedPlan = null;
  let paying = false;
  let payError = '';
  let payment = null;
  let statusMsg = '';
  let trialMsg = '';
  let trialBusy = false;

  const rp = (n) => 'Rp' + Number(n || 0).toLocaleString('id-ID');

  function planDays(p) {
    return p.duration_days || p.duration || p.days || null;
  }
  function planName(p) {
    return p.name || p.id;
  }

  async function load() {
    loading = true;
    error = '';
    try {
      const [pl, pm] = await Promise.all([getPlans(), getPaymentMethods()]);
      plans = pl.plans || [];
      methods = (pm.methods || []).filter((m) => m.is_active !== false);
      if ($user) {
        try {
          premium = await api('/api/v1/premium/status');
        } catch {
          premium = null;
        }
      }
    } catch (e) {
      error = e.message || 'Gagal memuat paket.';
    } finally {
      loading = false;
    }
  }

  async function buy() {
    if (!selectedPlan) return;
    if (!$user) {
      window.location.href = '/masuk';
      return;
    }
    paying = true;
    payError = '';
    payment = null;
    try {
      payment = await checkout({
        plan_id: selectedPlan.id,
        provider: 'tokopay',
        purpose: 'subscription',
        channel: 'QRIS'
      });
    } catch (e) {
      payError = e.message || 'Checkout gagal.';
    } finally {
      paying = false;
    }
  }

  async function checkStatus() {
    if (!payment?.session_id) return;
    statusMsg = 'Mengecek…';
    try {
      const res = await paymentStatus(payment.session_id);
      const st = res.status || res.payment_status || res.session?.status;
      if (st === 'paid') {
        statusMsg = '✓ Pembayaran lunas! Premium aktif — muat ulang halaman untuk memperbarui status akunmu.';
      } else {
        statusMsg = 'Status: ' + (st || 'pending') + ' — belum lunas. Coba lagi sesudah membayar.';
      }
    } catch (e) {
      statusMsg = 'Gagal mengecek: ' + (e.message || 'kesalahan server');
    }
  }

  async function claimTrial() {
    if (!$user) {
      window.location.href = '/masuk';
      return;
    }
    trialBusy = true;
    trialMsg = '';
    try {
      await api('/api/v1/premium/trial', { method: 'POST' });
      trialMsg = '✓ Uji coba Premium 3 hari aktif! Nikmati semua episode.';
      premium = await api('/api/v1/premium/status');
    } catch (e) {
      trialMsg = e.message || 'Uji coba gagal.';
    } finally {
      trialBusy = false;
    }
  }

  onMount(load);
</script>

<div class="container">
  <h1>Premium 👑</h1>
  <p class="muted">
    Gratis: semua episode anime &amp; donghua. Premium: semua film tanpa batas, tanpa perlu menonton iklan.
    Pembayaran diproses Tokopay (QRIS) — status dicek langsung ke Tokopay, jadi tidak ada dobel aktivasi.
  </p>

  {#if $user && premium?.is_premium}
    <p class="card ok">
      ✓ Akunmu Premium sampai {premium.premium_until
        ? new Date(premium.premium_until).toLocaleDateString('id-ID', {
            day: 'numeric',
            month: 'long',
            year: 'numeric'
          })
        : '—'}.
    </p>
  {:else if $user && premium?.trial_available}
    <div class="card">
      <strong>Belum pernah uji coba? Klaim Premium gratis 3 hari.</strong>
      <div class="row">
        <button class="btn" on:click={claimTrial} disabled={trialBusy}>
          {trialBusy ? 'Memproses…' : 'Klaim Uji Coba 3 Hari'}
        </button>
        {#if trialMsg}<span>{trialMsg}</span>{/if}
      </div>
    </div>
  {/if}

  {#if loading}
    <div class="plans"><Skeleton kind="plan" count={3} /></div>
  {:else if error}
    <p class="error">{error}</p>
  {:else}
    <div class="plans">
      {#each plans as p (p.id)}
        <button class="plan card" class:selected={selectedPlan?.id === p.id} on:click={() => (selectedPlan = p)}>
          <div class="plan-name">{planName(p)}</div>
          <div class="plan-price">{rp(p.price)}</div>
          {#if planDays(p)}<div class="muted">{planDays(p)} hari</div>{/if}
        </button>
      {/each}
    </div>

    {#if selectedPlan}
      <div class="card buy">
        <div>
          <strong>{planName(selectedPlan)}</strong> — {rp(selectedPlan.price)}
          <div class="muted">Pembayaran: QRIS (Tokopay){methods.length ? ' • ' + methods.length + ' metode aktif' : ''}</div>
        </div>
        <button class="btn" on:click={buy} disabled={paying}>
          {paying ? 'Memproses…' : $user ? 'Bayar Sekarang' : 'Masuk untuk Membeli'}
        </button>
      </div>
      {#if payError}<p class="error">{payError}</p>{/if}
    {/if}

    {#if payment}
      <div class="card pay-result">
        <h2>Pesanan dibuat</h2>
        <p>
          Sesi: <code>{payment.session_id}</code><br />
          Jumlah: <strong>{rp(payment.amount)}</strong><br />
          Berlaku sampai: {payment.expires_at ? new Date(payment.expires_at).toLocaleString('id-ID') : '—'}
        </p>
        {#if payment.qr_url || payment.payment_url || payment.checkout_url || payment.qr_code}
          <p>
            {#if payment.qr_url}
              <img class="qr" src={payment.qr_url} alt="Kode QR pembayaran" />
            {/if}
            {#if payment.payment_url || payment.checkout_url}
              <a class="btn" href={payment.payment_url || payment.checkout_url} target="_blank" rel="noreferrer">
                Buka Halaman Pembayaran
              </a>
            {/if}
            {#if payment.qr_code}<br /><code>{payment.qr_code}</code>{/if}
          </p>
        {:else}
          <p class="muted">
            Detail pembayaran (kode QR / tautan) akan tampil di sini sesudah checkout. Simpan ID sesi di
            atas untuk pengecekan.
          </p>
        {/if}
        <div class="row">
          <button class="btn ghost" on:click={checkStatus}>Cek Pembayaran</button>
          {#if statusMsg}<span>{statusMsg}</span>{/if}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  h1 { margin: 24px 0 8px; font-size: 28px; }
  .ok { border-color: #2f7d32; background: #122b16; }
  .row { display: flex; gap: 12px; align-items: center; margin-top: 12px; flex-wrap: wrap; }
  .plans { display: grid; gap: 12px; grid-template-columns: repeat(2, 1fr); margin: 20px 0; }
  @media (min-width: 800px) { .plans { grid-template-columns: repeat(3, 1fr); } }
  .plan { text-align: left; color: #fff; cursor: pointer; }
  .plan:hover { border-color: #777; }
  .plan.selected { border-color: var(--red); box-shadow: 0 0 0 1px var(--red); }
  .plan-name { font-weight: 700; font-size: 16px; }
  .plan-price { font-size: 22px; font-weight: 800; margin: 6px 0; }
  .buy { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; }
  .pay-result { margin-top: 16px; }
  .pay-result h2 { margin-top: 0; font-size: 18px; }
  .qr { width: 220px; max-width: 100%; background: #fff; padding: 8px; border-radius: 8px; }
  code { background: rgba(255,255,255,.08); padding: 2px 6px; border-radius: 4px; font-size: 13px; }
</style>
