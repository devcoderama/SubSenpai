<script>
  import { onMount } from 'svelte';
  import { getSchedule, posterUrl } from '$lib/api.js';
  import TitleCard from '$lib/components/TitleCard.svelte';

  const TABS = [
    { id: 'anime', label: 'Anime' },
    { id: 'donghua', label: 'Donghua' },
    { id: 'film', label: 'Film' }
  ];

  let tab = 'anime';
  let loading = true;
  let error = '';
  let groups = [];
  let films = [];

  async function load(t) {
    loading = true;
    error = '';
    groups = [];
    films = [];
    try {
      const res = await getSchedule(t);
      if (t === 'film') {
        films = res.items || [];
      } else {
        groups = res.groups || [];
      }
    } catch (e) {
      error = e.message || 'Gagal memuat jadwal.';
    } finally {
      loading = false;
    }
  }

  function setTab(t) {
    tab = t;
    load(t);
  }

  onMount(() => load('anime'));
</script>

<div class="container">
  <h1>Jadwal Rilis</h1>
  <div class="tabs">
    {#each TABS as t}
      <button class="chip" class:active={tab === t.id} on:click={() => setTab(t.id)}>{t.label}</button>
    {/each}
  </div>

  {#if loading}
    <p class="loading">Memuat jadwal…</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if tab === 'film'}
    <div class="poster-grid">
      {#each films as item (item.title_id || item.slug)}
        <TitleCard
          item={{
            id: item.title_id,
            title: item.title,
            poster_url: item.poster,
            year: item.year,
            type: 'film'
          }}
        />
      {/each}
    </div>
    {#if !films.length}<p class="muted">Belum ada film terbaru.</p>{/if}
  {:else}
    {#each groups as g (g.day || g.name)}
      <section class="day">
        <h2>{g.day || g.name}</h2>
        <div class="items">
          {#each g.items || [] as it (it.title_id || it.slug)}
            <a class="item" href={it.title_id ? '/judul/' + it.title_id : '/cari?q=' + encodeURIComponent(it.title)}>
              {#if it.poster}
                <img src={posterUrl(it.poster)} alt={it.title} loading="lazy" />
              {/if}
              <div class="item-meta">
                <div class="item-title">{it.title}</div>
                <div class="muted">
                  {it.time || ''}{it.episode ? ' • Ep ' + it.episode : ''}
                </div>
              </div>
            </a>
          {/each}
        </div>
      </section>
    {/each}
    {#if !groups.length}<p class="muted">Jadwal belum tersedia — coba lagi nanti.</p>{/if}
  {/if}
</div>

<style>
  h1 { margin: 24px 0 8px; font-size: 28px; }
  .tabs { display: flex; gap: 8px; margin-bottom: 20px; }
  .day { margin-bottom: 28px; }
  .day h2 { font-size: 19px; margin: 0 0 12px; }
  .items { display: flex; flex-direction: column; gap: 8px; }
  .item {
    display: flex; gap: 12px; align-items: center;
    background: var(--panel); border: 1px solid var(--border);
    border-radius: 10px; padding: 10px;
  }
  .item:hover { border-color: #777; text-decoration: none; }
  .item img { width: 52px; aspect-ratio: 2 / 3; object-fit: cover; border-radius: 6px; }
  .item-title { font-weight: 600; font-size: 15px; }
</style>
