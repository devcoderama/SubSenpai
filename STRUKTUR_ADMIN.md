# STRUKTUR_ADMIN.md — Panel Admin SubSenpai

Rancangan panel admin untuk **mengelola** (katalog, subtitle, user, langganan) dan **memantau** (antrean worker, kesehatan layanan, aktivitas) SubSenpai. Panel ini adalah halaman frontend Next.js di route `/admin` (bukan aplikasi terpisah); seluruh datanya dilayani **backend Go** lewat endpoint admin `/api/v1/admin/*` dengan cek role di backend. Skema database pendukung ada di [DATABASE.md](DATABASE.md); struktur aplikasi utama di [STRUKTUR.md](STRUKTUR.md).

## 1. Akses & Keamanan

- **Siapa admin**: kolom `role` di tabel `profiles` (`user` / `admin`). Login tetap **wajib Google OAuth**, sama seperti user biasa — yang membedakan hanya role-nya.
- **Cek di server, selalu**: setiap request ke `/admin` dan API admin memverifikasi sesi + `role = 'admin'` di server (middleware + cek ulang di API). Menyembunyikan menu di UI BUKAN pengaman.
- **Tulis lewat service role**: operasi admin (ubah katalog, status premium, retry subtitle) dijalankan API server dengan service role key Supabase — key tidak pernah dikirim ke browser.
- **Jejak audit**: setiap aksi penting admin dicatat ke tabel `admin_logs` (siapa, aksi apa, target apa, kapan).
- **Akun admin pertama**: ditetapkan manual sekali dari Supabase Dashboard (ubah `role` user pemilik menjadi `admin`); setelah itu admin bisa mengangkat admin lain dari panel.

## 2. Struktur Folder

```
apps/web/
├── app/
│   └── admin/                  # Panel admin (route /admin)
│       ├── page.tsx            # Dashboard ringkasan
│       ├── titles/             # Kelola katalog (judul, episode, chapter)
│       ├── subtitles/          # Antrean & hasil subtitle
│       ├── users/              # Kelola user & premium manual
│       ├── subscriptions/      # Langganan & transaksi
│       ├── vouchers/           # Voucher premium (buat, bagikan, pantau klaim)
│       ├── chat/               # Moderasi Chat Global (pesan, kanal, stiker)
│       ├── reports/            # Laporan user (link mati, subtitle salah)
│       ├── monitoring/         # Kesehatan sistem & metrik
│       └── settings/           # Pengaturan platform
├── components/admin/           # Komponen UI panel (tabel, form, kartu metrik)
└── lib/admin.ts                # Helper UI: klien API admin, format metrik

API admin TIDAK berada di folder frontend — endpoint-nya hidup di backend Go
(backend/internal/httpapi, grup /api/v1/admin/*) dan semua cek role,
penulisan service-role, audit log, dan agregasi metrik terjadi di sana.
```

## 3. Modul Panel

### 3.1 Dashboard
- Kartu ringkasan: total judul per tipe, total episode & chapter, user terdaftar, premium aktif.
- Status antrean subtitle sekilas: `queued` / `processing` / `failed` hari ini.
- Aktivitas terakhir: user baru, langganan baru, laporan belum ditangani.

### 3.2 Manajemen Katalog
- CRUD `titles` (judul, tipe, genre, poster, status tayang).
- Kelola `episodes`: tambah/edit URL HLS, nomor episode, tanggal tayang.
- Kelola `chapters` + `chapter_pages`: unggah halaman komik (gambar dikirim ke R2, urutan halaman bisa disusun ulang).
- Impor massal metadata dari CSV/JSON untuk mempercepat pengisian katalog awal.
- Poster diunggah ke R2 dari panel (tidak perlu URL manual).

### 3.3 Manajemen Subtitle
- Daftar antrean berdasarkan status (`queued` / `processing` / `done` / `failed`) + alasan gagal.
- **Retry** subtitle yang gagal, per episode atau massal.
- Unggah SRT manual (menimpa hasil mesin) & hapus subtitle yang buruk.
- Picu pembuatan subtitle untuk episode tertentu di luar antrean otomatis.

### 3.4 Manajemen User
- Daftar user (dari `profiles` + Supabase Auth): email, tanggal daftar, status premium.
- Beri/cabut premium manual (mis. kompensasi, promo) — tercatat di `admin_logs`.
- Blokir user bermasalah (ban di Supabase Auth).

### 3.5 Langganan & Pembayaran
- Daftar `subscriptions`: plan, status, masa berlaku, provider (Midtrans/Xendit) + referensi transaksi.
- Lacak webhook pembayaran yang gagal diproses untuk direkonsiliasi manual.
- Sesi pembayaran GoPay (jalur polling): daftar sesi `pending`/`expired` yang belum cocok dengan transaksi masuk — rekonsiliasi manual dengan mencocokkan nominal unik ke mutasi merchant, lalu tandai lunas bila terbukti dibayar.

### 3.6 Voucher Premium
- **Buat voucher promo**: admin menerbitkan kode — satu kode khusus (mis. `SENPAI2026`) atau batch kode acak — dengan durasi premium (hari), batas waktu penukaran, dan kuota sesuai jumlah kode.
- **Bagikan ke user**: voucher bisa ditujukan ke user tertentu (`assigned_to`) atau dibagikan terbuka; user menukarnya dari halaman penukaran kode setelah login.
- **Pantau klaim**: daftar voucher per status (`active` / `redeemed` / `expired` / `revoked`), siapa menukar dan kapan; admin bisa **mencabut** (revoke) voucher yang bocor/disalahgunakan.
- **Voucher pembelian user**: user juga bisa MEMBELI voucher dari halaman premium untuk dihadiahkan ke user lain — pembayaran lewat Midtrans/Xendit, kode terbit otomatis setelah pembayaran terkonfirmasi. Admin memantau penjualan & penukarannya di modul ini (tidak bisa mengubah kode milik user, hanya mencabut bila ada sengketa/penipuan).
- Penukaran bersifat sekali pakai dan atomik di API: premium penerima diperpanjang sejumlah `duration_days` (menumpuk dari sisa premium bila masih aktif) dan tercatat sebagai `subscriptions` berprovider `voucher`.

### 3.7 Laporan User
- User bisa melaporkan: link video mati, subtitle salah/tidak sinkron, chapter/halaman rusak (tabel `reports`).
- Admin menindaklanjuti: perbaiki sumbernya, lalu tandai laporan `resolved` / `dismissed`.

### 3.8 Monitoring
- **Antrean worker**: jumlah per status + episode yang paling lama menunggu; lonjakan `failed` = tanda sumber/API transkripsi bermasalah.
- **Aktivitas platform** (agregasi dari `watch_history`, `read_history`, `guest_access`): tontonan/bacaan per hari, judul terlaris, rasio tamu yang mentok batas 1 episode (calon user login).
- **Kesehatan layanan luar**: tautan cepat ke status Supabase, Vercel, Cloudflare, dan API transkripsi; sisa kuota/kuota terpakai dicatat manual di halaman ini.
- **Penyimpanan**: perkiraan pemakaian R2 (poster + halaman komik + SRT) terhadap batas gratis 10 GB.
- Catatan: ambang sederhana (mis. subtitle `failed` > 10 dalam sehari) bisa dikembangkan menjadi notifikasi otomatis ke Telegram/email admin.

### 3.9 Pengaturan
- Harga & durasi paket premium.
- Batas akses tamu (bawaan: 1 episode/chapter).
- Saklar iklan (tampil/sembunyi untuk user gratis) & mode maintenance.
- Metode pembayaran yang aktif (Midtrans / Xendit / GoPay pihak ketiga) — jalur GoPay bisa dimatikan cepat dari sini bila API-nya bermasalah.

### 3.10 Moderasi Chat Global
- Pesan terlaporkan (dari `reports` bertarget `chat_message`): hapus pesan (soft-delete), bisukan pengirimnya (`chat_muted_until`, bisa berdurasi), atau abaikan laporan.
- Kelola kanal chat: tambah/ubah nama, deskripsi, dan urutan kanal; atur teks banner pengumuman di atas chat.
- Kelola katalog stiker: unggah paket stiker (gambar ke R2) dan tandai mana yang khusus premium.
- Atur filter kata terlarang dan mode lambat (jeda minimum antar-pesan) per kanal bila chat sedang rusuh.

## 4. Tabel Pendukung

Panel admin memakai tabel yang sama dengan aplikasi utama, ditambah:

| Tabel | Fungsi |
| --- | --- |
| `profiles.role` | Penanda admin (`user` / `admin`) |
| `reports` | Laporan user soal konten rusak — ditindaklanjuti admin |
| `admin_logs` | Jejak audit aksi admin |
| `vouchers` | Kode voucher premium (promo admin / dibeli user sebagai hadiah) — detail di DATABASE.md |

Detail kolom, relasi, dan SQL-nya ada di [DATABASE.md](DATABASE.md).

## 5. Contoh Alur

**Subtitle gagal diproses** — Dashboard menunjukkan `failed` naik → buka modul Subtitle → lihat alasan (mis. API transkripsi timeout) → perbaiki penyebabnya → Retry → status kembali `queued` → worker memproses ulang → admin memeriksa hasilnya `done`.

**Laporan link mati** — User melaporkan episode → laporan masuk modul Laporan → admin membuka episode-nya, mengganti `video_url` di modul Katalog → tandai laporan `resolved` (tercatat di `admin_logs`).

**Memberi premium manual** — Modul User → pilih user → "Jadikan premium" + durasi → `subscriptions` dibuat berprovider `manual` dan `profiles.is_premium` diperbarui → tercatat di `admin_logs`.

**Voucher promo** — Modul Voucher → admin membuat kode promo 30 hari untuk 100 penukar pertama → kode dibagikan → user login dan menukar kode di halaman penukaran → voucher berubah `redeemed`, premium user bertambah 30 hari, dan admin memantau sisa kuota dari modul yang sama.

**Voucher hadiah antar-user** — User A membeli voucher 30 hari di halaman premium (bayar QRIS) → menerima kode → memberikannya ke User B → B menukar kode → premium B aktif; A bisa melihat status vouchernya (sudah ditukar/belum) dari halaman premium.
