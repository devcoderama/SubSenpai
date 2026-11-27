<script>
  import { createEventDispatcher } from 'svelte';

  export let sources = [];
  export let title = '';

  const dispatch = createEventDispatcher();
  let current = 0;
  let iframeEl = null;
  let blocked = false;
  let autoAdvanced = false;

  $: src = sources?.[current];
  $: isDirect = !!src?.url && /\.(mp4|m3u8|webm)(\?|$)/i.test(src.url);

  // Ganti episode -> mulai lagi dari server pertama.
  $: if (sources) {
    current = 0;
    autoAdvanced = false;
  }
  // Ganti server -> kartu blokir ikut direset.
  $: if (src?.url) blocked = false;

  // Posisi video langsung dilaporkan ke halaman Nonton untuk riwayat.
  function onTime(e) {
    dispatch('progress', { position: e.target.currentTime || 0 });
  }

  function openExternal() {
    if (src?.url) window.open(src.url, '_blank', 'noopener');
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
  <div class="servers">
    {#each sources as s, i}
      <button class:active={i === current} on:click={() => (current = i)}>
        {s.label || 'Server ' + (i + 1)}{s.lang ? ' • ' + s.lang : ''}
      </button>
    {/each}
  </div>
  <div class="player">
    {#key src.url}
      {#if isDirect}
        <!-- Subtitle ditangani sumber (hardsub/softsub), bukan track video: -->
        <!-- svelte-ignore a11y_media_has_caption -->
        <video
          src={src.url}
          controls
          autoplay
          playsinline
          on:timeupdate={onTime}
        ></video>
      {:else if blocked}
        <div class="blocked">
          <strong>Server ini tidak bisa diputar di dalam halaman web.</strong>
          <p>
            Pemutar {src.label || ''} menolak dibuka di dalam frame dari situs lain — di aplikasi ia
            dibuka langsung sebagai halaman, jadi lolos. Buka di tab baru, atau coba server lain.
          </p>
          <div class="blocked-actions">
            <button class="btn" on:click={openExternal}>Buka di tab baru ↗</button>
            <button class="btn ghost" on:click={nextSource}>Coba server lain</button>
          </div>
        </div>
      {:else}
        <iframe
          title={title || 'Pemutar video'}
          src={src.url}
          allowfullscreen
          allow="autoplay; fullscreen; encrypted-media; picture-in-picture"
          sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
          referrerpolicy="no-referrer"
          bind:this={iframeEl}
          on:load={onFrameLoad}
        ></iframe>
      {/if}
    {/key}
  </div>
  <p class="muted hint">
    Server {current + 1} dari {sources.length}
    {#if !isDirect}
      — pemutar embed dari sumber; kalau macet, coba server lain atau
      <button class="linklike" on:click={openExternal}>buka di tab baru ↗</button>.
    {/if}
  </p>
{/if}

<style>
  .servers { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; }
  .servers button {
    background: var(--panel); border: 1px solid var(--border); color: #fff;
    border-radius: 999px; padding: 7px 14px; font-size: 13px;
  }
  .servers button.active { background: var(--red); border-color: var(--red); font-weight: 600; }
  .player {
    position: relative; width: 100%; aspect-ratio: 16 / 9;
    background: #000; border-radius: 10px; overflow: hidden;
  }
  .player video, .player iframe { width: 100%; height: 100%; border: 0; display: block; }
  .hint { font-size: 13px; margin-top: 10px; }
  .linklike {
    background: none; border: 0; padding: 0; color: #fff;
    font-size: 13px; text-decoration: underline; cursor: pointer;
  }
  .blocked {
    width: 100%; height: 100%; display: flex; flex-direction: column;
    align-items: center; justify-content: center; gap: 10px;
    text-align: center; padding: 20px; background: var(--panel);
  }
  .blocked p { color: var(--muted); font-size: 14px; line-height: 1.55; margin: 0; max-width: 420px; }
  .blocked-actions { display: flex; gap: 10px; flex-wrap: wrap; justify-content: center; }
</style>
