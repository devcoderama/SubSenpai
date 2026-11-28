<script>
  import { createEventDispatcher } from 'svelte';

  // Pemutar ala aplikasi: server aktif dipegang halaman Nonton
  // (tombol "Server: …" di bawah pemutar butuh labelnya), begitu juga
  // pilihan subtitle. Komponen ini hanya me-render video/iframe-nya.
  export let sources = [];
  export let current = 0;
  export let title = '';
  export let subtitles = /** @type {{ lang: string, file_url: string, status: string }[]} */ ([]); // subtitle siap pakai {lang, file_url, status}
  export let selectedSub = /** @type {string | null} */ (null); // lang subtitle terpilih, atau null
  export let resumeAt = 0; // detik — posisi tersimpan untuk video langsung (iframe tak bisa di-seek)

  const dispatch = createEventDispatcher();
  let videoEl = null;
  let iframeEl = null;
  let blocked = false;
  let autoAdvanced = false;
  let resumed = false; // seek resume sudah dilakukan sekali per episode

  $: src = sources?.[current];
  $: isDirect = !!src?.url && /\.(mp4|m3u8|webm)(\?|$)/i.test(src.url);
  /** @type {{ lang: string, file_url: string, status: string }[]} */
  let readySubs = [];
  $: readySubs = (subtitles || []).filter((s) => s.status === 'done' && s.file_url);

  // Ganti episode -> mulai lagi dari server pertama.
  $: if (sources) {
    current = 0;
    autoAdvanced = false;
    resumed = false;
  }
  // Ganti server -> kartu blokir ikut direset.
  $: if (src?.url) blocked = false;

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
      videoEl.currentTime = resumeAt;
      resumed = true;
      dispatch('resumed', { position: resumeAt });
    }
  }

  function onMeta() {
    applySubTrack();
    seekResume();
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
      {#if isDirect}
        <!-- svelte-ignore a11y_media_has_caption -->
        <video
          src={src.url}
          controls
          autoplay
          playsinline
          bind:this={videoEl}
          on:timeupdate={onTime}
          on:loadedmetadata={onMeta}
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
  .blocked-actions { display: flex; gap: 10px; flex-wrap: wrap; justify-content: center; }
</style>
