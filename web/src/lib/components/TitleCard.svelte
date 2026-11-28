<script>
  import { posterUrl } from '$lib/api.js';

  export let item;
  export let href = null;
  export let sub = null;
  export let progress = /** @type {number | null} */ (null); // 0..1 — bar progres Lanjutkan Menonton (null = tak ada data)

  $: progressPct = progress != null && progress > 0 ? Math.min(100, progress * 100) : 0;

  $: link = href || '/judul/' + item.id;
  $: detailHref = '/judul/' + item.id;
  $: typeLabel =
    item.type === 'donghua'
      ? 'Donghua'
      : item.type === 'anime'
        ? 'Anime'
        : item.type === 'series'
          ? 'Serial TV'
          : 'Film';
</script>

<div class="card">
  <div class="poster-wrap">
    <a class="poster" href={link}>
      {#if item.poster_url}
        <img src={posterUrl(item.poster_url)} alt={item.title} loading="lazy" />
      {:else}
        <div class="no-poster">SUBSENPAI</div>
      {/if}
      {#if item.rating}
        <span class="rating"><i class="fa-solid fa-star"></i> {Number(item.rating).toFixed(1)}</span>
      {/if}
    </a>
    {#if progressPct > 0}
      <div class="progress" aria-hidden="true"><span style="width: {progressPct}%"></span></div>
    {/if}
  </div>
  <div class="meta">
    <a class="title" href={link}>{item.title}</a>
    <div class="sub muted">
      {#if sub}{sub}{:else}{typeLabel}{item.year ? ' • ' + item.year : ''}{item.status ? ' • ' + item.status : ''}{/if}
    </div>
    {#if item.genres?.length}
      <div class="genres muted">{item.genres.slice(0, 3).join(' • ')}</div>
    {/if}
  </div>
</div>

<style>
  .card {
    display: block; padding: 0; overflow: hidden; position: relative;
    transition: transform .15s, box-shadow .15s;
  }
  .card:hover {
    transform: scale(1.05); text-decoration: none; border-color: #555;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5); z-index: 5;
  }
  .poster-wrap { position: relative; }
  .progress { position: absolute; left: 0; right: 0; bottom: 0; height: 3px; background: rgba(255, 255, 255, 0.3); z-index: 4; pointer-events: none; overflow: hidden; }
  .progress span { display: block; height: 100%; background: var(--red); }
  .poster {
    display: block; position: relative; aspect-ratio: 2 / 3;
    background: #000; overflow: hidden;
  }
  .poster img { width: 100%; height: 100%; object-fit: cover; }
  .no-poster {
    width: 100%; height: 100%; display: flex; align-items: center; justify-content: center;
    color: var(--red); font-weight: 900; font-size: 13px; letter-spacing: 1px;
  }
  .rating {
    position: absolute; top: 6px; left: 6px;
    background: rgba(0,0,0,.75); color: var(--gold);
    font-size: 12px; font-weight: 700; padding: 2px 6px; border-radius: 4px;
  }
  .meta { padding: 10px; }
  .title {
    color: inherit; text-decoration: none;
    font-size: 14px; font-weight: 600; line-height: 1.3;
    display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
  }
  .title:hover { text-decoration: none; }
  .sub { font-size: 12px; margin-top: 4px; }
  .genres {
    font-size: 11px; margin-top: 2px;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
</style>
