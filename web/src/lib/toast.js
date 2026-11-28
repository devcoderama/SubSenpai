import { writable } from 'svelte/store';

// Toast global (28 Nov 2026, rekomendasi no. 4): umpan balik sementara
// untuk aksi yang gagal/berhasil — menggantikan teks error inline
// (Daftar Saya di hero/Detail/kartu, aksi PostCard, salin di Premium).
// Maks 3 tampil sekaligus; error 5 dtk, lainnya 4 dtk; ketuk × menutup.

let nextId = 0;
export const toasts = writable([]);

export function showToast(message, type = 'info', ms = null) {
  const id = ++nextId;
  const duration = ms ?? (type === 'error' ? 5000 : 4000);
  toasts.update((list) => [...list.slice(-2), { id, message, type }]);
  setTimeout(() => dismissToast(id), duration);
  return id;
}

export function dismissToast(id) {
  toasts.update((list) => list.filter((t) => t.id !== id));
}

export const toastSuccess = (message, ms) => showToast(message, 'success', ms);
export const toastError = (message, ms) => showToast(message, 'error', ms);
export const toastInfo = (message, ms) => showToast(message, 'info', ms);
