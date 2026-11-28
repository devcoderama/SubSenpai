<script>
  import { onMount } from 'svelte';
  import { user } from '$lib/auth.js';
  import { getNotifications, markNotificationsRead } from '$lib/api.js';
  import { refreshUnread } from '$lib/notifications.js';
  import { fmtWhen } from '$lib/history.js';
  import Skeleton from '$lib/components/Skeleton.svelte';

  /** Halaman Notifikasi — lonceng di navbar (permintaan Rams 27 Nov 2026,
      screenshot HP: aplikasi punya lonceng di bar atas, web belum).
      Sama seperti layar Notifikasi aplikasi (FeedNotificationsScreen):
      tab Semua / Admin / Postingan / Episode Baru, baris belum dibaca
      berlatar ungu redup, ketuk baris membuka judul (Episode Baru) atau
      Post (aktivitas sosial), dan semua ditandai dibaca saat halaman
      dibuka. API: GET /api/v1/feed/notifications[?kind=] + POST
      /api/v1/feed/notifications/read (sudah ada di backend). */

  const TABS = [
    { id: '', label: 'Semua' },
    { id: 'broadcast', label: 'Admin' },
    { id: 'social', label: 'Postingan' },
    { id: 'release', label: 'Episode Baru' }
  ];
  const EMPTY = {
    '': 'Belum ada notifikasi. Pengumuman admin, aktivitas postingan, dan episode baru judul favoritmu akan muncul di sini.',
    broadcast: 'Belum ada pengumuman dari admin. Pengumuman penting akan muncul di sini.',
    social: 'Belum ada aktivitas postingan. Like, komentar, dan balasan pada postinganmu akan muncul di sini.',
    release:
      'Belum ada episode baru. Tambahkan anime, donghua, atau film ke Daftar Saya — episode barunya akan diberitahu di sini.'
  };

  let tab = '';
  let loading = true;
  let error = '';
  let items = [];

  $: shown = tab ? items.filter((n) => n.kind === tab) : items;

  async function load() {
    if (!$user) {
      loading = false;
      return;
    }
    loading = true;
    error = '';
    try {
      const res = await getNotifications();
      items = res.items || [];
      // Seperti aplikasi: membuka halaman menandai semuanya dibaca.
      await markNotificationsRead();
      refreshUnread();
    } catch (e) {
      error = e.message || 'Gagal memuat notifikasi.';
    } finally {
      loading = false;
    }
  }

  function notifHref(n) {
    if (n.kind === 'release' && n.ref_title_id) return '/judul/' + n.ref_title_id;
    if (n.ref_post_id) return '/post';
    if (n.ref_title_id) return '/judul/' + n.ref_title_id;
    return null;
  }

  function kindIcon(n) {
    if (n.kind === 'broadcast') return 'fa-bullhorn';
    if (n.kind === 'release') return 'fa-calendar-days';
    return 'fa-user';
  }

  onMount(load);
</script>

<svelte:head>
  <title>Notifikasi — SubSenpai</title>
</svelte:head>

<div class="container">
  <h1>Notifikasi</h1>

  {#if !$user}
    <div class="guest-card">
      <i class="fa-solid fa-bell"></i>
      <p><strong>Masuk untuk melihat notifikasi.</strong></p>
      <p class="muted">Pengumuman admin, aktivitas postingan, dan episode baru judul favoritmu akan muncul di sini.</p>
      <a class="btn" href="/masuk">Masuk</a>
    </div>
  {:else}
    <div class="tabs">
      {#each TABS as t}
        <button class="chip" class:active={tab === t.id} on:click={() => (tab = t.id)}>{t.label}</button>
      {/each}
    </div>

    {#if loading}
      <Skeleton kind="row" count={8} />
    {:else if error}
      <p class="error">{error}</p>
      <div class="more"><button class="btn ghost" on:click={load}>Coba Lagi</button></div>
    {:else if !shown.length}
      <p class="muted empty">{EMPTY[tab]}</p>
    {:else}
      <div class="rows">
        {#each shown as n (n.id)}
          <a class="row" class:unread={!n.read_at} class:no-link={!notifHref(n)} href={notifHref(n)}>
            <span class="ava">
              {#if n.actor_avatar_url}
                <img src={n.actor_avatar_url} alt="" />
              {:else}
                <i class="fa-solid {kindIcon(n)}"></i>
              {/if}
            </span>
            <span class="row-meta">
              {#if n.kind === 'broadcast'}
                <span class="kicker"><i class="fa-solid fa-bullhorn"></i> PENGUMUMAN ADMIN</span>
              {/if}
              <span class="row-title">{n.title}</span>
              {#if n.body}
                <span class="row-body">{n.body}</span>
              {/if}
              <span class="row-time">{fmtWhen(n.created_at)}</span>
            </span>
            {#if !n.read_at}
              <span class="dot" aria-label="Belum dibaca"></span>
            {/if}
          </a>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  h1 { margin: 24px 0 8px; font-size: 28px; }
  .tabs { display: flex; gap: 8px; margin-bottom: 16px; overflow-x: auto; scrollbar-width: none; }
  .tabs::-webkit-scrollbar { display: none; }
  .tabs .chip { flex-shrink: 0; }
  .guest-card {
    background: var(--panel); border: 1px solid var(--border); border-radius: 12px;
    padding: 32px 20px; text-align: center;
  }
  .guest-card > i { font-size: 32px; color: var(--muted); }
  .guest-card p { margin: 12px 0; }
  .guest-card .btn { display: inline-block; margin-top: 8px; }
  .empty { margin: 16px 0; line-height: 1.6; }
  .rows { display: flex; flex-direction: column; }
  /* Baris notifikasi ala aplikasi: belum dibaca berlatar ungu redup
     + titik penanda; sudah dibaca polos. */
  .row {
    display: flex; gap: 12px; align-items: flex-start;
    padding: 12px 8px; color: #fff; border-radius: 4px;
  }
  .row:hover { text-decoration: none; background: rgba(255, 255, 255, 0.04); }
  .row.no-link { cursor: default; }
  .row.unread { background: rgba(139, 92, 246, 0.14); }
  .row.unread:hover { background: rgba(139, 92, 246, 0.2); }
  .ava {
    flex-shrink: 0; width: 40px; height: 40px; border-radius: 50%;
    background: #2a2a2a; color: var(--muted);
    display: flex; align-items: center; justify-content: center; overflow: hidden;
  }
  .ava img { width: 100%; height: 100%; object-fit: cover; }
  .ava i { font-size: 16px; }
  .row-meta { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .kicker {
    color: #a78bfa; font-size: 11px; font-weight: 700; letter-spacing: 0.5px;
    display: flex; align-items: center; gap: 5px;
  }
  .row-title { font-size: 15px; line-height: 1.35; }
  .row.unread .row-title { font-weight: 700; }
  .row-body {
    font-size: 13px; color: var(--muted); line-height: 1.45;
    display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2;
    -webkit-box-orient: vertical; overflow: hidden;
  }
  .row-time { font-size: 12px; color: var(--muted); }
  .dot {
    flex-shrink: 0; width: 8px; height: 8px; border-radius: 50%;
    background: #8b5cf6; margin-top: 8px;
  }
  .more { text-align: center; margin: 24px 0; }
</style>
