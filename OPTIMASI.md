# OPTIMASI.md — Optimasi Resmi SubSenpai

Dokumen **resmi** untuk optimasi O9–O15 dari REKOMENDASI.md putaran 3, O16 dan O18–O21 dari putaran 4, dan O22–O27 dari putaran 5 (disetujui 20 Nov 2026; O17 Gerbang Versi Minimum ada di KEAMANAN.md), melengkapi optimasi yang sudah lebih dulu masuk [STRUKTUR.md](STRUKTUR.md) pada putaran 1–2 (cache katalog, Cloudflare, endpoint beranda gabungan, pagination kursor, FTS, sinkronisasi delta, prefetch, dsb.). Pendamping [DATABASE.md](database/DATABASE.md), [FITUR.md](FITUR.md), dan [KEAMANAN.md](KEAMANAN.md). Perubahan skema dirinci per item.

---

## O9 — Failover Sumber Video + Pemeriksa Kesehatan Sumber

Satu `video_url` per episode adalah titik gagal tunggal, padahal sumber hasil scraping sering mati atau berganti.

**Perubahan data**
- Tabel baru `episode_sources`: `id`, `episode_id` (FK), `url`, `label` (mis. kualitas/sumber, boleh kosong), `is_alive` (bawaan benar), `fail_count` (bawaan 0), `last_checked_at`, `created_at`. `episodes.video_url` yang lama diperlakukan sebagai sumber utama dan dimigrasikan menjadi baris pertama tabel ini.

**Cara kerja**
- Worker memeriksa kesehatan tiap sumber secara berkala (permintaan ringan HEAD/range, dengan jeda sopan agar tidak membebani sumber): sumber yang gagal berurutan ditandai `is_alive = false`.
- Endpoint pemutaran mengembalikan **daftar sumber yang sehat terurut**; player mencoba dari yang pertama dan **otomatis berpindah** ke sumber berikutnya saat pemutaran gagal, tanpa user perlu melakukan apa pun.
- Player melaporkan kegagalan putar ke backend (menambah `fail_count` sumber itu) — pemeriksaan worker dan laporan nyata user saling melengkapi.
- Monitoring admin menampilkan daftar sumber mati agar admin bisa mengganti/menambah sumber (modul Monitoring/Katalog di STRUKTUR_ADMIN.md).

## O10 — Skor Tren & Populer Dihitung Worker

- "Sedang Tren" dan "Populer Minggu Ini" **tidak** dihitung dengan agregasi berat setiap beranda dibuka. Worker menghitungnya berkala (mis. tiap jam) dari `watch_history`, `read_history`, dan `title_ratings`.
- Hasilnya disimpan di tabel agregat kecil `trending_cache` (`title_id`, `score`, `window` ∈ `day|week`, `computed_at`); endpoint beranda tinggal membaca hasil jadi.
- Bila perhitungan tertunda, beranda memakai hasil hitungan terakhir (tidak pernah kosong hanya karena worker terlambat).

## O11 — Retensi Data & Pembersihan Otomatis

Database lokal berbagi disk VPS (patokan 50 GB) dengan sistem — harus ada kebijakan buang yang berjalan sendiri:
- Worker menjalankan pembersihan berkala: sesi pembayaran kedaluwarsa, baris akses tamu lama, pesan chat yang terhapus lunak lebih dari 90 hari, token perangkat yang tidak aktif lama, dan agregasi riwayat mentah yang sudah teringkas di `daily_stats`.
- Setelah pembersihan besar, jalankan pemeliharaan PostgreSQL seperlunya (vacuum/analyze terjadwal).
- Monitoring admin memberi **peringatan pemakaian disk pada 70% dan 85%** — jauh sebelum "disk penuh" melumpuhkan layanan.

## O12 — Cadangan Inkremental (Arsip WAL) ke R2

- Melengkapi `pg_dump` malam yang sudah dirancang: aktifkan pengarsipan WAL PostgreSQL secara kontinu ke bucket R2 khusus (alat standar seperti WAL-G/pgBackRest, terkompresi), retensi arsip WAL 7 hari + cadangan dasar mingguan.
- Pemulihan bisa **point-in-time**: kehilangan data saat bencana menyusut dari maks. 24 jam menjadi beberapa menit.
- Uji pemulihan triwulanan yang sudah dirancang di STRUKTUR.md kini mencakup jalur point-in-time ini, bukan hanya jalur `pg_dump`.

## O13 — Kompresi Respons (Brotli/Gzip) di Caddy

- Caddy mengaktifkan kompresi (zstd/gzip) untuk respons yang melintas: manfaat terbesar pada konten tidak terenkripsi yang besar (berkas subtitle, aset, respons publik), dan tetap ada susut sebagian pada payload `xdata` (base64 masih bisa dimampatkan walau rasionya terbatas — dicatat apa adanya).
- Aset statis & berkas subtitle dari R2/CDN diberi header cache yang benar agar tidak diunduh berulang oleh aplikasi.

## O14 — Pencarian Lokal Instan di Perangkat

- Aplikasi sudah menyimpan katalog lokal dari sinkronisasi delta — pencarian judul berjalan **di perangkat** lebih dulu (instan, tetap bekerja tanpa sinyal), memakai indeks judul + judul alternatif pada basis data lokal.
- Pencarian server (FTS PostgreSQL) dipakai bila pencarian lokal tidak memberi hasil atau untuk pencarian mendalam (deskripsi/genre) — hasil server menggantikan hasil lokal untuk kueri itu.

## O15 — Poster Multi-Ukuran Saat Unggah

- Saat admin menambahkan/mengganti poster, worker membuat **tiga varian WebP** di R2: kecil (±240px untuk daftar), sedang (±480px untuk kartu), besar (±960px untuk halaman detail). `titles.poster_url` menunjuk varian sedang; varian lain mengikuti konvensi nama berkas yang sama.
- Aplikasi meminta ukuran sesuai tempat tampil — halaman daftar berisi puluhan poster tidak lagi mengunduh gambar ukuran detail. Melengkapi aturan WebP yang sudah ada dan menjadi pasangan Mode Hemat Data (F19).

---

# Gelombang Keempat (O16, O18–O21)

## O16 — Halaman Komik Terkompresi Multi-Varian

- Aset terberat yang kita simpan adalah halaman komik di R2. Saat chapter ditambahkan, worker membuat **varian baca terkompresi** (WebP/AVIF, lebar standar baca ±1080px) di samping berkas asli.
- Reader memuat varian ringan per halaman secara bawaan; resolusi penuh hanya diambil saat user memperbesar halaman itu.
- Pasangan O15 (poster multi-ukuran) untuk aset yang volumenya jauh lebih besar.

**Perubahan data**
- `chapter_pages.light_url` (boleh kosong) — URL varian ringan; kosong berarti varian belum dibuat dan reader memakai `image_url` seperti biasa.

## O18 — Metrik & Log Terstruktur untuk Monitoring

- API & worker menulis **log JSON terstruktur** (id permintaan, kelompok endpoint, latensi, status, id user teranonimkan bila perlu) agar masalah bisa ditelusuri tanpa membaca log teks bebas.
- Endpoint metrik internal (hanya bisa diakses dari localhost/monitoring) memaparkan: latensi API per kelompok endpoint, kedalaman antrean subtitle, tingkat kegagalan sumber video (O9), dan tingkat keberhasilan polling pembayaran GoPay.
- Ringkasan metrik disimpan per jam ke tabel `metrics_hourly` (`kind`, `bucket` jam, `value`) dan ditampilkan sebagai grafik sederhana di modul Monitoring admin — masalah terlihat sebelum user melapor. Melengkapi `/healthz` yang sudah ada (yang hanya menjawab hidup/mati).

## O19 — Dedupe Unggahan Gambar via Hash

- Gambar/stiker yang sama sering diunggah berulang oleh banyak user. Sebelum mengunggah ke ImgBB, backend menghitung **hash isi gambar** (setelah kompresi baku kita); bila hash itu sudah dikenal, backend memakai URL ImgBB yang sudah tersimpan — tidak ada unggahan baru.
- Efeknya: kuota ImgBB hemat, stiker populer termuat seketika, dan penghapusan moderasi cukup dilakukan sekali per gambar unik.

**Perubahan data**
- Tabel baru `image_uploads`: `hash` (unik), `imgbb_url`, `thumb_url`, `delete_url` (server saja), `created_by`, `created_at` — registri gambar unik hasil unggahan user.

## O20 — Mode Pemeliharaan Terencana

- Admin mengatur jendela pemeliharaan dari pengaturan platform (waktu mulai–selesai + pesan pengumuman).
- Menjelang jendela, aplikasi menampilkan pengumuman; selama jendela berlangsung aplikasi masuk **mode baca-saja dari cache lokal** (katalog & progres tetap bisa dilihat; aksi tulis ditahan dengan penjelasan), dan API menolak permintaan tulis dengan kode khusus `MAINTENANCE` yang dipahami aplikasi — sementara login dan baca tetap berjalan untuk kebutuhan admin.
- Downtime terencana (migrasi database, ganti paket VPS) berhenti terasa seperti aplikasi rusak.

## O21 — Pengingat Melanjutkan yang Cerdas

- Worker memeriksa harian: judul di watchlist user yang terakhir ditonton ≥7 hari lalu, belum tamat, **dan** punya episode/chapter baru sejak user berhenti — hanya kombinasi itu yang layak diingatkan.
- Maks. **satu pengingat per user per minggu**, dikirim sebagai notifikasi rilis biasa, menghormati jam tenang (F20).
- Pengingat untuk suatu judul berhenti permanen setelah user melanjutkannya, atau setelah dua kali diabaikan.

**Perubahan data**
- `watchlist.last_nudge_at` (boleh kosong) dan `watchlist.nudge_ignored_count` (bawaan 0) — penanda agar aturan di atas bisa ditegakkan.

---

# Gelombang Kelima (O22–O27)

## O22 — Partisi Tabel Besar per Bulan

- Tabel yang tumbuh tanpa batas di database lokal VPS — `chat_messages`, `watch_history`, `read_history`, `notifications` — dijadikan **tabel terpartisi per bulan** (partisi rentang bawaan PostgreSQL pada kolom waktu buat).
- Manfaatnya langsung: kueri data terbaru hanya menyentuh partisi bulan berjalan; pembersihan retensi (O11) berubah dari DELETE besar menjadi **melepas partisi lama** (seketika, tanpa bloat); ukuran cadangan per partisi lebih terkendali.
- Partisi bulan berikutnya dibuat otomatis oleh worker (tugas terjadwal), dan pekerjaan ini dilakukan **selagi data masih kecil** — mempartisi tabel raksasa yang sudah terlanjur besar jauh lebih menyakitkan.

## O23 — Disiplin Migrasi Berversi + Uji Kontrak API

- Semua perubahan skema hanya boleh lewat **migrasi berversi** (alat standar seperti goose) yang bisa maju dan mundur; tidak ada perubahan skema manual di database produksi.
- **Uji kontrak API** berjalan di CI: bentuk respons endpoint yang sudah dipakai aplikasi versi yang masih didukung tidak boleh berubah tanpa kenaikan versi API — mencegah kelas bug "aplikasi lama rusak karena skema/respons berubah diam-diam". Pasangan teknis dari gerbang versi (O17).

## O24 — Pra-Saring Audio (VAD) Sebelum Transkripsi

> **Status: DITANGGUHKAN sejak F40 (2026-11-22).** Jalur utama subtitle kini sumber siap pakai per bahasa (F40) — tidak ada transkripsi, jadi pra-saring ini tidak terpakai. Isi di bawah dipertahankan hanya bila jalur transkripsi dihidupkan lagi kelak.

Biaya variabel terbesar SubSenpai adalah menit transkripsi Whisper; jangan membayar menit yang tidak berisi dialog:
- Sebelum audio dikirim ke API transkripsi, worker menjalankan **deteksi aktivitas suara (VAD)**: segmen hening panjang dipangkas, dan rentang intro/outro yang sudah ditandai (F10) tidak ditranskripsikan sebagai dialog.
- Yang dikirim hanya segmen berisi ucapan, **beserta peta offset waktunya** — sehingga timestamp hasil transkripsi dipetakan kembali ke garis waktu asli episode dan file SRT akhir tetap tepat.
- Penghematannya terukur per episode (menit terkirim vs menit asli dicatat di metrik O18) dan tidak mengubah kualitas subtitle dialog.

## O25 — Digest Notifikasi Rilis per Judul

- Bila satu judul merilis banyak episode/chapter sekaligus (rilis borongan), worker **menggabungkannya menjadi satu notifikasi push per judul** dalam jendela penggabungan singkat ("5 episode baru dari X") — bukan 5–10 push terpisah.
- Kotak masuk `notifications` tetap mencatat tiap rilis satu per satu; yang digabung hanya push-nya. Aturan ini melengkapi jam tenang (F20) dan pengingat cerdas (O21) sebagai lapis disiplin notifikasi ketiga.

## O26 — Load Shedding Anggun Saat VPS Sibuk

- Saat beban VPS melewati ambang (dari metrik O18), API **melepas beban tidak kritis lebih dulu**: rekomendasi & tren dilayani dari cache walau agak basi, tugas polling/statistik worker ditunda, dan endpoint tidak kritis bisa menjawab "coba lagi nanti" yang dipahami aplikasi.
- Jalur kritis selalu didahulukan: login, pemutaran/bacaan, pembayaran, dan chat. Di jam ramai layanan terasa sedikit lebih lambat — bukan mati.

## O27 — Peluncuran APK Bertahap via Gerbang Versi

- Karena distribusi lewat website resmi & GitHub (lihat NOTE.md), rilis APK baru diluncurkan **bertahap**: pemeriksaan pembaruan di aplikasi menerima versi baru hanya untuk sebagian user dulu (persentase diatur admin dari pengaturan platform; penentuan kelompok bersifat deterministik dari id perangkat).
- Selama tahap awal, tingkat crash (O6) dan metrik (O18) dipantau; sehat → persentase dinaikkan sampai 100%; bermasalah → penawaran dihentikan sebelum semua user terkena, dan gerbang versi minimum (O17) tidak ikut dinaikkan sampai rilis dinyatakan stabil.
- Tanpa infrastruktur baru — murni memakai mekanisme pemeriksaan pembaruan & pengaturan platform yang sudah dirancang.
