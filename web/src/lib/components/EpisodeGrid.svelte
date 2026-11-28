<script>
  import { tick } from 'svelte';

  // Pemilih episode halaman Nonton — grid 5 kolom ala aplikasi
  // (EpisodeSelector di PlayerScreen.kt): maks 190px + scroll vertikal
  // sendiri, langsung melompat ke episode yang sedang diputar, sel
  // aktif ungu, episode terkunci menuju halaman Premium.
  export let episodes = [];
  export let currentId = '';

  let gridEl = null;
  let lastId = '';
  // Serial TV: grid disaring per season (S1E1 dan S2E1 sama-sama
  // "Ep 1" bila digabung). Satu season = tampil persis seperti dulu.
  let selectedSeason = 0;
  $: seasons = [...new Set((episodes || []).map((e) => e.season_no || 1))].sort((a, b) => a - b);
  $: curSeason = (episodes || []).find((e) => e.id === currentId)?.season_no || 1;
  $: shownEpisodes =
    seasons.length > 1 && selectedSeason
      ? (episodes || []).filter((e) => (e.season_no || 1) === selectedSeason)
      : episodes || [];

  async function scrollToCurrent() {
    await tick();
    if (!gridEl) return;
    const active = gridEl.querySelector('.active');
    if (active) active.scrollIntoView({ block: 'nearest' });
  }

  $: if (currentId && currentId !== lastId) {
    lastId = currentId;
    const s = (episodes || []).find((e) => e.id === currentId)?.season_no;
    if (s && seasons.length > 1) selectedSeason = s;
    scrollToCurrent();
  }
  $: if (seasons.length > 1 && selectedSeason && !seasons.includes(selectedSeason)) {
    selectedSeason = curSeason;
  }
</script>

{#if episodes?.length}
  {#if seasons.length > 1}
    <div class="season-row">
      {#each seasons as s}
        <button class="season-chip" class:active={selectedSeason === s} on:click={() => (selectedSeason = s)}>
          S{s}
        </button>
      {/each}
    </div>
  {/if}
  <div class="ep-grid" bind:this={gridEl}>
    {#each shownEpisodes as ep (ep.id)}
      {#if ep.locked}
        <a class="cell" class:active={ep.id === currentId} href="/premium" title="Khusus Premium">
          <i class="fa-solid fa-lock"></i> Ep {ep.episode_no}
        </a>
      {:else}
        <a class="cell" class:active={ep.id === currentId} href={'/nonton/' + ep.id}>
          Ep {ep.episode_no}
        </a>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .season-row { display: flex; gap: 8px; overflow-x: auto; margin-bottom: 8px; padding-bottom: 4px; }
  .season-chip {
    border: 1px solid var(--border); border-radius: 999px;
    padding: 6px 14px; font-size: 13px; color: var(--muted);
    background: transparent; white-space: nowrap; cursor: pointer;
  }
  .season-chip.active { background: var(--purple); color: #fff; border-color: var(--purple); font-weight: 600; }
  .ep-grid {
    display: grid; grid-template-columns: repeat(5, 1fr); gap: 8px;
    max-height: 190px; overflow-y: auto; padding: 4px 0;
    scrollbar-width: thin;
  }
  .cell {
    display: block; text-align: center; white-space: nowrap; overflow: hidden;
    text-overflow: ellipsis; background: var(--surface); color: #fff;
    border-radius: 4px; padding: 8px 4px; font-size: 14px;
  }
  .cell:hover { text-decoration: none; background: #232328; }
  .cell.active { background: var(--purple); font-weight: 700; }
  .cell .fa-lock { font-size: 11px; margin-right: 2px; }
</style>
