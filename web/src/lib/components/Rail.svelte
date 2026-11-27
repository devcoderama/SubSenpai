<script>
  import TitleCard from './TitleCard.svelte';

  export let title;
  export let items = [];
  export let moreHref = null;
</script>

{#if items?.length}
  <section class="rail-section">
    <div class="rail-head">
      <h2>{title}</h2>
      {#if moreHref}<a href={moreHref} class="muted more">Lihat semua ›</a>{/if}
    </div>
    <div class="rail">
      {#each items as item (item.id)}
        <TitleCard {item} />
      {/each}
    </div>
  </section>
{/if}

<style>
  .rail-section { margin: 28px 0; }
  .rail-head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 12px; }
  .rail-head h2 { margin: 0; font-size: 19px; }
  .more { font-size: 13px; }
</style>
