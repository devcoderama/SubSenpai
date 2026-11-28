<script>
  import { onMount, tick } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    getPlayback, getTitle, getTitles, getHistory,
    adUnlockStart, adUnlockComplete, isPremiumRequired,
    reportHistory, createReport
  } from '$lib/api.js';
  import { user, token } from '$lib/auth.js';
  import { addLocalHistory, updateLocalPosition, getLocalHistory, fmtPosition } from '$lib/history.js';
  import Player from '$lib/components/Player.svelte';
  import { PartySocket } from '$lib/party.js';
  import EpisodeGrid from '$lib/components/EpisodeGrid.svelte';
  import Rail from '$lib/components/Rail.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { toastInfo, toastSuccess, toastError } from '$lib/toast.js';
  import { seoEpisodeTitle, seoDescription, seoEpisodeJsonLd, jsonLdScriptTag, SITE_URL } from '$lib/seo.js';

  const AD_URL = 'https://uplcm.com/4/12330317';
  const REPORT_REASONS = [
    ['semua_server_gagal', 'Semua server tidak bisa diputar'],
    ['link_rusak', 'Link rusak / video tidak muncul'],
    ['video_salah', 'Video salah / tidak sesuai episode'],
    ['subtitle', 'Subtitle bermasalah'],
    ['lainnya', 'Lainnya']
  ];

  let loading = true;
  let error = '';
  let locked = false;
  let playback = null;
  let resumeAt = 0; // posisi tersimpan untuk video langsung (0 = mulai dari awal)
  let episodes = [];
  let title = null;
  let recommendations = [];
  let current = 0; // server aktif (bind ke Player)
  let selectedSub = null; // lang subtitle terpilih (bind ke Player)
  let descExpanded = false;
  let copied = false;
  let shareOpen = false;
  let partyOpen = false;
  let serverOpen = false;
  let subOpen = false;
  let playerWrap = null; // pembungkus pemutar (target fullscreen)
  let fsSupported = false;
  let isFullscreen = false;

  // Popover Server/Subtitle/Bagikan: position: fixed + posisi dihitung
  // dari tombol yang diklik (27 Nov 2026). Sebelumnya popover itu
  // absolute di dalam .actions-row yang overflow-x:auto, sehingga
  // terpotong tak terlihat — tombol tampak "tidak bisa diklik".
  let popPos = { top: 0, maxH: 400, left: 0 };
  let lastPopRect = null; // rect tombol pemicu popover terakhir

  // v4: pilih posisi popover dari tinggi AKTUAL panel (scrollHeight =
  // tinggi penuh konten meski dibatasi maxH): di bawah tombol bila muat
  // penuh, di atas bila muat penuh, bila keduanya tak muat tetap di
  // bawah + gulir internal. Idempotent — aman dipanggil berulang.
  function flipPopIfNeeded() {
    const el = document.querySelector('.pop');
    if (!el || !lastPopRect) return;
    const h = el.scrollHeight;
    const r = lastPopRect;
    const belowTop = Math.max(8, Math.min(r.bottom + 8, window.innerHeight - 160));
    const aboveTop = r.top - h - 8;
    let top;
    if (belowTop + h <= window.innerHeight - 8) top = belowTop;
    else if (aboveTop >= 8) top = aboveTop;
    else top = belowTop;
    popPos = { ...popPos, top, maxH: Math.max(120, window.innerHeight - top - 8) };
  }
  async function togglePop(which, ev) {
    const r = ev.currentTarget.getBoundingClientRect();
    lastPopRect = r;
    // Panel harus muat di sisa ruang viewport di bawah tombol (temuan
    // verifikasi #8, 09:57 — v1 gagal: max-height tetap 100dvh tak
    // membatasi panel setinggi konten). maxH dihitung per buka supaya
    // panel bisa digulir di dalam viewport (overflow-y di .pop).
    let top = Math.max(8, Math.min(r.bottom + 8, window.innerHeight - 160));
    popPos = {
      top,
      maxH: Math.max(120, window.innerHeight - top - 8),
      left: Math.max(8, Math.min(r.left, window.innerWidth - 268))
    };
    serverOpen = which === 'server' ? !serverOpen : false;
    subOpen = which === 'sub' ? !subOpen : false;
    shareOpen = which === 'share' ? !shareOpen : false;
    partyOpen = which === 'party' ? !partyOpen : false;
    await tick();
    flipPopIfNeeded();
  }
  let showReport = false;
  let reportReason = 'link_rusak';
  let reportNote = '';
  let reportSending = false;
  let reportDone = '';
  let reportError = '';

  // Alur buka-iklan (sama dengan aplikasi): start -> iklan dibuka di
  // TAB YANG SAMA -> kembali (tombol Kembali peramban) -> complete.
  // Sebelumnya iklan dibuka di tab baru (window.open) sehingga pengguna
  // bisa menutup tab iklan lalu menunggu di halaman episode (27 Nov
  // 2026). Durasi 60 detik ditegakkan SERVER (wait_seconds dari start).
  const AD_PENDING_KEY = 'subsenpai_ad_pending';
  let adState = 'idle'; // idle | waiting | done
  let adToken = null;
  let adLeft = 60;
  let adChecking = false;
  let adError = '';
  let adTimer = null;

  $: epId = $page.params.episodeId;
  // SEO (28 Nov 2026): <title> + meta + JSON-LD per episode.
  $: epNo = playback?.episode?.episode_no ?? curEp?.episode_no ?? null;
  $: headTitle = seoEpisodeTitle(title, epNo);
  $: headDesc = seoDescription(title);
  $: jsonLdTag = jsonLdScriptTag(seoEpisodeJsonLd(title, epNo, epId));
  $: titleParam = $page.url.searchParams.get('title') || '';
  $: idx = episodes.findIndex((e) => e.id === epId);
  $: curEp = idx >= 0 ? episodes[idx] : null;
  $: epTitle = curEp?.title || '';
  $: epSeason = playback?.episode?.season_no ?? curEp?.season_no ?? null;
  // Serial TV: judul episode backend sudah "Episode N" — jangan
  // tampilkan dua kali sesudah "Season S · Episode N".
  $: epTitleShown = title?.type === 'series' ? '' : epTitle;
  $: prevEp = idx > 0 ? episodes[idx - 1] : null;
  $: nextEp = idx >= 0 && idx < episodes.length - 1 ? episodes[idx + 1] : null;
  $: src = playback?.sources?.[current];
  $: sourceLabel = src?.label || (src?.lang ? 'Video ' + src.lang.toUpperCase() : 'Server');
  $: readySubs = (playback?.subtitles || []).filter((s) => s.status === 'done' && s.file_url);
  $: subLabel = selectedSub ? 'Sub ' + selectedSub.toUpperCase() : 'Subtitle';
  // --- Nonton Bareng (#8, cermin PartySheet aplikasi) ---
  // Ruang efemer lewat WebSocket /ws (auth ?token=): server hanya
  // meneruskan sinyal putar/jeda/lompat ke anggota lain — tak ada daftar
  // anggota, tak ada chat, tak ada ikatan episode (episode dibawa klien
  // lewat tautan undangan). Sinkron putar hanya untuk video langsung;
  // embed tak bisa dikendalikan lintas-origin (catatan yang sama ada di
  // sheet aplikasi: "(pemutar langsung)").
  let partyCode = null;
  let partyJoinCode = '';
  let partySocket = null;
  let partyStatus = '';
  let partyGone = false; // koneksi putus sesudah 3x coba sambung ulang
  let syncAction = null; // perintah sinkron ke Player (video langsung)
  $: isDirect = !!src?.url && /\.(mp4|m3u8|webm)(\?|$)/i.test(src.url);

  function partyConnect() {
    if (!$user || !$token || partySocket) return;
    const socket = new PartySocket($token, onPartyEvent);
    partySocket = socket;
    socket.connect();
  }

  function onPartyEvent(ev) {
    if (ev.type === 'party_created' || ev.type === 'party_joined') {
      partyCode = ev.code;
      if (partySocket) partySocket.code = ev.code;
      partyGone = false;
      toastSuccess(ev.type === 'party_created' ? 'Ruang dibuat — kode ' + ev.code : 'Masuk ruang ' + ev.code);
    } else if (ev.type === 'party_sync') {
      const pos = ev.position_sec || 0;
      partyStatus =
        (ev.action === 'play' ? 'Memutar di ' : ev.action === 'pause' ? 'Dijeda di ' : 'Lompat ke ') +
        fmtPosition(pos);
      if (isDirect) syncAction = { action: ev.action, position: pos, at: Date.now() };
    } else if (ev.type === 'error') {
      toastError(ev.error || 'Gagal Nonton Bareng.');
    } else if (ev.type === 'party_disconnected') {
      partyGone = true;
      toastError('Koneksi Nonton Bareng terputus.');
    }
  }

  function createParty() {
    if (!partySocket) return;
    partySocket.create(epId);
  }

  function joinParty(code) {
    const c = (code || '').trim().toUpperCase();
    if (!partySocket || c.length !== 6) return;
    partySocket.join(c);
  }

  function leaveParty() {
    if (partySocket && partyCode) partySocket.leave(partyCode);
    if (partySocket) partySocket.code = null;
    partyCode = null;
    partyStatus = '';
    partyGone = false;
  }

  function copyParty(text, msg) {
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(text).then(
        () => toastSuccess(msg),
        () => window.prompt('Salin ini:', text)
      );
    } else {
      window.prompt('Salin ini:', text);
    }
  }

  function onPlayState(e) {
    if (!partyCode || !isDirect || !partySocket) return;
    partySocket.sync(partyCode, e.detail.action, Math.floor(e.detail.position || 0));
  }

  // Reposisi popover setiap kali isinya berubah tinggi (v4): panel party
  // tumbuh sesudah buat/gabung ruang; panel server tumbuh sesudah
  // playback termuat. void(...) mendaftarkan variabel sebagai dependensi.
  $: if (serverOpen || subOpen || shareOpen || partyOpen) {
    void (partyCode, partyStatus, partyGone, playback);
    void tick().then(flipPopIfNeeded);
  }

  // Meta ala aplikasi — bedanya dengan Detail, status di sini mentah
  // ("ongoing"), persis seperti PlayerScreen.kt.
  $: metaLine = title
    ? [
        title.rating ? '★ ' + Number(title.rating).toFixed(1) : null,
        title.year || null,
        title.duration_min ? title.duration_min + ' mnt/ep' : null,
        title.type ? title.type[0].toUpperCase() + title.type.slice(1) : null,
        title.status || null
      ]
        .filter(Boolean)
        .join(' · ')
    : '';

  // Episode berganti (Prev/Next/grid) tanpa remount komponen — muat
  // ulang playback tiap epId berubah.
  let loadedEp = '';
  $: if (epId && epId !== loadedEp) {
    loadedEp = epId;
    load();
  }

  // Kembali dari halaman iklan (tab yang sama): peramban bisa
  // memulihkan halaman dari bfcache tanpa memuat ulang — sinkronkan
  // lagi hitung mundur dari sesi yang tersimpan.
  onMount(() => {
    const onShow = () => resumeAd();
    window.addEventListener('pageshow', onShow);
    fsSupported = !!document.fullscreenEnabled;
    const onFsChange = () => { isFullscreen = !!document.fullscreenElement; };
    document.addEventListener('fullscreenchange', onFsChange);
    // Nonton Bareng (#8): tautan undangan ?party=XXXXXX.
    const inviteCode = $page.url.searchParams.get('party');
    if ($user) {
      partyConnect();
      if (inviteCode) joinParty(inviteCode);
    } else if (inviteCode) {
      toastInfo('Masuk dulu untuk gabung Nonton Bareng (kode ' + inviteCode.trim().toUpperCase() + ').');
    }
    return () => {
      window.removeEventListener('pageshow', onShow);
      document.removeEventListener('fullscreenchange', onFsChange);
      if (partySocket) {
        if (partyCode) partySocket.leave(partyCode);
        partySocket.close();
      }
    };
  });

  function goBack() {
    if (window.history.length > 1) window.history.back();
    else if (title) goto('/judul/' + title.id);
    else goto('/');
  }

  function goToEp(ep) {
    if (!ep) return;
    if (ep.locked) goto('/premium');
    else goto('/nonton/' + ep.id);
  }

  // Sesi iklan yang tertunda (disimpan sebelum pindah ke halaman iklan
  // di tab yang sama) — dibaca lagi saat pengguna kembali.
  function saveAdPending(token, wait) {
    try {
      localStorage.setItem(
        AD_PENDING_KEY,
        JSON.stringify({ epId, token, startedAt: Date.now(), wait })
      );
    } catch {
      /* abaikan */
    }
  }
  function readAdPending() {
    try {
      const raw = localStorage.getItem(AD_PENDING_KEY);
      if (!raw) return null;
      const p = JSON.parse(raw);
      return p && p.epId === epId && p.token ? p : null;
    } catch {
      return null;
    }
  }
  function clearAdPending() {
    try {
      localStorage.removeItem(AD_PENDING_KEY);
    } catch {
      /* abaikan */
    }
  }
  function startAdCountdown() {
    clearInterval(adTimer);
    adTimer = setInterval(() => {
      adLeft -= 1;
      if (adLeft <= 0) {
        clearInterval(adTimer);
        // Jeda 3 detik sebelum complete: toleransi selisih jam
        // klien-server supaya complete tidak ditolak server.
        adChecking = true;
        setTimeout(() => {
          adChecking = false;
          completeAd();
        }, 3000);
      }
    }, 1000);
  }
  // Lanjutkan sesi iklan yang tertunda (pengguna kembali dari halaman
  // iklan — via muat ulang atau tombol Kembali peramban). Hitung mundur
  // dihitung ulang dari waktu mulai supaya waktu di halaman iklan ikut
  // terhitung; token kedaluwarsa (>10 mnt) dibuang.
  function resumeAd() {
    const p = readAdPending();
    if (!p) return false;
    const elapsed = Math.floor((Date.now() - p.startedAt) / 1000);
    if (elapsed > 600) {
      clearAdPending();
      return false;
    }
    adToken = p.token;
    adState = 'waiting';
    adError = '';
    adChecking = false;
    adLeft = Math.max(p.wait - elapsed, 0);
    if (adLeft <= 0) {
      adChecking = true;
      setTimeout(() => {
        adChecking = false;
        completeAd();
      }, 3000);
    } else {
      startAdCountdown();
    }
    return true;
  }

  async function load() {
    loading = true;
    error = '';
    locked = false;
    playback = null;
    resumeAt = 0;
    current = 0;
    selectedSub = null;
    descExpanded = false;
    adState = 'idle';
    adError = '';
    clearInterval(adTimer);
    try {
      playback = await getPlayback(epId);
      clearAdPending();
      // Respons playback TIDAK punya title_id di top-level — id judul
      // ada di dalam playback.episode.title_id. Cek lama
      // (playback.title_id) selalu undefined, sehingga title/episode/
      // rekomendasi tak pernah dimuat (27 Nov 2026, laporan Rams).
      const tid = playback.episode?.title_id || playback.title_id;
      if (tid) {
        const det = await getTitle(tid);
        title = det.title || det;
        episodes = det.episodes || [];
        resumeAt = await resolveResume();
        recordWatch(resumeAt);
        loadRecommendations();
      }
    } catch (e) {
      if (isPremiumRequired(e)) {
        locked = true;
        // Lanjutkan sesi iklan bila pengguna baru kembali dari
        // halaman iklan di tab yang sama.
        resumeAd();
        // Info judul dari ?title= bila ada (tautan dari baris episode
        // terkunci di halaman Detail) — respons 403 tak membawa title_id.
        if (titleParam) {
          try {
            const det = await getTitle(titleParam);
            title = det.title || det;
            episodes = det.episodes || [];
          } catch {
            /* abaikan */
          }
        }
      } else {
        error = e.message || 'Gagal memuat episode.';
      }
    } finally {
      loading = false;
    }
  }

  async function loadRecommendations() {
    recommendations = [];
    const g = title?.genres?.[0];
    if (!g) return;
    try {
      const r = await getTitles({ genre: g, type: title.type || '', limit: 13 });
      recommendations = (r.items || []).filter((t) => t.id !== title.id).slice(0, 12);
    } catch {
      /* abaikan — Rekomendasi opsional */
    }
  }

  async function startAd() {
    adError = '';
    try {
      const res = await adUnlockStart(epId);
      adToken = res.token;
      if (!adToken) throw new Error('Server tidak mengembalikan token iklan.');
      // Durasi dari server (60 dtk sesudah installer jalan; 15 dtk
      // sebelum itu) — sama seperti aplikasi yang membaca wait_seconds.
      const wait = Number(res.wait_seconds) || 60;
      saveAdPending(adToken, wait);
      adState = 'waiting';
      adLeft = wait;
      startAdCountdown();
      // TAB YANG SAMA: halaman episode ditinggal ke halaman iklan;
      // pengguna kembali lewat tombol Kembali peramban, lalu resumeAd()
      // melanjutkan hitung mundur (waktu di halaman iklan terhitung).
      window.location.href = AD_URL;
    } catch (e) {
      adError = e.message || 'Gagal memulai iklan.';
    }
  }

  async function completeAd() {
    try {
      await adUnlockComplete(epId, { token: adToken });
      clearAdPending();
      adState = 'done';
      await load();
    } catch (e) {
      clearAdPending();
      adState = 'idle';
      adError = e.message || 'Iklan belum selesai — tonton sampai selesai dulu.';
    }
  }

  async function shareCopy() {
    const url = window.location.href;
    try {
      await navigator.clipboard.writeText(url);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      window.prompt('Salin tautan ini:', url);
    }
  }

  // Bagikan native (Web Share API) — fallback ke Salin tautan bila
  // tidak tersedia atau gagal (selain pengguna membatalkan dialog).
  async function shareNative() {
    const url = window.location.href;
    if (navigator.share) {
      try {
        await navigator.share({
          title: title?.title || 'SubSenpai',
          text: title?.title
            ? title.title + (curEp?.episode_no != null ? ' — Episode ' + curEp.episode_no : '')
            : '',
          url
        });
        return;
      } catch (e) {
        if (e?.name === 'AbortError') return;
      }
    }
    shareCopy();
  }

  // Layar penuh pada pembungkus pemutar — jalan untuk <video> langsung
  // maupun iframe embed, karena yang di-fullscreen-kan div-nya.
  async function toggleFullscreen() {
    if (!playerWrap) return;
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else await playerWrap.requestFullscreen();
    } catch {
      /* abaikan — mis. pengguna menekan Esc bersamaan */
    }
  }

  async function submitReport() {
    reportSending = true;
    reportError = '';
    try {
      await createReport(epId, reportReason, reportNote.trim());
      showReport = false;
      reportNote = '';
      reportDone = 'Laporan terkirim. Terima kasih!';
      setTimeout(() => (reportDone = ''), 4000);
    } catch (e) {
      reportError = e.message || 'Gagal mengirim laporan, coba lagi.';
    } finally {
      reportSending = false;
    }
  }

  // Catat ke riwayat: server bila masuk, lokal selalu (untuk tamu).
  // Kegagalan pencatatan tidak boleh mengganggu pemutaran.
  // PENTING (28 Nov 2026): jangan pernah mencatat 0 di sini — panggilan
  // lama recordWatch(0) menimpa posisi tersimpan sehingga resume tak
  // pernah punya data. Posisi yang dicatat = posisi resume yang baru
  // dihitung; durasi lama (bila sudah dipelajari) ikut dipertahankan.
  let lastSavedPos = -1;
  let lastSavedAt = 0;
  function recordWatch(position) {
    if (!playback || !title) return;
    const prev = getLocalHistory().find((x) => x.episode_id === epId);
    const item = {
      episode_id: epId,
      title_id: playback.episode?.title_id || playback.title_id,
      title: title.title,
      poster_url: title.poster_url,
      episode_no: playback.episode?.episode_no,
      position_sec: position
    };
    if (prev?.duration_sec > 0) item.duration_sec = prev.duration_sec;
    addLocalHistory(item);
    if ($user) {
      reportHistory(epId, position).catch(() => {});
    }
  }

  // Posisi tersimpan untuk episode ini: lokal dulu (perangkat ini),
  // lalu server bila masuk dan tak ada catatan lokal. Hanya video
  // langsung yang bisa di-seek; iframe tak pernah melaporkan posisi.
  async function resolveResume() {
    let pos = getLocalHistory().find((x) => x.episode_id === epId)?.position_sec || 0;
    if (!pos && $user) {
      try {
        const res = await getHistory(50);
        pos = (res.items || []).find((x) => x.episode_id === epId)?.position_sec || 0;
      } catch {
        /* abaikan — mulai dari awal */
      }
    }
    return pos > 5 ? pos : 0;
  }

  function onResumed(e) {
    toastInfo('Melanjutkan dari ' + fmtPosition(e.detail?.position || 0));
  }
  function onProgress(e) {
    const pos = e.detail?.position || 0;
    const now = Date.now();
    if (pos - lastSavedPos < 10 && now - lastSavedAt < 15000) return;
    lastSavedPos = pos;
    lastSavedAt = now;
    updateLocalPosition(epId, pos, e.detail?.duration || 0);
    if ($user) reportHistory(epId, pos).catch(() => {});
  }
</script>

<svelte:head>
  <title>{headTitle}</title>
  <meta name="description" content={headDesc} />
  <link rel="canonical" href={SITE_URL + '/nonton/' + epId} />
  <meta property="og:type" content="video.episode" />
  <meta property="og:site_name" content="SubSenpai" />
  <meta property="og:title" content={headTitle} />
  <meta property="og:description" content={headDesc} />
  <meta property="og:url" content={SITE_URL + '/nonton/' + epId} />
  {#if title?.poster_url}
    <meta property="og:image" content={title.poster_url} />
    <meta name="twitter:image" content={title.poster_url} />
  {/if}
  <meta name="twitter:card" content="summary_large_image" />
  {@html jsonLdTag}
</svelte:head>

<div class="watch">
  <div class="watch-inner">
    <!-- Bar atas ala aplikasi: kembali + SubSenpai ungu + judul. -->
    <div class="watch-head">
      <button class="back-btn" on:click={goBack} aria-label="Kembali">
        <i class="fa-solid fa-arrow-left"></i>
      </button>
      <div class="head-titles">
        <div class="brand">SubSenpai</div>
        {#if title}<div class="head-sub">{title.title}</div>{/if}
      </div>
    </div>

    {#if loading}
      <Skeleton kind="player" />
    {:else if error}
      <div class="watch-body">
        <p class="error">{error}</p>
        <p><a href="/"><i class="fa-solid fa-arrow-left"></i> Kembali ke Beranda</a></p>
      </div>
    {:else if locked}
      <div class="watch-body">
        <div class="card lock">
          <h1><i class="fa-solid fa-lock"></i> Episode ini khusus Premium</h1>
          <p class="muted">
            {#if title}{title.title} — Episode {playback?.episode?.episode_no || curEp?.episode_no || ''}{/if}
            Anime, donghua &amp; serial TV bebas ditonton semua episode. Film khusus Premium — atau buka episode
            ini dengan menonton iklan 60 detik. Iklan dibuka di tab ini — setelah selesai, kembali
            ke halaman ini dengan tombol Kembali. Aturan ini ditegakkan server, sama seperti di aplikasi.
          </p>
          <div class="actions">
            <a class="btn" href="/premium">Lihat Premium</a>
            {#if !$user}
              <a class="btn ghost" href="/masuk">Masuk</a>
            {/if}
            {#if adState === 'idle'}
              <button class="btn ghost" on:click={startAd}><i class="fa-solid fa-tv"></i> Buka iklan 60 detik untuk membuka episode ini</button>
            {:else if adState === 'waiting'}
              <button class="btn ghost" disabled>
                {adChecking ? 'Memeriksa iklan…' : 'Menunggu iklan… ' + adLeft + ' detik'}
              </button>
            {:else}
              <span class="muted"><i class="fa-solid fa-circle-check"></i> Terbuka! Memuat ulang…</span>
            {/if}
          </div>
          {#if adError}<p class="error">{adError}</p>{/if}
          {#if $user?.is_premium}
            <p class="muted">Akunmu sudah Premium tapi episode ini masih terkunci — coba muat ulang halaman.</p>
          {/if}
        </div>
        {#if title}
          <div class="info">
            <h1 class="watch-title">{title.title}</h1>
            {#if metaLine}<div class="meta">{metaLine}</div>{/if}
          </div>
        {/if}
        {#if episodes.length}
          <div class="ep-head">
            <h2>Episode</h2>
            <div class="ep-nav">
              <button class="nav-btn" disabled={!prevEp} on:click={() => goToEp(prevEp)}>‹ Prev</button>
              <button class="nav-btn" disabled={!nextEp} on:click={() => goToEp(nextEp)}>Next ›</button>
            </div>
          </div>
          <EpisodeGrid {episodes} currentId={epId} />
        {/if}
      </div>
    {:else if playback}
      <div class="player-wrap" bind:this={playerWrap}>
        <Player
          bind:current
          sources={playback.sources || []}
          subtitles={playback.subtitles || []}
          bind:selectedSub
          title={title?.title || ''}
          resumeAt={resumeAt}
          syncAction={syncAction}
          on:progress={onProgress}
          on:resumed={onResumed}
          on:playstate={onPlayState}
        />
      </div>

      <div class="watch-body">
        {#if title}
          <div class="info">
            <h1 class="watch-title">{title.title}</h1>
            {#if playback.episode?.episode_no != null}
              <div class="ep-sub">
                {#if title?.type === 'series' && epSeason}Season {epSeason} · {/if}Episode {playback.episode
                  .episode_no}{epTitleShown ? ' — ' + epTitleShown : ''}
              </div>
            {/if}
            {#if metaLine}<div class="meta">{metaLine}</div>{/if}
            {#if title.genres?.length}
              <div class="genre-pills">
                {#each title.genres as g}<span class="genre-pill">{g}</span>{/each}
              </div>
            {/if}
          </div>
        {/if}

        <!-- Bar aksi ala aplikasi: Server / Subtitle / Bagikan / Laporkan. -->
        <div class="actions-row">
          <div class="pop-wrap">
            <button class="action-btn" on:click={(e) => togglePop('server', e)}>
              Server: {sourceLabel}
            </button>
            {#if serverOpen}
              <div class="pop" style="top: {popPos.top}px; left: {popPos.left}px; max-height: {popPos.maxH}px">
                <div class="pop-title">Pilih Server</div>
                {#each playback.sources || [] as s, i}
                  <button class="pop-row" on:click={() => { current = i; serverOpen = false; }}>
                    <span>{s.label || 'Server ' + (i + 1)}{s.lang ? ' · Video ' + s.lang.toUpperCase() : ''}</span>
                    {#if i === current}<i class="fa-solid fa-check"></i>{/if}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
          <div class="pop-wrap">
            <button class="action-btn" on:click={(e) => togglePop('sub', e)}>
              {subLabel}
            </button>
            {#if subOpen}
              <div class="pop" style="top: {popPos.top}px; left: {popPos.left}px; max-height: {popPos.maxH}px">
                <div class="pop-title">Subtitle</div>
                {#if readySubs.length}
                  <button class="pop-row" on:click={() => { selectedSub = null; subOpen = false; }}>
                    <span>Tanpa subtitle</span>
                    {#if !selectedSub}<i class="fa-solid fa-check"></i>{/if}
                  </button>
                  {#each readySubs as s}
                    <button class="pop-row" on:click={() => { selectedSub = s.lang; subOpen = false; }}>
                      <span>Sub {s.lang.toUpperCase()}</span>
                      {#if selectedSub === s.lang}<i class="fa-solid fa-check"></i>{/if}
                    </button>
                  {/each}
                {:else}
                  <div class="pop-note">Belum ada subtitle terpisah — subtitle sudah ada di dalam video.</div>
                {/if}
              </div>
            {/if}
          </div>
          <div class="pop-wrap">
            <button class="action-btn" on:click={(e) => togglePop('share', e)}>
              Bagikan
            </button>
            {#if shareOpen}
              <div class="pop" style="top: {popPos.top}px; left: {popPos.left}px; max-height: {popPos.maxH}px">
                <button class="pop-row" on:click={() => { shareOpen = false; shareNative(); }}>
                  <span>Bagikan…</span>
                </button>
                <button class="pop-row" on:click={() => { shareOpen = false; goto('/post?title=' + title.id); }}>
                  <span>Bagikan ke Post</span>
                </button>
                <button class="pop-row" on:click={() => { shareOpen = false; shareCopy(); }}>
                  <span>{copied ? 'Tautan disalin' : 'Salin tautan episode'}</span>
                </button>
              </div>
            {/if}
          </div>
          {#if $user}
            <div class="pop-wrap">
              <button class="action-btn" class:on={partyCode} on:click={(e) => togglePop('party', e)}>
                {partyCode ? 'Party ' + partyCode : 'Nonton Bareng'}
              </button>
              {#if partyOpen}
                <div class="pop" style="top: {popPos.top}px; left: {popPos.left}px; max-height: {popPos.maxH}px">
                  <div class="pop-title">Nonton Bareng</div>
                  {#if !partyCode}
                    <button class="pop-row" on:click={() => createParty()}>
                      <span>Buat Ruang Baru</span>
                    </button>
                    <div class="party-join">
                      <input
                        type="text"
                        placeholder="Kode party (6 karakter)"
                        value={partyJoinCode}
                        maxlength="6"
                        on:input={(e) => (partyJoinCode = e.currentTarget.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 6))}
                      />
                      <button class="btn" on:click={() => joinParty(partyJoinCode)} disabled={partyJoinCode.trim().length !== 6}>
                        Gabung
                      </button>
                    </div>
                    <p class="pop-note">Bagikan kode atau tautan undangan ke temanmu — pastikan semua menonton episode yang sama.</p>
                  {:else}
                    <div class="party-code">{partyCode}</div>
                    <p class="pop-note">Bagikan kode ini — teman gabung dari halaman Nonton episode yang sama.</p>
                    <button class="pop-row" on:click={() => copyParty(partyCode, 'Kode disalin.')}>
                      <span>Salin kode</span>
                    </button>
                    <button
                      class="pop-row"
                      on:click={() => copyParty(window.location.origin + '/nonton/' + epId + '?party=' + partyCode, 'Tautan undangan disalin.')}
                    >
                      <span>Salin tautan undangan</span>
                    </button>
                    {#if partyStatus}<p class="pop-note">Terakhir: {partyStatus}</p>{/if}
                    {#if !isDirect}
                      <p class="pop-note">Sinkron putar hanya untuk video langsung — embed tak bisa dikendalikan dari luar.</p>
                    {/if}
                    {#if partyGone}
                      <p class="pop-note">
                        Koneksi terputus — <button class="linklike" on:click={() => partySocket?.connect()}>sambungkan lagi</button>.
                      </p>
                    {/if}
                    <button class="pop-row" on:click={() => { leaveParty(); partyOpen = false; }}>
                      <span>Keluar dari party</span>
                    </button>
                  {/if}
                </div>
              {/if}
            </div>
          {/if}
          <button class="action-btn" on:click={() => { showReport = true; reportDone = ''; }}>Laporkan</button>
          {#if fsSupported}
            <button class="action-btn" on:click={toggleFullscreen} aria-label={isFullscreen ? 'Keluar dari layar penuh' : 'Layar penuh'}>
              <i class={'fa-solid ' + (isFullscreen ? 'fa-compress' : 'fa-expand')}></i>
              {isFullscreen ? 'Keluar layar penuh' : 'Layar penuh'}
            </button>
          {/if}
        </div>
        {#if serverOpen || subOpen || shareOpen || partyOpen}
          <button
            class="pop-backdrop"
            on:click={() => { serverOpen = false; subOpen = false; shareOpen = false; partyOpen = false; }}
            aria-label="Tutup menu"
          ></button>
        {/if}
        {#if reportDone}<p class="report-done"><i class="fa-solid fa-circle-check"></i> {reportDone}</p>{/if}

        {#if episodes.length}
          <div class="ep-head">
            <h2>Episode</h2>
            <div class="ep-nav">
              <button class="nav-btn" disabled={!prevEp} on:click={() => goToEp(prevEp)}>‹ Prev</button>
              <button class="nav-btn" disabled={!nextEp} on:click={() => goToEp(nextEp)}>Next ›</button>
            </div>
          </div>
          <EpisodeGrid {episodes} currentId={epId} />
        {/if}

        {#if title?.synopsis}
          <button class="syn-card" on:click={() => (descExpanded = !descExpanded)}>
            <div class="syn-head">Sinopsis</div>
            <div class="syn-text" class:clamp={!descExpanded}>{title.synopsis}</div>
            <div class="syn-more">{descExpanded ? 'Tutup' : 'Selengkapnya'}</div>
          </button>
        {/if}

        {#if recommendations.length}
          <Rail title="Rekomendasi" items={recommendations} />
        {/if}
      </div>
    {/if}
  </div>
</div>

{#if showReport}
  <div class="modal-backdrop" on:click={() => (showReport = false)}></div>
  <div class="modal" role="dialog" aria-label="Laporkan Masalah">
    <h2>Laporkan Masalah</h2>
    <p class="muted">Ada masalah dengan episode ini? Ceritakan ke admin biar segera diperbaiki.</p>
    <div class="reasons">
      {#each REPORT_REASONS as [value, label]}
        <label class="reason">
          <input type="radio" name="report-reason" value={value} bind:group={reportReason} />
          <span>{label}</span>
        </label>
      {/each}
    </div>
    <textarea
      placeholder="Catatan (opsional)"
      bind:value={reportNote}
      maxlength="500"
      rows="3"
    ></textarea>
    {#if reportError}<p class="error">{reportError}</p>{/if}
    <div class="modal-actions">
      <button class="btn ghost" on:click={() => (showReport = false)}>Batal</button>
      <button class="btn" on:click={submitReport} disabled={reportSending}>
        {reportSending ? 'Mengirim…' : 'Kirim Laporan'}
      </button>
    </div>
  </div>
{/if}

<style>
  .watch { background: var(--watch-bg); min-height: 100vh; }
  /* Pemutar ala Netflix di desktop (27 Nov 2026): tanpa batas 1280px —
     pemutar mengikuti pinggir 4vw seperti konten lain, jauh lebih besar
     dari sebelumnya (di 1920px: ~1766px, bukan 1248px). */
  .watch-inner { margin: 0 auto; padding: 0 16px 48px; }
  @media (min-width: 768px) { .watch-inner { padding: 0 32px 48px; } }
  @media (min-width: 1280px) { .watch-inner { padding: 0 4vw 48px; } }
  .watch-head { display: flex; align-items: center; gap: 4px; padding: 6px 0; }
  .back-btn {
    background: none; border: 0; color: #fff; font-size: 20px;
    padding: 8px 10px 8px 4px;
  }
  .head-titles { flex: 1; min-width: 0; }
  .brand { color: var(--purple); font-weight: 900; font-size: 17px; line-height: 1.2; }
  .head-sub {
    color: var(--watch-muted); font-size: 13px; line-height: 1.3;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .player-wrap { margin-top: 4px; }
  /* Layar penuh: pembungkus mengisi layar, pemutar di dalamnya ikut
     mengisi (jalan untuk <video> maupun iframe embed). */
  .player-wrap:fullscreen, .player-wrap:-webkit-full-screen { background: #000; display: flex; align-items: center; }
  .player-wrap:fullscreen :global(.player), .player-wrap:-webkit-full-screen :global(.player) {
    height: 100%; aspect-ratio: auto; border-radius: 0;
  }
  @media (max-width: 767px) {
    .player-wrap { margin-left: -16px; margin-right: -16px; }
    .player-wrap :global(.player) { border-radius: 0; }
  }
  .watch-body { max-width: 720px; margin: 0 auto; }
  @media (min-width: 1280px) {
    /* Info + daftar episode di bawah pemutar yang kini lebar: 1080px
       supaya tidak tampak hilang di tengah, tapi sinopsis tetap nyaman
       dibaca (bukan selebar pemutar). */
    .watch-body { max-width: 1080px; }
  }
  .info { padding: 12px 0 0; }
  .watch-title { font-size: 22px; font-weight: 700; margin: 0; line-height: 1.25; }
  .ep-sub { font-size: 15px; margin-top: 2px; }
  .meta { margin-top: 4px; color: var(--watch-muted); font-size: 15px; }
  .genre-pills { display: flex; gap: 6px; margin-top: 8px; overflow-x: auto; padding-bottom: 2px; }
  .genre-pill {
    flex-shrink: 0; background: var(--surface); color: #fff;
    border-radius: 999px; padding: 4px 10px; font-size: 12px;
  }
  .actions-row {
    display: flex; gap: 8px; margin-top: 12px; overflow-x: auto;
    padding: 4px 0; scrollbar-width: none;
  }
  .actions-row::-webkit-scrollbar { display: none; }
  .action-btn {
    flex-shrink: 0; background: var(--surface); color: #fff; border: 0;
    border-radius: 4px; padding: 9px 14px; font-size: 14px; white-space: nowrap;
  }
  .pop-wrap { position: relative; flex-shrink: 0; }
  .pop {
    position: fixed; z-index: 41;
    min-width: 240px; max-width: 320px; background: var(--surface);
    border: 1px solid var(--border); border-radius: 12px; padding: 8px;
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.6);
    max-height: calc(100dvh - 16px); overflow-y: auto;
  }
  .pop-title { font-weight: 700; font-size: 15px; padding: 6px 10px 8px; }
  .pop-row {
    display: flex; justify-content: space-between; align-items: center; gap: 12px;
    width: 100%; text-align: left; background: none; border: 0; color: #fff;
    font-size: 14px; padding: 10px; border-radius: 4px;
  }
  .pop-row:hover { background: rgba(255, 255, 255, 0.08); }
  .pop-row .fa-check { color: var(--purple); }
  .pop-note { font-size: 13px; color: var(--watch-muted); padding: 6px 10px 10px; line-height: 1.5; }
  .pop-backdrop { position: fixed; inset: 0; background: transparent; border: 0; z-index: 40; }
  .report-done { color: #4ade80; font-size: 14px; margin: 8px 0 0; }
  .action-btn.on { background: var(--purple); }
  .party-code { font-size: 28px; font-weight: 800; letter-spacing: 6px; text-align: center; padding: 10px 0 2px; }
  .party-join { display: flex; gap: 8px; padding: 6px 10px 10px; }
  .party-join input {
    flex: 1; min-width: 0; background: var(--panel); border: 1px solid var(--border);
    border-radius: 4px; color: var(--text); font: inherit; font-size: 14px;
    padding: 9px 10px; text-transform: uppercase; letter-spacing: 2px;
  }
  .linklike { background: none; border: 0; padding: 0; color: var(--purple); font: inherit; cursor: pointer; }
  .ep-head { display: flex; align-items: center; margin-top: 14px; }
  .ep-head h2 { flex: 1; font-size: 19px; font-weight: 700; margin: 0; }
  .ep-nav { display: flex; gap: 2px; }
  .nav-btn { background: none; border: 0; color: #fff; font-size: 15px; padding: 6px 8px; }
  .nav-btn:disabled { color: var(--watch-muted); }
  .syn-card {
    display: block; width: 100%; text-align: left; background: var(--surface);
    border: 0; border-radius: 12px; padding: 14px; margin-top: 14px; color: #fff;
  }
  .syn-head { font-weight: 700; font-size: 15px; margin-bottom: 4px; }
  .syn-text { color: var(--watch-muted); font-size: 15px; line-height: 1.6; }
  .syn-text.clamp {
    display: -webkit-box; -webkit-line-clamp: 3; line-clamp: 3;
    -webkit-box-orient: vertical; overflow: hidden;
  }
  .syn-more { color: var(--purple); font-size: 14px; font-weight: 600; margin-top: 6px; }
  .lock { margin-top: 16px; }
  .lock h1 { font-size: 22px; margin: 0 0 8px; }
  .actions { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 16px; }
  .modal-backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.7); z-index: 70; }
  .modal {
    position: fixed; z-index: 71; left: 50%; top: 50%; transform: translate(-50%, -50%);
    width: min(480px, calc(100vw - 32px)); max-height: calc(100vh - 48px); overflow-y: auto;
    background: var(--panel); border: 1px solid var(--border); border-radius: 14px; padding: 20px;
  }
  .modal h2 { margin: 0 0 6px; font-size: 19px; }
  .modal p { margin: 0 0 12px; font-size: 14px; }
  .reasons { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; }
  .reason { display: flex; align-items: center; gap: 10px; font-size: 15px; padding: 6px 0; cursor: pointer; }
  .reason input { accent-color: var(--purple); width: 18px; height: 18px; }
  .modal textarea {
    width: 100%; background: var(--panel-2); border: 1px solid var(--border);
    border-radius: 4px; color: #fff; padding: 12px; font-size: 15px; resize: vertical;
    font-family: inherit;
  }
  .modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 14px; }
</style>
