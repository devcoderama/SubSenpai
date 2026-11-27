<script>
  import { posterUrl } from '$lib/api.js';

  export let item;
  export let firstEpisodeId = null;

  $: typeLabel = item.type === 'donghua' ? 'Donghua' : item.type === 'anime' ? 'Anime' : 'Film';
</script>

<section class="hero">
  <div class="backdrop">
    {#if item.poster_url}
      <img src={posterUrl(item.poster_url)} alt="" />
    {/if}
    <div class="shade"></div>
  </div>
  <div class="container hero-body">
    <div class="eyebrow">SUBSENPAI • {typeLabel.toUpperCase()}</div>
    <h1>{item.title}</h1>
    {#if item.title_alt}
      <div class="alt">{item.title_alt}</div>
    {/if}
    <div class="meta">
      {#if item.rating}<span class="rate"><i class="fa-solid fa-star"></i> {Number(item.rating).toFixed(1)}</span>{/if}
      {item.year || ''}
      {#if item.status}• {item.status}{/if}
      {#if item.genres?.length}• {item.genres.join(' • ')}{/if}
    </div>
    {#if item.synopsis}
      <p class="synopsis">{item.synopsis}</p>
    {/if}
    <div class="actions">
      {#if firstEpisodeId}
        <a class="btn" href={'/nonton/' + firstEpisodeId}><i class="fa-solid fa-play"></i> Nonton</a>
      {/if}
      <a class="btn ghost" href={'/judul/' + item.id}><i class="fa-solid fa-circle-info"></i> Detail</a>
    </div>
  </div>
</section>

<style>
  .hero { position: relative; margin-bottom: 8px; overflow: hidden; }
  .backdrop { position: absolute; inset: 0; }
  .backdrop img {
    width: 100%; height: 100%; object-fit: cover; object-position: center 20%;
    filter: blur(2px) brightness(.55); transform: scale(1.05);
  }
  .shade {
    position: absolute; inset: 0;
    background: linear-gradient(to right, rgba(20,20,20,.92) 0%, rgba(20,20,20,.55) 55%, rgba(20,20,20,.15) 100%),
      linear-gradient(to top, var(--bg) 0%, transparent 40%);
  }
  .hero-body { position: relative; padding-top: 72px; padding-bottom: 64px; max-width: 640px; margin-left: 0; }
  .eyebrow { color: var(--red); font-weight: 800; letter-spacing: 2px; font-size: 13px; }
  h1 { margin: 10px 0 4px; font-size: clamp(28px, 5vw, 48px); line-height: 1.05; }
  .alt { color: var(--muted); font-size: 15px; }
  .meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; font-size: 14px; color: #ddd; }
  .rate { color: var(--gold); font-weight: 700; }
  .synopsis {
    margin: 14px 0 20px; font-size: 15px; line-height: 1.55; color: #e5e5e5;
    display: -webkit-box; -webkit-line-clamp: 4; line-clamp: 4; -webkit-box-orient: vertical; overflow: hidden;
  }
  .actions { display: flex; gap: 10px; }
  @media (min-width: 1280px) { .hero-body { margin-left: max(0px, calc((100vw - 1280px) / 2)); } }
</style>
