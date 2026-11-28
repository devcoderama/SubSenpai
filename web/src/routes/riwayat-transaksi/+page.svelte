<script>
  import { onMount } from 'svelte';
  import { user } from '$lib/auth.js';
  import { getTransactions, paymentStatus } from '$lib/api.js';
  import { toastSuccess, toastInfo, toastError } from '$lib/toast.js';
  import Skeleton from '$lib/components/Skeleton.svelte';

  /** Halaman Riwayat Transaksi (Lainnya — cermin TransactionsScreen
      aplikasi, BillingScreens.kt:56): sesi pembayaran user + tombol
      Cek Pembayaran untuk yang masih pending (server menanyakan
      gateway dan mengaktifkan sesi bila ternyata sudah lunas).
      API: GET /api/v1/transactions + GET /api/v1/payments/status/{id}. */

  let items = [];
  let loading = true;
  let error = '';
  let checkingId = null;

  const rp = (n) => 'Rp' + new Intl.NumberFormat('id-ID').format(n || 0);
  const statusLabel = (s) =>
    s === 'paid'
      ? 'Lunas'
      : s === 'pending'
        ? 'Menunggu pembayaran'
        : s === 'expired'
          ? 'Kedaluwarsa'
          : s;
  const when = (iso) => (iso || '').slice(0, 16).replace('T', ' ');

  async function load() {
    // Tamu: jangan panggil API transaksi — tampilkan kartu ajakan login.
    if (!$user) {
      loading = false;
      return;
    }
    loading = true;
    error = '';
    try {
      const res = await getTransactions();
      items = res.transactions || [];
    } catch (e) {
      error = e.message || 'Gagal memuat riwayat transaksi.';
    } finally {
      loading = false;
    }
  }

  async function check(id, label) {
    if (checkingId) return;
    checkingId = id;
    try {
      const o = await paymentStatus(id);
      if (o.status === 'paid') {
        toastSuccess(`Pembayaran ${label} terkonfirmasi lunas.`);
      } else {
        toastInfo('Belum terdeteksi lunas — selesaikan pembayaran dulu.');
      }
      // Segarkan daftar tanpa skeleton (load() penuh menampilkan
      // skeleton — janggal sesudah tiap pengecekan).
      const res = await getTransactions();
      items = res.transactions || [];
    } catch (e) {
      toastError(e.message || 'Gagal mengecek pembayaran.');
    } finally {
      checkingId = null;
    }
  }

  onMount(load);
</script>

<svelte:head><title>Riwayat Transaksi — SubSenpai</title></svelte:head>

<div class="container page">
  <a class="back" href="/lainnya">← Lainnya</a>
  <h1>Riwayat Transaksi</h1>

  {#if loading}
    <Skeleton kind="row" count={4} />
  {:else if !$user}
    <div class="card empty">
      <strong>Masuk untuk melihat riwayat transaksi.</strong>
      <span class="muted">Pembelian premium dan voucher yang kamu lakukan lewat akun ini.</span>
      <div class="row"><a class="btn" href="/masuk">Masuk dengan Google</a></div>
    </div>
  {:else if error}
    <p class="error">{error}</p>
    <button class="btn" on:click={load}>Coba Lagi</button>
  {:else if !items.length}
    <div class="card empty">
      <strong>Belum ada transaksi.</strong>
      <span class="muted">Beli premium atau voucher lewat <a href="/premium">Premium</a>.</span>
    </div>
  {:else}
    <div class="rows">
      {#each items as t (t.id)}
        <div class="trx-row">
          <div class="top">
            <strong>{t.plan_label || t.purpose}</strong>
            <span class="status" class:paid={t.status === 'paid'} class:pending={t.status === 'pending'}>
              {statusLabel(t.status)}
            </span>
          </div>
          <span class="muted"
            >{t.purpose === 'voucher' ? 'Beli voucher' : 'Langganan'} ·
            {t.provider ? t.provider.charAt(0).toUpperCase() + t.provider.slice(1) : '—'} ·
            {rp(t.amount)}</span
          >
          <span class="muted small">{when(t.created_at)}</span>
          {#if t.status === 'pending'}
            <div class="row">
              <button
                class="btn ghost small"
                disabled={checkingId !== null}
                on:click={() => check(t.id, t.plan_label || t.purpose)}
                >{checkingId === t.id ? 'Mengecek…' : 'Cek Pembayaran'}</button
              >
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page { padding-top: 24px; max-width: 720px; }
  .back { display: inline-block; margin-bottom: 8px; color: var(--muted); }
  .page h1 { font-size: 26px; margin-bottom: 16px; }
  .empty { display: flex; flex-direction: column; gap: 8px; }
  .row { display: flex; gap: 12px; align-items: center; margin-top: 8px; flex-wrap: wrap; }
  .rows { display: flex; flex-direction: column; gap: 10px; }
  .trx-row {
    display: flex; flex-direction: column; gap: 4px;
    background: var(--panel); border: 1px solid var(--border);
    border-radius: 4px; padding: 14px 16px;
  }
  .trx-row .top { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; }
  .status { font-size: 13px; font-weight: 700; color: var(--muted); white-space: nowrap; }
  .status.paid { color: #4caf50; }
  .status.pending { color: #ffc107; }
  .small { font-size: 12px; }
</style>
