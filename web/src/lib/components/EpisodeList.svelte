<script>
  export let episodes = [];
  export let titleId = '';
</script>

{#if episodes?.length}
  <section>
    <h2>Daftar Episode ({episodes.length})</h2>
    <div class="ep-grid">
      {#each episodes as ep (ep.id)}
        {#if ep.locked}
          <a class="ep locked" href={'/judul/' + titleId} title="Khusus Premium">
            <span class="num">🔒 {ep.episode_no}</span>
            <span class="name">{ep.title || 'Episode ' + ep.episode_no}</span>
            <span class="tag">Premium</span>
          </a>
        {:else}
          <a class="ep" href={'/nonton/' + ep.id}>
            <span class="num">{ep.episode_no}</span>
            <span class="name">{ep.title || 'Episode ' + ep.episode_no}</span>
          </a>
        {/if}
      {/each}
    </div>
  </section>
{:else}
  <p class="muted">Belum ada episode.</p>
{/if}

<style>
  h2 { font-size: 19px; margin: 28px 0 12px; }
  .ep-grid {
    display: grid; gap: 8px;
    grid-template-columns: repeat(2, 1fr);
  }
  @media (min-width: 600px) { .ep-grid { grid-template-columns: repeat(3, 1fr); } }
  @media (min-width: 900px) { .ep-grid { grid-template-columns: repeat(5, 1fr); } }
  .ep {
    display: flex; flex-direction: column; gap: 2px;
    background: var(--panel); border: 1px solid var(--border);
    border-radius: 8px; padding: 10px 12px;
  }
  .ep:hover { border-color: #777; text-decoration: none; }
  .ep .num { font-weight: 700; font-size: 15px; }
  .ep .name {
    font-size: 12px; color: var(--muted);
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .ep.locked { opacity: .75; }
  .ep.locked .tag { font-size: 11px; color: var(--gold); font-weight: 700; }
</style>
