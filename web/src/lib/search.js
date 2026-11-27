// Pencarian kontekstual navbar (27 Nov 2026): tombol cari di navbar
// memfilter halaman yang sedang dibuka, bukan selalu pindah ke /cari.
//   Beranda  -> cari judul (API, dalam jenis/chip aktif)
//   Post     -> cari postingan (API feed, q)
//   Jadwal   -> cari jadwal (filter data yang sudah dimuat)
//   Riwayat  -> cari riwayat (filter data yang sudah dimuat)
//   Halaman lain -> perilaku lama: buka /cari?q=...
//
// Navbar menulis ke store ini; keempat halaman di atas membacanya dan
// menerapkan filter masing-masing. Pindah halaman membersihkan pencarian
// (Navbar mengawasi perubahan rute).
import { writable } from 'svelte/store';

export const searchOpen = writable(false);
export const searchQuery = writable('');

export function openSearch() {
  searchOpen.set(true);
}

// Sembunyikan bilah cari, tapi kata kunci tetap (dipakai sesudah Enter di
// halaman yang pencariannya kontekstual — hasil tetap terfilter).
export function hideSearchRow() {
  searchOpen.set(false);
}

// Batal / pindah halaman: sembunyikan bilah DAN bersihkan kata kunci,
// supaya halaman kembali ke tampilan normal.
export function closeSearch() {
  searchOpen.set(false);
  searchQuery.set('');
}

export function toggleSearch() {
  searchOpen.update((v) => {
    if (v) searchQuery.set('');
    return !v;
  });
}
