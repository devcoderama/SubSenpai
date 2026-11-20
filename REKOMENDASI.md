# Rekomendasi Fitur & Optimasi — Putaran 5

> Status: **USULAN** (20 Nov 2026) — belum keputusan. Semua item di sini BARU: tidak mengulang putaran 1 (A/B), 2 (F1–F12, O1–O7), 3 (F13–F22, O8–O15), maupun 4 (F23–F30, O16–O21) yang sudah resmi masuk dokumen rancangan (STRUKTUR.md, DATABASE.md, STRUKTUR_ADMIN.md, FITUR.md, KEAMANAN.md, OPTIMASI.md, NOTE.md). Penomoran melanjutkan putaran sebelumnya.
>
> Cara menyetujui: sebutkan kode itemnya (mis. "F31, F36, O24 masuk") — item yang disetujui akan dipindahkan ke dokumen rancangan yang sesuai, lalu file ini dihapus, seperti putaran-putaran sebelumnya.

Prioritas: **P1** = berdampak besar/usaha kecil-menengah, sebaiknya awal · **P2** = berharga, setelah fondasi jalan · **P3** = pelengkap, kerjakan bila sempat.

## Fitur

### F31 — Terjemahan Balon Komik via OCR · P1
Pasangan alami subtitle otomatis, tapi untuk komik: banyak manga/manhwa/manhua kita hanya tersedia dalam bahasa asal. Saat chapter ditambahkan, worker menjalankan **OCR** pada halaman untuk menemukan balon teks dan membaca teks bahasa asalnya, lalu menyimpan wilayah teks itu (koordinat + teks) sebagai data chapter. Saat user membaca dalam bahasa lain, **aplikasi menerjemahkan teks tiap balon di perangkat** (endpoint `gtx` dari perangkat user, pola yang sama dengan subtitle) dan menggambarnya sebagai lapisan terjemahan di atas balon asli — user bisa beralih "asli/terjemahan" sekali ketuk. Kualitasnya setara subtitle mesin kita ("cukup dimengerti", bukan scanlation) dan diberi label jujur seperti itu. Ini fitur pembeda terbesar yang belum dimiliki aplikasi sejenis di kelas gratis.

### F32 — Ulasan Tertulis User · P2
Rating bintang sudah ada; tambahkan **ulasan tertulis** per judul (teks ulasan + penanda spoiler wajib bila mengandung bocoran, isinya disembunyikan sampai diketuk). User lain bisa memberi suara "membantu" pada ulasan; ulasan teratas tampil di halaman detail. Ulasan tunduk pada aturan yang sama dengan komentar (filter kata, larangan tautan, bisa dilaporkan & dimoderasi).

### F33 — Daftar Bersama Teman · P2
Selain koleksi publik (F5), sediakan **daftar bersama privat**: buat daftar, undang teman, semua anggota bisa menambah/menghapus judul dan melihat siapa sudah menonton apa di daftar itu — untuk merencanakan tontonan bersama ("daftar nonton bareng liburan"). Terhubung alami ke nonton bareng (F3): satu ketuk dari daftar bersama untuk membuka sesi.

### F34 — Paket Kenyamanan Player · P2
Kumpulan kecil yang membuat player terasa matang, satu paket:
- **Pengatur waktu tidur**: putar berhenti setelah X menit atau di akhir episode.
- **Gestur**: geser vertikal kiri = kecerahan, kanan = volume; ketuk dua kali sisi = maju/mundur 10 detik.
- **Kecepatan putar diingat per judul** (bukan global): judul yang biasa ditonton 1,25× tetap 1,25× saat dibuka lagi.

### F35 — Scrobbling Otomatis ke AniList/MyAnimeList · P3
Untuk user yang menghubungkan akun AniList/MAL-nya (token milik user, disimpan terenkripsi di server): setiap episode selesai ditonton/chapter selesai dibaca, progresnya **otomatis tercatat** di layanan itu (seperti scrobbling musik). Opt-in penuh, bisa diputus kapan pun, dan kegagalan sinkron tidak pernah mengganggu pemutaran. Pasangan dua arah dari impor (F16) & ekspor (F30).

### F36 — Kanal Pengumuman Rilis Otomatis ke Telegram/Discord · P2
Admin bisa menghubungkan **kanal Telegram atau webhook Discord resmi proyek** dari panel admin: setiap episode/chapter baru terbit, worker mengirim pengumuman otomatis (poster + judul + tautan aplikasi) ke kanal itu. Mesin pertumbuhan di luar aplikasi — komunitas yang belum memasang aplikasi pun melihat rilisnya dan tertarik memasang. Dibatasi agar tidak banjir: digabung per judul per jam, hanya untuk judul populer atau semua rilis sesuai pengaturan admin.

### F37 — Identifikasi Judul dari Tangkapan Layar · P2
User mengunggah tangkapan layar adegan anime; aplikasi mengirimkannya **dari perangkat user** ke layanan pencocok frame eksternal (jenis layanan seperti trace.moe) dan menampilkan hasil kecocokannya: judul, nomor episode, dan timestamp adegannya — bila judul itu ada di katalog kita, satu ketuk membuka episode tepat di posisi itu. Tanpa beban server (panggilan dari perangkat, pola hemat yang sama), dan gambar tidak disimpan oleh kita.

### F38 — Tantangan Antar Teman · P3
User bisa menantang teman: "siapa selesai menamatkan judul ini duluan" atau "siapa membaca lebih banyak chapter minggu ini". Progres kedua pihak terlihat di kartu tantangan (memakai data riwayat yang sudah ada), pemenang mendapat XP & lencana khusus tantangan. Gamifikasi sosial ringan di atas pertemanan & misi (F25) yang sudah ada.

## Optimasi

### O22 — Partisi Tabel Besar per Bulan · P2
Tabel yang tumbuh tanpa batas di database lokal VPS — `chat_messages`, `watch_history`, `read_history`, `notifications` — dijadikan **tabel terpartisi per bulan** (fitur bawaan PostgreSQL). Kueri terbaru hanya menyentuh partisi bulan berjalan, pembersihan retensi (O11) berubah dari DELETE besar menjadi melepas partisi lama (seketika, tanpa bloat), dan cadangan ikut lebih ringan. Dikerjakan sebelum data membesar, bukan sesudahnya.

### O23 — Disiplin Migrasi Berversi + Uji Kontrak API · P2
Skema kita sudah berubah berkali-kali selama perancangan (36+ tabel dan terus bertambah). Tegakkan: semua perubahan skema hanya lewat **migrasi berversi** (alat standar seperti goose) yang bisa maju dan mundur, dan **uji kontrak API** otomatis di CI yang memastikan respons lama tidak berubah bentuk tanpa kenaikan versi. Mencegah kelas bug "aplikasi lama rusak karena skema berubah diam-diam" — pasangan teknis dari gerbang versi (O17).

### O24 — Pra-Saring Audio (VAD) Sebelum Transkripsi · P1
Biaya variabel terbesar kita adalah menit transkripsi Whisper. Sebelum audio dikirim ke API transkripsi, worker menjalankan **deteksi aktivitas suara (VAD)** dan memangkas segmen hening panjang serta menandai segmen musik (OP/ED yang penanda intro/outro-nya sudah kita punya, F10) untuk tidak ditranskripsikan sebagai dialog. Hasilnya: menit yang ditagih API menyusut nyata per episode, tanpa mengubah kualitas subtitle dialog. Hemat langsung, terukur, dan tidak terlihat oleh user.

### O25 — Digest Notifikasi Rilis per Judul · P2
Bila satu judul merilis banyak episode/chapter sekaligus (rilis borongan), user berlonceng jangan menerima 10 push terpisah. Worker **menggabungkan rilis per judul** dalam jendela singkat menjadi satu notifikasi ("5 episode baru dari X"), sementara kotak masuk tetap mencatat tiap rilis. Melengkapi jam tenang (F20) dan pengingat cerdas (O21) sebagai lapis ketiga disiplin notifikasi.

### O26 — Load Shedding Anggun Saat VPS Sibuk · P3
VPS 1 vCPU akan sesekali sibuk. Saat beban melewati ambang, API **melepas beban tidak kritis lebih dulu** — rekomendasi, tren, dan Tanya-Senpai-context dilayani dari cache walau agak basi; polling & statistik ditunda — sementara jalur kritis (login, pemutaran, pembayaran, chat) tetap diprioritaskan. Layanan terasa "sedikit lebih lambat", bukan "mati", di jam ramai.

### O27 — Peluncuran APK Bertahap via Gerbang Versi · P2
Karena distribusi kita lewat website & GitHub (NOTE.md), rilis APK baru bisa diluncurkan **bertahap**: versi baru ditawarkan dulu ke sebagian user (mis. 10% selama 2 hari) lewat mekanisme pemeriksaan pembaruan, sambil metrik crash (O6) dipantau; bila sehat, dibuka ke semua; bila bermasalah, penawaran dihentikan sebelum semua user terkena. Memakai gerbang versi (O17) yang sudah dirancang — tanpa infrastruktur baru.

## Urutan penerapan yang disarankan

1. **Hemat & pembeda**: O24 (pra-saring transkripsi), F31 (terjemahan balon komik).
2. **Pertumbuhan**: F36 (kanal pengumuman otomatis), F37 (identifikasi dari screenshot), O27 (rilis APK bertahap).
3. **Kedalaman komunitas**: F32 (ulasan), F33 (daftar bersama), F34 (kenyamanan player), O25 (digest notifikasi).
4. **Fondasi jangka panjang**: O22 (partisi tabel), O23 (migrasi & kontrak API), O26 (load shedding), F35 (scrobbling), F38 (tantangan teman).
