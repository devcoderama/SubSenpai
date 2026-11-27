<script>
  import { onMount } from 'svelte';
  import { getSchedule, posterUrl } from '$lib/api.js';
  import { searchQuery } from '$lib/search.js';
  import TitleCard from '$lib/components/TitleCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal } from '$lib/reveal.js';

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

  // Pencarian kontekstual navbar (27 Nov 2026): filter data jadwal yang
  // sudah dimuat — semua jadwal seminggu sudah di tangan, tanpa API baru.
  // Ganti tab saat mencari = cari dalam jenis baru.
  $: searchQ = $searchQuery || '';
  $: qnorm = searchQ.trim().toLowerCase();
  $: filteredGroups = qnorm
    ? groups
        .map((g) => ({
          ...g,
          items: (g.items || []).filter((it) => (it.title || '').toLowerCase().includes(qnorm))
        }))
        .filter((g) => g.items.length)
    : groups;
  $: filteredFilms = qnorm ? films.filter((it) => (it.title || '').toLowerCase().includes(qnorm)) : films;
  $: searchCount =
    tab === 'film' ? filteredFilms.length : filteredGroups.reduce((n, g) => n + (g.items || []).length, 0);

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
    <Skeleton kind="row" count={8} />
  {:else if error}
    <p class="error">{error}</p>
  {:else}
    {#if qnorm}
      <p class="muted search-note">{searchCount} jadwal untuk “{searchQ.trim()}”</p>
    {/if}
  {/if}
  {#if !loading && !error}
  {#if tab === 'film'}
    <div class="poster-grid">
      {#each filteredFilms as item, i (item.title_id || item.slug)}
        <div use:reveal={(i % 12) * 45}>
          <TitleCard
            item={{
              id: item.title_id,
              title: item.title,
              poster_url: item.poster,
              year: item.year,
              type: 'film'
            }}
          />
        </div>
      {/each}
    </div>
    {#if !filteredFilms.length}
      <p class="muted">
        {qnorm ? 'Tidak ada film untuk “' + searchQ.trim() + '”. Coba kata kunci lain.' : 'Belum ada film terbaru.'}
      </p>
    {/if}
  {:else}
    {#each filteredGroups as g, gi (g.day || g.name)}
      <section class="day" use:reveal={gi * 60}>
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
    {#if !filteredGroups.length}
      <p class="muted">
        {qnorm
          ? 'Tidak ada jadwal untuk “' + searchQ.trim() + '”. Coba kata kunci lain.'
          : 'Jadwal belum tersedia — coba lagi nanti.'}
      </p>
    {/if}
  {/if}
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
