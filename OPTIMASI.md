# OPTIMASI.md — Optimasi Resmi SubSenpai (Gelombang Ketiga)

Dokumen **resmi** untuk optimasi O9–O15 dari REKOMENDASI.md putaran 3 (disetujui 20 Nov 2026), melengkapi optimasi yang sudah lebih dulu masuk [STRUKTUR.md](STRUKTUR.md) pada putaran 1–2 (cache katalog, Cloudflare, endpoint beranda gabungan, pagination kursor, FTS, sinkronisasi delta, prefetch, dsb.). Pendamping [DATABASE.md](DATABASE.md), [FITUR.md](FITUR.md), dan [KEAMANAN.md](KEAMANAN.md). Perubahan skema dirinci per item.

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
