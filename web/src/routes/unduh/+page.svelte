<script>
  import { onMount } from 'svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { canInstall, isStandalone, isIOS, promptInstall } from '$lib/pwa.js';

  let installing = false;
  async function install() {
    if (installing) return;
    installing = true;
    try {
      await promptInstall();
    } finally {
      installing = false;
    }
  }

  // FALLBACK — BUMP saat rilis APK baru (samakan dengan
  // landing-src/index.src.html). Dipakai bila API GitHub gagal.
  const FALLBACK_URL =
    'https://github.com/devcoderama/SubSenpai/releases/download/v0.24.39/SubSenpai-v0.24.39.apk';
  const FALLBACK_VERSION = 'v0.24.39';

  let status = 'loading'; // loading | ok | fallback
  let rel = null; // { tag, name, size, published, url }
  let total = null; // jumlah unduhan .apk semua versi
  let totalErr = false;

  function fmtBytes(n) {
    if (!n) return '';
    return (n / 1048576).toLocaleString('id-ID', { maximumFractionDigits: 1 }) + ' MB';
  }
  function fmtDate(s) {
    if (!s) return '';
    return new Date(s).toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' });
  }
  function apkOf(r) {
    return (r.assets || []).find((a) => /\.apk$/i.test(a.name || ''));
  }

  async function loadLatest() {
    try {
      const res = await fetch('https://api.github.com/repos/devcoderama/SubSenpai/releases?per_page=10', {
        headers: { Accept: 'application/vnd.github+json' }
      });
      if (!res.ok) throw new Error('github ' + res.status);
      const list = await res.json();
      // SEMUA rilis SubSenpai adalah pre-release, jadi ambil rilis
      // pertama yang punya aset .apk (sama seperti halaman unduh lama).
      const hit = (list || []).find((r) => apkOf(r));
      if (!hit) throw new Error('tidak ada apk');
      const a = apkOf(hit);
      rel = {
        tag: hit.tag_name || hit.name || '',
        name: hit.name || hit.tag_name || '',
        size: a.size,
        published: hit.published_at,
        url: a.browser_download_url
      };
      status = 'ok';
    } catch {
      rel = { tag: FALLBACK_VERSION, name: FALLBACK_VERSION, size: 0, published: '', url: FALLBACK_URL };
      status = 'fallback';
    }
  }

  async function loadTotal() {
    // Jumlahkan download_count aset .apk dari SELURUH rilis (paginasi).
    try {
      let n = 0;
      let page = 1;
      for (;;) {
        const res = await fetch(
          'https://api.github.com/repos/devcoderama/SubSenpai/releases?per_page=100&page=' + page,
          { headers: { Accept: 'application/vnd.github+json' } }
        );
        if (!res.ok) throw new Error('github ' + res.status);
        const list = await res.json();
        for (const r of list || []) {
          for (const a of r.assets || []) {
            if (/\.apk$/i.test(a.name || '')) n += a.download_count || 0;
          }
        }
        if (!list || list.length < 100) break;
        page += 1;
      }
      total = n;
    } catch {
      totalErr = true;
    }
  }

  onMount(() => {
    loadLatest();
    loadTotal();
  });
</script>

<svelte:head>
  <title>Unduh Aplikasi Android — SubSenpai</title>
  <meta
    name="description"
    content="Unduh aplikasi Android SubSenpai (APK) — nonton anime, donghua & film subtitle Indonesia. Gratis, langsung dari situs resmi."
  />
</svelte:head>

<div class="container page">
  <div class="hero">
    <img src="/favicon.png" alt="SubSenpai" class="logo" />
    <h1>Unduh Aplikasi Android</h1>
    <p class="muted">
      Nonton anime, donghua &amp; film subtitle Indonesia di HP kamu. Gratis — langsung dari situs resmi,
      tanpa Play Store.
    </p>
  </div>

  <div class="card dl">
    {#if status === 'loading'}
      <Skeleton kind="plan" count={1} />
    {:else}
      <div class="ver">
        <span class="tag">{rel.tag}</span>
        <span class="muted">{rel.size ? fmtBytes(rel.size) : 'APK Android'} · Android 7.0+</span>
        {#if rel.published}<span class="muted">· {fmtDate(rel.published)}</span>{/if}
      </div>
      <a class="btn big" href={rel.url}><i class="fa-solid fa-download"></i> Unduh APK</a>
      {#if status === 'fallback'}
        <p class="muted small warn">
          Tidak bisa memeriksa rilis terbaru — memakai tautan cadangan {FALLBACK_VERSION}.
        </p>
      {/if}
      {#if total !== null}
        <p class="muted small">Total <strong>{total.toLocaleString('id-ID')}</strong> unduhan · semua versi</p>
      {/if}
    {/if}
  </div>

  <div class="card pwa">
    <strong>Atau install versi web</strong>
    <span class="muted"
      >Ikon SubSenpai di home screen, dibuka fullscreen kayak aplikasi — tanpa APK, tanpa Play
      Store. Tetap butuh internet untuk nonton.</span
    >
    {#if $isStandalone}
      <span class="muted small">✓ Kamu sedang membuka versi web yang sudah terinstall.</span>
    {:else if $canInstall}
      <div>
        <button class="btn" disabled={installing} on:click={install}
          >{installing ? 'Membuka…' : 'Install aplikasi'}</button
        >
      </div>
    {:else if isIOS}
      <span class="muted small"
        >Di iPhone/iPad: buka situs ini di Safari → ketuk Bagikan → "Tambah ke Layar Utama".</span
      >
    {:else}
      <span class="muted small"
        >Tombol install muncul otomatis di Chrome/Brave Android bila perangkat mendukung.</span
      >
    {/if}
  </div>

  <ol class="steps">
    <li><strong>Unduh APK-nya</strong><span class="muted"> — tombol merah di atas, satu file saja.</span></li>
    <li>
      <strong>Buka file APK &amp; izinkan instalasi</strong><span class="muted">
        — saat diminta, izinkan peramban memasang aplikasi dari sumber ini.</span
      >
    </li>
    <li>
      <strong>Masuk dengan Google, langsung nonton</strong><span class="muted">
        — anime &amp; donghua gratis semua episode; film khusus Premium atau buka dengan iklan.</span
      >
    </li>
  </ol>

  <p class="muted small">
    Semua rilis:
    <a href="https://github.com/devcoderama/SubSenpai/releases" rel="noreferrer"
      >github.com/devcoderama/SubSenpai/releases</a
    >
    · SubSenpai belum tersedia di Play Store — unduh hanya dari situs ini atau GitHub Releases resmi.
  </p>
</div>

<style>
  .page { padding-top: 32px; max-width: 640px; }
  .hero { text-align: center; margin-bottom: 20px; }
  .hero .logo { width: 72px; height: 72px; border-radius: 16px; }
  .hero h1 { font-size: 28px; margin: 12px 0 8px; }
  .hero p { font-size: 15px; line-height: 1.6; }
  .dl { text-align: center; padding: 24px 20px; margin-bottom: 20px; }
  .ver { display: flex; gap: 8px; justify-content: center; align-items: baseline; flex-wrap: wrap; margin-bottom: 16px; }
  .tag { background: var(--red); color: #fff; font-weight: 700; border-radius: 4px; padding: 2px 10px; }
  .btn.big { display: inline-block; font-size: 17px; padding: 12px 36px; }
  .dl p { margin-top: 12px; }
  .pwa { display: flex; flex-direction: column; gap: 8px; padding: 20px; margin-bottom: 20px; }
  .warn { color: #f5c518; }
  .steps { display: flex; flex-direction: column; gap: 12px; margin: 0 0 20px; padding-left: 22px; }
  .steps li { line-height: 1.5; }
  .steps strong { color: #fff; }
</style>
