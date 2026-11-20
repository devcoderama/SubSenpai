# Rekomendasi Fitur & Optimasi — Putaran 3

> Status: **USULAN** (20 Nov 2026) — belum keputusan. Semua item di sini BARU: tidak mengulang putaran 1 (A/B) dan putaran 2 (F1–F12, O1–O7) yang sudah resmi masuk dokumen rancangan, dan tidak mengulang fitur yang sudah dirancang dari permintaan langsung. Penomoran melanjutkan putaran 2.
>
> Cara menyetujui: sebutkan kode itemnya (mis. "F14, F15, O9 masuk") — item yang disetujui akan dipindahkan ke dokumen rancangan yang sesuai (STRUKTUR.md / DATABASE.md / STRUKTUR_ADMIN.md / README.md), lalu file ini dihapus, seperti putaran-putaran sebelumnya.

Prioritas: **P1** = berdampak besar/usaha kecil-menengah, sebaiknya awal · **P2** = berharga, setelah fondasi jalan · **P3** = pelengkap, kerjakan bila sempat.

## Fitur

### F13 — Kontribusi terjemahan sukarela (opt-in) · P2
Terjemahan subtitle dibuat di perangkat user dan secara bawaan TIDAK terbagi antar-user (keputusan desain demi bebas limit server). Item ini menjembatani trade-off itu secara sukarela: user boleh memilih "Bagikan terjemahan ini" setelah menonton — aplikasi mengunggah file subtitle hasil terjemahan perangkatnya ke server. Server memeriksa format secara otomatis (jumlah baris & urutan timestamp harus sama persis dengan subtitle asal) lalu menyimpannya sebagai varian subtitle komunitas yang bisa dipilih user lain di pemilih bahasa ("Subtitle komunitas — dari @user"). Bawaan **mati**, hanya atas tindakan eksplisit user, dan admin bisa menarik varian yang dilaporkan buruk. Dampak DB: `subtitles.source` bertambah nilai `community` + kolom penunjuk kontributor.

### F14 — Jadwal rilis & kalender tayang · P1
Judul yang sedang tayang punya jadwal tetap. Tambahkan hari & jam tayang per judul (mengikuti zona waktu sumber, ditampilkan terkonversi ke zona user), halaman **Kalender** mingguan berisi judul yang rilis hari ini/minggu ini, dan hitung mundur "episode berikutnya ±2 hari 4 jam" di halaman detail. Lonceng favorit yang sudah ada otomatis lebih berguna karena user tahu kapan menunggu. Dampak DB: kolom jadwal tayang di `titles`.

### F15 — Cast ke TV (Chromecast) · P1
Streaming app hidup di TV. Player Media3 mendukung ekstensi Cast resmi: satu ketuk mengirim video + subtitle aktif ke Chromecast/Android TV, HP menjadi remote (play/pause, lompat intro, ganti episode). Tanpa perubahan server sama sekali — murni sisi aplikasi, usaha kecil untuk dampak besar.

### F16 — Impor daftar dari MyAnimeList / AniList · P2
Pengguna anime umumnya sudah punya daftar bertahun-tahun di MAL/AniList. Sediakan impor: user mengunggah file ekspor (XML/JSON) dari akunnya, backend mencocokkan judul (judul utama + judul alternatif + judul Jepang), lalu mengisi watchlist beserta status (menonton/selesai/rencana) dan progres episode terakhir. Menurunkan biaya pindah ke SubSenpai secara drastis — fitur pertumbuhan yang murah.

### F17 — Profil publik yang bisa dibagikan · P2
Profil user (nama, level, pencapaian, koleksi publik, Rekap Senpai) bisa dibagikan keluar aplikasi sebagai kartu bagikan resmi berisi tautan aplikasi — pola yang sama dengan tautan undangan grup. User memamerkan koleksinya di media sosial, aplikasi dapat anggota baru. Yang tampil hanya yang diatur publik oleh user (koleksi privat & daftar privat tidak ikut).

### F18 — Lapor cepat dari player & reader · P1
Sumber konten hasil scraping pasti sesekali rusak. Tambahkan tombol lapor di dalam player/reader dengan alasan terstruktur siap ketuk: *video mati, episode tertukar, subtitle meleset, chapter hilang/gambar rusak*. Laporan langsung terikat ke episode/chapter yang sedang dibuka (memakai tabel `reports` yang sudah ada, ditambah kolom alasan), dan antrean admin mengurutkan dari jumlah laporan — satu episode yang dilaporkan 50 orang naik ke atas sendiri. Kualitas katalog terjaga oleh penggunanya.

### F19 — Mode hemat data · P2
Satu saklar di pengaturan: kualitas video bawaan dibatasi, poster dimuat ukuran kecil, gambar chat hanya versi thumbnail sampai diketuk, pratinjau tidak memuat apa pun di latar. Penting untuk user Indonesia dengan kuota terbatas — dan tidak butuh perubahan server, hanya kebijakan muat di aplikasi.

### F20 — Jam tenang notifikasi · P2
User mengatur jam tidak diganggu (mis. 22.00–06.00, mengikuti zona perangkat). Worker menahan push yang tidak kritis selama jam tenang (rilis baru & broadcast menunggu sampai jam tenang berakhir; pesan masuk tetap tersimpan di kotak masuk `notifications`). Mengurangi user mematikan notifikasi sepenuhnya — yang jauh lebih buruk. Dampak DB: kolom jam tenang di `profiles`.

### F21 — Unduhan pintar (premium) · P2
Melengkapi unduhan offline yang sudah ada: unduh hanya saat Wi-Fi, jadwal unduh pada jam yang dipilih (mis. tengah malam), dan **unduh otomatis episode berikutnya** dari judul favorit yang loncengnya aktif begitu rilis — user membuka aplikasi dan episode sudah siap. Semua diatur per user di pengaturan unduhan; berjalan dengan pengelola unduhan Android (batasan Wi-Fi/pengisian daya dari sistem).

### F22 — Moderator komunitas untuk chat global · P3
Chat global akan ramai; admin platform tidak bisa berjaga 24 jam. Admin bisa mengangkat user tepercaya menjadi **Moderator Komunitas**: haknya terbatas dan hanya di kanal global — menghapus pesan & membisukan sementara (maks. durasi diatur admin). Semua tindakan moderator tercatat dan bisa diaudit/dicabut admin kapan pun. Skala moderasi tanpa menambah admin penuh.

## Optimasi

### O8 — Play Integrity untuk endpoint sensitif · P1
API kita terbuka untuk aplikasi kita sendiri — dan untuk kloningan. Verifikasi **Google Play Integrity** pada aksi sensitif (klaim voucher, unduhan premium, pembuatan sesi unduhan) memastikan permintaan datang dari aplikasi resmi yang tidak dimodifikasi. Menekan penyalahgunaan API & pembajakan fitur premium dengan biaya integrasi kecil (token diverifikasi backend).

### O9 — Failover sumber video + pemeriksa kesehatan sumber · P1
Satu `video_url` per episode adalah titik gagal tunggal, padahal sumber scraping sering mati/berganti. Dukung banyak sumber per episode (tabel `episode_sources` atau daftar cermin), worker memeriksa kesehatan tiap tautan secara berkala dan menandai yang mati, player otomatis berpindah ke cermin yang sehat tanpa user sadar, dan admin melihat daftar sumber mati di monitoring untuk diganti. Ini optimasi keandalan paling penting untuk aplikasi berbasis scraping.

### O10 — Skor tren & populer dihitung worker · P2
"Sedang Tren" dan "Populer Minggu Ini" jangan dihitung dengan agregasi berat setiap ada yang membuka beranda. Worker menghitungnya berkala (mis. tiap jam) dari `watch_history`/`read_history` ke tabel agregat kecil, dan endpoint beranda tinggal membaca hasil jadi. Beranda tetap ringan walau data riwayat membesar.

### O11 — Retensi data & pembersihan otomatis · P1
Database lokal VPS berbagi disk 50 GB dengan sistem — harus ada kebijakan buang. Worker membersihkan berkala: sesi pembayaran kedaluwarsa, akses tamu lama, pesan chat yang terhapus lunak lebih dari 90 hari, dan agregasi riwayat lama; monitoring admin memberi peringatan pemakaian disk pada 70% dan 85%. Mencegah insiden "disk penuh" yang melumpuhkan seluruh layanan.

### O12 — Cadangan inkremental (arsip WAL) ke R2 · P2
Cadangan malam `pg_dump` berarti kehilangan data bisa sampai 24 jam. Tambahkan pengarsipan WAL PostgreSQL secara kontinu ke R2 (alat standar seperti WAL-G/pgBackRest): pemulihan bisa *point-in-time* dengan kehilangan data hanya beberapa menit. Biaya penyimpanan kecil (WAL terkompresi), uji pemulihan triwulanan yang sudah dirancang tetap berlaku untuk jalur ini.

### O13 — Kompresi respons API (Brotli/gzip) · P1
Respons API kita JSON terenkripsi (`xdata`) dan katalog ukurannya besar. Aktifkan kompresi Brotli/gzip di Caddy untuk respons API: payload menyusut besar sebelum enkripsi diterapkan pada jalur yang tepat, unduhan katalog & sinkronisasi delta jadi jauh lebih cepat dan hemat kuota user. Konfigurasi satu tempat, manfaat ke semua endpoint.

### O14 — Pencarian lokal instan di perangkat · P2
Aplikasi sudah menyimpan katalog lokal hasil sinkronisasi delta — manfaatkan: pencarian judul berjalan **di perangkat** (instan, tetap bekerja tanpa sinyal), dan pencarian server (FTS) hanya dipakai untuk pencarian mendalam/katalog yang belum tersinkron. Hasilnya terasa jauh lebih cepat daripada menunggu jaringan untuk setiap ketikan.

### O15 — Poster multi-ukuran saat unggah · P2
Saat admin menambahkan poster, worker membuat tiga ukuran WebP di R2 (kecil untuk daftar, sedang untuk kartu, besar untuk detail). Aplikasi meminta ukuran sesuai tempat tampil — daftar judul yang berisi puluhan poster tidak lagi mengunduh gambar ukuran detail. Melengkapi aturan WebP yang sudah ada dengan ukuran yang benar.

## Urutan penerapan yang disarankan

1. **Fondasi keandalan dulu**: O9 (failover sumber), O11 (retensi disk), O13 (kompresi), F18 (lapor cepat) — empat ini menjaga layanan tetap hidup & katalog tetap benar.
2. **Pengalaman menonton**: F15 (Cast), F14 (kalender rilis), F21 + F19 (unduhan pintar & hemat data).
3. **Pertumbuhan**: F16 (impor MAL/AniList), F17 (profil publik), F13 (kontribusi terjemahan).
4. **Skala & keamanan**: O8 (Play Integrity), O10 (tren agregat), O12 (WAL), O14 (pencarian lokal), O15 (poster multi-ukuran), F20 (jam tenang), F22 (moderator komunitas).
