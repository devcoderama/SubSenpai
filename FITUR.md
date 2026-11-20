# FITUR.md — Fitur Tambahan SubSenpai

Dokumen rancangan **resmi** untuk fitur-fitur gelombang ketiga (F13–F22), disetujui 20 Nov 2026 dan dipindahkan dari REKOMENDASI.md putaran 3. Dokumen ini adalah pendamping [STRUKTUR.md](STRUKTUR.md) (arsitektur & fitur inti), [DATABASE.md](DATABASE.md) (skema dasar), [KEAMANAN.md](KEAMANAN.md), dan [OPTIMASI.md](OPTIMASI.md). Perubahan skema dirinci per fitur dan menjadi bagian dari skema saat fase pembangunan; bila ada pertentangan, dokumen yang lebih spesifik untuk topiknya yang berlaku.

---

## F13 — Kontribusi Terjemahan Sukarela (opt-in)

Menjembatani trade-off terjemahan di perangkat: hasil terjemahan satu user bisa dibagikan agar user lain tidak perlu menerjemahkan ulang — tetapi hanya atas tindakan eksplisit pemiliknya.

**Cara kerja**
- Setelah menonton dengan subtitle hasil terjemahan perangkat, player menawarkan menu **"Bagikan terjemahan ini"** (hanya untuk user yang login; tamu tidak bisa berkontribusi).
- Aplikasi mengunggah berkas SRT hasil terjemahannya (beserta id episode & bahasa tujuan) ke endpoint kontribusi backend.
- Backend **memeriksa format secara otomatis**: berkas SRT valid; jumlah blok subtitle sama persis dengan subtitle asal; setiap timestamp identik dengan asalnya; tidak mengandung tautan/HTML; ukuran wajar. Satu pemeriksaan gagal → unggahan ditolak beserta alasannya.
- Yang lolos disimpan sebagai baris `subtitles` baru: `lang` = bahasa tujuan, `source = 'community'`, berkasnya di R2. Di pemilih bahasa subtitle, varian ini berlabel **"Komunitas · @nama"**.
- User lain yang memilih bahasa itu memakai berkas komunitas langsung dari server (tidak menerjemahkan di perangkat), kecuali ia memilih "terjemahkan sendiri di perangkat".

**Aturan & keamanan**
- Bawaan **mati**: tidak ada unggahan otomatis; kontribusi selalu tindakan sadar user.
- Batas kontribusi per user per hari; kontributor yang unggahannya berulang kali ditolak/dilaporkan kehilangan hak kontribusi (keputusan admin, tercatat).
- User bisa melaporkan "terjemahan ini buruk" dari pemilih bahasa (memakai `reports`); admin meninjau dan bisa **menarik** varian komunitas — setelah ditarik, user kembali ke terjemahan perangkat masing-masing.
- Pemeriksaan & batasan unggahan dirinci juga di KEAMANAN.md Bagian 3.

**Perubahan data**
- `subtitles.source` bertambah nilai `community`.
- `subtitles.contributed_by` (uuid, boleh kosong, FK ke `profiles`) — penunjuk kontributor untuk varian komunitas.

## F14 — Jadwal Rilis & Kalender Tayang

- Judul yang sedang tayang punya jadwal tetap: `titles.airing_day` (0–6 mengikuti hari di negara sumber, boleh kosong = tidak terjadwal), `titles.airing_time` (jam tayang), `titles.airing_tz` (zona waktu sumber, mis. `Asia/Tokyo`).
- Halaman **Kalender**: tampilan satu minggu dikelompokkan per hari, semua jam **terkonversi ke zona waktu perangkat user**; judul yang ada di watchlist user ditandai khusus.
- Halaman detail menampilkan perkiraan episode berikutnya: "Episode berikutnya ± Jumat 23.30 (2 hari 4 jam lagi)" — dihitung dari jadwal + episode terakhir yang tersedia. Selalu diberi label **perkiraan** karena rilis sumber bisa lebih cepat/lambat.
- Lonceng favorit yang sudah ada menjadi pasangan alaminya: jadwal memberi tahu *kapan*, lonceng memberi tahu *sudah rilis*.
- Admin mengisi/mengubah jadwal dari form katalog (modul Katalog di STRUKTUR_ADMIN.md).

## F15 — Cast ke TV (Chromecast)

- Player memakai ekstensi Cast resmi Media3 (Google Cast): satu ketuk mengirim video ke Chromecast/Android TV; HP menjadi remote — putar/jeda, seek, lewati intro, ganti kualitas, episode berikutnya, dan antrean episode.
- Subtitle yang ikut ke TV adalah subtitle yang **punya URL di server** (bahasa asal atau varian komunitas F13). Terjemahan lokal perangkat tidak bisa dikirim ke penerima Cast; dalam kasus itu player memberi tahu user dan memakai subtitle server yang tersedia (atau tanpa subtitle bila user memilih begitu).
- Murni sisi aplikasi — tanpa perubahan server maupun skema.

## F16 — Impor Daftar dari MyAnimeList / AniList

- Di Pengaturan → **Impor daftar**: user memilih berkas ekspor dari MyAnimeList (XML) atau AniList (JSON); aplikasi mengirimnya ke backend untuk diproses (aturan keamanan berkas di KEAMANAN.md Bagian 3).
- Backend mencocokkan tiap entri ke `titles` memakai judul utama, judul alternatif, dan judul Jepang: pencocokan persis didahulukan, pencocokan mirip yang terkendali sesudahnya; entri yang ambigu tidak ditebak — dilaporkan agar user memilih manual dari saran yang diberikan.
- Yang terimpor: entri `watchlist` dengan status terpetakan (watching → `watching`, completed → `completed`, plan to watch → `plan`, dropped → `dropped`; on-hold → `watching`), dan progres berupa penanda episode/chapter terakhir pada riwayat (bukan riwayat tontonan penuh).
- Impor bersifat **satu arah dan tidak menghapus**: entri yang sudah ada digabung — status yang kosong diisi, progres mengambil yang lebih jauh.
- Selesai impor, user menerima laporan: berapa cocok, berapa tidak ditemukan (beserta daftar judulnya), berapa ambigu.

## F17 — Profil Publik yang Bisa Dibagikan

- User mengatur profilnya **publik/privat** (bawaan: daftar tontonan privat; koleksi mengikuti pengaturan privat/publik koleksinya sendiri).
- Profil publik bisa dibagikan keluar aplikasi sebagai **kartu bagikan resmi** (tautan aplikasi) berisi nama, avatar, level, lencana/pencapaian, koleksi publik, dan Rekap Senpai — pola yang sama dengan tautan undangan grup: dibagikan keluar aplikasi, bukan ditempel ke dalam chat.
- Yang tidak pernah tampil di profil publik: riwayat detail per episode, daftar privat, keanggotaan grup privat, dan apa pun dari DM.

## F18 — Lapor Cepat dari Player & Reader

- Tombol lapor di dalam player dan reader dengan alasan terstruktur siap ketuk: `video_mati`, `episode_tertukar`, `subtitle_meleset`, `audio_bisu`, `chapter_rusak`, `lainnya`.
- Laporan langsung terikat ke episode/chapter yang sedang dibuka: `reports` dengan `target_kind` episode/chapter yang sudah didukung, ditambah kolom `reports.reason` berisi kode alasan di atas.
- Antrean laporan admin **diurutkan dari jumlah laporan**: episode yang dilaporkan banyak user naik ke atas sendiri; monitoring menandai episode dengan lonjakan laporan dalam 24 jam.
- Batas anti-spam: satu laporan per user per episode/chapter untuk alasan yang sama; batas harian per user.

## F19 — Mode Hemat Data

Satu saklar di pengaturan aplikasi (berlaku per perangkat, tanpa perubahan server):
- Kualitas video bawaan dibatasi (pilihan user, mis. maks. 480p).
- Poster dimuat ukuran kecil (memakai varian multi-ukuran O15 di OPTIMASI.md).
- Gambar chat hanya versi thumbnail sampai diketuk.
- Prefetch latar dimatikan; unduhan tetap hanya via Wi-Fi.

## F20 — Jam Tenang Notifikasi

- User mengatur jam tidak diganggu di `profiles` (`quiet_start`, `quiet_end`, mengikuti zona waktu perangkat): mis. 22.00–06.00.
- Selama jam tenang, worker **menahan push** kategori rilis baru & broadcast; salinan kotak masuk `notifications` tetap ditulis seperti biasa.
- Saat jam tenang berakhir, tertahan yang menumpuk dikirim sebagai **satu ringkasan** ("3 episode baru selama jam tenangmu"), bukan push terpisah-pisah.
- Push yang menyangkut keamanan akun tidak pernah ditahan.

## F21 — Unduhan Pintar (Premium)

Melengkapi unduhan offline premium yang sudah dirancang:
- **Hanya Wi-Fi** (bawaan aktif) dan **jadwal unduh** (mis. mulai 00.00) — dijalankan pengelola unduhan Android (WorkManager) dengan batasan jaringan/pengisian daya sistem.
- **Unduh otomatis episode berikutnya**: untuk judul favorit yang loncengnya aktif, episode baru diunduh otomatis begitu rilis — user membuka aplikasi dan episode sudah siap.
- Batas penyimpanan per user (GB) dan jumlah episode tersimpan; episode terlama yang **sudah selesai ditonton** terhapus otomatis lebih dulu saat batas tercapai.
- Semua pengaturan per user di halaman Pengaturan Unduhan; status premium tetap diverifikasi server saat sesi unduhan dibuat.

## F22 — Moderator Komunitas untuk Chat Global

- Admin platform bisa mengangkat user tepercaya menjadi **Moderator Komunitas** untuk kanal-kanal global.
- Haknya sempit dan hanya di kanal global: **menghapus pesan** (hapus lunak, sama seperti moderasi biasa) dan **membisukan sementara** (durasi maks. diatur admin, mis. 24 jam). Moderator TIDAK bisa membaca DM, mengelola user/akun, membuka modul admin lain, atau menyentuh kanal grup.
- Semua tindakan moderator tercatat di `admin_logs` (pelaku = moderator) dan bisa diaudit; admin bisa mencabut status moderator kapan pun, dan penyalahgunaan berakibat pencabutan + sanksi akun.
- Dikelola dari modul Moderasi Chat di panel admin (perluasan STRUKTUR_ADMIN.md Bagian 3.10).

**Perubahan data**
- Tabel baru `chat_moderators`: `user_id` (FK profiles), `granted_by` (FK profiles), `created_at`, dan cakupan kanal global (satu baris per moderator; kanal global tidak memerlukan id kanal).
