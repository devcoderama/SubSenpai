<script>
  import { onMount } from 'svelte';
  import { user } from '$lib/auth.js';
  import { getDevices, getDeviceId, revokeDevice } from '$lib/api.js';
  import { fmtWhen } from '$lib/history.js';
  import Skeleton from '$lib/components/Skeleton.svelte';

  /** Halaman Perangkat — daftar perangkat login akun ini (batas sesi:
      premium maks 2, gratis maks 1; login baru menendang perangkat
      yang paling lama tidak aktif). Perangkat saat ini ditandai dan
      tidak punya tombol Cabut (mencabut diri sendiri = logout).
      API: GET /api/v1/devices + POST /api/v1/devices/revoke. */

  let devices = [];
  let loading = true;
  let error = '';
  let revokeError = '';
  let revokingId = null;
  let myDeviceId = '';

  async function load() {
    // Tamu: jangan panggil API perangkat — tampilkan kartu ajakan login.
    if (!$user) {
      loading = false;
      return;
    }
    loading = true;
    error = '';
    try {
      const res = await getDevices();
      devices = res.devices || [];
    } catch (e) {
      error = e.message || 'Gagal memuat perangkat.';
    } finally {
      loading = false;
    }
  }

  async function revoke(id) {
    if (revokingId || id === myDeviceId) return;
    revokingId = id;
    revokeError = '';
    try {
      await revokeDevice(id);
      // Refresh daftar dari server sesudah cabut.
      const res = await getDevices();
      devices = res.devices || [];
    } catch (e) {
      revokeError = e.message || 'Gagal mencabut perangkat.';
    } finally {
      revokingId = null;
    }
  }

  onMount(() => {
    myDeviceId = getDeviceId();
    load();
  });
</script>

<svelte:head><title>Perangkat — SubSenpai</title></svelte:head>

<div class="container page">
  <h1>Perangkat</h1>

  {#if $user}
    <p class="muted intro">
      Akun ini bisa login di maks. {$user.is_premium ? '2' : '1'} perangkat{#if !$user.is_premium}
        (Premium: 2 perangkat){/if}. Bila login di perangkat baru melewati batas, perangkat yang
      paling lama tidak aktif akan dikeluarkan.
    </p>
  {/if}

  {#if loading}
    <Skeleton kind="row" count={4} />
  {:else if !$user}
    <div class="card empty">
      <strong>Masuk untuk melihat perangkat.</strong>
      <span class="muted">Lihat perangkat yang login ke akunmu dan cabut yang tidak dikenal.</span>
      <div class="row"><a class="btn" href="/masuk">Masuk dengan Google</a></div>
    </div>
  {:else if error}
    <p class="error">{error}</p>
    <button class="btn" on:click={load}>Coba Lagi</button>
  {:else if !devices.length}
    <div class="card empty">
      <strong>Belum ada perangkat tercatat.</strong>
      <span class="muted">Perangkat dicatat saat login. Keluar lalu masuk lagi untuk mencatat perangkat ini.</span>
    </div>
  {:else}
    {#if revokeError}<p class="error">{revokeError}</p>{/if}
    <div class="rows">
      {#each devices as d (d.device_id)}
        <div class="dev-row" class:mine={d.device_id === myDeviceId}>
          <span class="ic"><i class="fa-solid fa-mobile-screen"></i></span>
          <span class="meta">
            <span class="name">{d.device_name || 'Perangkat ' + d.device_id.slice(0, 8)}</span>
            <span class="id muted">{d.device_id}</span>
            <span class="time muted">Terakhir aktif {fmtWhen(d.last_seen_at) || '—'}</span>
          </span>
          {#if d.device_id === myDeviceId}
            <span class="badge">Perangkat ini</span>
          {:else}
            <button
              class="btn ghost small"
              disabled={revokingId === d.device_id}
              on:click={() => revoke(d.device_id)}
              >{revokingId === d.device_id ? 'Mencabut…' : 'Cabut'}</button
            >
          {/if}
        </div>
      {/each}
    </div>
    <p class="muted small">
      Mencabut perangkat akan mengeluarkannya dari akun ini — perangkat itu harus masuk lagi untuk
      memakai SubSenpai.
    </p>
  {/if}
</div>

<style>
  .page { padding-top: 24px; max-width: 720px; }
  .page h1 { font-size: 26px; margin-bottom: 8px; }
  .intro { margin-bottom: 16px; line-height: 1.6; }
  .empty { display: flex; flex-direction: column; gap: 8px; }
  .row { display: flex; gap: 12px; align-items: center; margin-top: 4px; flex-wrap: wrap; }
  .rows { display: flex; flex-direction: column; gap: 10px; }
  .dev-row {
    display: flex; gap: 12px; align-items: center;
    background: var(--panel); border: 1px solid var(--border);
    border-radius: 4px; padding: 14px 16px;
  }
  .dev-row.mine {
    border-color: rgba(139, 92, 246, 0.5);
    background: rgba(139, 92, 246, 0.08);
  }
  .ic {
    flex-shrink: 0; width: 40px; height: 40px; border-radius: 50%;
    background: #2a2a2a; color: var(--muted);
    display: flex; align-items: center; justify-content: center; font-size: 16px;
  }
  .meta { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .name { font-size: 15px; font-weight: 600; word-break: break-all; }
  .id { font-size: 12px; word-break: break-all; }
  .time { font-size: 12px; }
  .badge {
    flex-shrink: 0; font-size: 12px; font-weight: 700; color: #a78bfa;
    background: rgba(139, 92, 246, 0.15); padding: 4px 10px; border-radius: 999px;
  }
  .btn.small { padding: 6px 12px; font-size: 13px; }
  .small { font-size: 13px; }
</style>
