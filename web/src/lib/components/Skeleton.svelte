<script>
  // Skeleton loader dengan shimmer — bentuknya meniru konten yang
  // akan tampil (kartu poster, hero, baris, postingan, paket, player).
  export let kind = 'card'; // card | hero | row | post | plan | player | detail
  export let count = 1;
</script>

{#if kind === 'hero'}
  <div class="skeleton sk-hero" aria-hidden="true">
    <div class="sk-hero-lines">
      <div class="skeleton sk-line"></div>
      <div class="skeleton sk-line"></div>
      <div class="skeleton sk-line short"></div>
    </div>
  </div>
{:else if kind === 'player'}
  <div class="skeleton sk-player" aria-hidden="true"></div>
{:else if kind === 'detail'}
  <div class="sk-detail" aria-hidden="true">
    <div class="skeleton sk-detail-poster"></div>
    <div class="sk-detail-info">
      <div class="skeleton sk-line" style="width: 70%; height: 28px;"></div>
      <div class="skeleton sk-line short"></div>
      <div class="skeleton sk-line" style="width: 90%;"></div>
      <div class="skeleton sk-line" style="width: 85%;"></div>
      <div class="skeleton sk-line" style="width: 60%;"></div>
      <div><span class="skeleton sk-btn"></span><span class="skeleton sk-btn"></span></div>
    </div>
  </div>
{:else}
  {#each Array.from({ length: count }) as _}
    {#if kind === 'card'}
      <div class="sk-card" aria-hidden="true">
        <div class="skeleton sk-poster"></div>
        <div class="skeleton sk-line" style="width: 82%;"></div>
        <div class="skeleton sk-line short"></div>
      </div>
    {:else if kind === 'row'}
      <div class="sk-row" aria-hidden="true">
        <div class="skeleton sk-thumb"></div>
        <div class="sk-row-lines">
          <div class="skeleton sk-line" style="width: 70%;"></div>
          <div class="skeleton sk-line short"></div>
        </div>
      </div>
    {:else if kind === 'post'}
      <div class="sk-post" aria-hidden="true">
        <div class="sk-post-head">
          <div class="skeleton sk-avatar"></div>
          <div class="sk-row-lines">
            <div class="skeleton sk-line" style="width: 40%;"></div>
            <div class="skeleton sk-line short"></div>
          </div>
        </div>
        <div class="skeleton sk-line" style="width: 95%;"></div>
        <div class="skeleton sk-line" style="width: 70%;"></div>
        <div class="skeleton sk-post-img"></div>
      </div>
    {:else if kind === 'plan'}
      <div class="skeleton sk-plan" aria-hidden="true"></div>
    {/if}
  {/each}
{/if}
