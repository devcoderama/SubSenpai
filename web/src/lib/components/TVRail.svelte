<script>
  import TVChannelCard from './TVChannelCard.svelte';
  import { reveal } from '$lib/reveal.js';

  // Rel horizontal kanal TV Langsung di Beranda — pola yang sama
  // dengan Rail.svelte, tapi kartunya kanal TV (bukan TitleCard).
  /** @type {any[]} */
  export let channels = [];
  export let title = 'TV Langsung';
  export let moreHref = '/tv';
</script>

{#if channels?.length}
  <section class="tv-rail-section" use:reveal>
    <div class="tv-rail-head">
      <h2>{title}</h2>
      {#if moreHref}<a href={moreHref} class="muted more">Lihat semua <i class="fa-solid fa-chevron-right"></i></a>{/if}
    </div>
    <div class="tv-rail">
      {#each channels as ch (ch.id)}
        <div class="tv-rail-item">
          <TVChannelCard channel={ch} />
        </div>
      {/each}
    </div>
  </section>
{/if}

<style>
  .tv-rail-section {
    margin: 28px 0;
  }
  @media (min-width: 1280px) {
    .tv-rail-section {
      margin: 36px 0;
    }
  }
  .tv-rail-head {
    display: flex;
    align-items: baseline;
    gap: 12px;
    margin-bottom: 12px;
  }
  .tv-rail-head h2 {
    margin: 0;
    font-size: 19px;
  }
  .more {
    font-size: 13px;
  }
  .more .fa-chevron-right {
    font-size: 11px;
    margin-left: 2px;
  }
  .tv-rail {
    display: flex;
    gap: 12px;
    overflow-x: auto;
    padding: 8px 0 12px;
    scrollbar-width: thin;
  }
  .tv-rail-item {
    flex: 0 0 auto;
    width: 200px;
  }
  @media (min-width: 900px) {
    .tv-rail-item {
      width: 220px;
    }
  }
</style>
