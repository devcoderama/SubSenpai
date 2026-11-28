<script>
  import { reveal } from '$lib/reveal.js';

  // Daftar episode halaman Detail — baris ala aplikasi (DetailScreen.kt):
  // nomor besar, judul + durasi + "Khusus Premium", ikon putar/gembok.
  // Episode terkunci menuju halaman Nonton — kartu kunci di sana yang
  // menawarkan Premium / buka-iklan.
  export let episodes = [];
  export let titleId = '';
</script>

{#if episodes?.length}
  <section>
    <h2>Episode</h2>
    <div class="ep-rows">
      {#each episodes as ep, i (ep.id)}
        <div class="ep-row-wrap" use:reveal={(i % 12) * 45}>
          <a class="ep-row" href={'/nonton/' + ep.id + (ep.locked ? '?title=' + titleId : '')}>
            <span class="num">{ep.episode_no}</span>
            <span class="mid">
              <span class="name">{ep.title || 'Episode ' + ep.episode_no}</span>
              {#if ep.duration_sec}
                <span class="dur">{Math.floor(ep.duration_sec / 60)} mnt</span>
              {/if}
              {#if ep.locked}
                <span class="lock-tag">Khusus Premium</span>
              {/if}
            </span>
            <span class="play">
              <i class={'fa-solid ' + (ep.locked ? 'fa-lock' : 'fa-play')}></i>
            </span>
          </a>
        </div>
      {/each}
    </div>
  </section>
{:else}
  <p class="muted">Episode belum tersedia di sumber untuk judul ini.</p>
{/if}

<style>
  h2 { font-size: 19px; margin: 28px 0 12px; }
  .ep-rows { display: flex; flex-direction: column; gap: 8px; }
  .ep-row-wrap { display: block; }
  .ep-row {
    display: flex; align-items: center; gap: 14px;
    background: #1f1f1f; border-radius: 4px; padding: 12px 14px;
  }
  .ep-row:hover { text-decoration: none; background: #262626; }
  .ep-row .num {
    flex: 0 0 42px; font-size: 22px; font-weight: 700; color: #fff;
  }
  .ep-row .mid { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
  .ep-row .name {
    font-size: 16px; color: #fff;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .ep-row .dur { font-size: 13px; color: var(--muted); }
  .ep-row .lock-tag { font-size: 13px; color: var(--gold); font-weight: 700; }
  .ep-row .play { flex: 0 0 28px; text-align: center; font-size: 20px; color: #fff; }
  .ep-row .play .fa-lock { color: var(--gold); }
</style>
