# Redesign Web SubSenpai ala Netflix — Rencana (LANGKAH 0)

Perintah Rams 27 Nov 2026 17:35 WIB. Aturan: tanpa ubah logika backend,
routing, atau data. Hanya tampilan + interaksi frontend.

## Audit (terverifikasi dari kode)

- Stack: **SvelteKit 2 + Svelte 5, adapter-static SPA** (`ssr = false`,
  fallback `200.html`), total ±2.845 baris di `src/`.
- Komponen (8): `Navbar, BottomNav, Hero, Rail, TitleCard, EpisodeList,
  Player, PostCard`. Rute (12): `/, /cari, /jadwal, /judul/[id],
  /lainnya, /masuk, /nonton/[episodeId], /post, /premium, /riwayat,
  /unduh`, + `lib/api.js, auth.js, history.js`.
- `app.css` sudah memakai palet yang diminta (`--bg #141414, --panel
  #1d1d1d, --red #e50914, --muted #b3b3b3, --gold #f5c518`) — yang
  kurang: radius 4px (saat ini 6–10px), font Inter (saat ini system
  stack), padding ~4% (saat ini container 1280px), skeleton (saat ini
  teks "Memuat…"), scrollbar tipis gelap, focus-visible,
  prefers-reduced-motion.
- **Data yang SUDAH tersedia (tanpa backend baru):**
  - Watchlist: `GET/POST/DELETE /api/v1/watchlist[/…]` — ada di grup
    auth-Required backend (`api.go`). Web belum memakainya sama sekali.
    Jadi halaman Daftar Saya + tombol "+ Daftar Saya" hanya butuh
    wrapper di `api.js` + halaman baru. Wajib login (sama seperti aplikasi).
  - `getLiveBrowse(type, genre, page)` sudah ada di `api.js` —
    dipakai aplikasi untuk rel "trending". Jadi baris "Sedang Tren"
    dan "Top 10" bisa memakai ini (urutan dari sumber live).
    Backend TIDAK punya endpoint peringkat/top khusus — Top 10 di sini
    = 10 pertama hasil live-browse, BUKAN peringkat tontonan nyata.
  - Episode detail membawa `duration_sec` (`catalog.go:458`) — durasi
    dan progress bar nyata per episode bisa ditampilkan (riwayat
    menyimpan `position_sec` per episode).
  - `history.js` sudah ada (lokal + server) + `reportHistory` — yang
    belum ada: Player tidak pernah SEEK ke posisi tersimpan (resume
    hanya tercatat, tidak dipakai).
- **Batasan nyata yang harus diakui:**
  - Kontrol custom (progress, ±10 dtk, volume, speed, PiP) HANYA bisa
    untuk sumber `<video>` langsung. Mayoritas sumber adalah iframe
    pihak ketiga (Animeku, vidstuck, Doodstream, …) yang tidak bisa
    dikontrol cross-origin — untuk iframe: rapikan wrapper (16:9,
    radius, shadow) + Prev/Next + daftar episode di luar player.
    Overlay "Episode Berikutnya" + hitung mundur otomatis juga hanya
    untuk video langsung (event `ended`); iframe tidak memberi tahu
    kapan episode selesai.
  - Episode TIDAK punya thumbnail/sinopsis sendiri (API hanya
    `id, episode_no, title, duration_sec, locked`) — thumbnail baris
    episode memakai poster judul (16:9 crop) / gradient fallback.
  - Hero saat ini 1 judul statis dari `rails.anime[0]`; rotasi 8 detik
    memakai 5 kandidat dari live-browse/trending (aplikasi memakai
    pola yang sama: `heroList = trending.take(5)`).
  - Sinopsis: render kita menampilkan `title_alt` sekali + sinopsis.
    Bila sinopsis API diakhiri pengulangan judul alternatif, itu data
    API — perbaikan frontend: potong duplikat itu saat render.
- **Efek samping pindah domain (17:21 WIB, sedang jalan):**
  tautan Ketentuan/Privasi di `/masuk` menunjuk ke
  `www.subsenpai.web.id/terms.html` & `/privacy.html` — sesudah domain
  pindah ke web app, kedua URL itu 404 (file-nya ada di site landing
  lama). Maka redesign ini SEKALIAN menambahkan rute `/terms` +
  `/privacy` (isi dari `landing/terms.html` + `landing/privacy.html`).

## Keputusan yang perlu Rams jawab (sebelum/saat Tahap 1)

**DIPOTONG DULU (27 Nov 2026 17:38 WIB):** Rams memerintahkan slice
"skeleton + scroll berurutan" dibangun SEKARANG, mendahului Tahap 1–6:
`app.css` (skeleton shimmer + `.reveal` + sentinel + reduced-motion),
`lib/reveal.js` (aksi `reveal` + `infiniteScroll`), komponen
`Skeleton.svelte`, skeleton di semua loading state (Beranda hero/rel/grid,
Cari, Jadwal, Riwayat, Detail, Nonton, Post, Premium, Unduh), reveal
berurutan (jeda 45ms per kartu, 40ms per baris) di grid/rel/daftar
episode/riwayat/post, dan infinite scroll (sentinel + skeleton saat
memuat + retry bila gagal) menggantikan tombol "Muat lagi" di Beranda
(tab), Cari, dan Post. Commit menyusul di bawah.

**DIPOTONG DULU (27 Nov 2026 17:52 WIB):** Rams memerintahkan chip
Semua/Film/Anime/Donghua pindah ke ATAS (tepat di bawah navbar, hero di
bawahnya) agar mirip Beranda aplikasi Android — commit `85cf52d`.

**DIPOTONG DULU (27 Nov 2026 17:53 WIB):** Rams memerintahkan SEMUA
ikon memakai Font Awesome (bukan emoji) agar terlihat premium.
Implementasi: `app.html` memuat FA 6.5.2 dari cdnjs (sama seperti
dashboard admin); seluruh emoji/simbol di `web/src` diganti
`<i class="fa-solid/fa-regular/fa-brands ...">` (bintang, gembok,
play, info, mahkota emas, hati, komentar, repost, bookmark, share,
sampah, chevron, panah, cek, tautan, kalender, cari, TV, unduh,
paperclip, silang, brand Discord/Telegram) termasuk 5 SVG inline
BottomNav + SVG cari Navbar. Verifikasi: scan emoji di `web/src` = 0.

**DIPOTONG DULU (27 Nov 2026 17:58 WIB):** Rams memerintahkan halaman
Premium web disamakan dengan aplikasi Android — 3 kategori: Langganan,
Beli Voucher, Tukar Voucher. Implementasi (commit menyusul): tab bar 3
kartu ala aplikasi; Langganan + Beli Voucher memakai alur 3 langkah
aplikasi (1 pilih paket — kartu paket + ≈ harga/bulan, 2 pilih metode
bayar dikelompokkan, 3 kartu pembayaran ala Netflix: total besar, QR
+ Unduh QRIS, nomor VA + Salin, Cek Pembayaran otomatis sekali);
Beli Voucher tambah kartu "Mau dibagikan ke berapa orang?" (input
jumlah 1–10, total = harga × jumlah) + daftar "Voucher Saya"
(GET /api/v1/vouchers/mine) + kode voucher hasil checkout dengan
tombol Salin; Tukar Voucher: input kode + POST /api/v1/vouchers/redeem
→ "Voucher ditukar! Premium sampai …". Perangkap yang tertangkap saat
cek live: API payment-methods mengembalikan field `group` (huruf kecil,
mis. "qris"), BUKAN `grup` — pengelompokan metode harus pakai
`m.group` lowercase, kalau tidak langkah 2 kosong.

1. **Bottom nav mobile — KONFLIK dengan perintah lama.** Spek ini minta
   (Beranda, Cari, Jadwal, Daftar Saya, Akun). Tapi 27 Nov 16:53 WIB
   Rams memerintahkan tab web disamakan persis dengan aplikasi:
   (Beranda, Post, Jadwal, Riwayat, Lainnya) — dan itu sudah live.
   Rekomendasi: PERTAHANKAN tab aplikasi; Daftar Saya masuk ke menu
   Lainnya + dropdown avatar. Bila Rams mau ganti ke spek, kabari.
2. **Top 10** = 10 pertama live-browse (bukan peringkat tontonan).
   Bila mau peringkat nyata, butuh endpoint backend baru — di luar
   scope "tanpa ubah backend".
3. Font Inter: muat dari Google Fonts (CDN) atau self-host via
   `@fontsource/inter` (npm). Rekomendasi: fontsource (tidak tergantung
   Google saat offline).

## Tahapan (satu push + verifikasi live per tahap)

### Tahap 1 — Design system, Navbar, Footer, fondasi
- `app.css`: radius 4px, font Inter, padding konten ~4% (container
  1440px max + padding 4vw), scrollbar tipis gelap, `:focus-visible`,
  `@media (prefers-reduced-motion)`, skeleton shimmer, `.toast`,
  breakpoint 480/768/1024/1440.
- `Navbar.svelte`: fixed; transparan + gradient di puncak → solid
  #141414 saat scroll; search ikon → melebar jadi input (animasi +
  tombol clear); setelah login avatar + dropdown (Daftar Saya, Riwayat,
  Keluar). Menu tetap Beranda/Jadwal/Genre/Premium/Unduh + Masuk.
- `+layout.svelte`: footer 4 kolom kecil abu-abu.
- Baru: `components/Skeleton.svelte, Toast.svelte, Modal.svelte,
  Badge.svelte`; `routes/+error.svelte` (404); `routes/terms/+page.svelte`,
  `routes/privacy/+page.svelte`; `<svelte:head>` title per halaman.
- `BottomNav.svelte`: TIDAK diubah dulu (menunggu jawaban keputusan 1).

### Tahap 2 — Beranda
- `Hero.svelte`: 80vh, backdrop penuh + gradient kiri & bawah, badge
  (tahun, status, rating, SUB INDO, HD), sinopsis maks 3 baris,
  tombol "Putar" putih + "Info Lainnya" abu transparan; auto-rotate
  8 dtk (5 kandidat) + indikator titik, berhenti saat hover.
- `+page.svelte`: filter chip di bawah hero (tetap memfilter);
  baris baru: Lanjutkan Menonton (progress bar merah tipis dari
  `position_sec/duration_sec`), Baru Rilis / Episode Baru, Top 10
  Hari Ini (angka outline besar), Sedang Tren, per-genre populer
  (2–3 genre teratas dari `getGenres`, dibatasi agar tidak banjir
  request).
- `Rail.svelte` (+ `Carousel`): scroll-snap, panah kiri/kanan saat
  hover baris, swipe mobile, padding + fade tepi.
- `TitleCard.svelte`: hover desktop membesar 1.2x + panel info
  (Play, + Daftar Saya, panah bawah → modal); mobile tap → detail.
- Baru: `TitleModal.svelte` (overlay: backdrop, sinopsis, genre,
  Putar, judul serupa).
- Skeleton loader menggantikan "Memuat katalog…".

### Tahap 3 — Detail + Daftar Saya
- `judul/[id]/+page.svelte`: hero backdrop lebar (poster blur/gradient
  bila backdrop tidak ada), judul besar, judul alternatif kecil
  abu-abu 1 baris, metadata pemisah titik, badge, genre sebagai teks
  berpemisah (bukan pill), sinopsis (potong duplikat judul alternatif
  di akhir), tombol "Putar Episode 1 / Lanjutkan Ep X" + "+ Daftar
  Saya" + "Bagikan".
- Tab: Episode | Judul Serupa | Detail.
- `EpisodeList.svelte`: baris vertikal Netflix — nomor besar,
  thumbnail 16:9 (poster/gradient), judul, durasi, ikon play saat
  hover, progress bar, penanda episode terakhir ditonton; pemilih
  rentang (1–24, 25–48, …) bila >24 episode.
- Judul Serupa: carousel Tahap 2 (data: `getTitles({genre})` dari
  genre pertama judul).
- Baru: `routes/daftar-saya/+page.svelte` + wrapper watchlist di
  `api.js` (`getWatchlist, checkWatchlist, addWatchlist,
  removeWatchlist`).

### Tahap 4 — Nonton
- `nonton/[episodeId]/+page.svelte`: latar hitam penuh, tombol kembali
  (panah) ke detail, judul + episode di atas, daftar episode sidebar
  (desktop) / bawah player (mobile).
- `Player.svelte`: pilihan server pindah ke menu gear/dropdown
  "Server" (bukan chip di atas player); server terakhir diingat di
  localStorage (`ssweb.lastServer` per judul).
- Kontrol custom untuk `<video>` langsung: bar progres merah,
  mundur/maju 10 dtk, volume slider, kecepatan, PiP, fullscreen,
  hilang otomatis 3 dtk, shortcut Spasi/K, ←/→, ↑/↓, F, M, N.
  Untuk iframe: wrapper 16:9 + radius + shadow, Prev/Next + daftar
  episode di luar player (kontrol tetap milik pemutar sumber).
- Overlay "Episode Berikutnya" + hitung mundur — hanya video
  langsung; iframe cukup tombol Next.
- Resume: video langsung SEEK ke `position_sec` tersimpan saat load.
  `history.js` + `onProgress` tetap (throttle yang sudah ada).

### Tahap 5 — Halaman lain
- `jadwal`: tab hari (Senin–Minggu) + grid kartu per hari.
- `cari`: search live dengan debounce (±350ms), infinite scroll
  (IntersectionObserver), skeleton, grid 6/3/2 kolom.
- `premium, masuk, riwayat, unduh, lainnya`: restyle ke design
  system Tahap 1 (logika tidak berubah); `/masuk` di atas backdrop
  poster; `/riwayat` memakai `duration_sec` untuk progress bar
  (ganti heuristik /1500); `/premium` kartu paket ala Netflix.
- `post`: hanya mengikuti gaya global (tidak diubah strukturnya).

### Tahap 6 — QA + checklist sebelum/sesudah
- `svelte-check` 0/0 + build hijau tiap tahap; verifikasi live di
  Netlify per tahap (deploy Published → cek rute yang berubah).
- Checklist perbandingan sebelum vs sesudah per halaman (Beranda,
  Detail, Nonton, Jadwal, Cari, Premium, Unduh, Masuk, Daftar Saya,
  Riwayat, 404, footer) — ditulis di akhir.
- Yang HANYA bisa dicek Rams di HP: render mobile ~390px (browser
  agen terkunci 1919px), feed login + riwayat terisi, playback film
  (Premium), tap Daftar Saya sebagai user login.
