<script>
  import { createEventDispatcher } from 'svelte';

  export let sources = [];
  export let title = '';

  const dispatch = createEventDispatcher();
  let current = 0;
  $: src = sources?.[current];
  $: isDirect = !!src?.url && /\.(mp4|m3u8|webm)(\?|$)/i.test(src.url);

  // Posisi video langsung dilaporkan ke halaman Nonton untuk riwayat.
  function onTime(e) {
    dispatch('progress', { position: e.target.currentTime || 0 });
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
        <video src={src.url} controls autoplay playsinline on:timeupdate={onTime}></video>
      {:else}
        <iframe
          title={title || 'Pemutar video'}
          src={src.url}
          allowfullscreen
          allow="autoplay; fullscreen; encrypted-media; picture-in-picture"
          sandbox="allow-scripts allow-same-origin allow-forms allow-presentation"
        ></iframe>
      {/if}
    {/key}
  </div>
  <p class="muted hint">
    Server {current + 1} dari {sources.length}
    {#if !isDirect}— pemutar embed dari sumber; kalau macet, coba server lain.{/if}
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
</style>
