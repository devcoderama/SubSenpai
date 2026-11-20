# FITUR.md — Fitur Tambahan SubSenpai

Dokumen rancangan **resmi** untuk fitur-fitur gelombang ketiga (F13–F22) dan keempat (F23–F30), disetujui 20 Nov 2026 dan dipindahkan dari REKOMENDASI.md putaran 3 dan 4. Dokumen ini adalah pendamping [STRUKTUR.md](STRUKTUR.md) (arsitektur & fitur inti), [DATABASE.md](DATABASE.md) (skema dasar), [KEAMANAN.md](KEAMANAN.md), dan [OPTIMASI.md](OPTIMASI.md). Perubahan skema dirinci per fitur dan menjadi bagian dari skema saat fase pembangunan; bila ada pertentangan, dokumen yang lebih spesifik untuk topiknya yang berlaku.

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

---

# Gelombang Keempat (F23–F30)

## F23 — Tanya Senpai: Asisten Rekomendasi AI

Asisten tanya-jawab rekomendasi di dalam aplikasi: user bertanya dengan bahasa sehari-hari ("anime pendek yang mirip Frieren tapi lebih lucu", "donghua balas dendam yang sudah tamat") dan menerima jawaban berupa **kartu judul dari katalog** yang bisa langsung diketuk untuk dibuka.

**Mesin AI — sama persis dengan Chat AI di aplikasi D TRADE**
- Layanan AI web gratis **Pollinations** (`text.pollinations.ai`, gaya API OpenAI), **tanpa kunci API dan tanpa akun**, dipanggil **dari aplikasi di perangkat user** — bukan dari server. Server menanggung nol biaya AI, dan panggilan berangkat dari IP masing-masing user (pola yang sama yang membuat Chat AI D TRADE berjalan).
- Teks saja, seperti batas jalur gratisnya di D TRADE (model gratis tanpa kunci tidak membaca gambar; fitur ini memang tidak membutuhkan gambar).

**Pengaturan khusus SubSenpai (yang membedakannya dari Chat AI D TRADE)**
Perilaku asisten diatur oleh **prompt sistem khusus SubSenpai** — berkas keahlian "SKILL Tanya Senpai" yang tertanam di aplikasi dan terbit bersama versi aplikasi (pola yang sama dengan berkas keahlian AI di D TRADE), tidak bisa diubah user. Isi pengaturannya:
1. **Persona "Senpai"** — ramah dan ringkas, selalu menjawab dalam bahasa aplikasi user (`app_language`).
2. **Konteks katalog** — aplikasi melampirkan ringkasan katalog pada percakapan (id judul, judul + judul alternatif, tipe, genre, sinopsis singkat, rating, jumlah episode/chapter, status tayang) ditambah petunjuk selera dari data lokal user (genre yang paling sering ditonton/dibaca, judul favorit). Yang dikirim hanya konteks katalog & selera agregat — **bukan** data pribadi; personalisasi bisa dimatikan user di pengaturan.
3. **Batas pengetahuan yang tegas** — asisten hanya boleh merekomendasikan judul yang ada di konteks; **dilarang mengarang** judul, episode, atau fakta di luar konteks. Bila tidak ada yang cocok, ia mengatakannya terus terang, menawarkan yang paling mendekati, dan mengarahkan user ke fitur **Permintaan Judul** (F4).
4. **Kontrak keluaran** — jawaban = kalimat pembuka singkat + maks. 5 pilihan yang masing-masing merujuk **id judul** dengan satu baris alasan yang menyambung ke selera yang disebut user; aplikasi merender rujukan id itu menjadi kartu judul. Tidak ada tautan luar dalam jawaban.
5. **Bukan chatbot umum** — pertanyaan di luar rekomendasi dan katalog dialihkan dengan sopan kembali ke rekomendasi.
6. **Menghormati filter** — judul dewasa tidak pernah direkomendasikan bila `hide_adult` aktif atau profil Anak (F26) yang sedang aktif.

**Batas layanan & perilaku cadangan**
- Tingkat gratis Pollinations membatasi laju panggilan per IP (pengalaman dari D TRADE): aplikasi memberi jeda antar-pertanyaan dan pesan ramah saat layanan ramai ("Senpai sedang ramai — coba lagi sebentar lagi"), serta menyimpan jawaban untuk pertanyaan + konteks yang sama di cache lokal perangkat untuk sementara.
- Riwayat percakapan Tanya Senpai tersimpan **lokal di perangkat saja** (seperti riwayat Chat AI D TRADE), tidak dikirim ke server kita.
- Admin bisa mematikan Tanya Senpai dari pengaturan platform (pola saklar yang sama dengan fitur Terjemahkan).

**Perubahan data** — tidak ada; fitur ini tidak menambah tabel maupun kolom.

## F24 — Polling & Kuis di Chat

- `chat_messages` mendapat jenis pesan baru **`poll`**: kolom isi polling (jsonb) berisi pertanyaan, daftar pilihan, dan waktu berakhir. Satu akun satu suara; hasil terlihat langsung dan berubah secara waktu-nyata lewat WebSocket yang sama.
- Pembuat polling: semua user di kanal grupnya sendiri; di kanal global hanya admin & moderator komunitas. Polling bisa disematkan.
- **Kuis trivia**: polling bertanda kuis (jawaban benar tersembunyi sampai polling berakhir, lalu diumumkan) — dijalankan terjadwal oleh admin/moderator di kanal global; skor kuis memberi **XP** ke sistem level & pencapaian yang sudah ada, dengan papan skor mingguan per kanal.

**Perubahan data**
- `chat_messages.kind` bertambah nilai `poll` + kolom jsonb untuk definisi polling (termasuk penanda kuis & kunci jawaban terenkripsi waktu — kunci hanya dibuka server setelah berakhir).
- Tabel baru `chat_poll_votes`: `message_id`, `user_id`, `option_index`, `created_at`; unik per (message_id, user_id).

## F25 — Misi Musiman Berhadiah

- Admin membuat **misi terbatas waktu** (mingguan/musiman): target terukur dari aktivitas yang sudah tercatat — menonton N episode (boleh disaring genre), membaca N chapter, memberi rating N judul, dan sejenisnya.
- Progres user dihitung worker dari aktivitasnya; halaman Misi menampilkan progres berjalan. Misi selesai → hadiah **XP, lencana khusus musim, atau voucher premium** (memakai sistem voucher yang sudah ada) yang harus diklaim user.
- Misi menjadi bahan broadcast yang bernilai (pengumuman misi baru), bukan sekadar promosi.

**Perubahan data**
- Tabel baru `missions`: `title`, `description`, `target` (jsonb: jenis aktivitas + jumlah + saringan), `reward_kind` (∈ `xp|badge|voucher`), `reward_value`, `starts_at`, `ends_at`, `is_active`.
- Tabel baru `user_missions`: `user_id`, `mission_id`, `progress`, `completed_at`, `claimed_at`; unik per (user_id, mission_id).
- `vouchers.source` bertambah nilai `mission`.

## F26 — Multi-Profil dalam Satu Akun

- Satu akun bisa memiliki maks. **3 profil penonton**; tiap profil punya watchlist, riwayat, progres, dan bahasa aplikasinya sendiri. Premium & langganan tetap melekat pada akun (semua profil ikut menikmati).
- **Profil Anak**: `hide_adult` dipaksa aktif dan tidak bisa dimatikan dari dalam profil Anak; pemilik akun bisa membatasi genre yang tampil; **keluar dari profil Anak memerlukan PIN 4 digit pemilik akun** (hash PIN disimpan & diverifikasi di server, pola PBKDF2 yang sama dengan PIN lain di rancangan ini).
- Berpindah profil dari menu akun; profil aktif menentukan semua data tontonan yang dibaca/ditulis aplikasi.

**Perubahan data**
- Tabel baru `viewing_profiles`: `id`, `account_id` (FK ke `profiles`), `name`, `is_kids`, `pin_hash` (boleh kosong; wajib untuk profil Anak), `app_language`, `created_at`.
- Kolom `viewing_profile_id` ditambahkan ke `watchlist`, `watch_history`, dan `read_history`; baris lama dianggap milik profil utama yang dibuat otomatis untuk tiap akun.

## F27 — Picture-in-Picture & Mode Audio Saja

- **PiP**: player memakai picture-in-picture bawaan Android/Media3 — video menjadi jendela melayang saat user pindah aplikasi atau menjelajah halaman lain; kontrol dasar (putar/jeda, tutup, kembali) tersedia di jendela PiP.
- **Mode audio saja**: saklar di player mematikan trek video (hanya suara yang diputar dan diunduh jaringannya) — hemat data & baterai untuk user yang mendengarkan sambil bekerja atau dengan layar mati.
- Keduanya murni sisi player; gerbang premium & sesi pemutaran tidak berubah. Tanpa perubahan data.

## F28 — Penanda Momen & Catatan Pribadi

- Saat menonton, user bisa menandai timestamp dengan catatan singkat ("momen ini!"); saat membaca, menandai halaman chapter.
- Penanda tersimpan **privat** per user (dan per profil penonton bila F26 aktif), tampil sebagai daftar di halaman detail judul, dan bisa dilompati langsung — player/reader membuka tepat di posisi penanda.

**Perubahan data**
- Tabel baru `bookmarks`: `user_id`, `episode_id` (boleh kosong), `chapter_id` (boleh kosong), `position_sec` (untuk video), `page_number` (untuk chapter), `note`, `created_at`.

## F29 — Widget Layar Utama & Pintasan Aplikasi

- Widget Android **"Lanjutkan"**: judul terakhir + batang progres; satu ketuk membuka aplikasi langsung memutar dari posisi terakhir.
- Widget **"Rilis Hari Ini"**: daftar rilis hari ini dari kalender tayang (F14), menandai yang ada di watchlist user.
- Pintasan tekan-lama ikon aplikasi: **Cari**, **Lanjutkan**, **Kalender**, **Chat Global**.
- Semua membaca data lokal hasil sinkronisasi — tanpa perubahan server maupun data.

## F30 — Ekspor Daftar (Pasangan Impor F16)

- Dari Pengaturan → **Ekspor daftar**: user mengunduh watchlist + progresnya sebagai berkas **XML kompatibel MyAnimeList** atau **CSV**, dibuat server dari data milik user itu sendiri.
- Berfungsi sebagai cadangan pribadi dan bukti data user tidak terkunci di SubSenpai; pemetaan judulnya memakai pekerjaan pencocokan yang sama dengan F16.
- Ekspor dibatasi laju (mis. maks. beberapa kali sehari) dan tercatat sebagai aktivitas akun. Tanpa perubahan data.
