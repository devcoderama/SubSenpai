# SubSenpai 🎌

**Anime, donghua, film, dan komik dalam satu aplikasi — konten video dengan subtitle otomatis multi-bahasa.**

SubSenpai adalah aplikasi streaming & baca: anime dan donghua dari sumber asalnya (raw, tanpa subtitle) dengan subtitle yang dibuat otomatis lewat pipeline Whisper + terjemahan mesin, film, serta manga/manhua/manhwa untuk dibaca. Tamu bisa mencicipi 1 episode/chapter; sisanya dibuka lewat login — gratis dengan iklan, atau premium tanpa iklan.

> 🚧 **MASIH DALAM TAHAP PENGEMBANGAN** — repo ini baru berisi rancangan dan struktur awal. Belum ada versi yang bisa dipakai.

## ✨ Fitur yang Direncanakan

- 📺 **Katalog multi-konten** — anime, donghua, film, manga, manhua, manhwa; telusur & cari berdasarkan judul, tipe, genre, musim, dan tahun
- ▶️ **Streaming langsung** — video diputar langsung dari sumbernya (HLS) ke player
- 📖 **Mode baca** — chapter komik per halaman gambar, nyaman di HP
- 🗣️ **Subtitle otomatis multi-bahasa** — transkripsi dibuat pipeline Whisper di server menjadi subtitle bahasa asal, lalu **aplikasi menerjemahkannya di perangkat user** ke bahasa pilihan user memakai Google Translate gratis (endpoint `gtx`, pola D TRADE — tanpa API key, dipanggil dari IP perangkat user sendiri sehingga jarang kena limit); hasil terjemahannya tersimpan di cache lokal perangkat
- ⚡ **Jalur cepat caption** — kalau sumbernya punya closed caption bahasa asal, caption langsung diterjemahkan (timing presisi, tanpa transkripsi)
- 🌐 **Aplikasi multi-bahasa + tombol Terjemahkan** — antarmuka aplikasi tersedia dalam banyak bahasa (diatur di Pengaturan; bawaan mengikuti bahasa perangkat), dan sinopsis, komentar, serta pesan chat bisa diterjemahkan sekali ketuk memakai Google Translate gratis (endpoint publik `gtx` seperti di D TRADE) **langsung dari perangkat user** — hasilnya di-cache lokal agar tidak menerjemahkan teks yang sama dua kali; server tidak memanggil layanan terjemahan sama sekali
- 👤 **Akun pengguna** — masuk **wajib dengan Google** (anti-bot; tanpa form email/password), watchlist, dan riwayat tontonan/bacaan (auth ditangani backend Go sendiri — tanpa Supabase)
- 🎟️ **Freemium** — tamu: 1 episode/chapter; login gratis: lanjut dengan iklan; premium: tanpa iklan, semua terbuka
- 🎁 **Voucher premium** — admin membagikan kode promo; user juga bisa membeli voucher untuk dihadiahkan ke user lain. Kode sekali pakai, ditukar menjadi hari premium. Pembelian voucher bisa langsung memilih teman sebagai penerima — terkirim otomatis lewat notifikasi, tanpa menyalin kode
- 💬 **Chat Global** — obrolan realtime per kanal topik (Umum, Anime, Donghua, Manga, Rekomendasi) untuk user login. User **premium** bisa mengustomisasi warna nickname, memasang emoji di nama, dan memakai stiker eksklusif. Semua user bisa kirim gambar & stiker pribadi — dihosting via ImgBB, yang disimpan di database hanya URL-nya. Ada balasan pesan (reply) ala media sosial, berbagi hanya lewat kartu berbagi resmi aplikasi, dan tautan luar/promosi dilarang di chat & komentar
- 👥 **Teman & Grup** — tambah teman lewat kode teman, buat grup publik/privat dengan kanal chatnya sendiri, undang teman, dan kelola anggota dengan peran owner/admin/member. Semua anggota grup bisa membagikan **tautan undangan resmi grup** ke media sosial agar anggota baru mudah bergabung
- 💌 **DM, kehadiran & nonton bareng** — chat pribadi 1:1 antar teman (aturan yang sama seperti Chat Global), status online & "sedang menonton apa" di daftar teman (bisa dimatikan di pengaturan privasi), dan ruang nonton bareng tersinkron untuk grup/teman
- 🔔 **Notifikasi pintar** — user mengaktifkan lonceng di judul favoritnya (anime/donghua/film/manga/manhua/manhwa) agar tidak ketinggalan episode/chapter baru (push FCM + kotak masuk di aplikasi); admin juga bisa mengirim pengumuman & promo manual, mis. event voucher
- ⭐ **Komentar, rating & rekomendasi** — komentar per episode/chapter, rating 1–10 dari user, dan blok rekomendasi "karena kamu menonton ini"
- 🗳️ **Permintaan judul & koleksi komunitas** — minta judul yang belum ada di katalog + dukung permintaan user lain dengan suara; susun koleksi kurasi bertema yang bisa diikuti user lain
- 🏆 **Pencapaian, streak & Rekap Senpai** — lencana tonggak aktivitas, rentetan hari aktif, dan halaman statistik pribadi (jam menonton, chapter dibaca, genre teratas) yang bisa dibagikan sebagai kartu berbagi resmi
- ⏭️ **Maraton nyaman** — tombol Lewati Opening/Outro dari penanda episode, putar otomatis episode berikutnya, prefetch halaman/episode berikutnya, dan katalog yang tersinkron lokal di aplikasi (beranda tampil seketika)
- 🎁 **Trial premium 3 hari** — user baru bisa mencoba premium gratis sekali per akun; akun premium aktif di maks 2 perangkat, lengkap dengan halaman keamanan akun (lihat & cabut perangkat lain)
- ⬇️ **Download offline (premium)** — unduh episode/chapter terenkripsi ke perangkat untuk dinikmati tanpa internet; referral teman juga memberi hari premium untuk kedua pihak
- 🔐 **Response API terenkripsi** — semua response JSON backend dibungkus envelope `xdata`/`xtime` (AES-256-GCM dengan **kunci sesi sekali pakai hasil tukar kunci ECDH** — tidak ada kunci yang tertanam di aplikasi) dan baru dibuka di aplikasi — anti-scraping, anti-tamper, anti-replay (detail di STRUKTUR.md Bagian 11)
- 🌙 **Mode gelap** — tema bawaan untuk nonton malam

## 🗣️ Cara Kerja Subtitle Otomatis

```
Video raw ──► ekstrak audio ──► Whisper (transkripsi + timestamp)
                                       │
                                       ▼
                 teks bahasa asal ──► terjemahan mesin ──► file .srt/.ass
                                       │
                                       ▼
                           tampil sebagai subtitle di player
```

1. **Transkripsi** — audio ditranskripsi oleh Whisper (large-v3; WhisperX untuk timing per kata) menjadi teks bertimestamp.
2. **Terjemahan (di aplikasi)** — file subtitle bahasa asal tersimpan di server; saat user memutar, **aplikasi** menerjemahkan baris-barisnya ke bahasa user memakai Google Translate gratis (endpoint `gtx`) langsung dari perangkat user (IP tiap user berbeda — jarang kena limit, seperti di D TRADE), lalu menyimpan hasil terjemahannya di cache lokal perangkat.
3. **Penyajian** — player memuat subtitle bahasa asal dari object storage, menerjemahkannya di perangkat bila bahasa user berbeda, dan menampilkannya sebagai track subtitle; pemutaran berikutnya memakai cache lokal perangkat.

Catatan jujur: kualitas subtitle mesin adalah "cukup untuk mengikuti cerita", bukan setara fansub manusia — nama karakter, istilah khas, dan lagu pembuka/penutup adalah titik lemahnya. Subtitle otomatis hanya untuk konten video; menerjemahkan komik (OCR + menimpa teks pada gambar) adalah persoalan terpisah yang jauh lebih berat.

## 🎟️ Model Akses (Freemium)

| Pengguna | Akses |
| --- | --- |
| Tamu (belum login) | Maksimal **1 episode / 1 chapter** |
| Login gratis | Semua katalog, **dengan iklan** |
| Premium (langganan) | Semua katalog, **tanpa iklan** |

Gerbang akses ditegakkan di server (API), bukan sekadar disembunyikan di tampilan. Pembayaran premium direncanakan lewat payment gateway lokal (Midtrans/Xendit — bisa QRIS). Alternatif bootstrap: GoPay Merchant lewat API pihak ketiga (QRIS dinamis + polling mutasi, tanpa webhook — alur lengkap & risikonya di STRUKTUR.md Bagian 7). Premium juga bisa diperoleh dari **voucher**: kode promo dari admin, atau voucher yang dibeli user untuk dihadiahkan ke user lain. Jaringan iklan untuk konten seperti ini umumnya bukan AdSense; pilihannya diteliti di fase monetisasi.

## 🏗️ Arsitektur

Pemisahan berdasarkan **jenis data**, bukan per genre:

| Bagian | Teknologi | Fungsi |
| --- | --- | --- |
| Klien | Aplikasi Android (Kotlin) di perangkat user | Katalog, player, reader, Chat Global, panel admin — semua tampilan & interaksi; tidak ada versi web |
| Backend API | **Go** di VPS (binary + systemd, di belakang Caddy) | Semua logika: katalog, gerbang akses, subtitle, chat, premium, voucher, admin + enkripsi response `xdata`/`xtime` |
| Database + Auth | PostgreSQL **lokal di VPS** + auth oleh backend Go | Katalog multi-konten, episode/chapter, subtitle, user, langganan — penyimpanan database mengikuti disk VPS (jauh lebih besar dari batas paket gratis Supabase); login Google diverifikasi backend dan sesi memakai JWT buatan backend, tanpa Supabase |
| File storage | Cloudflare R2 | Poster, halaman komik & file subtitle (.srt/.ass) |
| Video | Stream langsung dari sumber (HLS) | Tidak lewat server sendiri — hemat bandwidth |
| Worker subtitle | Job latar Go (antrean transkripsi + SRT bahasa asal) + API transkripsi cloud | Transkripsi dititipkan ke API Whisper (Groq/OpenAI); worker Python mandiri hanya bila perlu full offline. Worker TIDAK menerjemahkan — itu urusan aplikasi di perangkat user |
| Penerjemahan | Google Translate **gratis** — endpoint publik `client=gtx`, dipanggil **aplikasi dari perangkat user** (bukan server) | Subtitle diterjemahkan di perangkat + fitur Terjemahkan untuk sinopsis/komentar/chat; hasil terjemahan di-cache lokal di perangkat user |
| Pembayaran | Midtrans / Xendit + GoPay Merchant API pihak ketiga | Langganan premium & voucher (QRIS, e-wallet, transfer). Jalur GoPay: QRIS dinamis + konfirmasi polling mutasi (tanpa webhook) — [Api-Gopar-Merch](https://github.com/ZeroByte000/Api-Gopar-Merch) |
| Panel admin | Route `/admin` (role admin) | Kelola katalog, subtitle, user, langganan + monitoring — [STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md) |

Detail folder, skema database, model akses, dan alur data: lihat **[STRUKTUR.md](STRUKTUR.md)**. Diagram ERD dan draf SQL database: lihat **[DATABASE.md](DATABASE.md)**. Panel pengelolaan & pemantauan: lihat **[STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md)**. Fitur gelombang ketiga & keempat (kontribusi terjemahan, kalender rilis, Cast, impor daftar, Tanya Senpai AI, polling & kuis, multi-profil, dan lainnya): lihat **[FITUR.md](FITUR.md)**. Rancangan keamanan (termasuk Play Integrity & gerbang versi minimum): lihat **[KEAMANAN.md](KEAMANAN.md)**. Optimasi gelombang ketiga & keempat (failover sumber video, retensi data, cadangan WAL, halaman komik terkompresi, mode pemeliharaan, dan lainnya): lihat **[OPTIMASI.md](OPTIMASI.md)**. Catatan keputusan distribusi (aplikasi hanya diunduh dari website resmi & GitHub; Play Store ditunda) beserta pertimbangannya: lihat **[NOTE.md](NOTE.md)**.

## 🗺️ Roadmap

- [ ] Fase 0 — Rancangan & struktur repo *(sedang berjalan)*
- [ ] Fase 1 — Katalog multi-konten + halaman detail + player HLS
- [ ] Fase 2 — Worker subtitle otomatis (transkripsi → terjemah → SRT)
- [ ] Fase 3 — Akun, watchlist, riwayat
- [ ] Fase 4 — Gerbang freemium (tamu 1 episode) + iklan + premium + Chat Global, teman & grup + DM antar teman & nonton bareng
- [ ] Fase 5 — Mode baca manga/manhua/manhwa (termasuk mode gulir vertikal untuk manhwa/manhua)
- [ ] Fase 6 — Pencarian lanjutan, rekomendasi, permintaan judul, koleksi komunitas, pencapaian & Rekap Senpai, dan pemolesan UI

## ☕ Dukung SubSenpai (Donasi)

SubSenpai dikembangkan secara mandiri dan masih tahap awal — biaya terbesar adalah sewa VPS untuk server & databasenya. Kalau kamu merasa proyek ini bermanfaat, kamu bisa ikut mendukung lewat QRIS berikut (bisa dari semua aplikasi pembayaran & e-wallet):

<img src="donasi/qris-donasi.jpg" alt="QRIS Donasi SubSenpai" width="320">

Terima kasih banyak atas dukungannya! 🙏

## ⚠️ Catatan

Proyek hobi/eksperimen untuk belajar. Konten berasal dari sumber pihak ketiga; sebagian besar sumber resmi memakai kunci wilayah dan DRM, sehingga ketersediaan sumber adalah tantangan tersendiri. Subtitle dihasilkan mesin dan dapat mengandung kesalahan.
