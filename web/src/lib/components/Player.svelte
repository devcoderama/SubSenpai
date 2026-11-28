<script>
  import { createEventDispatcher, onDestroy } from 'svelte';
  import Hls from 'hls.js';

  // Pemutar ala aplikasi: server aktif dipegang halaman Nonton
  // (tombol "Server: …" di bawah pemutar butuh labelnya), begitu juga
  // pilihan subtitle. Komponen ini hanya me-render video/iframe-nya.
  /** @type {{ url: string, label?: string, userAgent?: string }[]} */
  export let sources = [];
  export let current = 0;
  export let title = '';
  export let subtitles = /** @type {{ lang: string, file_url: string, status: string }[]} */ ([]); // subtitle siap pakai {lang, file_url, status}
  export let selectedSub = /** @type {string | null} */ (null); // lang subtitle terpilih, atau null
  export let resumeAt = 0; // detik — posisi tersimpan untuk video langsung (iframe tak bisa di-seek)
  export let syncAction = null; // {action, position, at} — perintah sinkron dari ruang party (video langsung saja)
  export let mixedContentNote = ''; // catatan bila stream http:// diblokir mixed content di situs https (TV Langsung)

  const dispatch = createEventDispatcher();
  let videoEl = null;
  let iframeEl = null;
  let blocked = false;
  let autoAdvanced = false;
  let resumed = false; // seek resume sudah dilakukan sekali per episode

  $: src = sources?.[current];
  $: isDirect = !!src?.url && /\.(mp4|m3u8|webm)(\?|$)/i.test(src.url);
  $: isHls = isDirect && /\.m3u8(\?|$)/i.test(src?.url || '');
  // MPEG-DASH (.mpd): peramban tidak bisa memutarnya tanpa pustaka
  // khusus — kanal DASH hanya jalan di aplikasi (ExoPlayer).
  // Tanpa ini, URL .mpd jatuh ke cabang iframe dan tampil rusak.
  $: isDash = !!src?.url && /\.mpd(\?|$)/i.test(src.url);
  // Mixed content: situs ini https, stream http:// diblokir peramban
  // sebelum sempat diputar — tampilkan catatan, bukan spinner rusak.
  $: isMixedContent =
    !!src?.url && /^http:\/\//i.test(src.url) && typeof location !== 'undefined' && location.protocol === 'https:';
  // HLS native hanya Safari; Chromium (Chrome/Brave) butuh hls.js
  // karena <video> polos tak bisa memutar .m3u8.
  let nativeHls = false;
  $: if (videoEl) nativeHls = !!videoEl.canPlayType('application/vnd.apple.mpegurl');
  $: useHls = isHls && !nativeHls && Hls.isSupported();
  let hls = /** @type {import('hls.js').default | null} */ (null);
  let hlsForUrl = '';
  let hlsError = '';
  let nativeError = false;

  function destroyHls() {
    if (hls) {
      hls.destroy();
      hls = null;
    }
  }
  onDestroy(destroyHls);

  function attachHls() {
    destroyHls();
    if (!videoEl || !src?.url) return;
    // Live stream lebih toleran (28 Nov 2026): manifest/varian dimuat
    // ulang lebih sering di jaringan goyah sebelum hls.js menyerah.
    const h = new Hls({
      manifestLoadingMaxRetry: 3,
      levelLoadingMaxRetry: 6,
      fragLoadingMaxRetry: 6,
      xhrSetup(xhr) {
        // User-Agent kanal (mis. CNN/CNBC/ANTV) diteruskan bila ada.
        // CATATAN: User-Agent adalah forbidden header di peramban —
        // Chrome/Brave mengabaikannya diam-diam, jadi stream yang
        // benar-benar menolak tanpa UA tetap gagal. Referer juga tidak
        // bisa disetel dari JS sama sekali (lihat laporan TV).
        if (src.userAgent) {
          try {
            xhr.setRequestHeader('User-Agent', src.userAgent);
          } catch {
            /* abaikan — header terlarang */
          }
        }
      }
    });
    // Kegagalan segmen sesaat (timeout/putus di tengah jalan) itu WAJAR
    // di live stream — JANGAN langsung menyerah. Pola standar hls.js:
    // NETWORK_ERROR fatal -> startLoad() lagi (maks 3x beruntun, lalu
    // menyerah), MEDIA_ERROR fatal -> recoverMediaError() (maks 2x).
    // Tanpa ini, satu segmen gagal = pemutar langsung mati + panel
    // error: Trans TV/CNN mati di web 18:52 padahal segmennya sehat
    // (200, ~450 KB dari VM) — CNN sempat jalan 10-30 detik dulu.
    // Hitungan gagal di-reset tiap ada segmen yang berhasil masuk.
    let netRetries = 0;
    let mediaRetries = 0;
    h.on(Hls.Events.FRAG_LOADED, () => {
      netRetries = 0;
      mediaRetries = 0;
    });
    h.on(Hls.Events.ERROR, (_e, data) => {
      if (!data?.fatal) return;
      if (data.type === Hls.ErrorTypes.NETWORK_ERROR && netRetries < 3) {
        netRetries += 1;
        h.startLoad();
        return;
      }
      if (data.type === Hls.ErrorTypes.MEDIA_ERROR && mediaRetries < 2) {
        mediaRetries += 1;
        h.recoverMediaError();
        return;
      }
      hlsError =
        data.type === Hls.ErrorTypes.NETWORK_ERROR
          ? 'Stream tidak bisa dimuat — kanal mungkin sedang mati.'
          : 'Pemutar gagal memutar stream ini.';
      destroyHls();
      hlsForUrl = '';
    });
    h.on(Hls.Events.MANIFEST_PARSED, () => {
      videoEl?.play().catch(() => {});
    });
    h.loadSource(src.url);
    h.attachMedia(videoEl);
    hls = h;
    hlsForUrl = src.url;
  }

  // Elemen video dibuat ulang tiap ganti URL ({#key src.url}):
  // pasang/lepas hls.js mengikuti elemen + URL aktif.
  $: if (videoEl && isDirect && src?.url) {
    if (useHls) {
      if (hlsForUrl !== src.url) attachHls();
    } else {
      destroyHls();
      hlsForUrl = '';
    }
  }

  function retryPlay() {
    hlsError = '';
    nativeError = false;
    hlsForUrl = '';
  }
  /** @type {{ lang: string, file_url: string, status: string }[]} */
  let readySubs = [];
  $: readySubs = (subtitles || []).filter((s) => s.status === 'done' && s.file_url);

  // Ganti episode -> mulai lagi dari server pertama.
  $: if (sources) {
    current = 0;
    autoAdvanced = false;
    resumed = false;
  }
  // Ganti server -> kartu blokir + pesan error ikut direset.
  $: if (src?.url) {
    blocked = false;
    hlsError = '';
    nativeError = false;
  }

  // Terapkan pilihan subtitle ke video langsung tanpa memutar ulang:
  // tiap <track> diaktifkan/dimatikan lewat TextTrack.mode. Tanpa
  // crossorigin di <video> (agar pemutaran mp4 tak bergantung header
  // CORS host video); track yang host-nya tak mengizinkan CORS gagal
  // diam-diam dan video tetap jalan.
  function applySubTrack() {
    if (!videoEl) return;
    const sel = (selectedSub || '').toLowerCase();
    const tracks = videoEl.textTracks;
    for (let i = 0; i < tracks.length; i++) {
      const want = sel && (readySubs[i]?.lang || '').toLowerCase() === sel;
      tracks[i].mode = want ? 'showing' : 'disabled';
    }
  }
  $: if (videoEl && selectedSub !== undefined) applySubTrack();

  // Posisi video langsung dilaporkan ke halaman Nonton untuk riwayat,
  // beserta durasi video (satu-satunya sumber durasi yang pasti —
  // episode.duration_sec di API selalu kosong).
  function onTime(e) {
    dispatch('progress', { position: e.target.currentTime || 0, duration: e.target.duration || 0 });
  }

  // Seek sekali ke posisi tersimpan saat metadata video langsung termuat.
  // Tak berlaku untuk iframe (lintas-origin tak bisa di-seek).
  function seekResume() {
    if (resumed || !videoEl) return;
    const dur = videoEl.duration;
    if (resumeAt > 5 && (!Number.isFinite(dur) || resumeAt < dur - 10)) {
      suppressSeeked = true;
      videoEl.currentTime = resumeAt;
      resumed = true;
      dispatch('resumed', { position: resumeAt });
    }
  }

  function onMeta() {
    applySubTrack();
    seekResume();
  }

  // --- Nonton Bareng (#8): sinkron putar video langsung ---
  // Perintah dari ruang party (play/pause/seek) diterapkan ke <video>.
  // Toleransi 2 detik seperti aplikasi (PlayerScreen.kt). applyingRemote
  // menahan event lokal supaya tidak diteruskan balik ke ruang (gema).
  let applyingRemote = false;
  let suppressSeeked = false; // seek resume sekali — bukan lompatan pengguna
  function applySync(a) {
    if (!videoEl || !isDirect || !a) return;
    applyingRemote = true;
    if ((a.action === 'seek' || a.action === 'play') && Math.abs(videoEl.currentTime - (a.position || 0)) > 2) {
      videoEl.currentTime = a.position || 0;
    }
    if (a.action === 'play') videoEl.play().catch(() => {});
    else if (a.action === 'pause') videoEl.pause();
    setTimeout(() => (applyingRemote = false), 500);
  }
  $: if (syncAction) applySync(syncAction);

  function onPlay() {
    if (!applyingRemote) dispatch('playstate', { action: 'play', position: videoEl?.currentTime || 0 });
  }
  function onPause() {
    if (!applyingRemote) dispatch('playstate', { action: 'pause', position: videoEl?.currentTime || 0 });
  }
  function onSeeked() {
    if (suppressSeeked) {
      suppressSeeked = false;
      return;
    }
    if (!applyingRemote) dispatch('playstate', { action: 'seek', position: videoEl?.currentTime || 0 });
  }

  // Server diblokir: buka langsung di situs sumber — di TAB YANG SAMA
  // (27 Nov 2026: semua tab baru dimatikan untuk mencegah iklan).
  // Tombol Kembali browser mengembalikan ke halaman Nonton.
  function openSource() {
    if (src?.url) window.location.href = src.url;
  }

  function nextSource() {
    if (!sources?.length) return;
    current = (current + 1) % sources.length;
  }

  // Frame yang ditolak X-Frame-Options punya contentWindow null saat
  // load (Chrome/Firefox) — sementara frame lintas-origin yang normal
  // tetap punya contentWindow (tak bisa dibaca, tapi tidak null).
  function onFrameLoad() {
    if (iframeEl && !iframeEl.contentWindow) {
      blocked = true;
      // Sekali saja per episode: langsung loncat ke server berikut
      // supaya video tetap jalan tanpa perlu diklik pengguna.
      if (!autoAdvanced && sources.length > 1) {
        autoAdvanced = true;
        nextSource();
      }
    }
  }
</script>

{#if !sources?.length}
  <p class="muted">Tidak ada server untuk episode ini.</p>
{:else}
  <div class="player">
    {#key src.url}
      {#if isMixedContent || isDash}
        <div class="blocked">
          <strong>Channel ini hanya bisa diputar di aplikasi.</strong>
          <p>
            {#if isDash}
              Stream kanal ini memakai MPEG-DASH — peramban tidak bisa memutarnya, hanya aplikasi Android yang bisa.
            {:else}
              {mixedContentNote ||
                'Stream kanal ini masih http://, sedangkan situs ini https:// — peramban memblokirnya sebelum sempat diputar.'}
            {/if}
          </p>
          <p class="muted small">Coba channel lain, atau pasang aplikasi Android SubSenpai (unduh di menu Lainnya).</p>
        </div>
      {:else if isDirect}
        {#if hlsError || nativeError}
          <div class="blocked">
            <strong>{hlsError || 'Video tidak bisa diputar.'}</strong>
            <p>Stream ini tidak stabil — kadang mati. Coba lagi, atau pindah ke channel lain.</p>
            <div class="blocked-actions">
              <button class="btn" on:click={retryPlay}>Coba lagi</button>
            </div>
          </div>
        {:else}
          <!-- svelte-ignore a11y_media_has_caption -->
          <video
            src={useHls ? null : src.url}
            controls
            autoplay
            playsinline
            bind:this={videoEl}
            on:timeupdate={onTime}
            on:loadedmetadata={onMeta}
            on:play={onPlay}
            on:pause={onPause}
            on:seeked={onSeeked}
            on:error={() => {
              if (!useHls) nativeError = true;
            }}
          >
            {#each readySubs as s}
              <track
                kind="subtitles"
                srclang={s.lang}
                label={s.lang.toUpperCase()}
                src={s.file_url}
                default={!!selectedSub && s.lang.toLowerCase() === selectedSub.toLowerCase()}
              />
            {/each}
          </video>
        {/if}
      {:else if blocked}
        <div class="blocked">
          <strong>Server ini tidak bisa diputar di dalam halaman web.</strong>
          <p>
            Pemutar {src.label || ''} menolak dibuka di dalam frame dari situs lain — di aplikasi ia
            dibuka langsung sebagai halaman, jadi lolos. Buka langsung di situs sumber, atau coba server lain.
          </p>
          <div class="blocked-actions">
            <button class="btn" on:click={openSource}>Buka di situs sumber <i class="fa-solid fa-arrow-up-right-from-square"></i></button>
            <button class="btn ghost" on:click={nextSource}>Coba server lain</button>
          </div>
        </div>
      {:else}
        <iframe
          title={title || 'Pemutar video'}
          src={src.url}
          allow="autoplay; fullscreen; encrypted-media; picture-in-picture"
          referrerpolicy="no-referrer"
          bind:this={iframeEl}
          on:load={onFrameLoad}
        ></iframe>
      {/if}
    {/key}
  </div>
{/if}

<style>
  .player {
    position: relative; width: 100%; aspect-ratio: 16 / 9;
    background: #000; border-radius: 4px; overflow: hidden;
  }
  .player video, .player iframe { width: 100%; height: 100%; border: 0; display: block; }
  .blocked {
    width: 100%; height: 100%; display: flex; flex-direction: column;
    align-items: center; justify-content: center; gap: 10px;
    text-align: center; padding: 20px; background: var(--panel);
  }
  .blocked p { color: var(--muted); font-size: 14px; line-height: 1.55; margin: 0; max-width: 420px; }
  .blocked .small { font-size: 13px; }
  .blocked-actions { display: flex; gap: 10px; flex-wrap: wrap; justify-content: center; }
</style>
