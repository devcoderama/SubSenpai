<script>
  import { goto } from '$app/navigation';
  import { posterUrl } from '$lib/api.js';

  // Kartu kanal TV Langsung: dipakai rel Beranda (TVRail) + grid /tv.
  // Klik -> /tv?channel=<slug> (halaman TV memutarnya di Player).
  /** @type {{ id: string, name: string, slug: string, logo_url?: string | null, stream_url: string, country: string, category: string, quality?: string | null, stability: string, source: string, web_ok?: boolean } | null} */
  export let channel = null;

  /** @type {string} */
  export let href = '';

  $: link = href || (channel ? '/tv?channel=' + channel.slug : '/tv');

  function open() {
    goto(link);
  }
</script>

{#if channel}
  <button class="tv-card" on:click={open} aria-label={'Putar ' + channel.name}>
    <span class="logo">
      {#if channel.logo_url}
        <img src={posterUrl(channel.logo_url)} alt={channel.name} loading="lazy" />
      {:else}
        <span class="no-logo"><i class="fa-solid fa-tv"></i></span>
      {/if}
      {#if channel.quality}
        <span class="quality">{channel.quality}</span>
      {/if}
    </span>
    <span class="meta">
      <span class="name">{channel.name}</span>
      {#if channel.stability === 'bagus'}
        <span class="badge ok"><i class="fa-solid fa-circle-check"></i> Resmi</span>
      {:else}
        <span class="badge warn"><i class="fa-solid fa-triangle-exclamation"></i> Tidak stabil</span>
      {/if}
      {#if channel.web_ok === false}
        <span class="badge app"><i class="fa-solid fa-mobile-screen"></i> Aplikasi saja</span>
      {/if}
    </span>
  </button>
{/if}

<style>
  .tv-card {
    all: unset;
    display: block;
    box-sizing: border-box;
    width: 100%;
    cursor: pointer;
    border: 1px solid var(--border);
    border-radius: 4px;
    overflow: hidden;
    background: var(--panel);
    transition: transform 0.15s, border-color 0.15s;
  }
  .tv-card:hover {
    transform: scale(1.03);
    border-color: #555;
  }
  .logo {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    aspect-ratio: 16 / 9;
    background: #000;
    padding: 10px;
  }
  .logo img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }
  .no-logo {
    color: var(--muted);
    font-size: 28px;
  }
  .quality {
    position: absolute;
    top: 6px;
    right: 6px;
    background: rgba(0, 0, 0, 0.75);
    color: #fff;
    font-size: 11px;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 4px;
  }
  .meta {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px 10px 10px;
  }
  .name {
    font-size: 14px;
    font-weight: 600;
    line-height: 1.3;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .badge {
    align-self: flex-start;
    font-size: 11px;
    font-weight: 700;
    padding: 2px 7px;
    border-radius: 4px;
  }
  .badge.ok {
    background: rgba(52, 211, 153, 0.15);
    color: #34d399;
  }
  .badge.warn {
    background: rgba(245, 197, 24, 0.15);
    color: var(--gold);
  }
  .badge.app {
    background: rgba(139, 92, 246, 0.15);
    color: #a78bfa;
  }
</style>
