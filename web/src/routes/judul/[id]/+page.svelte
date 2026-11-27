<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { getTitle, posterUrl } from '$lib/api.js';
  import EpisodeList from '$lib/components/EpisodeList.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';

  let loading = true;
  let error = '';
  let title = null;
  let episodes = [];
  let copied = false;

  $: id = $page.params.id;
  $: typeLabel =
    title?.type === 'donghua' ? 'Donghua' : title?.type === 'anime' ? 'Anime' : 'Film';

  async function share() {
    const url = window.location.href;
    try {
      await navigator.clipboard.writeText(url);
      copied = true;
      setTimeout(() => (copied = false), 2000);
    } catch {
      window.prompt('Salin tautan ini:', url);
    }
  }

  onMount(async () => {
    try {
      const res = await getTitle(id);
      title = res.title || res;
      episodes = res.episodes || [];
    } catch (e) {
      error = e.message || 'Judul tidak ditemukan.';
    } finally {
      loading = false;
    }
  });
</script>

<div class="container">
  {#if loading}
    <Skeleton kind="detail" />
  {:else if error}
    <p class="error">{error}</p>
    <p><a href="/"><i class="fa-solid fa-arrow-left"></i> Kembali ke Beranda</a></p>
  {:else if title}
    <div class="detail">
      <div class="poster">
        {#if title.poster_url}
          <img src={posterUrl(title.poster_url)} alt={title.title} />
        {/if}
      </div>
      <div class="info">
        <div class="eyebrow">{typeLabel.toUpperCase()}</div>
        <h1>{title.title}</h1>
        {#if title.title_alt}<div class="alt muted">{title.title_alt}</div>{/if}
        <div class="meta">
          {#if title.rating}<span class="rate"><i class="fa-solid fa-star"></i> {Number(title.rating).toFixed(1)}</span>{/if}
          {title.year || ''}
          {#if title.status}• {title.status}{/if}
          {#if title.season}• Musim {title.season}{/if}
          {#if title.network}• {title.network}{/if}
          {#if title.studio}• {title.studio}{/if}
        </div>
        {#if title.genres?.length}
          <div class="genres">
            {#each title.genres as g}
              <a class="chip" href={'/cari?genre=' + encodeURIComponent(g)}>{g}</a>
            {/each}
          </div>
        {/if}
        {#if title.synopsis}
          <p class="synopsis">{title.synopsis}</p>
        {/if}
        <div class="actions">
          {#if episodes.length}
            {#if episodes[0].locked}
              <a class="btn" href="/premium"><i class="fa-solid fa-lock"></i> Nonton — Khusus Premium</a>
            {:else}
              <a class="btn" href={'/nonton/' + episodes[0].id}><i class="fa-solid fa-play"></i> Nonton Episode {episodes[0].episode_no}</a>
            {/if}
          {/if}
          <button class="btn ghost" on:click={share}>
            {#if copied}<i class="fa-solid fa-check"></i> Tautan disalin{:else}<i class="fa-solid fa-link"></i> Bagikan{/if}
          </button>
        </div>
      </div>
    </div>

    <EpisodeList {episodes} titleId={title.id} />
  {/if}
</div>

<style>
  .detail { display: flex; gap: 24px; margin-top: 24px; flex-wrap: wrap; }
  .poster { flex: 0 0 220px; }
  .poster img { width: 100%; border-radius: 10px; }
  .info { flex: 1 1 320px; min-width: 0; }
  .eyebrow { color: var(--red); font-weight: 800; letter-spacing: 2px; font-size: 13px; }
  h1 { margin: 8px 0 4px; font-size: 32px; line-height: 1.1; }
  .alt { font-size: 15px; }
  .meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 12px; color: #ddd; font-size: 14px; }
  .rate { color: var(--gold); font-weight: 700; }
  .genres { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 14px; }
  .synopsis { margin-top: 16px; line-height: 1.65; color: #e5e5e5; font-size: 15px; }
  .actions { display: flex; gap: 10px; margin-top: 20px; flex-wrap: wrap; }
  @media (max-width: 600px) { .poster { flex-basis: 150px; } h1 { font-size: 24px; } }
</style>
