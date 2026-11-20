# STRUKTUR_ADMIN.md — Panel Admin SubSenpai

Rancangan panel admin untuk **mengelola** (katalog, subtitle, user, langganan) dan **memantau** (antrean worker, kesehatan layanan, aktivitas) SubSenpai. Panel ini adalah bagian dari aplikasi Next.js yang sama, terpisah di route `/admin` — bukan aplikasi terpisah. Skema database pendukung ada di [DATABASE.md](DATABASE.md); struktur aplikasi utama di [STRUKTUR.md](STRUKTUR.md).

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
│       ├── reports/            # Laporan user (link mati, subtitle salah)
│       ├── monitoring/         # Kesehatan sistem & metrik
│       └── settings/           # Pengaturan platform
├── app/api/admin/              # API khusus admin (semua cek role di server)
├── components/admin/           # Komponen UI panel (tabel, form, kartu metrik)
└── lib/admin.ts                # Helper: guard role, audit log, agregasi metrik
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

### 3.6 Laporan User
- User bisa melaporkan: link video mati, subtitle salah/tidak sinkron, chapter/halaman rusak (tabel `reports`).
- Admin menindaklanjuti: perbaiki sumbernya, lalu tandai laporan `resolved` / `dismissed`.

### 3.7 Monitoring
- **Antrean worker**: jumlah per status + episode yang paling lama menunggu; lonjakan `failed` = tanda sumber/API transkripsi bermasalah.
- **Aktivitas platform** (agregasi dari `watch_history`, `read_history`, `guest_access`): tontonan/bacaan per hari, judul terlaris, rasio tamu yang mentok batas 1 episode (calon user login).
- **Kesehatan layanan luar**: tautan cepat ke status Supabase, Vercel, Cloudflare, dan API transkripsi; sisa kuota/kuota terpakai dicatat manual di halaman ini.
- **Penyimpanan**: perkiraan pemakaian R2 (poster + halaman komik + SRT) terhadap batas gratis 10 GB.
- Catatan: ambang sederhana (mis. subtitle `failed` > 10 dalam sehari) bisa dikembangkan menjadi notifikasi otomatis ke Telegram/email admin.

### 3.8 Pengaturan
- Harga & durasi paket premium.
- Batas akses tamu (bawaan: 1 episode/chapter).
- Saklar iklan (tampil/sembunyi untuk user gratis) & mode maintenance.

## 4. Tabel Pendukung

Panel admin memakai tabel yang sama dengan aplikasi utama, ditambah:

| Tabel | Fungsi |
| --- | --- |
| `profiles.role` | Penanda admin (`user` / `admin`) |
| `reports` | Laporan user soal konten rusak — ditindaklanjuti admin |
| `admin_logs` | Jejak audit aksi admin |

Detail kolom, relasi, dan SQL-nya ada di [DATABASE.md](DATABASE.md).

## 5. Contoh Alur

**Subtitle gagal diproses** — Dashboard menunjukkan `failed` naik → buka modul Subtitle → lihat alasan (mis. API transkripsi timeout) → perbaiki penyebabnya → Retry → status kembali `queued` → worker memproses ulang → admin memeriksa hasilnya `done`.

**Laporan link mati** — User melaporkan episode → laporan masuk modul Laporan → admin membuka episode-nya, mengganti `video_url` di modul Katalog → tandai laporan `resolved` (tercatat di `admin_logs`).

**Memberi premium manual** — Modul User → pilih user → "Jadikan premium" + durasi → `subscriptions` dibuat berprovider `manual` dan `profiles.is_premium` diperbarui → tercatat di `admin_logs`.
