<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { getPlans, getPaymentMethods, checkout, paymentStatus, api, redeemVoucher, getMyVouchers } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import { refreshPremium } from '$lib/ads.js';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { toastSuccess } from '$lib/toast.js';

  // Tiga kategori seperti di aplikasi Android (PremiumScreen.kt):
  // 0 = Langganan, 1 = Beli Voucher, 2 = Tukar Voucher.
  const TABS = ['Langganan', 'Beli Voucher', 'Tukar Voucher'];
  const GROUP_LABELS = { qris: 'QRIS', emoney: 'E-Wallet', va: 'Virtual Account', retail: 'Gerai Retail', pulsa: 'Pulsa' };
  const GROUP_ORDER = ['qris', 'emoney', 'va', 'retail', 'pulsa'];

  let tab = 0;
  let step = 1; // alur beli: 1 pilih paket, 2 metode bayar, 3 pembayaran
  let loading = true;
  let error = '';
  let plans = [];
  let methods = [];
  let premium = null;
  let selectedPlan = null;
  let selectedMethod = null;
  let qtyText = '1';
  let paying = false;
  let payError = '';
  let payment = null;
  let payPaid = false;
  let checking = false;
  let statusMsg = '';
  let statusIcon = ''; // '' | 'check' | 'exclamation'
  let trialIcon = ''; // '' | 'check' | 'exclamation'
  let trialMsg = '';
  let trialBusy = false;
  let qrDownloading = false;

  // Tukar voucher.
  let redeemCode = '';
  let redeemBusy = false;
  let redeemMsg = '';
  let redeemIcon = ''; // '' | 'check' | 'exclamation'

  // Voucher Saya (di tab Beli Voucher, seperti di aplikasi: Lainnya › Voucher Saya).
  let vouchers = [];
  let vouchersLoading = false;

  $: isVoucher = tab === 1;
  $: qty = Math.min(10, Math.max(1, parseInt(qtyText || '1', 10) || 1));
  $: total = (selectedPlan?.price || 0) * (isVoucher ? qty : 1);
  $: methodGroups = GROUP_ORDER.map((g) => ({
    grup: g,
    label: GROUP_LABELS[g] || g,
    items: methods.filter((m) => (m.group || '').toLowerCase() === g)
  })).filter((g) => g.items.length);

  const rp = (n) => 'Rp' + Number(n || 0).toLocaleString('id-ID');
  const fmtDate = (iso) =>
    iso ? new Date(iso).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' }) : '—';

  function planDays(p) {
    return p.duration_days || p.duration || p.days || null;
  }
  function planName(p) {
    return p.name || p.id;
  }
  function perMonth(p) {
    const d = planDays(p);
    return d && d > 30 ? Math.floor((p.price * 30) / d) : null;
  }

  function setTab(i) {
    tab = i;
    step = 1;
    payment = null;
    payPaid = false;
    payError = '';
    statusMsg = '';
  }

  function onQty(e) {
    qtyText = e.target.value.replace(/[^0-9]/g, '').slice(0, 2);
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
        loadVouchers();
      }
    } catch (e) {
      error = e.message || 'Gagal memuat paket.';
    } finally {
      loading = false;
    }
  }

  async function loadVouchers() {
    if (!$user) return;
    vouchersLoading = true;
    try {
      const res = await getMyVouchers();
      vouchers = res.vouchers || [];
    } catch {
      vouchers = [];
    } finally {
      vouchersLoading = false;
    }
  }

  function normalizePayment(r) {
    return {
      session_id: r.session_id,
      amount: r.total_bayar ?? r.amount,
      expires_at: r.expires_at,
      qr_url: r.qr_url || r.qr_link,
      payment_url: r.payment_url || r.pay_url || r.checkout_url,
      qr_code: r.qr_code || r.nomor_va,
      voucher_codes: r.voucher_codes || (r.voucher_code ? [r.voucher_code] : []),
      panduan: r.panduan || ''
    };
  }

  async function buy() {
    if (!selectedPlan || !selectedMethod) return;
    if (!$user) {
      window.location.href = '/masuk';
      return;
    }
    paying = true;
    payError = '';
    payment = null;
    try {
      const res = await checkout({
        plan_id: selectedPlan.id,
        provider: 'tokopay',
        purpose: isVoucher ? 'voucher' : 'subscription',
        channel: selectedMethod.code,
        quantity: isVoucher ? qty : 1
      });
      payment = normalizePayment(res);
      payPaid = false;
      step = 3;
      // Cek sekali otomatis saat masuk langkah pembayaran (seperti di aplikasi).
      checkStatus();
    } catch (e) {
      payError = e.message || 'Checkout gagal.';
    } finally {
      paying = false;
    }
  }

  async function checkStatus() {
    if (!payment?.session_id || checking) return;
    checking = true;
    statusMsg = 'Mengecek…';
    statusIcon = '';
    try {
      const res = await paymentStatus(payment.session_id);
      const st = res.status || res.payment_status || res.session?.status;
      if (st === 'paid') {
        payPaid = true;
        statusIcon = 'check';
        if (isVoucher) {
          const n = payment.voucher_codes?.length || 0;
          statusMsg =
            'Pembayaran lunas' +
            (n ? ' — ' + n + ' kode voucher aktif.' : '.') +
            ' Lihat & bagikan di Voucher Saya di bawah.';
          loadVouchers();
        } else {
          statusMsg = 'Pembayaran lunas! Premium aktif — status akunmu sudah diperbarui.';
        }
        try {
          premium = await api('/api/v1/premium/status');
          // Langganan (bukan voucher): akun kini premium -> segarkan profil
          // tersimpan + hilangkan iklan (muat ulang bila sudah dimuat).
          if (!isVoucher) refreshPremium();
        } catch {
          /* status lama tetap ditampilkan */
        }
      } else {
        statusIcon = '';
        statusMsg = 'Status: ' + (st || 'pending') + ' — belum lunas. Coba lagi sesudah membayar.';
      }
    } catch (e) {
      statusIcon = 'exclamation';
      statusMsg = 'Gagal mengecek: ' + (e.message || 'kesalahan server');
    } finally {
      checking = false;
    }
  }

  async function claimTrial() {
    if (!$user) {
      window.location.href = '/masuk';
      return;
    }
    trialBusy = true;
    trialMsg = '';
    trialIcon = '';
    try {
      await api('/api/v1/premium/trial', { method: 'POST' });
      trialIcon = 'check';
      trialMsg = 'Uji coba Premium 3 hari aktif! Nikmati semua episode.';
      premium = await api('/api/v1/premium/status');
      // Akun kini premium -> segarkan profil tersimpan + hilangkan iklan.
      refreshPremium();
    } catch (e) {
      trialIcon = 'exclamation';
      trialMsg = e.message || 'Uji coba gagal.';
    } finally {
      trialBusy = false;
    }
  }

  async function redeem() {
    const code = redeemCode.trim();
    if (!code) return;
    if (!$user) {
      window.location.href = '/masuk';
      return;
    }
    redeemBusy = true;
    redeemMsg = '';
    redeemIcon = '';
    try {
      const res = await redeemVoucher(code);
      redeemIcon = 'check';
      redeemCode = '';
      redeemMsg =
        'Voucher ditukar! Premium sampai ' +
        fmtDate(res.premium_until) +
        (res.days_added ? ' (+' + res.days_added + ' hari).' : '.');
      try {
        premium = await api('/api/v1/premium/status');
        // Akun kini premium -> segarkan profil tersimpan + hilangkan iklan.
        refreshPremium();
      } catch {
        /* abaikan */
      }
    } catch (e) {
      redeemIcon = 'exclamation';
      redeemMsg = e.message || 'Penukaran gagal.';
    } finally {
      redeemBusy = false;
    }
  }

  async function copyText(t, doneMsg) {
    try {
      await navigator.clipboard.writeText(t);
      toastSuccess(doneMsg || 'Disalin.');
    } catch {
      window.prompt('Salin:', t);
    }
  }

  async function downloadQr() {
    if (!payment?.qr_url) return;
    qrDownloading = true;
    try {
      const res = await fetch(payment.qr_url);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'QRIS-SubSenpai-' + (payment.session_id || '').slice(0, 8) + '.png';
      a.click();
      URL.revokeObjectURL(url);
      toastSuccess('QRIS tersimpan — lihat di unduhan.');
    } catch {
      // Gagal unduh QR: tampilkan gambar QR di tab yang sama (tanpa tab
      // baru, 27 Nov 2026) — tombol Kembali mengembalikan ke Premium.
      window.location.href = payment.qr_url;
    } finally {
      qrDownloading = false;
    }
  }

  function voucherStatusLabel(v) {
    if (v.status === 'redeemed') return 'Ditukar' + (v.redeemed_by_name ? ' oleh ' + v.redeemed_by_name : '');
    if (v.status === 'expired') return 'Kedaluwarsa';
    if (v.status === 'active') return v.payment_status === 'pending' ? 'Menunggu bayar' : 'Aktif';
    return v.status || '—';
  }
  function voucherSourceLabel(v) {
    return v.source === 'purchase' ? 'Pembelian' : v.source === 'admin' ? 'Hadiah admin' : v.source || '—';
  }

  onMount(load);
</script>

<div class="container">
  <h1>Premium <i class="fa-solid fa-crown" style="color: var(--gold)"></i></h1>
  <p class="muted">
    Gratis: semua episode anime &amp; donghua. Premium: semua film tanpa batas, tanpa perlu menonton iklan.
    Pembayaran diproses Tokopay — status dicek langsung ke Tokopay, jadi tidak ada dobel aktivasi.
  </p>

  {#if $user && premium?.is_premium}
    <p class="card ok">
      <i class="fa-solid fa-circle-check"></i> Akunmu Premium sampai {fmtDate(premium.premium_until)}.
    </p>
  {/if}

  <div class="tabs3">
    {#each TABS as label, i}
      <button class="tab3" class:active={tab === i} on:click={() => setTab(i)}>{label}</button>
    {/each}
  </div>


  {#if tab === 2}
    <!-- Tukar Voucher — tanpa langkah pembayaran, seperti di aplikasi. -->
    <p class="muted">
      Masukkan kode voucher yang kamu punya — premium langsung aktif di akunmu begitu kode berhasil
      ditukar.
    </p>
    <div class="card redeem">
      <label class="field">
        <span class="field-label">Kode voucher</span>
        <input type="text" placeholder="SS-XXXX-XXXX" bind:value={redeemCode} on:keydown={(e) => e.key === 'Enter' && redeem()} />
      </label>
      <button class="btn" on:click={redeem} disabled={!redeemCode.trim() || redeemBusy}>
        {redeemBusy ? 'Memproses…' : $user ? 'Tukar Voucher' : 'Masuk untuk Menukar'}
      </button>
      {#if redeemMsg}
        <p class={redeemIcon === 'check' ? 'ok-text' : 'error'}>
          {#if redeemIcon === 'check'}<i class="fa-solid fa-circle-check"></i>{:else if redeemIcon === 'exclamation'}<i class="fa-solid fa-circle-exclamation"></i>{/if}
          {redeemMsg}
        </p>
      {/if}
    </div>
  {:else}
    {#if step > 1}
      <button class="linklike back" on:click={() => step--}><i class="fa-solid fa-arrow-left"></i> Kembali</button>
    {/if}
    <p class="muted step-line">
      Langkah {step} dari 3 • {step === 1 ? 'Pilih paket' : step === 2 ? 'Metode pembayaran' : 'Pembayaran'}
    </p>

    {#if loading}
      <div class="plans"><Skeleton kind="plan" count={3} /></div>
    {:else if error}
      <p class="error">{error}</p>
    {:else if step === 1}
      {#if tab === 0 && $user && premium?.trial_available}
        <div class="card">
          <strong>Belum pernah uji coba? Klaim Premium gratis 3 hari.</strong>
          <div class="row">
            <button class="btn" on:click={claimTrial} disabled={trialBusy}>
              {trialBusy ? 'Memproses…' : 'Klaim Uji Coba 3 Hari'}
            </button>
            {#if trialMsg}<span>{#if trialIcon === 'check'}<i class="fa-solid fa-circle-check"></i>{:else if trialIcon === 'exclamation'}<i class="fa-solid fa-circle-exclamation"></i>{/if} {trialMsg}</span>{/if}
          </div>
        </div>
      {/if}

      <div class="plans">
        {#each plans as p (p.id)}
          <button class="plan card" class:selected={selectedPlan?.id === p.id} on:click={() => (selectedPlan = p)}>
            {#if p.original_price > p.price && p.discount_percent > 0}
              <span class="plan-disc">-{p.discount_percent}%</span>
            {/if}
            <div class="plan-name">{planName(p)}</div>
            {#if p.original_price > p.price && p.discount_percent > 0}
              <div class="plan-orig">{rp(p.original_price)}</div>
            {/if}
            <div class="plan-price">{rp(p.price)}</div>
            {#if planDays(p)}<div class="muted">{planDays(p)} hari</div>{/if}
            {#if perMonth(p)}<div class="per-month">≈ {rp(perMonth(p))}/bulan</div>{/if}
            {#if selectedPlan?.id === p.id}
              <span class="plan-check"><i class="fa-solid fa-check"></i></span>
            {/if}
          </button>
        {/each}
      </div>

      {#if isVoucher}
        <div class="card qty">
          <strong>Mau dibagikan ke berapa orang?</strong>
          <div class="row">
            <label class="field inline">
              <span class="field-label">Jumlah orang</span>
              <input type="text" inputmode="numeric" value={qtyText} on:input={onQty} />
            </label>
            <span class="muted">kode voucher (maks 10)</span>
          </div>
          <p class="muted small">
            Total: {rp(total)} — {qty} kode × {selectedPlan ? planDays(selectedPlan) : 0} hari premium. Kode bisa
            dibagikan ke siapa saja dan dipakai siapa pun yang menukarkannya.
          </p>
        </div>
      {/if}

      <button
        class="btn wide"
        disabled={!selectedPlan}
        on:click={() => {
          if (!$user) window.location.href = '/masuk';
          else step = 2;
        }}
      >
        {$user ? 'Lanjut' : 'Masuk untuk Membeli'}
      </button>
    {:else if step === 2}
      {#if selectedPlan}
        <p class="summary">
          {(isVoucher ? 'Voucher ' : 'Langganan ') + planName(selectedPlan) +
            (isVoucher ? ' × ' + qty + ' kode' : '') +
            ' — ' + rp(total)}
        </p>
      {/if}
      {#if !methodGroups.length}
        <p class="muted">Metode pembayaran belum tersedia. Coba lagi sebentar lagi.</p>
      {:else}
        {#each methodGroups as g}
          <h3 class="group-title">{g.label}</h3>
          <div class="methods">
            {#each g.items as m}
              <button class="method card" class:selected={selectedMethod?.code === m.code} on:click={() => (selectedMethod = m)}>
                {#if m.icon_url}
                  <img class="method-icon" src={m.icon_url} alt="" />
                {:else}
                  <span class="method-letter">{(m.name || '?')[0].toUpperCase()}</span>
                {/if}
                <span class="method-name">{m.name}</span>
                {#if selectedMethod?.code === m.code}<i class="fa-solid fa-check method-check"></i>{/if}
              </button>
            {/each}
          </div>
        {/each}
      {/if}
      <button class="btn wide" disabled={!selectedPlan || !selectedMethod || paying} on:click={buy}>
        {paying ? 'Memproses…' : 'Bayar ' + rp(total)}
      </button>
      {#if payError}<p class="error">{payError}</p>{/if}
    {:else}
      {#if !payment}
        <p class="muted">Belum ada pembayaran yang dibuat.</p>
      {:else}
        <div class="card pay-result">
          <h2>Selesaikan Pembayaran</h2>
          {#if selectedMethod}<p class="muted">via {selectedMethod.name}</p>{/if}
          <p class="muted small">Total bayar</p>
          <p class="pay-total">{rp(payment.amount)}</p>
          <p class="muted small">
            Sesi: <code>{payment.session_id}</code><br />
            Berlaku sampai: {payment.expires_at ? new Date(payment.expires_at).toLocaleString('id-ID') : '—'}
          </p>
          {#if payment.qr_url}
            <img class="qr" src={payment.qr_url} alt="Kode QR pembayaran" />
            <button class="btn ghost wide" on:click={downloadQr} disabled={qrDownloading}>
              <i class="fa-solid fa-download"></i>
              {qrDownloading ? 'Mengunduh…' : 'Unduh QRIS'}
            </button>
          {/if}
          {#if payment.qr_code}
            <p class="muted small">Nomor VA / Kode Bayar</p>
            <p class="pay-va">{payment.qr_code}</p>
            <button class="btn ghost wide" on:click={() => copyText(payment.qr_code, 'Nomor VA disalin.')}>
              <i class="fa-solid fa-copy"></i> Salin Nomor
            </button>
          {/if}
          {#if payment.payment_url}
            <a class="btn wide" href={payment.payment_url} target="_blank" rel="noreferrer">
              <i class="fa-solid fa-arrow-up-right-from-square"></i> Buka Halaman Pembayaran
            </a>
          {/if}
          {#if !payment.qr_url && !payment.payment_url && !payment.qr_code}
            <p class="muted">
              Detail pembayaran (kode QR / tautan) akan tampil di sini sesudah checkout. Simpan ID sesi di
              atas untuk pengecekan.
            </p>
          {/if}
          {#if payment.panduan}
            <p class="muted small panduan">{payment.panduan.replace(/<[^>]+>/g, ' ')}</p>
          {/if}
          {#if payment.voucher_codes?.length}
            <h3 class="group-title">Kode voucher kamu</h3>
            <div class="codes">
              {#each payment.voucher_codes as c}
                <div class="code-row">
                  <code>{c}</code>
                  <button class="btn ghost small" on:click={() => copyText(c, 'Kode voucher disalin.')}>
                    <i class="fa-solid fa-copy"></i> Salin
                  </button>
                </div>
              {/each}
            </div>
            <p class="muted small">Kode aktif sesudah pembayaran lunas — bagikan ke siapa saja.</p>
          {/if}
          <div class="row">
            <button class="btn ghost" on:click={checkStatus} disabled={checking || payPaid}>
              {checking ? 'Mengecek…' : payPaid ? 'Pembayaran Lunas' : 'Sudah bayar? Cek Pembayaran'}
            </button>
            {#if statusMsg}<span>{#if statusIcon === 'check'}<i class="fa-solid fa-circle-check"></i>{:else if statusIcon === 'exclamation'}<i class="fa-solid fa-circle-exclamation"></i>{/if} {statusMsg}</span>{/if}
          </div>
          {#if payPaid}
            <!-- Tombol Selesai seperti di aplikasi: kembali ke Beranda. -->
            <button class="btn wide" on:click={() => goto('/')}>
              <i class="fa-solid fa-check"></i> Selesai
            </button>
          {/if}
        </div>
      {/if}
    {/if}

    {#if tab === 1 && $user}
      <h2 class="section-title">Voucher Saya</h2>
      {#if vouchersLoading}
        <p class="muted">Memuat voucher…</p>
      {:else if !vouchers.length}
        <p class="muted">Belum ada voucher. Beli voucher di atas untuk dibagikan, atau tukar kode yang kamu terima di tab Tukar Voucher.</p>
      {:else}
        <div class="voucher-list">
          {#each vouchers as v}
            <div class="card voucher-row">
              <div class="voucher-main">
                <code>{v.code}</code>
                <span class="muted small">
                  {v.duration_days} hari • {voucherSourceLabel(v)} • {voucherStatusLabel(v)}
                  {#if v.expires_at}• berlaku sampai {fmtDate(v.expires_at)}{/if}
                </span>
              </div>
              {#if v.status === 'active'}
                <button class="btn ghost small" on:click={() => copyText(v.code, 'Kode voucher disalin.')}>
                  <i class="fa-solid fa-copy"></i> Salin
                </button>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    {/if}
  {/if}
</div>

<style>
  h1 { margin: 24px 0 8px; font-size: 28px; }
  .ok { border-color: #2f7d32; background: #122b16; }
  .ok-text { color: #7ec97e; }
  .row { display: flex; gap: 12px; align-items: center; margin-top: 12px; flex-wrap: wrap; }
  .wide { width: 100%; margin-top: 12px; }
  .linklike { background: none; border: 0; color: var(--red); cursor: pointer; font-size: 14px; padding: 0; }
  .linklike:hover { text-decoration: underline; }
  .back { margin: 8px 0 0; padding: 0; }
  .step-line { margin: 12px 0 4px; font-size: 13px; }

  /* Tiga kategori seperti di aplikasi: kartu sejajar, aktif berbingkai merah. */
  .tabs3 { display: flex; gap: 8px; margin: 16px 0 8px; }
  .tab3 {
    flex: 1; padding: 12px 6px; border-radius: 4px; border: 2px solid transparent;
    background: var(--panel); color: #fff; font-size: 14px; font-weight: 700; cursor: pointer;
  }
  .tab3.active { border-color: var(--red); background: #2a1f24; }

  .plans { display: grid; gap: 12px; grid-template-columns: repeat(2, 1fr); margin: 20px 0; }
  @media (min-width: 800px) { .plans { grid-template-columns: repeat(3, 1fr); } }
  @media (min-width: 1280px) {
    /* Desktop lebar (27 Nov 2026): kartu paket mengisi baris — 5 paket
       muat berdampingan, bukan 3 kartu raksasa. */
    .plans { grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
  }
  .plan { position: relative; text-align: left; color: #fff; cursor: pointer; }
  .plan:hover { border-color: #777; }
  .plan.selected { border-color: var(--red); box-shadow: 0 0 0 1px var(--red); }
  .plan-name { font-weight: 700; font-size: 16px; }
  .plan-price { font-size: 22px; font-weight: 800; margin: 6px 0; }
  .plan-orig { text-decoration: line-through; color: var(--muted); font-size: 14px; }
  .plan-disc {
    position: absolute;
    top: 10px;
    right: 10px;
    background: var(--red);
    color: #fff;
    font-weight: 800;
    font-size: 13px;
    padding: 3px 8px;
    border-radius: 999px;
  }
  .per-month { color: #c9a7ff; font-size: 13px; margin-top: 4px; }
  .plan-check {
    position: absolute; top: 10px; right: 10px; width: 24px; height: 24px; border-radius: 50%;
    background: var(--red); color: #fff; display: flex; align-items: center; justify-content: center; font-size: 13px;
  }

  .qty { margin-bottom: 12px; }
  .field { display: flex; flex-direction: column; gap: 6px; }
  .field-label { font-size: 13px; color: var(--muted); }
  .field input {
    background: rgba(0,0,0,.6); border: 1px solid #555; color: #fff; border-radius: 4px;
    padding: 10px 12px; font-size: 15px; width: 100%;
  }
  .field input:focus { outline: none; border-color: #fff; }
  .field.inline input { width: 130px; }
  .small { font-size: 13px; }

  .summary { font-weight: 700; font-size: 16px; margin: 12px 0; }
  .group-title { font-size: 14px; color: var(--muted); margin: 18px 0 8px; }
  .methods { display: flex; flex-direction: column; gap: 8px; }
  .method {
    display: flex; align-items: center; gap: 12px; width: 100%; text-align: left;
    color: #fff; cursor: pointer; padding: 12px;
  }
  .method:hover { border-color: #777; }
  .method.selected { border-color: var(--red); box-shadow: 0 0 0 1px var(--red); }
  .method-icon { width: 30px; height: 30px; object-fit: contain; background: #fff; border-radius: 4px; padding: 3px; }
  .method-letter {
    width: 30px; height: 30px; border-radius: 50%; background: #3a3a44; color: #fff;
    display: flex; align-items: center; justify-content: center; font-weight: 700; flex-shrink: 0;
  }
  .method-name { flex: 1; font-size: 15px; }
  .method-check { color: var(--red); }

  .pay-result { margin-top: 16px; text-align: center; }
  .pay-result h2 { margin-top: 0; font-size: 18px; }
  .pay-total { font-size: 32px; font-weight: 800; margin: 4px 0 12px; }
  .pay-va { font-size: 22px; font-weight: 800; margin: 4px 0 12px; letter-spacing: 1px; }
  .qr { width: 220px; max-width: 100%; background: #fff; padding: 8px; border-radius: 4px; display: block; margin: 12px auto; }
  .pay-result .row { justify-content: center; }
  .panduan { white-space: pre-line; text-align: left; }
  .codes { display: flex; flex-direction: column; gap: 8px; margin: 8px 0; }
  .code-row {
    display: flex; align-items: center; justify-content: space-between; gap: 8px;
    background: rgba(255,255,255,.05); border: 1px solid var(--border); border-radius: 4px; padding: 8px 12px;
  }
  .code-row code { font-size: 15px; }

  .section-title { font-size: 19px; margin: 28px 0 12px; }
  .voucher-list { display: flex; flex-direction: column; gap: 8px; }
  .voucher-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .voucher-main { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
  .voucher-main code { font-size: 15px; word-break: break-all; }

  .redeem { display: flex; flex-direction: column; gap: 12px; }
  code { background: rgba(255,255,255,.08); padding: 2px 6px; border-radius: 4px; font-size: 13px; }
  .btn.small { padding: 6px 12px; font-size: 13px; }
</style>
