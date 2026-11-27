<script>
  import { onMount } from 'svelte';
  import { getSchedule, posterUrl } from '$lib/api.js';
  import { searchQuery } from '$lib/search.js';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal } from '$lib/reveal.js';

  /** Halaman Jadwal Rilis — dibuat sama seperti aplikasi (permintaan Rams
      27 Nov 2026, dari screenshot HP): tab jenis Anime/Donghua/Film
      (merah, sama lebar), baris TOMBOL PILIH HARI (Senin..Minggu, bisa
      digeser) untuk Anime & Donghua, catatan sumber di bawahnya, lalu
      daftar baris poster + judul + Episode/Jam per hari yang dipilih.
      Film tidak punya pilihan hari — API mengembalikan satu grup
      "Rilis Terbaru" saja, jadi baris harinya tidak tampil.
      Bonus perbaikan: tab Film web SEBELUMNYA SELALU KOSONG — kodenya
      membaca res.items, padahal API film mengembalikan res.groups. */

  const TABS = [
    { id: 'anime', label: 'Anime' },
    { id: 'donghua', label: 'Donghua' },
    { id: 'film', label: 'Film' }
  ];
  const NOTES = {
    anime: 'Jadwal estimasi dari Kuronime — bisa lebih cepat atau terlambat.',
    donghua: 'Jadwal dari Anichin.',
    film: 'Film yang paling baru masuk katalog SubSenpai.'
  };
  const DAYS = ['Minggu', 'Senin', 'Selasa', 'Rabu', 'Kamis', 'Jumat', 'Sabtu'];
  function todayLabel() {
    return DAYS[new Date().getDay()];
  }

  let tab = 'anime';
  let loading = true;
  let error = '';
  let groups = [];
  // Hari pilihan diingat per jenis (seperti sesi aplikasi): pindah jenis
  // lalu kembali, tetap di hari yang sama.
  let dayByType = { anime: null, donghua: null, film: null };

  // Hari yang tampil: pilihan tersimpan (bila masih ada di data) → hari ini
  // (bila ada di data) → grup pertama. Grup sudah diurutkan Senin..Minggu
  // oleh backend ("Update Acak" di akhir).
  $: days = groups.map((g) => g.day || g.name).filter(Boolean);
  $: effectiveDay =
    (dayByType[tab] && days.includes(dayByType[tab]) && dayByType[tab]) ||
    (days.includes(todayLabel()) && todayLabel()) ||
    days[0] ||
    null;
  $: dayItems = (groups.find((g) => (g.day || g.name) === effectiveDay)?.items || []);

  // Pencarian kontekstual dari navbar (27 Nov 2026): saat ada kata kunci,
  // tampilkan hasil dari SEMUA hari sebagai bagian per hari (bukan satu hari).
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
  $: searchCount = filteredGroups.reduce((n, g) => n + (g.items || []).length, 0);

  async function load(t) {
    loading = true;
    error = '';
    try {
      const res = await getSchedule(t);
      groups = res.groups || [];
    } catch (e) {
      error = e.message || 'Gagal memuat jadwal.';
    } finally {
      loading = false;
    }
  }

  function setTab(t) {
    tab = t;
    load(t);
    window.scrollTo({ top: 0 });
  }

  function setDay(d) {
    dayByType = { ...dayByType, [tab]: d };
    window.scrollTo({ top: 0 });
  }

  function itemHref(it) {
    return it.title_id ? '/judul/' + it.title_id : '/cari?q=' + encodeURIComponent(it.title);
  }

  // Teks jam persis seperti aplikasi: "Jam 17:41", "Sudah Rilis"
  // (hijau tebal), "Jadwal acak" untuk "??", tahun apa adanya untuk film.
  function timeText(it) {
    if (tab === 'film') return it.time || '';
    if (it.time === 'Sudah Rilis') return 'Sudah Rilis';
    if (it.time === '??') return 'Jadwal acak';
    if (it.time) return 'Jam ' + it.time;
    return '';
  }

  onMount(() => load('anime'));
</script>

<svelte:head>
  <title>Jadwal Rilis — SubSenpai</title>
</svelte:head>

<div class="container">
  <h1>Jadwal Rilis</h1>

  <div class="type-tabs" role="tablist" aria-label="Jenis jadwal">
    {#each TABS as t}
      <button type="button" class="type-tab" class:active={tab === t.id} on:click={() => setTab(t.id)}>
        {t.label}
      </button>
    {/each}
  </div>

  {#if loading}
    <Skeleton kind="row" count={8} />
  {:else if error}
    <p class="error">{error}</p>
    <div class="more"><button class="btn ghost" on:click={() => load(tab)}>Coba Lagi</button></div>
  {:else if !groups.length}
    <p class="muted">Jadwal belum tersedia — coba lagi nanti.</p>
  {:else if qnorm}
    <p class="note">{NOTES[tab]}</p>
    <p class="muted search-note">{searchCount} jadwal untuk “{searchQ.trim()}”</p>
    {#if !filteredGroups.length}
      <p class="muted">Tidak ada jadwal untuk “{searchQ.trim()}”. Coba kata kunci lain.</p>
    {:else}
      {#each filteredGroups as g, gi (g.day || g.name)}
        <section class="day-sec" use:reveal={gi * 60}>
          <h2>{g.day || g.name}</h2>
          <div class="rows">
            {#each g.items as it (it.title_id || it.slug)}
              <a class="row" href={itemHref(it)}>
                {#if it.poster}
                  <img src={posterUrl(it.poster)} alt={it.title} loading="lazy" />
                {:else}
                  <div class="no-poster" aria-hidden="true">S</div>
                {/if}
                <div class="row-meta">
                  <div class="row-title">{it.title}</div>
                  <div class="row-sub">
                    {#if tab !== 'film' && it.episode > 0}
                      <span>Episode {it.episode}</span>
                    {/if}
                    {#if timeText(it)}
                      <span class:released={it.time === 'Sudah Rilis'}>{timeText(it)}</span>
                    {/if}
                  </div>
                </div>
                <i class="fa-solid fa-chevron-right row-chev" aria-hidden="true"></i>
              </a>
            {/each}
          </div>
        </section>
      {/each}
    {/if}
  {:else}
    {#if days.length > 1}
      <div class="day-pills">
        {#each days as d}
          <button type="button" class="day-pill" class:active={d === effectiveDay} on:click={() => setDay(d)}>
            {d}
          </button>
        {/each}
      </div>
    {/if}
    <p class="note">{NOTES[tab]}</p>
    {#if !dayItems.length}
      <p class="muted">Tidak ada jadwal untuk hari ini.</p>
    {:else}
      <div class="rows">
        {#each dayItems as it, i (it.title_id || it.slug)}
          <a class="row" href={itemHref(it)} use:reveal={(i % 10) * 40}>
            {#if it.poster}
              <img src={posterUrl(it.poster)} alt={it.title} loading="lazy" />
            {:else}
              <div class="no-poster" aria-hidden="true">S</div>
            {/if}
            <div class="row-meta">
              <div class="row-title">{it.title}</div>
              <div class="row-sub">
                {#if tab !== 'film' && it.episode > 0}
                  <span>Episode {it.episode}</span>
                {/if}
                {#if timeText(it)}
                  <span class:released={it.time === 'Sudah Rilis'}>{timeText(it)}</span>
                {/if}
              </div>
            </div>
            <i class="fa-solid fa-chevron-right row-chev" aria-hidden="true"></i>
          </a>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  h1 { margin: 24px 0 8px; font-size: 28px; }
  /* Tab jenis ala aplikasi: sama lebar, aktif merah penuh, nonaktif
     teks + garis merah (bukan chip putih seperti Beranda). */
  .type-tabs { display: flex; gap: 8px; margin-bottom: 10px; }
  .type-tab {
    flex: 1; padding: 10px 0; border-radius: 999px;
    border: 1px solid var(--red); background: transparent;
    color: var(--red); font-size: 14px; font-weight: 600; cursor: pointer;
  }
  .type-tab.active { background: var(--red); color: #fff; }
  /* Baris tombol pilih hari ala aplikasi: bisa digeser, aktif merah,
     nonaktif abu gelap #2a2a2a, teks putih. */
  .day-pills {
    display: flex; gap: 8px; overflow-x: auto;
    padding: 0 0 8px; scrollbar-width: none;
  }
  .day-pills::-webkit-scrollbar { display: none; }
  .day-pill {
    flex-shrink: 0; padding: 7px 14px; border-radius: 16px; border: 0;
    background: #2a2a2a; color: #fff; font-size: 14px; cursor: pointer;
  }
  .day-pill.active { background: var(--red); }
  /* Catatan sumber di bawah tombol hari, persis kata-katanya dari aplikasi. */
  .note { color: #808080; font-size: 13px; margin: 0 0 4px; }
  .day-sec h2 { font-size: 19px; margin: 20px 0 4px; }
  .day-sec:first-of-type h2 { margin-top: 8px; }
  .rows { display: flex; flex-direction: column; }
  /* Baris jadwal ala aplikasi: tanpa kartu — poster 54x76, judul 2 baris,
     "Episode X" + "Jam HH:MM" / "Sudah Rilis" hijau tebal, panah > kanan. */
  .row {
    display: flex; gap: 12px; align-items: center;
    padding: 8px 0; color: #fff;
  }
  .row:hover { text-decoration: none; }
  .row:hover .row-title { text-decoration: underline; }
  .row img, .row .no-poster {
    width: 54px; height: 76px; flex-shrink: 0;
    border-radius: 6px; object-fit: cover; background: #2a2a2a;
  }
  .row .no-poster {
    display: flex; align-items: center; justify-content: center;
    color: var(--red); font-size: 22px; font-weight: 900;
  }
  .row-meta { flex: 1; min-width: 0; }
  .row-title {
    font-size: 16px; line-height: 1.3;
    display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2;
    -webkit-box-orient: vertical; overflow: hidden;
  }
  .row-sub { display: flex; gap: 10px; margin-top: 3px; font-size: 13px; color: var(--muted); }
  .row-sub .released { color: #46d369; font-weight: 700; }
  .row-chev { color: #808080; font-size: 14px; flex-shrink: 0; }
  .more { text-align: center; margin: 24px 0; }
</style>
