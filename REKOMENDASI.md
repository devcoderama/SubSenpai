# REKOMENDASI.md — Rekomendasi Fitur & Optimasi SubSenpai

Daftar rekomendasi dari sisi perancangan (belum keputusan — tandai yang disetujui untuk dipindah ke STRUKTUR.md / roadmap README). Prioritas: **P1** = sebaiknya sejak awal/awal rilis, **P2** = setelah rilis & ada user, **P3** = pengembangan lanjutan. Estimasi usaha: K (kecil), S (sedang), B (besar).

## A. Optimasi Teknis

| # | Rekomendasi | Alasan | Prioritas / Usaha |
| --- | --- | --- | --- |
| A1 | **Cache katalog di backend Go** — endpoint beranda & telusur di-cache di memori (TTL 1–5 menit, invalidasi saat admin mengubah katalog) | Katalog adalah bacaan terbanyak dan jarang berubah; di VPS 1 core, cache memangkas sebagian besar query Postgres | P1 / K |
| A2 | **Cloudflare di depan domain API** — DNS + cache edge untuk response publik & proteksi DDoS dasar (gratis) | Mengurangi beban VPS dan melindungi dari lonjakan/serangan sederhana | P1 / K |
| A3 | **Pencarian full-text PostgreSQL** (`tsvector` pada judul + judul alternatif, indeks GIN) | Pencarian `ILIKE %kata%` tidak memakai indeks dan makin lambat saat katalog besar; FTS cepat dan paham peringkat relevansi | P1 / S |
| A4 | **Pagination berbasis cursor** untuk katalog & riwayat chat (bukan `OFFSET`) | `OFFSET` besar makin lambat dan bisa menggeser hasil; cursor stabil & konstan | P1 / K |
| A5 | **Endpoint gabungan beranda** — satu request mengembalikan seluruh isi layar beranda (sorotan, per tipe, lanjutkan menonton/baca) | Mengurangi jumlah request per buka aplikasi (tiap request = dekripsi + cek sesi) dan membuat aplikasi terasa lebih cepat | P1 / K |
| A6 | **Poster & halaman komik versi WebP + thumbnail** saat admin mengunggah ke R2 | Ukuran turun 30–60% dari JPEG/PNG — hemat kuota user dan mempercepat muat daftar | P1 / S |
| A7 | **Rate limit khusus endpoint handshake** (`/api/v1/session`) per IP | Operasi ECDH memakan CPU; tanpa batas laju, endpoint ini bisa dipakai menguras CPU VPS kecil | P1 / K |
| A8 | **Backup berkala dari VPS** — `pg_dump` terjadwal (cron systemd) diunggah ke R2 | Supabase free tidak punya point-in-time recovery; dump kecil ini penyelamat bila project ter-pause/terhapus | P1 / K |
| A9 | **Antrean subtitle berprioritas** — episode dari judul yang sedang banyak ditonton & episode baru didahulukan; episode baru langsung dibuatkan subtitle saat ditambahkan admin (tidak menunggu diminta user) | User pertama yang membuka episode baru tidak menunggu proses; sumber daya worker dipakai untuk yang paling berdampak | P2 / S |
| A10 | **Health check + pemantau uptime eksternal** (mis. UptimeRobot gratis) ke endpoint `/healthz` backend | Satu VPS = satu titik gagal; pemantau memberi tahu admin dalam hitungan menit, bukan dari keluhan user | P1 / K |
| A11 | **Statistik tontonan untuk admin** — agregasi berkala (harian) dari `watch_history` ke tabel ringkasan | Dashboard admin tidak membebani tabel riwayat mentah yang besar; tren judul terlaris terbaca cepat | P2 / S |

## B. Fitur

| # | Rekomendasi | Alasan | Prioritas / Usaha |
| --- | --- | --- | --- |
| B1 | **Notifikasi episode/chapter baru (FCM)** — user diberi tahu saat judul di watchlist-nya rilis episode/chapter baru | Pengungkit retensi terbesar untuk aplikasi streaming; FCM gratis | P1 / S |
| B2 | **Komentar per episode & chapter** — kolom komentar di halaman nonton/baca, bisa dilaporkan seperti pesan chat | Komunitas terbentuk di sekitar konten, bukan hanya di Chat Global; menaikkan waktu pakai | P2 / S |
| B3 | **Rating & ulasan user** — bintang 1–10 per judul; `titles.rating` menjadi agregatnya | Membantu user memilih tontonan dan membuat katalog terasa hidup | P2 / S |
| B4 | **Rekomendasi "karena kamu menonton ini"** — judul segenre + sama-sama ditonton user lain, dihitung sederhana dari riwayat | Tanpa ML berat; cukup query agregat berkala di worker | P2 / S |
| B5 | **Referral premium** — user mengundang teman; pendaftar baru & pengundang masing-masing dapat hari premium (diterapkan lewat sistem voucher yang sudah ada) | Pertumbuhan organik murah; menyatu dengan fitur voucher — tidak perlu sistem baru | P2 / S |
| B6 | **Download untuk ditonton offline (khusus premium)** — unduh segmen HLS + SRT ke perangkat, berkas dienkripsi kunci perangkat | Nilai jual premium terkuat untuk user Indonesia (hemat kuota, sinyal tidak stabil); catatan: hanya untuk sumber yang segmennya bisa diunduh | P2 / B |
| B7 | **Level & lencana aktivitas** — poin dari menonton, membaca, dan chat; level & lencana tampil di profil dan di samping nama pada chat (seperti contoh aplikasi referensi) | Gamifikasi ringan yang membuat profil terasa "milik sendiri" dan mendorong kebiasaan harian | P3 / S |
| B8 | **Mode baca vertikal (gulir webtoon)** untuk manhwa/manhua + unduh chapter | Format asli manhwa adalah gulir vertikal; mode halaman saja terasa salah untuk genre itu | P2 / S |
| B9 | **Daftar "lanjutkan" yang menonjol di beranda** — sudah dimungkinkan `watch_history`/`read_history`; pastikan jadi blok pertama beranda | Mengurangi gesekan kembali menonton — kebiasaan yang membuat user balik setiap hari | P1 / K |
| B10 | **Mode anak / filter genre dewasa** di pengaturan profil | Katalog anime/donghua bisa memuat judul 18+; filter sederhana melindungi platform & user | P3 / K |

## C. Urutan Penerapan yang Disarankan

1. **Sebelum rilis**: A1, A2, A4, A5, A7, A8, A10 (fondasi murah yang mahal bila ditambah belakangan) + B9.
2. **Rilis awal**: A3, A6, B1.
3. **Setelah ada user aktif**: A9, A11, B2, B3, B4, B5, B8.
4. **Fase lanjutan**: B6, B7, B10.

Item yang disetujui cukup dicentang di sini, lalu dipindahkan ke dokumen rancangan terkait (STRUKTUR.md / DATABASE.md / STRUKTUR_ADMIN.md) pada commit berikutnya.
