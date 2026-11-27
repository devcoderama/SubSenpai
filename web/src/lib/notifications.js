import { writable, get } from 'svelte/store';
import { getNotifications } from './api.js';
import { user } from './auth.js';

/** Jumlah notifikasi belum dibaca — dipakai lencana lonceng di Navbar.
    Diperbarui saat login berubah dan sesudah halaman /notifikasi menandai
    semuanya dibaca. Gagal memuat = lencana hilang, bukan error halaman. */
export const unreadCount = writable(0);

export async function refreshUnread() {
  if (!get(user)) {
    unreadCount.set(0);
    return;
  }
  try {
    const res = await getNotifications();
    unreadCount.set(res.unread || 0);
  } catch {
    // Diam saja.
  }
}
