<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    getTitles, getTitle, getHome, getHistory,
    checkWatchlist, addWatchlist, removeWatchlist, getFilters
  } from '$lib/api.js';
  import { homeCache } from '$lib/homeCache.js';
  import { user, isPremium } from '$lib/auth.js';
  import { searchQuery } from '$lib/search.js';
  import { getLocalHistory, fmtWhen } from '$lib/history.js';
  import Hero from '$lib/components/Hero.svelte';
  import Rail from '$lib/components/Rail.svelte';
  import TitleCard from '$lib/components/TitleCard.svelte';
  import Skeleton from '$lib/components/Skeleton.svelte';
  import { reveal, infiniteScroll } from '$lib/reveal.js';

  const TABS = [
    { id: '', label: 'Semua' },
    { id: 'film', label: 'Film' },
    { id: 'anime', label: 'Anime' },
    { id: 'donghua', label: 'Donghua' }
  ];

  const THIS_YEAR = new Date().getFullYear();

  let tab = '';
  let loading = true;
  let error = '';
  // Hero berganti otomatis (27 Nov 2026): daftar putar dari judul
  // paling banyak ditonton (trending 24 jam backend) — 5 teratas.
  let heroItems = []; // [{ title, firstEp }]
  let heroIdx = 0;
  let trending = []; // kartu trending (tanpa sinopsis)
  let filmYear = []; // film tahun berjalan (Film Terbaru)
  let animeYear = []; // anime tahun berjalan (Anime Terbaru)
  let donghuaYear = []; // donghua tahun berjalan (Donghua Terbaru)
  let filmPopuler = []; // film dari trending 24 jam (Film Populer)
  let animeOngoing = []; // anime berstatus ongoing (dimuat di latar)
  let donghuaOngoing = []; // donghua berstatus ongoing (dimuat di latar)
  let heroTimer = null;
  let heroPaused = false;
  let heroOffscreen = false;
  let pageHidden = false;
  let reduceMotion = false;
  const HERO_MS = 6000;
  const heroDetailCache = new Map();
  // Toggle Daftar Saya di hero (27 Nov 2026): mengikuti judul yang
  // sedang tampil, seperti Hero aplikasi (LaunchedEffect hero.id).
  let heroInWatchlist = false;
  let heroWlBusy = false;
  let heroWlError = '';
  let heroWlFor = '';
  let heroWlLogged = false;
  let rails = { anime: [], donghua: [], film: [] };
  let grid = [];
  let nextCursor = '';
  let loadingMore = false;
  let moreError = '';
  let ioSupported = true;
  let continueWatching = [];

  // Pencarian kontekstual navbar (27 Nov 2026): kata kunci dari navbar
  // memfilter Beranda di tempat — hasil cari (API, dalam jenis/chip aktif)
  // menggantikan hero + rel + grid selama kata kunci terisi. Ganti chip
  // saat mencari = cari dalam jenis baru; hapus kata kunci = kembali ke
  // tampilan tab normal.
  let searchMode = false;
  let searchText = '';
  let searchResults = [];
  let searchCursor = '';
  let searchLoading = false;
  let searchLoadingMore = false;
  let searchMoreError = '';
  let searchTimer = null;
  let searchSeq = 0;

  $: if ($searchQuery !== undefined) onSearchQuery($searchQuery);

  function onSearchQuery(q) {
    clearTimeout(searchTimer);
    const query = (q || '').trim();
    // Hidrasi dari cache: kata kunci tidak berubah sejak pergi —
    // hasil pencarian sudah dipulihkan, jangan cari ulang.
    if (hydrating) {
      hydrating = false;
      const cachedText = (tab && homeCache.tabs[tab]?.searchText) || '';
      if (query && query === cachedText) return;
    }
    if (!query) {
      searchMode = false;
      searchText = '';
      searchResults = [];
      searchCursor = '';
      searchMoreError = '';
      if (!filterMode) restartHeroTimer();
      return;
    }
    searchText = query;
    if (!searchMode) {
      searchMode = true;
      stopHeroTimer();
    }
    // Bersihkan hasil lama + tampilkan skeleton langsung (bukan sesudah
    // jeda ketik) supaya tidak berkedip.
    searchResults = [];
    searchCursor = '';
    searchLoading = true;
    searchTimer = setTimeout(() => runSearch(query), 300);
  }

  async function runSearch(query) {
    const seq = ++searchSeq;
    const tt = tab;
    searchLoading = true;
    searchMoreError = '';
    try {
      const res = await getTitles({ type: tt, q: query });
      if (seq !== searchSeq || tab !== tt) return;
      searchResults = res.items || [];
      searchCursor = res.next_cursor || '';
      listsFor = tt;
    } catch (e) {
      if (seq !== searchSeq || tab !== tt) return;
      searchResults = [];
      searchCursor = '';
      searchMoreError = e.message || 'Pencarian gagal.';
    } finally {
      if (seq === searchSeq) searchLoading = false;
    }
  }

  async function loadMoreSearch() {
    if (!searchCursor || searchLoadingMore) return;
    const tt = tab;
    const cur = searchCursor;
    searchLoadingMore = true;
    searchMoreError = '';
    try {
      const res = await getTitles({ type: tt, q: searchText, cursor: cur });
      if (tab !== tt) return;
      searchResults = [...searchResults, ...(res.items || [])];
      searchCursor = res.next_cursor || '';
    } catch (e) {
      if (tab !== tt) return;
      searchMoreError = e.message || 'Gagal memuat lagi.';
    } finally {
      searchLoadingMore = false;
    }
  }

  // Filter genre + lanjutan di tab jenis (28 Nov 2026, cerminan
  // aplikasi): baris chip di bawah chip jenis — hanya di tab
  // Film/Anime/Donghua. Genre atau filter aktif = mode Hasil (hero
  // disembunyikan, grid diganti hasil getTitles berfilter + infinite
  // scroll). Pilihan diingat per jenis supaya pindah tab lalu kembali
  // tidak hilang.
  let genre = '';
  let fYear = '';
  let fCountry = '';
  let fNetwork = '';
  let facets = null; // {genres, years, countries, networks} jenis aktif
  let facetsFor = '';
  let filterOpen = false;
  let filterMode = false;
  let results = [];
  let resultsCursor = '';
  let resultsLoading = false;
  let resultsLoadingMore = false;
  let resultsError = '';
  let resultsMoreError = '';
  let filterSeq = 0;
  // listsFor: tab yang sedang ditampilkan daftarnya di variabel
  // grid/results/searchResults (lihat bagian Cache di bawah). HARUS
  // dideklarasi SEBELUM blok hidrasi — applyTabState dipanggil saat
  // inisiasi dan menulis variabel ini.
  let listsFor = '';
  // Cache sesi Beranda (28 Nov 2026, ala HomeSession di aplikasi):
  // pilihan filter per jenis kini hidup di homeCache (ikut grid,
  // Hasil filter, hasil cari, dan posisi scroll per tab) supaya
  // pindah ke Detail lalu kembali tidak memuat ulang dari nol.
  // Hidrasi berjalan di sini — SEBELUM render pertama — supaya
  // tidak ada skeleton dan posisi scroll bisa kembali tepat.
  let hydrating = homeCache.loaded;
  if (hydrating) {
    tab = homeCache.tab;
    rails = homeCache.rails;
    trending = homeCache.trending;
    filmYear = homeCache.filmYear;
    animeYear = homeCache.animeYear;
    donghuaYear = homeCache.donghuaYear;
    filmPopuler = homeCache.filmPopuler;
    animeOngoing = homeCache.animeOngoing;
    donghuaOngoing = homeCache.donghuaOngoing;
    heroItems = homeCache.heroItems;
    continueWatching = homeCache.continueWatching;
    loading = false;
    searchMode = !!(tab && homeCache.tabs[tab]?.searchText);
    if (tab) applyTabState(tab);
  }

  // Hitungan filter aktif (28 Nov 2026): HARUS deklarasi reaktif
  // $:, bukan panggilan fungsi di template — Svelte tidak melacak
  // variabel di dalam badan fungsi, jadi label "Filter (n)" tidak
  // pernah berubah (ditemukan verifikasi live 05:17 WIB).
  $: filterCount = [genre, fYear, fCountry, fNetwork].filter(Boolean).length;
  $: hasExtra =
    !!facets &&
    ((facets.years?.length || 0) +
      (facets.countries?.length || 0) +
      (facets.networks?.length || 0) >
      0);

  function filterActive() {
    return !!(genre || fYear || fCountry || fNetwork);
  }

  async function loadFacets(t) {
    if (!t) {
      facets = null;
      facetsFor = '';
      return;
    }
    if (facetsFor === t && facets) return;
    try {
      const f = await getFilters(t);
      if (tab !== t) return;
      facets = f;
      facetsFor = t;
    } catch {
      facets = null;
      facetsFor = '';
    }
  }

  async function runFilter() {
    const seq = ++filterSeq;
    const tt = tab;
    filterMode = true;
    stopHeroTimer();
    resultsLoading = true;
    resultsError = '';
    resultsMoreError = '';
    results = [];
    resultsCursor = '';
    try {
      const res = await getTitles({
        type: tt,
        genre,
        year: fYear,
        country: fCountry,
        network: fNetwork
      });
      if (seq !== filterSeq || tab !== tt) return;
      results = res.items || [];
      resultsCursor = res.next_cursor || '';
      listsFor = tt;
    } catch (e) {
      if (seq !== filterSeq || tab !== tt) return;
      resultsError = e.message || 'Gagal memuat hasil filter.';
    } finally {
      if (seq === filterSeq) resultsLoading = false;
    }
  }

  async function loadMoreFilter() {
    if (!resultsCursor || resultsLoadingMore) return;
    const seq = filterSeq;
    const tt = tab;
    resultsLoadingMore = true;
    resultsMoreError = '';
    try {
      const res = await getTitles({
        type: tt,
        genre,
        year: fYear,
        country: fCountry,
        network: fNetwork,
        cursor: resultsCursor
      });
      if (seq !== filterSeq || tab !== tt) return;
      results = [...results, ...(res.items || [])];
      resultsCursor = res.next_cursor || '';
    } catch (e) {
      if (seq !== filterSeq || tab !== tt) return;
      resultsMoreError = e.message || 'Gagal memuat lagi.';
    } finally {
      if (seq === filterSeq) resultsLoadingMore = false;
    }
  }

  function applyFilterChange() {
    if (filterActive()) runFilter();
    else exitFilterMode();
  }

  function exitFilterMode() {
    filterMode = false;
    results = [];
    resultsCursor = '';
    resultsError = '';
    resultsMoreError = '';
    // Grid tab sudah di memori — tanpa muat ulang (28 Nov 2026).
  }

  function pickGenre(g) {
    genre = g;
    applyFilterChange();
  }

  function resetFilters() {
    genre = '';
    fYear = '';
    fCountry = '';
    fNetwork = '';
    filterOpen = false;
    exitFilterMode();
  }

  // Lanjutkan Menonton (seperti aplikasi): server bila masuk, lokal bila tamu.
  async function loadContinue() {
    try {
      const raw = $user ? (await getHistory(12)).items || [] : getLocalHistory();
      const seen = new Set();
      continueWatching = raw
        .filter((it) => {
          if (seen.has(it.title_id)) return false;
          seen.add(it.title_id);
          return true;
        })
        .slice(0, 12)
        .map((it) => ({
          id: it.title_id,
          poster_url: it.poster_url,
          title: it.title,
          sub: 'Episode ' + it.episode_no + ' • ' + fmtWhen(it.updated_at),
          href: '/nonton/' + it.episode_id
        }));
      homeCache.continueWatching = continueWatching;
    } catch {
      continueWatching = [];
    }
  }

  // Hero: ambil detail (sinopsis + episode pertama) untuk maks 5 judul
  // calon, paralel; yang gagal dilewati. Detail di-cache per id supaya
  // ganti tab tidak memuat ulang. Hanya panggilan terbaru yang boleh
  // mengganti hero — muat hero tab lama yang selesai belakangan tidak
  // boleh menimpa hero tab aktif (28 Nov 2026).
  let heroSeq = 0;
  async function loadHero(candidates) {
    const seq = ++heroSeq;
    const picks = (candidates || []).slice(0, 5);
    const settled = await Promise.allSettled(
      picks.map(async (c) => {
        if (heroDetailCache.has(c.id)) return heroDetailCache.get(c.id);
        const det = await getTitle(c.id);
        const item = {
          title: { ...(det.title || det), synopsis: det.title?.synopsis || det.synopsis },
          firstEp: det.episodes?.[0]?.id || null
        };
        heroDetailCache.set(c.id, item);
        return item;
      })
    );
    if (seq !== heroSeq) return;
    heroItems = settled.filter((r) => r.status === 'fulfilled').map((r) => r.value);
    heroIdx = 0;
    restartHeroTimer();
  }

  function restartHeroTimer() {
    stopHeroTimer();
    if (heroItems.length < 2 || reduceMotion || heroPaused || pageHidden || heroOffscreen) return;
    heroTimer = setInterval(() => {
      heroIdx = (heroIdx + 1) % heroItems.length;
    }, HERO_MS);
  }
  function stopHeroTimer() {
    if (heroTimer) {
      clearInterval(heroTimer);
      heroTimer = null;
    }
  }
  function goHero(i, manual = false) {
    if (!heroItems.length) return;
    heroIdx = (i + heroItems.length) % heroItems.length;
    if (manual) restartHeroTimer();
  }

  // Status Daftar Saya dibaca ulang tiap hero berganti (dan saat status
  // login berubah); tamu tidak dicek — ketuk langsung ke /masuk.
  $: {
    const hid = heroItems[heroIdx]?.title?.id;
    const logged = !!$user;
    if (hid && (hid !== heroWlFor || logged !== heroWlLogged)) {
      heroWlFor = hid;
      heroWlLogged = logged;
      heroInWatchlist = false;
      heroWlError = '';
      if (logged) refreshHeroWatchlist(hid);
    } else if (!hid) {
      heroWlFor = '';
      heroWlLogged = logged;
    }
  }

  async function refreshHeroWatchlist(id) {
    try {
      const c = await checkWatchlist(id);
      if (heroWlFor === id) heroInWatchlist = !!c.in_watchlist;
    } catch {
      /* abaikan — tombol tetap bisa diketuk, seperti Detail */
    }
  }

  // Sama seperti toggleWatchlist di Detail: tamu -> /masuk, sibuk
  // dikunci, sudah tersimpan -> hapus.
  async function toggleHeroWatchlist() {
    if (!$user) {
      goto('/masuk');
      return;
    }
    const id = heroItems[heroIdx]?.title?.id;
    if (!id || heroWlBusy) return;
    heroWlBusy = true;
    heroWlError = '';
    try {
      if (heroInWatchlist) {
        await removeWatchlist(id);
        heroInWatchlist = false;
      } else {
        await addWatchlist(id);
        heroInWatchlist = true;
      }
    } catch (e) {
      heroWlError = e.message || 'Gagal mengubah Daftar Saya.';
    } finally {
      heroWlBusy = false;
    }
  }

  // Geser (swipe) kiri/kanan di hero untuk ganti slide.
  let touchX = null;
  function onTouchStart(e) {
    touchX = e.changedTouches[0].clientX;
  }
  function onTouchEnd(e) {
    if (touchX == null) return;
    const dx = e.changedTouches[0].clientX - touchX;
    touchX = null;
    if (Math.abs(dx) > 50) goHero(heroIdx + (dx < 0 ? 1 : -1), true);
  }

  // Hero berhenti berputar saat sudah di luar layar (28 Nov 2026):
  // tiap ganti slide tingginya bisa berubah mengikuti panjang
  // sinopsis, dan itu menggeser seluruh halaman di bawahnya — posisi
  // scroll pengguna ikut hanyut padahal hero tidak terlihat.
  function observeHero(node) {
    if (!('IntersectionObserver' in window)) return {};
    const io = new IntersectionObserver(([e]) => {
      heroOffscreen = !e.isIntersecting;
      if (heroOffscreen) stopHeroTimer();
      else restartHeroTimer();
    });
    io.observe(node);
    return {
      destroy() {
        io.disconnect();
      }
    };
  }

  let loadTabSeq = 0;
  async function loadTab(t) {
    // Muat yang kedaluwarsa (pengguna sudah pindah tab saat muat
    // berjalan) tidak boleh menimpa daftar tab aktif — ini yang
    // membuat pindah tab kembali ke atas di verifikasi 06:02 WIB.
    const seq = ++loadTabSeq;
    loading = true;
    error = '';
    stopHeroTimer();
    try {
      if (t === '') {
        // Rel dimuat terpisah: satu jenis yang gagal tidak boleh
        // mengosongkan seluruh Beranda.
        const [a, d, f, h, fy, ay, dy] = await Promise.allSettled([
          getTitles({ type: 'anime' }),
          getTitles({ type: 'donghua' }),
          getTitles({ type: 'film' }),
          getHome(),
          getTitles({ type: 'film', year: THIS_YEAR }),
          getTitles({ type: 'anime', year: THIS_YEAR }),
          getTitles({ type: 'donghua', year: THIS_YEAR })
        ]);
        if (seq !== loadTabSeq || tab !== t) return;
        rails = {
          anime: a.status === 'fulfilled' ? a.value.items || [] : [],
          donghua: d.status === 'fulfilled' ? d.value.items || [] : [],
          film: f.status === 'fulfilled' ? f.value.items || [] : []
        };
        if (!rails.anime.length && !rails.donghua.length && !rails.film.length) {
          const failed = [a, d, f].find((r) => r.status === 'rejected');
          if (failed && failed.status === 'rejected') throw failed.reason;
        }
        trending = h.status === 'fulfilled' ? h.value.trending || [] : [];
        filmYear = fy.status === 'fulfilled' ? fy.value.items || [] : [];
        animeYear = ay.status === 'fulfilled' ? ay.value.items || [] : [];
        donghuaYear = dy.status === 'fulfilled' ? dy.value.items || [] : [];
        filmPopuler = trending.filter((x) => x.type === 'film');
        // Ongoing dimuat di latar (tidak menahan skeleton): rel muncul
        // saat siap. Donghua ongoing jarang di halaman teratas (8 dari
        // 100 judul terbaru), jadi dipindai sampai 8 halaman.
        loadOngoing('anime');
        loadOngoing('donghua');
        grid = [];
        listsFor = '';
        // Hero: 5 judul paling banyak ditonton (trending 24 jam);
        // cadangan bila tren kosong: judul pertama tiap rel.
        const cands = trending.length
          ? trending
          : [rails.anime[0], rails.donghua[0], rails.film[0]].filter(Boolean);
        await loadHero(cands);
        if (seq !== loadTabSeq || tab !== t) return;
        saveHomeState();
      } else {
        const res = await getTitles({ type: t });
        if (seq !== loadTabSeq || tab !== t) return;
        grid = res.items || [];
        nextCursor = res.next_cursor || '';
        listsFor = t;
        // Rel Semua tetap di memori (tidak dihapus) supaya kembali
        // ke tab Semua tidak memuat ulang.
        // Hero jenis ini: trending jenis itu; cadangan: 5 pertama grid.
        const cands = trending.filter((x) => x.type === t).slice(0, 5);
        await loadHero(cands.length >= 2 ? cands : grid.slice(0, 5));
        if (seq !== loadTabSeq || tab !== t) return;
      }
    } catch (e) {
      if (seq === loadTabSeq) error = e.message || 'Gagal memuat beranda.';
    } finally {
      if (seq === loadTabSeq) loading = false;
    }
  }

  async function loadMore() {
    if (!nextCursor || loadingMore) return;
    const tt = tab;
    const cur = nextCursor;
    loadingMore = true;
    moreError = '';
    try {
      const res = await getTitles({ type: tt, cursor: cur });
      if (tab !== tt) return;
      grid = [...grid, ...(res.items || [])];
      nextCursor = res.next_cursor || '';
    } catch (e) {
      if (tab !== tt) return;
      // Gagal memuat halaman berikut tidak boleh mengosongkan grid —
      // tampilkan retry di bawah daftar.
      moreError = e.message || 'Gagal memuat lagi.';
    } finally {
      loadingMore = false;
    }
  }

  // Rel Ongoing: pindai halaman katalog jenis itu, saring kartu
  // berstatus ongoing (backend belum punya filter status) — sampai
  // 12 judul atau 8 halaman. Dijalankan tanpa await supaya tidak
  // menahan skeleton Beranda.
  async function loadOngoing(t) {
    const out = [];
    let cursor = '';
    let pages = 0;
    try {
      while (out.length < 12 && pages < 8) {
        const res = await getTitles({ type: t, ...(cursor ? { cursor } : {}) });
        const items = res.items || [];
        for (const it of items) if (it.status === 'ongoing' && out.length < 12) out.push(it);
        cursor = res.next_cursor || '';
        pages++;
        if (!cursor) break;
      }
    } catch {
      /* abaikan — rel ongoing disembunyikan bila gagal */
    }
    if (t === 'anime') {
      animeOngoing = out;
      homeCache.animeOngoing = out;
    } else if (t === 'donghua') {
      donghuaOngoing = out;
      homeCache.donghuaOngoing = out;
    }
  }

  // --- Cache sesi Beranda (28 Nov 2026) ---
  // Muat yang kedaluwarsa (pengguna sudah pindah tab saat muat
  // berjalan) tidak boleh menimpa daftar tab lain — baik saat
  // menampilkan maupun saat menyimpan cache.
  let restoringScroll = false;
  function saveScroll() {
    if (restoringScroll) return;
    homeCache.scroll[tab] = window.scrollY;
  }
  async function restoreScroll(y) {
    const target = y ?? (homeCache.scroll[tab] || 0);
    restoringScroll = true;
    await tick();
    window.scrollTo(0, target);
    const landed = window.scrollY;
    restoringScroll = false;
    // Bila tata letak di atas viewport masih berubah sesudah scroll
    // pertama (mis. hero berganti slide), coba sekali lagi — kecuali
    // pengguna sudah menggulir sendiri.
    await new Promise((r) => setTimeout(r, 250));
    if (window.scrollY === landed && landed < target - 50 && document.documentElement.scrollHeight > target) {
      window.scrollTo(0, target);
    }
  }
  function stashTabState(t) {
    if (t === undefined || t === null) return;
    const s = homeCache.tabs[t];
    if (!s) return;
    s.facets = facets;
    s.genre = genre;
    s.year = fYear;
    s.country = fCountry;
    s.network = fNetwork;
    s.filterMode = filterMode;
    // Daftar hanya disimpan bila memang milik tab ini.
    if (listsFor === t) {
      s.grid = grid;
      s.nextCursor = nextCursor;
      s.results = results;
      s.resultsCursor = resultsCursor;
      s.searchText = searchText;
      s.searchResults = searchResults;
      s.searchCursor = searchCursor;
    }
  }
  function applyTabState(t) {
    const s = homeCache.tabs[t];
    if (!s) return;
    grid = s.grid;
    nextCursor = s.nextCursor;
    facets = s.facets;
    facetsFor = s.facets ? t : '';
    genre = s.genre;
    fYear = s.year;
    fCountry = s.country;
    fNetwork = s.network;
    filterMode = s.filterMode;
    results = s.results;
    resultsCursor = s.resultsCursor;
    searchText = s.searchText;
    searchResults = s.searchResults;
    searchCursor = s.searchCursor;
    listsFor = t;
  }
  function saveHomeState() {
    homeCache.loaded = true;
    homeCache.tab = tab;
    homeCache.rails = rails;
    homeCache.trending = trending;
    homeCache.filmYear = filmYear;
    homeCache.animeYear = animeYear;
    homeCache.donghuaYear = donghuaYear;
    homeCache.filmPopuler = filmPopuler;
    homeCache.animeOngoing = animeOngoing;
    homeCache.donghuaOngoing = donghuaOngoing;
    homeCache.heroItems = heroItems;
    homeCache.continueWatching = continueWatching;
    saveScroll();
  }

  function setTab(t) {
    // Simpan posisi scroll + daftar tab lama sebelum pindah.
    const targetY = homeCache.scroll[t] || 0;
    saveScroll();
    stashTabState(tab);
    tab = t;
    filterOpen = false;
    loadFacets(t);
    const s = homeCache.tabs[t];
    const hadContent =
      s.grid.length ||
      s.results.length ||
      s.searchResults.length ||
      s.facets ||
      (t === '' && (rails.anime.length || rails.donghua.length || rails.film.length));
    if (hadContent) {
      // Tab ini sudah pernah dimuat: pulihkan tanpa muat ulang.
      applyTabState(t);
      loading = false;
      if (searchMode) {
        if (searchText !== s.searchText) runSearch(searchText).then(() => restoreScroll(targetY));
        else restoreScroll(targetY);
      } else {
        // Hero kembali ke jenis/tab ini (detail di-cache, jadi murah);
        // mode filter tidak punya hero, jadi jangan nyalakan timernya.
        if (!filterMode) {
          const cands =
            t === ''
              ? trending.length
                ? trending
                : [rails.anime[0], rails.donghua[0], rails.film[0]].filter(Boolean)
              : trending.filter((x) => x.type === t).slice(0, 5);
          if (cands.length) loadHero(t === '' ? cands : cands.length >= 2 ? cands : grid.slice(0, 5));
        }
        restoreScroll(targetY);
      }
      return;
    }
    // Tab baru: reset filter lalu muat seperti sebelumnya; posisi
    // scroll dipulihkan sesudah muat selesai (bila tab sempat
    // ditinggalkan saat muat berjalan, jangan ubah posisinya).
    genre = '';
    fYear = '';
    fCountry = '';
    fNetwork = '';
    filterMode = false;
    const tt = t;
    if (searchMode) runSearch(searchText).then(() => {
      if (tab === tt) restoreScroll(targetY);
    });
    else {
      loadTab(t).then(() => {
        if (tab === tt) restoreScroll(targetY);
      });
    }
  }

  onMount(() => {
    ioSupported = 'IntersectionObserver' in window;
    reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const onVis = () => {
      pageHidden = document.hidden;
      if (pageHidden) stopHeroTimer();
      else restartHeroTimer();
    };
    document.addEventListener('visibilitychange', onVis);
    window.addEventListener('scroll', saveScroll, { passive: true });
    if (homeCache.loaded) {
      // Kembali dari halaman lain: konten sudah dipulihkan saat
      // inisiasi; segarkan Lanjutkan Menonton (bisa bertambah
      // sesudah menonton) lalu kembali ke posisi scroll tab aktif.
      if (!searchMode && !filterMode) restartHeroTimer();
      const y = homeCache.scroll[tab] || 0;
      loadContinue().finally(() => restoreScroll(y));
    } else {
      loadTab('');
      loadContinue();
    }
    return () => {
      document.removeEventListener('visibilitychange', onVis);
      window.removeEventListener('scroll', saveScroll);
    };
  });
  onDestroy(() => {
    stopHeroTimer();
    saveScroll();
    stashTabState(tab);
    saveHomeState();
  });
</script>

<div class="container tabs-top">
  <div class="tabs">
    {#each TABS as t}
      <button class="chip" class:active={tab === t.id} on:click={() => setTab(t.id)}>{t.label}</button>
    {/each}
  </div>
</div>

{#if !searchMode && tab && facets}
  <!-- Baris filter genre (28 Nov 2026, cerminan aplikasi): Filter +
       Semua Genre + chip genre jenis aktif; aktif = merah. -->
  <div class="container genre-row-wrap">
    <div class="genre-row">
      {#if hasExtra}
        <button
          class="genre-chip filter-btn"
          class:active={filterCount > 0}
          on:click={() => (filterOpen = !filterOpen)}
        >
          <i class="fa-solid fa-sliders"></i> Filter{filterCount ? ` (${filterCount})` : ''}
        </button>
      {/if}
      <button class="genre-chip" class:active={!genre} on:click={() => pickGenre('')}>Semua Genre</button>
      {#each facets.genres || [] as g}
        <button class="genre-chip" class:active={genre === g.name} on:click={() => pickGenre(g.name)}
          >{g.name}</button
        >
      {/each}
    </div>
    {#if filterOpen}
      <div class="filter-panel">
        <!-- Genre di dalam panel (28 Nov 2026): semua genre tampil
             membungkus ke bawah, jadi tidak perlu geser baris chip. -->
        <div class="filter-genres">
          <span class="filter-label">Genre</span>
          <div class="genre-wrap">
            <button class="genre-chip" class:active={!genre} on:click={() => pickGenre('')}>Semua Genre</button>
            {#each facets.genres || [] as g}
              <button class="genre-chip" class:active={genre === g.name} on:click={() => pickGenre(g.name)}
                >{g.name}</button
              >
            {/each}
          </div>
        </div>
        <div class="filter-fields">
          {#if facets.years?.length}
            <label
              >Tahun
              <select
                value={fYear}
                on:change={(e) => {
                  fYear = e.currentTarget.value;
                  applyFilterChange();
                }}
              >
                <option value="">Semua</option>
                {#each facets.years as y}<option value={y.value}>{y.name}</option>{/each}
              </select>
            </label>
          {/if}
          {#if facets.countries?.length}
            <label
              >Negara
              <select
                value={fCountry}
                on:change={(e) => {
                  fCountry = e.currentTarget.value;
                  applyFilterChange();
                }}
              >
                <option value="">Semua</option>
                {#each facets.countries as c}<option value={c.value}>{c.name}</option>{/each}
              </select>
            </label>
          {/if}
          {#if facets.networks?.length}
            <label
              >Jaringan
              <select
                value={fNetwork}
                on:change={(e) => {
                  fNetwork = e.currentTarget.value;
                  applyFilterChange();
                }}
              >
                <option value="">Semua</option>
                {#each facets.networks as n}<option value={n.value}>{n.name}</option>{/each}
              </select>
            </label>
          {/if}
        </div>
        <div class="filter-actions">
          <button class="btn ghost" on:click={resetFilters}>Reset filter</button>
          <button class="btn ghost" on:click={() => (filterOpen = false)}>Tutup</button>
        </div>
      </div>
    {/if}
  </div>
{/if}

{#if loading}
  {#if tab === ''}
    <Skeleton kind="hero" />
    <div class="container">
      <div class="rail" aria-hidden="true"><Skeleton kind="card" count={6} /></div>
      <div class="rail" aria-hidden="true"><Skeleton kind="card" count={6} /></div>
    </div>
  {:else}
    <div class="container">
      <div class="poster-grid"><Skeleton kind="card" count={12} /></div>
    </div>
  {/if}
{:else if searchMode}
  <!-- Hasil cari navbar: menggantikan hero + rel + grid selama mencari. -->
  <div class="container search-results">
    <p class="muted">
      {searchLoading
        ? 'Mencari…'
        : searchResults.length + ' judul ditemukan untuk “' + searchText + '”'}{tab && !searchLoading
        ? ' · ' + (TABS.find((t) => t.id === tab)?.label || '')
        : ''}
    </p>
    {#if searchMoreError}
      <p class="error">{searchMoreError}</p>
    {/if}
    {#if searchLoading}
      <div class="poster-grid"><Skeleton kind="card" count={12} /></div>
    {:else}
      <div class="poster-grid">
        {#each searchResults as item, i (item.id)}
          <div use:reveal={(i % 12) * 45}><TitleCard {item} /></div>
        {/each}
      </div>
      {#if !searchResults.length && !searchMoreError}
        <p class="muted">Tidak ada judul untuk “{searchText}”. Coba kata kunci lain.</p>
      {/if}
      {#if searchCursor}
        <div
          class="sentinel"
          use:infiniteScroll={{ enabled: !!searchCursor && !searchLoadingMore && !searchMoreError, onLoad: loadMoreSearch }}
        ></div>
        {#if searchLoadingMore}
          <p class="load-more-note"><span class="bar skeleton"></span>Memuat lagi…</p>
        {:else if searchMoreError}
          <div class="more">
            <button
              class="btn ghost"
              on:click={() => {
                searchMoreError = '';
                loadMoreSearch();
              }}>Coba lagi</button
            >
          </div>
        {:else if !ioSupported}
          <div class="more">
            <button class="btn ghost" on:click={loadMoreSearch}>Muat lagi</button>
          </div>
        {/if}
      {/if}
    {/if}
  </div>
{:else}
  {#if heroItems.length && !filterMode}
    <div
      class="hero-wrap"
      role="region"
      aria-label="Hero judul populer"
      use:observeHero
      on:mouseenter={() => {
        heroPaused = true;
        stopHeroTimer();
      }}
      on:mouseleave={() => {
        heroPaused = false;
        restartHeroTimer();
      }}
      on:touchstart={onTouchStart}
      on:touchend={onTouchEnd}
    >
      {#each heroItems as h, i (h.title.id)}
        <div class="hero-slide" class:active={i === heroIdx}>
          <Hero item={h.title} firstEpisodeId={h.firstEp} />
        </div>
      {/each}
      {#if heroItems.length > 1}
        <div class="hero-dots">
          {#each heroItems as _, i}
            <button
              class="dot"
              class:active={i === heroIdx}
              on:click={() => goHero(i, true)}
              aria-label={'Hero ' + (i + 1) + ' dari ' + heroItems.length}
            >
              {#if i === heroIdx && !reduceMotion}
                <span
                  class="dot-fill"
                  style="animation-duration: {HERO_MS}ms; animation-play-state: {heroPaused || pageHidden
                    ? 'paused'
                    : 'running'}"
                ></span>
              {/if}
            </button>
          {/each}
        </div>
        <button class="hero-arrow left" on:click={() => goHero(heroIdx - 1, true)} aria-label="Hero sebelumnya"
          >‹</button
        >
        <button class="hero-arrow right" on:click={() => goHero(heroIdx + 1, true)} aria-label="Hero berikutnya"
          >›</button
        >
      {/if}
      <!-- Toggle Daftar Saya hero (27 Nov 2026): overlay di kanan atas —
           Hero.svelte di luar cakupan batch ini, jadi tombol hidup di sini
           seperti dot + panah; Nonton/Detail di dalam Hero tak disentuh. -->
      <button
        class="btn ghost hero-wl"
        class:active={heroInWatchlist}
        on:click={toggleHeroWatchlist}
        disabled={heroWlBusy}
        aria-pressed={heroInWatchlist}
        aria-label={heroInWatchlist ? 'Hapus dari Daftar Saya' : 'Simpan ke Daftar Saya'}
      >
        <i class={heroInWatchlist ? 'fa-solid fa-bookmark' : 'fa-regular fa-bookmark'}></i>
        {heroWlBusy ? 'Menyimpan…' : heroInWatchlist ? 'Tersimpan' : 'Daftar Saya'}
      </button>
      {#if heroWlError}
        <p class="hero-wl-error">{heroWlError}</p>
      {/if}
    </div>
  {/if}
  <div class="container">
    {#if error}
      <p class="error">{error}</p>
    {:else if tab === ''}
      {#if !isPremium($user)}
        <!-- Kartu promo Premium ala aplikasi (27 Nov 2026): gradien ungu
             + bintang emas; tamu + gratis saja, premium disembunyikan. -->
        <a class="promo-premium" href="/premium">
          <span class="promo-icon"><i class="fa-solid fa-star"></i></span>
          <span class="promo-text">
            <strong>SubSenpai Premium</strong>
            <span>Nonton tanpa iklan, semua film terbuka</span>
          </span>
          <span class="promo-cta">Lihat Premium <i class="fa-solid fa-chevron-right"></i></span>
        </a>
      {/if}
      {#if continueWatching.length}
        <Rail title="Lanjutkan Menonton" items={continueWatching} moreHref="/riwayat" />
      {/if}
      <!-- Urutan rel (28 Nov 2026, permintaan Rams): 10 rel, tiap rel
           punya Lihat semua. -->
      <Rail title="Film" items={rails.film.slice(0, 12)} moreHref="/cari?type=film" />
      {#if filmYear.length}
        <Rail
          title="Film Terbaru"
          items={filmYear.slice(0, 12)}
          moreHref={'/cari?type=film&year=' + THIS_YEAR}
        />
      {/if}
      {#if filmPopuler.length}
        <Rail title="Film Populer" items={filmPopuler.slice(0, 12)} moreHref="/cari?type=film" />
      {/if}
      <Rail title="Anime" items={rails.anime.slice(0, 12)} moreHref="/cari?type=anime" />
      {#if animeOngoing.length}
        <Rail
          title="Anime Ongoing"
          items={animeOngoing.slice(0, 12)}
          moreHref="/cari?type=anime&status=ongoing"
        />
      {/if}
      {#if animeYear.length}
        <Rail
          title="Anime Terbaru"
          items={animeYear.slice(0, 12)}
          moreHref={'/cari?type=anime&year=' + THIS_YEAR}
        />
      {/if}
      <Rail title="Donghua" items={rails.donghua.slice(0, 12)} moreHref="/cari?type=donghua" />
      {#if donghuaOngoing.length}
        <Rail
          title="Donghua Ongoing"
          items={donghuaOngoing.slice(0, 12)}
          moreHref="/cari?type=donghua&status=ongoing"
        />
      {/if}
      {#if donghuaYear.length}
        <Rail
          title="Donghua Terbaru"
          items={donghuaYear.slice(0, 12)}
          moreHref={'/cari?type=donghua&year=' + THIS_YEAR}
        />
      {/if}
    {:else if filterMode}
      <!-- Hasil filter genre/lanjutan: menggantikan hero + grid tab. -->
      <h2 class="results-title">Hasil</h2>
      {#if resultsLoading}
        <div class="poster-grid"><Skeleton kind="card" count={12} /></div>
      {:else if resultsError}
        <p class="error">{resultsError}</p>
        <div class="more">
          <button
            class="btn ghost"
            on:click={() => {
              resultsError = '';
              runFilter();
            }}>Coba lagi</button
          >
        </div>
      {:else}
        <div class="poster-grid">
          {#each results as item, i (item.id)}
            <div use:reveal={(i % 12) * 45}><TitleCard {item} /></div>
          {/each}
        </div>
        {#if !results.length}
          <p class="muted">Belum ada judul yang cocok. Coba genre atau filter lain.</p>
        {/if}
        {#if resultsCursor}
          <div
            class="sentinel"
            use:infiniteScroll={{
              enabled: !!resultsCursor && !resultsLoadingMore && !resultsMoreError,
              onLoad: loadMoreFilter
            }}
          ></div>
          {#if resultsLoadingMore}
            <p class="load-more-note"><span class="bar skeleton"></span>Memuat lagi…</p>
          {:else if resultsMoreError}
            <p class="error">{resultsMoreError}</p>
            <div class="more">
              <button
                class="btn ghost"
                on:click={() => {
                  resultsMoreError = '';
                  loadMoreFilter();
                }}>Coba lagi</button
              >
            </div>
          {:else if !ioSupported}
            <div class="more">
              <button class="btn ghost" on:click={loadMoreFilter}>Muat lagi</button>
            </div>
          {/if}
        {/if}
      {/if}
    {:else}
      <div class="poster-grid">
        {#each grid as item, i (item.id)}
          <div use:reveal={(i % 12) * 45}><TitleCard {item} /></div>
        {/each}
        {#if loadingMore}
          <Skeleton kind="card" count={6} />
        {/if}
      </div>
      {#if !grid.length}
        <p class="muted">Belum ada judul untuk jenis ini.</p>
      {/if}
      {#if nextCursor}
        <div
          class="sentinel"
          use:infiniteScroll={{ enabled: !!nextCursor && !loadingMore && !moreError, onLoad: loadMore }}
        ></div>
        {#if loadingMore}
          <p class="load-more-note"><span class="bar skeleton"></span>Memuat lagi…</p>
        {:else if moreError}
          <p class="error">{moreError}</p>
          <div class="more">
            <button
              class="btn ghost"
              on:click={() => {
                moreError = '';
                loadMore();
              }}>Coba lagi</button
            >
          </div>
        {:else if !ioSupported}
          <div class="more">
            <button class="btn ghost" on:click={loadMore}>Muat lagi</button>
          </div>
        {/if}
      {/if}
    {/if}
  </div>
{/if}

<style>
  /* Chip jenis menempel di bawah navbar (28 Nov 2026): tanpa ini,
     pengguna harus gulir ke atas dulu untuk pindah tab — dan guliran
     itu sendiri yang menimpa posisi scroll tersimpan jadi 0, sehingga
     kembali ke tab lama selalu mendarat di atas. Baris genre di bawah
     tidak ikut menempel: ganti genre memuat daftar baru, jadi posisi
     lama memang tidak berlaku. */
  .tabs-top {
    position: sticky; top: 60px; z-index: 40;
    padding-top: 12px; padding-bottom: 12px;
    background: var(--bg);
  }
  @media (min-width: 1280px) { .tabs-top { padding-top: 16px; } }
  .tabs { display: flex; gap: 8px; margin: 0; overflow-x: auto; justify-content: safe center; }
  .more { text-align: center; margin: 24px 0; }

  /* Hero berganti otomatis: slide ditumpuk, crossfade via opacity. */
  .hero-wrap { position: relative; }
  .hero-slide {
    opacity: 0;
    transition: opacity 0.6s ease;
    pointer-events: none;
  }
  .hero-slide.active { opacity: 1; pointer-events: auto; }
  .hero-slide:not(.active) { position: absolute; inset: 0; }
  .hero-dots {
    position: absolute; right: 20px; bottom: 18px; z-index: 5;
    display: flex; gap: 8px;
  }
  .dot {
    width: 24px; height: 4px; border: 0; padding: 0; border-radius: 999px;
    background: rgba(255, 255, 255, 0.35); overflow: hidden;
    position: relative; cursor: pointer;
  }
  .dot-fill {
    position: absolute; left: 0; top: 0; bottom: 0; width: 0;
    background: #fff; animation: hero-fill linear forwards;
  }
  @keyframes hero-fill { from { width: 0; } to { width: 100%; } }
  .hero-arrow {
    position: absolute; top: 50%; transform: translateY(-50%); z-index: 5;
    width: 44px; height: 44px; border-radius: 50%; border: 0;
    background: rgba(0, 0, 0, 0.45); color: #fff; font-size: 26px; line-height: 1;
    cursor: pointer; opacity: 0; transition: opacity 0.2s;
  }
  .hero-wrap:hover .hero-arrow { opacity: 1; }
  .hero-arrow.left { left: 16px; }
  .hero-arrow.right { right: 16px; }
  @media (hover: none) { .hero-arrow { display: none; } }
  /* Toggle Daftar Saya di hero: overlay kanan atas (seperti dot +
     panah); aktif = latar merah + bookmark penuh. */
  .hero-wl { position: absolute; top: 14px; right: 16px; z-index: 6; }
  .hero-wl.active { background: var(--red); }
  .hero-wl.active:hover { background: var(--red-dark); }
  .hero-wl-error {
    position: absolute; left: 16px; bottom: 12px; z-index: 6; margin: 0;
    font-size: 13px; color: #ffb4b4; background: rgba(0, 0, 0, 0.65);
    padding: 4px 8px; border-radius: 4px;
  }
  /* Kartu promo Premium ala aplikasi: gradien ungu + bintang emas. */
  .promo-premium {
    display: flex; align-items: center; gap: 12px;
    margin: 4px 0 20px; padding: 14px 16px; border-radius: 12px;
    background: linear-gradient(90deg, #3b2a5d, #7c3aed); color: #fff;
    text-decoration: none;
  }
  .promo-premium:hover { filter: brightness(1.1); text-decoration: none; }
  .promo-icon { color: var(--gold); font-size: 24px; flex: 0 0 auto; }
  .promo-text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .promo-text strong { font-size: 16px; }
  .promo-text span { font-size: 13px; color: #e9e4f5; }
  .promo-cta {
    flex: 0 0 auto; background: #fff; color: #000; font-weight: 700;
    font-size: 13px; padding: 8px 14px; border-radius: 999px; white-space: nowrap;
  }
  .promo-cta i { font-size: 11px; margin-left: 2px; }
  /* Baris filter genre (28 Nov 2026): geser horizontal seperti
     aplikasi; chip aktif merah, bukan putih seperti chip jenis. */
  .genre-row-wrap { padding-bottom: 4px; }
  .genre-row {
    display: flex; gap: 8px; overflow-x: auto; padding-bottom: 8px;
    scrollbar-width: none;
  }
  .genre-row::-webkit-scrollbar { display: none; }
  .genre-chip {
    display: inline-flex; align-items: center; gap: 6px;
    border: 1px solid var(--border); border-radius: 999px;
    padding: 6px 14px; font-size: 13px; color: var(--muted);
    background: transparent; white-space: nowrap; cursor: pointer;
  }
  .genre-chip:hover { color: #fff; border-color: #666; }
  .genre-chip.active { background: var(--red); color: #fff; border-color: var(--red); font-weight: 600; }
  .genre-chip i { font-size: 11px; }
  .filter-panel {
    background: var(--panel); border: 1px solid var(--border);
    border-radius: 12px; padding: 14px; margin: 4px 0 12px;
  }
  .filter-genres { margin-bottom: 14px; }
  .filter-label {
    display: block; font-size: 13px; color: var(--muted); margin-bottom: 8px;
  }
  .genre-wrap { display: flex; flex-wrap: wrap; gap: 8px; }
  .filter-fields { display: flex; flex-wrap: wrap; gap: 12px; }
  .filter-fields label {
    display: flex; flex-direction: column; gap: 6px; flex: 1 1 140px;
    font-size: 13px; color: var(--muted);
  }
  .filter-fields select {
    background: var(--bg); color: #fff; border: 1px solid var(--border);
    border-radius: 8px; padding: 9px 10px; font-size: 14px;
  }
  .filter-actions { display: flex; gap: 8px; margin-top: 12px; }
  .results-title { font-size: 18px; font-weight: 700; margin: 8px 0 14px; }
  @media (prefers-reduced-motion: reduce) {
    .hero-slide { transition: none; }
  }
</style>
