<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { getPlayback, getTitle, adUnlockStart, adUnlockComplete, isPremiumRequired } from '$lib/api.js';
  import { user } from '$lib/auth.js';
  import Player from '$lib/components/Player.svelte';

  const AD_URL = 'https://uplcm.com/4/12330317';

  let loading = true;
  let error = '';
  let locked = false;
  let playback = null;
  let episodes = [];
  let title = null;

  // Alur buka-iklan (sama dengan aplikasi): start -> buka iklan 15 detik
  // di tab baru -> complete. Server yang menegakkan waktunya.
  let adState = 'idle'; // idle | waiting | done
  let adToken = null;
  let adLeft = 15;
  let adError = '';
  let adTimer = null;

  $: epId = $page.params.episodeId;
  $: idx = episodes.findIndex((e) => e.id === epId);
  $: prevEp = idx > 0 ? episodes[idx - 1] : null;
  $: nextEp = idx >= 0 && idx < episodes.length - 1 ? episodes[idx + 1] : null;

  async function load() {
    loading = true;
    error = '';
    locked = false;
    try {
      playback = await getPlayback(epId);
      if (playback.title_id) {
        const det = await getTitle(playback.title_id);
        title = det.title || det;
        episodes = det.episodes || [];
      }
    } catch (e) {
      if (isPremiumRequired(e)) {
        locked = true;
        // Tetap muat info judul untuk tombol episode lain.
        try {
          const det = await getTitle(playback?.title_id || '');
          title = det.title || det;
          episodes = det.episodes || [];
        } catch {
          /* abaikan */
        }
      } else {
        error = e.message || 'Gagal memuat episode.';
      }
    } finally {
      loading = false;
    }
  }

  async function startAd() {
    adError = '';
    try {
      const res = await adUnlockStart(epId);
      adToken = res.token;
      if (!adToken) throw new Error('Server tidak mengembalikan token iklan.');
      window.open(AD_URL, '_blank', 'noopener');
      adState = 'waiting';
      adLeft = 15;
      clearInterval(adTimer);
      adTimer = setInterval(() => {
        adLeft -= 1;
        if (adLeft <= 0) {
          clearInterval(adTimer);
          completeAd();
        }
      }, 1000);
    } catch (e) {
      adError = e.message || 'Gagal memulai iklan.';
    }
  }

  async function completeAd() {
    try {
      await adUnlockComplete(epId, { token: adToken });
      adState = 'done';
      await load();
    } catch (e) {
      adState = 'idle';
      adError = e.message || 'Iklan belum selesai — tonton sampai selesai dulu.';
    }
  }

  onMount(load);
</script>

<div class="container">
  {#if loading}
    <p class="loading">Memuat pemutar…</p>
  {:else if error}
    <p class="error">{error}</p>
    <p><a href="/">← Kembali ke Beranda</a></p>
  {:else if locked}
    <div class="card lock">
      <h1>🔒 Episode ini khusus Premium</h1>
      <p class="muted">
        {#if title}{title.title} — Episode {playback?.episode_no || ''}{/if}
        Anime &amp; donghua bebas ditonton semua episode. Film khusus Premium — atau buka episode
        ini dengan menonton iklan 15 detik. Aturan ini ditegakkan server, sama seperti di aplikasi.
      </p>
      <div class="actions">
        <a class="btn" href="/premium">Lihat Premium</a>
        {#if adState === 'idle'}
          <button class="btn ghost" on:click={startAd}>📺 Buka iklan 15 detik untuk membuka episode ini</button>
        {:else if adState === 'waiting'}
          <button class="btn ghost" disabled>Menunggu iklan… {adLeft} detik</button>
        {:else}
          <span class="muted">✓ Terbuka! Memuat ulang…</span>
        {/if}
      </div>
      {#if adError}<p class="error">{adError}</p>{/if}
      {#if $user?.is_premium}
        <p class="muted">Akunmu sudah Premium tapi episode ini masih terkunci — coba muat ulang halaman.</p>
      {/if}
    </div>
  {:else if playback}
    {#if title}
      <h1 class="watch-title">
        <a href={'/judul/' + title.id}>{title.title}</a> — Episode {playback.episode_no}
      </h1>
    {/if}
    <Player sources={playback.sources || []} title={title?.title || ''} />
    <div class="epnav">
      {#if prevEp}
        <a class="btn ghost" href={'/nonton/' + prevEp.id}>← Ep {prevEp.episode_no}</a>
      {:else}
        <span></span>
      {/if}
      {#if title}<a class="btn ghost" href={'/judul/' + title.id}>Semua Episode</a>{/if}
      {#if nextEp}
        {#if nextEp.locked}
          <a class="btn ghost" href={'/judul/' + (title?.id || '')}>Ep {nextEp.episode_no} 🔒</a>
        {:else}
          <a class="btn" href={'/nonton/' + nextEp.id}>Ep {nextEp.episode_no} →</a>
        {/if}
      {/if}
    </div>
  {/if}
</div>

<style>
  .watch-title { font-size: 20px; margin: 20px 0 4px; }
  .watch-title a:hover { color: var(--red); }
  .lock { margin-top: 24px; }
  .lock h1 { font-size: 22px; margin: 0 0 8px; }
  .actions { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 16px; }
  .epnav { display: flex; justify-content: space-between; gap: 10px; margin: 16px 0 8px; }
</style>
