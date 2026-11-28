<script>
  import { posterUrl } from '$lib/api.js';
  import { reveal } from '$lib/reveal.js';

  // Rel "Top 10 Hari Ini" ala Netflix (28 Nov 2026, rekomendasi no. 1):
  // angka peringkat besar ber-outline di kiri poster. Data = trending
  // 24 jam backend (peringkat tontonan nyata: penonton unik x rating),
  // BUKAN live-browse (urutan listing sumber, bukan peringkat).
  export let items = [];
</script>

{#if items?.length}
  <section class="top10-section" use:reveal>
    <div class="top10-head">
      <h2>Top 10 Hari Ini</h2>
    </div>
    <div class="top10-rail">
      {#each items.slice(0, 10) as item, i (item.id)}
        <a
          class="top10-card"
          href={'/judul/' + item.id}
          aria-label={i + 1 + '. ' + item.title}
          title={i + 1 + '. ' + item.title}
        >
          <span class="rank" aria-hidden="true">{i + 1}</span>
          <span class="poster">
            {#if item.poster_url}
              <img src={posterUrl(item.poster_url)} alt="" loading="lazy" />
            {:else}
              <span class="no-poster">SUBSENPAI</span>
            {/if}
          </span>
        </a>
      {/each}
    </div>
  </section>
{/if}

<style>
  .top10-section { margin: 28px 0; }
  @media (min-width: 1280px) { .top10-section { margin: 36px 0; } }
  .top10-head { display: flex; align-items: baseline; gap: 12px; margin-bottom: 12px; }
  .top10-head h2 { margin: 0; font-size: 19px; }
  .top10-rail {
    display: flex; gap: 20px; overflow-x: auto; overflow-y: hidden;
    padding-bottom: 8px; scrollbar-width: thin;
  }
  .top10-card {
    flex: 0 0 auto; display: flex; align-items: flex-end;
    text-decoration: none; transition: transform .15s;
  }
  .top10-card:hover { transform: scale(1.03); }
  /* Angka peringkat ala Netflix: isi gelap + outline abu tebal,
     menjorok ke balik poster. Overlap kecil (-8px) supaya angka
     sempit seperti "1" tetap terbaca — overlap lama -26px menutup
     hampir seluruh batang "1" (28 Nov 2026, laporan Rams). */
  .rank {
    font-weight: 900; line-height: 0.75; letter-spacing: -0.08em;
    font-size: 140px; color: #000;
    -webkit-text-stroke: 4px #8a8a8a;
    margin-right: -8px; margin-bottom: -6px; z-index: 0;
    user-select: none;
  }
  .poster {
    position: relative; z-index: 1; width: 110px; aspect-ratio: 2 / 3;
    background: #000; overflow: hidden; border-radius: 4px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.5); flex: 0 0 auto;
  }
  .poster img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .no-poster {
    width: 100%; height: 100%; display: flex; align-items: center; justify-content: center;
    color: var(--red); font-weight: 900; font-size: 11px; letter-spacing: 1px;
  }
  @media (min-width: 900px) {
    .rank { font-size: 170px; margin-right: -10px; }
    .poster { width: 132px; }
  }
  @media (min-width: 1600px) {
    .rank { font-size: 200px; margin-right: -12px; }
    .poster { width: 150px; }
  }
</style>
