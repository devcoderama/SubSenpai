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

  async function scrollToCurrent() {
    await tick();
    if (!gridEl) return;
    const active = gridEl.querySelector('.active');
    if (active) active.scrollIntoView({ block: 'nearest' });
  }

  $: if (currentId && currentId !== lastId) {
    lastId = currentId;
    scrollToCurrent();
  }
</script>

{#if episodes?.length}
  <div class="ep-grid" bind:this={gridEl}>
    {#each episodes as ep (ep.id)}
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
  .ep-grid {
    display: grid; grid-template-columns: repeat(5, 1fr); gap: 8px;
    max-height: 190px; overflow-y: auto; padding: 4px 0;
    scrollbar-width: thin;
  }
  .cell {
    display: block; text-align: center; white-space: nowrap; overflow: hidden;
    text-overflow: ellipsis; background: var(--surface); color: #fff;
    border-radius: 10px; padding: 8px 4px; font-size: 14px;
  }
  .cell:hover { text-decoration: none; background: #232328; }
  .cell.active { background: var(--purple); font-weight: 700; }
  .cell .fa-lock { font-size: 11px; margin-right: 2px; }
</style>
