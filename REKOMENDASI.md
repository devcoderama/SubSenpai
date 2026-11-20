# REKOMENDASI.md — Rekomendasi Putaran 2 (Fitur & Peningkatan)

Rekomendasi lanjutan setelah putaran 1 seluruhnya diterapkan ke dokumen rancangan (riwayatnya ada di git). Status dokumen ini: **usulan, belum keputusan** — item yang disetujui akan dipindahkan ke STRUKTUR.md / DATABASE.md / STRUKTUR_ADMIN.md dan file ini dihapus, seperti putaran sebelumnya. Prioritas: **P1** = berdampak besar/awal, **P2** = setelahnya, **P3** = pelengkap. Usaha: K (kecil), S (sedang), B (besar).

## A. Fitur Baru

| # | Rekomendasi | Alasan | Prioritas / Usaha |
| --- | --- | --- | --- |
| F1 | **Chat pribadi (DM) antar teman** — obrolan 1:1 khusus sesama teman, memakai infrastruktur chat yang sama (reply, stiker, gambar ImgBB, larangan tautan luar ikut berlaku) | Sistem teman sudah ada tapi belum punya tempat bicara privat; DM adalah alasan utama user menambah teman | P1 / S |
| F2 | **Kehadiran sosial** — status online teman + "sedang menonton/membaca apa" (bisa dimatikan di pengaturan privasi) | Komunitas terasa hidup saat user melihat temannya aktif; mendorong menonton judul yang sama | P2 / K |
| F3 | **Nonton bareng (watch party)** — ruang putar tersinkron untuk grup/teman: play/pause/seek bersama + chat di samping player | Fitur pembeda terkuat untuk aplikasi anime komunitas; grup yang sudah ada langsung punya kegiatan | P2 / B |
| F4 | **Permintaan judul + voting** — user meminta judul yang belum ada & memberi suara; admin melihat daftar terurut berdasar permintaan | Katalog tumbuh ke arah yang benar-benar diminta user; modul admin katalog tinggal menambah satu tab | P1 / S |
| F5 | **Koleksi kurasi user** — user menyusun daftar publik ("Donghua xianxia wajib tonton", "Anime sedih terbaik") yang bisa diikuti user lain | Penemuan konten dari komunitas, bukan hanya algoritma; memberi peran bagi user aktif | P2 / S |
| F6 | **Hadiah premium langsung ke teman** — pembelian voucher bisa memilih penerima dari daftar teman; voucher terkirim otomatis lewat notifikasi (tanpa menyalin kode) | Mengurangi gesekan fitur hadiah yang sudah ada; momen "dibuatkan teman" mendorong penerima jadi pembeli berikutnya | P1 / K |
| F7 | **Uji coba premium gratis 3 hari** — sekali per akun untuk user baru (ditandai di profil agar tidak diakali daftar ulang) | Cara termurah menaikkan konversi: user merasakan bedanya premium sebelum membayar | P1 / K |
| F8 | **Pencapaian & rentetan harian (streak)** — lencana untuk tonggak (100 episode ditonton, streak 7/30 hari, komentar pertama) di atas sistem XP/level yang sudah ada | Sistem level sudah dirancang; pencapaian memberinya tujuan dan alasan membuka aplikasi tiap hari | P2 / S |
| F9 | **Rekap pribadi ("Rekap Senpai")** — halaman statistik user: total jam menonton, chapter dibaca, genre teratas, judul terlama diikuti; bisa dibagikan sebagai kartu berbagi | Konten yang membuat user bangga & membagikan aplikasi ke luar (lewat mekanisme berbagi resmi) | P2 / S |
| F10 | **Lewati intro/outro + putar otomatis episode berikutnya** — penanda rentang intro/outro per episode (diisi admin/komunitas), tombol "Lewati Opening", dan episode lanjut otomatis | Kenyamanan menonton maraton yang paling terasa sehari-hari; penanda cukup dua kolom timestamp per episode | P1 / S |
| F11 | **Subtitle bahasa kedua (Inggris)** — bahasa `en` di samping `id` bila sumber punya caption Inggris (diterjemahkan dari caption / dipakai apa adanya) | Kolom `lang` memang sudah mendukung banyak bahasa; biaya tambahannya kecil untuk episode bercaption | P3 / S |
| F12 | **Batas perangkat premium (maks 2)** + halaman keamanan akun (lihat & cabut sesi perangkat lain) | Mencegah berbagi akun premium massal tanpa menyulitkan user wajar; user juga bisa mengamankan akunnya sendiri | P2 / S |

## B. Peningkatan / Optimasi

| # | Rekomendasi | Alasan | Prioritas / Usaha |
| --- | --- | --- | --- |
| O1 | **Sinkronisasi katalog delta di aplikasi** — katalog disimpan lokal (database aplikasi) dan disinkronkan hanya perubahannya (penanda `updated_at`) | Beranda tampil seketika tanpa menunggu jaringan; hemat kuota user dan beban API | P1 / S |
| O2 | **Thumbnail ImgBB untuk daftar chat** — pesan gambar menampilkan versi `thumb` dari ImgBB; gambar penuh baru dimuat saat diketuk | URL thumbnail memang disediakan ImgBB; daftar chat jauh lebih ringan & cepat | P1 / K |
| O3 | **Kompresi unggahan di backend** — gambar chat diperkecil ke maks 1600px & WebP sebelum diteruskan ke ImgBB | Unggahan user dari kamera bisa 5–10 MB; memperkecilnya mempercepat kirim dan menghemat penyimpanan ImgBB | P1 / K |
| O4 | **Pratinjau muat (prefetch)** — reader memuat 3 halaman berikutnya; player memuat awal episode berikutnya saat episode berjalan mencapai 80% | Menghilangkan jeda paling mengganggu saat maraton baca/tonton | P2 / S |
| O5 | **Pengiriman broadcast bertahap** — broadcast admin dikirim worker dalam kelompok (FCM multicast per 500 token) dengan jeda, bukan sekaligus | Mencegah lonjakan beban VPS & batas laju FCM saat audiens besar; `sent_count` terisi progresif | P1 / K |
| O6 | **Laporan crash aplikasi** (GlitchTip/Sentry self-hosted ringan, atau Firebase Crashlytics gratis) | Error di perangkat user tidak terlihat dari server; laporan crash adalah satu-satunya cara mengetahuinya | P2 / K |
| O7 | **Uji pemulihan cadangan berkala** — tiap 3 bulan, dump `pg_dump` dicoba dipulihkan ke database kosong dan hasilnya dicatat | Cadangan yang belum pernah dipulihkan adalah harapan, bukan cadangan | P2 / K |

## C. Urutan Penerapan yang Disarankan

1. **Bersama rilis awal**: O1, O2, O3, O5 (murah, terasa langsung), F6, F7 (menggerakkan pendapatan), F4 (mengarahkan katalog).
2. **Setelah komunitas mulai aktif**: F1, F10, F8, F2.
3. **Fase pembeda & pendalaman**: F3, F5, F9, F12, O4, O6, O7, F11.

Item yang disetujui cukup disebutkan nomornya — sisanya tetap di sini sebagai cadangan ide.
