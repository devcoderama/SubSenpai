import { writable, get } from 'svelte/store';
import { checkWatchlist, addWatchlist, removeWatchlist } from './api.js';
import { user } from './auth.js';

// Status Daftar Saya BERSAMA untuk semua tombol di halaman (28 Nov 2026):
// satu judul bisa muncul di hero + beberapa rel — toggle di satu tempat
// harus langsung tercermin di semua duplikatnya. Per id: undefined =
// belum dicek, true/false = status. Cek yang sedang berjalan dishare
// antar pemanggil supaya tidak dobel.
export const wlMap = writable({});
export const wlPending = writable({});

const inflight = {};

export function ensureWl(id) {
  if (!id || get(wlMap)[id] !== undefined || !get(user)) return Promise.resolve(get(wlMap)[id]);
  if (!inflight[id]) {
    wlPending.update((p) => ({ ...p, [id]: true }));
    inflight[id] = checkWatchlist(id)
      .then((c) => {
        const v = !!c?.in_watchlist;
        wlMap.update((m) => ({ ...m, [id]: v }));
        return v;
      })
      .catch(() => undefined)
      .finally(() => {
        delete inflight[id];
        wlPending.update((p) => {
          const n = { ...p };
          delete n[id];
          return n;
        });
      });
  }
  return inflight[id];
}

// Toggle; lempar error bila API gagal (pemanggil menampilkan pesannya).
export async function toggleWl(id) {
  let cur = get(wlMap)[id];
  if (cur === undefined) cur = await ensureWl(id);
  wlPending.update((p) => ({ ...p, [id]: true }));
  try {
    if (cur) {
      await removeWatchlist(id);
      wlMap.update((m) => ({ ...m, [id]: false }));
      return false;
    }
    await addWatchlist(id);
    wlMap.update((m) => ({ ...m, [id]: true }));
    return true;
  } finally {
    wlPending.update((p) => {
      const n = { ...p };
      delete n[id];
      return n;
    });
  }
}
