# Rekomendasi Fitur & Optimasi — Putaran 4

> Status: **USULAN** (20 Nov 2026) — belum keputusan. Semua item di sini BARU: tidak mengulang putaran 1 (A/B), putaran 2 (F1–F12, O1–O7), maupun putaran 3 (F13–F22, O8–O15) yang sudah resmi masuk dokumen rancangan (STRUKTUR.md, DATABASE.md, STRUKTUR_ADMIN.md, FITUR.md, KEAMANAN.md, OPTIMASI.md). Penomoran melanjutkan putaran sebelumnya.
>
> Cara menyetujui: sebutkan kode itemnya (mis. "F23, F27, O17 masuk") — item yang disetujui akan dipindahkan ke dokumen rancangan yang sesuai, lalu file ini dihapus, seperti putaran-putaran sebelumnya.

Prioritas: **P1** = berdampak besar/usaha kecil-menengah, sebaiknya awal · **P2** = berharga, setelah fondasi jalan · **P3** = pelengkap, kerjakan bila sempat.

## Fitur

### F23 — Tanya Senpai: Asisten Rekomendasi AI di Aplikasi · P1
User sering bingung mau menonton apa. Tambahkan tab/tombol **Tanya Senpai**: user bertanya dengan bahasa sehari-hari ("anime pendek yang mirip Frieren tapi lebih lucu", "donghua balas dendam yang sudah tamat") dan asisten menjawab dengan **kartu judul dari katalog kita** yang bisa langsung dibuka. Mengikuti pola hemat yang sama seperti terjemahan: panggilan AI dilakukan **dari perangkat user** ke endpoint AI publik gratis tanpa kunci (pola Chat AI D TRADE), konteks yang dikirim hanya ringkasan katalog (judul, genre, sinopsis singkat, rating) — bukan data pribadi. Jawaban dibatasi pada judul yang benar-benar ada di katalog; bila tidak ada yang cocok, asisten mengatakannya terus terang. Server tidak menanggung biaya AI sama sekali.

### F24 — Polling & Kuis di Chat · P2
Chat global/grup mendapat jenis pesan **polling**: buat pertanyaan + pilihan, satu akun satu suara, hasil terlihat langsung, pembuat/admin bisa menyematkan polling. Di atasnya, admin (dan moderator komunitas) bisa menjalankan **kuis trivia terjadwal** di kanal global — pertanyaan berkala dengan papan skor mingguan yang memberi XP ke pemenang (menyambung sistem level & pencapaian yang sudah ada). Mesin keterlibatan komunitas yang murah: datanya kecil, efek ramainya besar.

### F25 — Misi Musiman Berhadiah · P2
Pencapaian & streak yang sudah ada bersifat permanen dan statis. Tambahkan **misi terbatas waktu** (mingguan/musiman) yang dibuat admin: "tonton 5 episode genre Aksi minggu ini", "baca 10 chapter", "beri rating 3 judul". Penyelesaian misi memberi XP, lencana khusus musim, atau voucher premium kecil (memakai sistem voucher yang sudah ada). Misi memberi alasan untuk kembali setiap minggu dan bahan broadcast yang tidak terasa seperti iklan.

### F26 — Multi-Profil dalam Satu Akun · P3
Satu akun sering dipakai bergantian (HP keluarga). Dukung maks. **3 profil penonton** dalam satu akun — masing-masing dengan watchlist, riwayat, progres, dan bahasa sendiri — termasuk **profil Anak**: hanya konten non-dewasa, genre bisa dibatasi admin keluarga (pemilik akun), dan keluar dari profil Anak memakai PIN pemilik akun. Premium tetap per akun (semua profil ikut premium). Ini perubahan skema yang paling besar di putaran ini (tabel profil penonton + penunjuk profil pada watchlist/riwayat), jadi ditempatkan P3 walau nilainya nyata.

### F27 — Picture-in-Picture & Mode Audio Saja · P1
Dua kemampuan player standar Android yang belum kita pakai:
- **PiP**: video mengecil jadi jendela melayang saat user menjelajah aplikasi lain — bawaan Media3/Android, usaha kecil.
- **Mode audio saja**: trek video dimatikan, hanya suara yang diputar (hemat data & baterai besar) untuk user yang "menonton" sambil bekerja atau mendengarkan dialog. Cocok juga dengan layar mati.

### F28 — Penanda Momen & Catatan Pribadi · P3
User bisa menandai timestamp di episode ("momen ini!") dengan catatan pribadi singkat, dan menandai halaman di chapter. Penanda tersimpan privat per user, tampil sebagai daftar di halaman detail dan bisa dilompati langsung dari player/reader. Kecil, personal, dan membuat riwayat user terasa miliknya — bukan sekadar daftar judul.

### F29 — Widget Layar Utama & Pintasan Aplikasi · P2
- Widget Android: **"Lanjutkan"** (judul terakhir + progres, satu ketuk langsung memutar) dan **"Rilis Hari Ini"** (dari kalender F14).
- Pintasan tekan-lama pada ikon aplikasi: Cari, Lanjutkan, Kalender, Chat Global.
Tanpa perubahan server — murni aplikasi, memanfaatkan data yang sudah tersinkron lokal.

### F30 — Ekspor Daftar (Pasangan F16) · P3
Bila impor dari MAL/AniList sudah ada, sediakan arah sebaliknya: **ekspor** watchlist + progres user ke berkas XML kompatibel MAL dan CSV dari Pengaturan. User merasa datanya miliknya (tidak terkunci), dan berkas ekspor berfungsi sebagai cadangan pribadi. Usaha kecil karena pemetaan judulnya memakai pekerjaan F16 yang sama.

## Optimasi

### O16 — Halaman Komik Terkompresi Multi-Varian · P1
Aset terberat kita bukan video (langsung dari sumber) melainkan **halaman komik di R2**. Saat chapter ditambahkan, worker membuat varian baca terkompresi (WebP/AVIF, lebar standar baca) di samping berkas asli; reader memuat varian ringan per halaman dan hanya mengambil resolusi penuh saat user memperbesar. Melengkapi O15 (poster) untuk aset yang jauh lebih besar volumenya — halaman baca jadi cepat dan kuota user hemat.

### O17 — Gerbang Versi Minimum Aplikasi · P1
Distribusi kita lewat APK/Play Store berarti akan selalu ada user di versi lama yang rusak. Admin mengatur **versi minimum** dari pengaturan platform; aplikasi di bawah versi itu menerima layar perbarui yang ramah (dengan tautan unduh resmi) dan berhenti memakai API yang sudah berubah. Mencegah laporan bug dari versi yang sudah tidak didukung dan membuat perubahan API berani dilakukan.

### O18 — Metrik & Log Terstruktur untuk Monitoring · P2
Monitoring admin sekarang membaca status & statistik; tambahkan fondasi pengamatannya: log JSON terstruktur dari API & worker, dan endpoint metrik internal (latensi API per kelompok endpoint, kedalaman antrean subtitle, tingkat kegagalan sumber video O9, tingkat keberhasilan polling GoPay). Monitoring menampilkan grafik sederhana dari metrik ini — masalah terlihat sebelum user melapor.

### O19 — Dedupe Unggahan Gambar via Hash · P2
Stiker dan gambar yang sama sering diunggah berulang kali oleh banyak user ke ImgBB. Backend menghitung **hash isi gambar** sebelum mengunggah; bila hash sudah dikenal, pakai URL ImgBB yang sudah ada — tidak ada unggahan baru. Menghemat kuota ImgBB, waktu unggah, dan membuat stiker populer termuat seketika.

### O20 — Mode Pemeliharaan Terencana · P2
Upgrade VPS (migrasi database, ganti paket, dsb.) akan terjadi. Sediakan saklar admin **mode pemeliharaan**: aplikasi menampilkan pengumuman terjadwal ("pemeliharaan Minggu 02.00–03.00"), selama jendela itu aplikasi masuk mode baca-saja dari cache lokal (katalog & progres tetap bisa dilihat, aksi tulis ditahan dengan pesan jelas), dan API menolak tulis dengan kode khusus yang dipahami aplikasi. Downtime terencana berhenti terasa seperti aplikasi rusak.

### O21 — Pengingat Melanjutkan yang Cerdas · P2
Worker mendeteksi judul yang berhenti ditonton ≥7 hari padahal belum tamat, lalu mengirim **maks. satu pengingat lembut per user per minggu** ("Kamu berhenti di episode 7 — episode 12 baru saja rilis"), menghormati jam tenang (F20) dan hanya untuk judul di watchlist user. Pengingat berhenti permanen untuk judul itu setelah user melanjutkan atau mengabaikan dua kali. Retensi naik tanpa menjadi spam — kebalikan dari broadcast massal.

## Urutan penerapan yang disarankan

1. **Murah & terasa langsung**: F27 (PiP + audio), O17 (gerbang versi), O16 (halaman komik ringan).
2. **Penemuan & retensi**: F23 (Tanya Senpai), O21 (pengingat cerdas), F29 (widget).
3. **Komunitas**: F24 (polling & kuis), F25 (misi musiman).
4. **Jangka panjang**: F26 (multi-profil), F28 (penanda momen), F30 (ekspor), O18 (metrik), O19 (dedupe gambar), O20 (mode pemeliharaan).
