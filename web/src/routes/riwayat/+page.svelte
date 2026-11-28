<script>
  import { onMount } from 'svelte';
  import { user } from '$lib/auth.js';
  import { searchQuery } from '$lib/search.js';
  import { API_BASE, getHistory } from '$lib/api.js';
  import { getLocalHistory, removeLocalHistory, clearLocalHistory, fmtPosition, fmtWhen } from '$lib/history.js';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal } from '$lib/reveal.js';

  let items = [];
  let loading = true;
  let error = '';
  let fromServer = false;

  // Pencarian kontekstual navbar (27 Nov 2026): filter riwayat yang sudah
  // dimuat (maks 50 dari server / lokal).
  $: searchQ = $searchQuery || '';
  $: qnorm = searchQ.trim().toLowerCase();
  $: shownItems = qnorm ? items.filter((it) => (it.title || '').toLowerCase().includes(qnorm)) : items;

  function poster(u) {
    if (!u) return '';
    if (!/^https?:\/\//i.test(u)) return u;
    return API_BASE + '/api/v1/img-proxy?u=' + encodeURIComponent(u);
  }

  async function load() {
    loading = true;
    error = '';
    try {
      if ($user) {
        const res = await getHistory(50);
        items = (res.items || []).map((it) => ({
          episode_id: it.episode_id,
          title_id: it.title_id,
          title: it.title,
          poster_url: it.poster_url,
          episode_no: it.episode_no,
          position_sec: it.position_sec || 0,
          updated_at: it.updated_at
        }));
        fromServer = true;
      } else {
        items = getLocalHistory();
        fromServer = false;
      }
    } catch (e) {
      // Riwayat server gagal (mis. sesi habis) — tampilkan lokal.
      items = getLocalHistory();
      fromServer = false;
      if ($user) error = e.message || '';
    } finally {
      loading = false;
    }
  }

  function removeItem(id) {
    removeLocalHistory(id);
    items = items.filter((x) => x.episode_id !== id);
  }

  function clearAll() {
    if (!confirm('Hapus semua riwayat lokal di perangkat ini?')) return;
    clearLocalHistory();
    items = [];
  }

  onMount(load);
</script>

<div class="container page">
  <h1>Riwayat</h1>
  {#if qnorm && !loading}
    <p class="muted search-note">{shownItems.length} riwayat untuk “{searchQ.trim()}”</p>
  {/if}
  {#if !fromServer && items.length}
    <div class="row-head">
      <span class="muted">Disimpan di perangkat ini.</span>
      <button class="btn ghost small" on:click={clearAll}>Bersihkan semua</button>
    </div>
  {/if}

  {#if loading}
    <Skeleton kind="row" count={6} />
  {:else if error}
    <p class="error">{error}</p>
  {:else if !shownItems.length}
    <div class="card empty">
      {#if qnorm}
        <strong>Tidak ada riwayat untuk “{searchQ.trim()}”.</strong>
        <span class="muted">Coba kata kunci lain.</span>
      {:else}
        <strong>Belum ada riwayat tontonan.</strong>
        <span class="muted">
          {#if $user}
            Episode yang kamu tonton akan muncul di sini.
          {:else}
            Masuk untuk menyimpan riwayat di semua perangkat — atau tonton dulu, riwayat tamu tersimpan di perangkat ini.
          {/if}
        </span>
        {#if !$user}<div class="row"><a class="btn" href="/masuk">Masuk dengan Google</a></div>{/if}
      {/if}
    </div>
  {:else}
    <div class="list">
      {#each shownItems as it, i (it.episode_id)}
        <div use:reveal={(i % 10) * 40}>
          <a class="row card" href={'/nonton/' + it.episode_id}>
            <div class="thumb">
              {#if it.poster_url}
                <img src={poster(it.poster_url)} alt={it.title} loading="lazy" />
              {:else}
                <div class="no-poster">S</div>
              {/if}
            </div>
            <div class="info">
              <strong>{it.title}</strong>
              <span class="muted">Episode {it.episode_no}</span>
              <span class="muted">
                {fmtWhen(it.updated_at)}{it.position_sec > 0 ? ' • Lanjut ' + fmtPosition(it.position_sec) : ''}
              </span>
              {#if it.position_sec > 0}
                <span class="bar"><span style="width: {Math.min(100, (it.position_sec / 1500) * 100)}%"></span></span>
              {/if}
            </div>
            {#if !fromServer}
              <button
                class="icon-btn"
                aria-label="Hapus dari riwayat"
                on:click|preventDefault={() => removeItem(it.episode_id)}><i class="fa-solid fa-xmark"></i></button
              >
            {/if}
          </a>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .page { padding-top: 24px; max-width: 720px; }
  .page h1 { font-size: 26px; margin-bottom: 16px; }
  .row-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
  .empty { display: flex; flex-direction: column; gap: 8px; }
  .list { display: flex; flex-direction: column; gap: 12px; }
  .row { display: flex; gap: 12px; padding: 12px; align-items: center; }
  .row:hover { text-decoration: none; border-color: #555; }
  .thumb { width: 64px; height: 96px; flex-shrink: 0; border-radius: 4px; overflow: hidden; background: #000; }
  .thumb img { width: 100%; height: 100%; object-fit: cover; }
  .no-poster {
    width: 100%; height: 100%; display: flex; align-items: center; justify-content: center;
    color: var(--red); font-weight: 900;
  }
  .info { flex: 1; display: flex; flex-direction: column; gap: 2px; font-size: 14px; min-width: 0; }
  .info strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .bar { display: block; height: 4px; background: var(--panel-2); border-radius: 999px; margin-top: 6px; overflow: hidden; }
  .bar span { display: block; height: 100%; background: var(--red); }
  .icon-btn { background: none; border: 0; color: var(--muted); font-size: 16px; cursor: pointer; padding: 8px; }
  .small { font-size: 13px; padding: 6px 12px; }
</style>
