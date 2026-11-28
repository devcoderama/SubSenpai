// Cache sesi Beranda ala HomeSession di aplikasi (28 Nov 2026).
// Modul ini hidup selama SPA berjalan: buka Detail lalu kembali
// tidak memuat ulang Beranda dari nol dan tidak kembali ke atas.
// Data + posisi scroll disimpan per tab (Semua/Film/Anime/Donghua);
// isi daftar per tab (grid / Hasil filter / hasil cari) ikut
// tersimpan, jadi infinite scroll lanjut dari halaman terakhir.
function freshTab() {
  return {
    grid: [], nextCursor: '',
    facets: null,
    genre: '', year: '', country: '', network: '',
    filterMode: false, results: [], resultsCursor: '',
    searchText: '', searchResults: [], searchCursor: '',
  };
}

export const homeCache = {
  loaded: false,
  tab: '',
  rails: { anime: [], donghua: [], film: [], series: [] },
  trending: [],
  filmYear: [], animeYear: [], donghuaYear: [], seriesYear: [],
  filmPopuler: [], seriesPopuler: [],
  tvChannels: [],
  animeOngoing: [], donghuaOngoing: [],
  heroItems: [],
  continueWatching: [],
  scroll: { '': 0, film: 0, anime: 0, donghua: 0, series: 0 },
  tabs: { '': freshTab(), film: freshTab(), anime: freshTab(), donghua: freshTab(), series: freshTab() },
};
