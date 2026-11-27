<script>
  import { posterUrl } from '$lib/api.js';

  export let item;
  export let href = null;
  export let sub = null;

  $: link = href || '/judul/' + item.id;
  $: typeLabel = item.type === 'donghua' ? 'Donghua' : item.type === 'anime' ? 'Anime' : 'Film';
</script>

<a class="card" href={link}>
  <div class="poster">
    {#if item.poster_url}
      <img src={posterUrl(item.poster_url)} alt={item.title} loading="lazy" />
    {:else}
      <div class="no-poster">SUBSENPAI</div>
    {/if}
    {#if item.rating}
      <span class="rating">★ {Number(item.rating).toFixed(1)}</span>
    {/if}
  </div>
  <div class="meta">
    <div class="title">{item.title}</div>
    <div class="sub muted">
      {#if sub}{sub}{:else}{typeLabel}{item.year ? ' • ' + item.year : ''}{item.status ? ' • ' + item.status : ''}{/if}
    </div>
    {#if item.genres?.length}
      <div class="genres muted">{item.genres.slice(0, 3).join(' • ')}</div>
    {/if}
  </div>
</a>

<style>
  .card { display: block; padding: 0; overflow: hidden; transition: transform .15s; }
  .card:hover { transform: scale(1.03); text-decoration: none; border-color: #555; }
  .poster { position: relative; aspect-ratio: 2 / 3; background: #000; overflow: hidden; }
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
    font-size: 14px; font-weight: 600; line-height: 1.3;
    display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
  }
  .sub { font-size: 12px; margin-top: 4px; }
  .genres {
    font-size: 11px; margin-top: 2px;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
</style>
