# STRUKTUR_ADMIN.md — Panel Admin SubSenpai

Rancangan panel admin untuk **mengelola** (katalog, subtitle, user, langganan) dan **memantau** (antrean worker, kesehatan layanan, aktivitas) SubSenpai. Panel ini adalah **layar admin di dalam aplikasi Android yang sama** (bukan aplikasi terpisah; menu admin hanya muncul untuk role `admin`); seluruh datanya dilayani **backend Go** lewat endpoint admin `/api/v1/admin/*` dengan cek role di backend. Skema database pendukung ada di [DATABASE.md](DATABASE.md); struktur aplikasi utama di [STRUKTUR.md](STRUKTUR.md).

## 1. Akses & Keamanan

- **Siapa admin**: kolom `role` di tabel `profiles` (`user` / `admin`). Login tetap **wajib Google OAuth**, sama seperti user biasa — yang membedakan hanya role-nya.
- **Cek di server, selalu**: setiap request ke `/admin` dan API admin memverifikasi sesi + `role = 'admin'` di server (middleware + cek ulang di API). Menyembunyikan menu di UI BUKAN pengaman.
- **Tulis lewat backend**: operasi admin (ubah katalog, status premium, retry subtitle) dijalankan backend Go — satu-satunya pemegang akses database (PostgreSQL lokal hanya mendengarkan localhost); aplikasi tidak pernah menyentuh database secara langsung.
- **Jejak audit**: setiap aksi penting admin dicatat ke tabel `admin_logs` (siapa, aksi apa, target apa, kapan).
- **Akun admin pertama**: ditetapkan manual sekali langsung di database VPS (ubah `role` user pemilik menjadi `admin` lewat SQL); setelah itu admin bisa mengangkat admin lain dari panel.

## 2. Struktur Folder

```
apps/android/.../ui/admin/      # Layar admin di aplikasi (satu layar per modul)
├── DashboardScreen             # Ringkasan
├── TitlesScreen                # Kelola katalog (judul, episode, chapter, permintaan judul)
├── SubtitlesScreen             # Antrean & hasil subtitle
├── UsersScreen                 # Kelola user, premium manual & perangkat akun
├── SubscriptionsScreen         # Langganan & transaksi
├── VouchersScreen              # Voucher premium (buat, bagikan, pantau klaim)
├── ChatModerationScreen        # Moderasi Chat Global (pesan, kanal, stiker)
├── ReportsScreen               # Laporan user (link mati, subtitle salah)
├── MonitoringScreen            # Kesehatan sistem & metrik
├── NotificationsScreen         # Broadcast notifikasi manual (promo/event)
└── SettingsScreen              # Pengaturan platform

API admin hidup di backend Go (backend/internal/httpapi, grup /api/v1/admin/*) —
semua cek role, penulisan privileged, audit log, dan agregasi metrik terjadi di sana.
```

## 3. Modul Panel

### 3.1 Dashboard
- Kartu ringkasan: total judul per tipe, total episode & chapter, user terdaftar, premium aktif.
- Status antrean subtitle sekilas: `queued` / `processing` / `failed` hari ini.
- Aktivitas terakhir: user baru, langganan baru, laporan belum ditangani.

### 3.2 Manajemen Katalog
- CRUD `titles` (judul, tipe, genre, poster, status tayang, tanda konten 18+ `is_adult`).
- Kelola `episodes`: tambah/edit URL HLS, nomor episode, tanggal tayang.
- Kelola `chapters` + `chapter_pages`: unggah halaman komik (gambar dikirim ke R2, urutan halaman bisa disusun ulang).
- Impor massal metadata dari CSV/JSON untuk mempercepat pengisian katalog awal.
- **Penanda intro/outro**: tiap episode bisa diberi rentang intro & outro (detik mulai–selesai) untuk tombol Lewati Opening/Outro di player.
- **Tab Permintaan Judul**: daftar `title_requests` terurut berdasar jumlah suara (`title_request_votes`) — admin menyetujui, menolak, atau menandai `added` setelah judulnya dibuat (peminta & pendukung otomatis ternotifikasi lewat sistem notifikasi).
- Poster diunggah ke R2 dari panel (tidak perlu URL manual).

### 3.3 Manajemen Subtitle
- Daftar antrean berdasarkan status (`queued` / `processing` / `done` / `failed`) + alasan gagal.
- **Retry** subtitle yang gagal, per episode atau massal.
- Unggah SRT manual (menimpa hasil mesin) & hapus subtitle yang buruk.
- Picu pembuatan subtitle untuk episode tertentu di luar antrean otomatis.

### 3.4 Manajemen User
- Daftar user (dari `profiles`): email, tanggal daftar, status premium.
- Beri/cabut premium manual (mis. kompensasi, promo) — tercatat di `admin_logs`.
- Blokir user bermasalah: tandai `profiles.is_banned` — sesi aktifnya dicabut oleh backend dan login berikutnya ditolak.
- Perangkat akun (`user_devices`): lihat jumlah perangkat aktif seorang user dan cabut perangkat dari panel bila ada laporan akun dipakai orang lain — user juga bisa mencabut perangkatnya sendiri dari halaman keamanan akun.

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
- **Hadiah langsung ke teman**: pembelian voucher yang penerimanya dipilih dari daftar teman dipantau di modul ini juga — voucher terbit ber-`assigned_to` penerima dan terkirim otomatis lewat notifikasi + kotak masuk, tanpa kode berpindah tangan.

### 3.7 Laporan User
- User bisa melaporkan: link video mati, subtitle salah/tidak sinkron, chapter/halaman rusak (tabel `reports`).
- Admin menindaklanjuti: perbaiki sumbernya, lalu tandai laporan `resolved` / `dismissed`.

### 3.8 Monitoring
- **Antrean worker**: jumlah per status + episode yang paling lama menunggu; lonjakan `failed` = tanda sumber/API transkripsi bermasalah.
- **Aktivitas platform**: dibaca dari tabel agregat `daily_stats` (dihitung worker tiap malam dari `watch_history`, `read_history`, `guest_access` agar dashboard tetap ringan): tontonan/bacaan per hari, judul terlaris, rasio tamu yang mentok batas 1 episode (calon user login).
- **Kesehatan layanan**: status VPS & PostgreSQL lokal (ukuran database berjalan, sisa disk), tautan cepat ke status Cloudflare dan API transkripsi; sisa kuota/kuota terpakai dicatat manual di halaman ini.
- **Penyimpanan**: perkiraan pemakaian R2 (poster + halaman komik + SRT) terhadap batas gratis 10 GB.
- Catatan: ambang sederhana (mis. subtitle `failed` > 10 dalam sehari) bisa dikembangkan menjadi notifikasi otomatis ke Telegram/email admin.
- **Laporan crash aplikasi**: ringkasan/tautan ke Crashlytics/GlitchTip (jumlah crash per versi rilis) — lonjakan crash setelah sebuah rilis adalah tanda harus segera diperbaiki/rollback.
- **Cadangan**: status dump `pg_dump` malam terakhir + tanggal **uji pemulihan** terakhir (uji berkala tiap 3 bulan: dump dipulihkan ke database kosong, hasilnya dicatat di sini).

### 3.9 Pengaturan
- Harga & durasi paket premium.
- Batas akses tamu (bawaan: 1 episode/chapter).
- Saklar iklan (tampil/sembunyi untuk user gratis) & mode maintenance.
- Metode pembayaran yang aktif (Midtrans / Xendit / GoPay pihak ketiga) — jalur GoPay bisa dimatikan cepat dari sini bila API-nya bermasalah.

### 3.10 Moderasi Chat Global
- Pesan terlaporkan (dari `reports` bertarget `chat_message`): hapus pesan (soft-delete), bisukan pengirimnya (`chat_muted_until`, bisa berdurasi), atau abaikan laporan.
- Kelola kanal chat: tambah/ubah nama, deskripsi, dan urutan kanal; atur teks banner pengumuman di atas chat.
- Kelola katalog stiker: unggah paket stiker (gambar ke R2) dan tandai mana yang khusus premium.
- Atur filter kata terlarang, **filter tautan luar** (pola URL yang otomatis ditolak server di chat & komentar — berlaku juga untuk promosi berkedok teks; domain resmi aplikasi dikecualikan karena dipakai kartu berbagi), dan mode lambat (jeda minimum antar-pesan) per kanal bila chat sedang rusuh.
- Grup terlaporkan (`reports` bertarget `group`): periksa grupnya — beri peringatan ke owner, batasi, atau bubarkan grup yang melanggar. Urusan sehari-hari di dalam grup (keluarkan anggota, hapus pesan) adalah wewenang admin grup, bukan admin platform.
- Tautan undangan grup yang disalahgunakan (mis. grup dilaporkan karena tautannya menyebar ke komunitas yang salah): admin platform bisa memaksa kode undangan grup diatur ulang atau penerimaan lewat tautan dimatikan dari sisi server.
- Gambar/stiker kiriman user yang dilaporkan: hapus gambarnya dari ImgBB memakai `delete_url` yang tersimpan di server, lalu soft-delete pesannya — menghapus baris pesan saja tidak menghapus gambar di ImgBB.
- Komentar terlaporkan (`reports` bertarget `comment`) ditangani seperti pesan chat: periksa konteksnya, soft-delete bila melanggar, bisukan penulisnya bila berulang.

### 3.11 Notifikasi & Pengumuman (Broadcast)
- Susun pengumuman manual: judul, isi, dan tautan tujuan opsional (mis. halaman judul atau promo voucher).
- Pilih audiens: **semua user**, **premium saja**, atau **gratis saja** — lalu kirim sekarang (pengiriman dijalankan worker sebagai push FCM + entri kotak masuk `notifications`).
- Riwayat broadcast: isi, audiens, jumlah terkirim (`broadcasts.sent_count`), dan waktu — tercatat juga di `admin_logs`.
- Pengiriman berjalan **bertahap** (kelompok FCM multicast maks 500 token dengan jeda antar-kelompok agar VPS & batas laju FCM tidak kewalahan); progres `sent_count` bisa dipantau dari riwayat sampai pengiriman selesai.
- Disiplin anti-spam: broadcast adalah alat pengumuman penting (event, promo, gangguan layanan), bukan harian — targetkan maks ±1–2 kali seminggu agar user tidak mematikan notifikasi aplikasi.

## 4. Tabel Pendukung

Panel admin memakai tabel yang sama dengan aplikasi utama, ditambah:

| Tabel | Fungsi |
| --- | --- |
| `profiles.role` | Penanda admin (`user` / `admin`) |
| `reports` | Laporan user soal konten rusak — ditindaklanjuti admin |
| `admin_logs` | Jejak audit aksi admin |
| `vouchers` | Kode voucher premium (promo admin / dibeli user sebagai hadiah) — detail di DATABASE.md |
| `title_requests` + `title_request_votes` | Permintaan judul dari user + suara dukungan — ditindaklanjuti dari modul Katalog |
| `user_devices` | Registri perangkat per akun — batas perangkat premium & pencabutan sesi |

Detail kolom, relasi, dan SQL-nya ada di [DATABASE.md](DATABASE.md).

## 5. Contoh Alur

**Subtitle gagal diproses** — Dashboard menunjukkan `failed` naik → buka modul Subtitle → lihat alasan (mis. API transkripsi timeout) → perbaiki penyebabnya → Retry → status kembali `queued` → worker memproses ulang → admin memeriksa hasilnya `done`.

**Laporan link mati** — User melaporkan episode → laporan masuk modul Laporan → admin membuka episode-nya, mengganti `video_url` di modul Katalog → tandai laporan `resolved` (tercatat di `admin_logs`).

**Memberi premium manual** — Modul User → pilih user → "Jadikan premium" + durasi → `subscriptions` dibuat berprovider `manual` dan `profiles.is_premium` diperbarui → tercatat di `admin_logs`.

**Voucher promo** — Modul Voucher → admin membuat kode promo 30 hari untuk 100 penukar pertama → kode dibagikan → user login dan menukar kode di halaman penukaran → voucher berubah `redeemed`, premium user bertambah 30 hari, dan admin memantau sisa kuota dari modul yang sama.

**Voucher hadiah antar-user** — User A membeli voucher 30 hari di halaman premium (bayar QRIS) → menerima kode → memberikannya ke User B → B menukar kode → premium B aktif; A bisa melihat status vouchernya (sudah ditukar/belum) dari halaman premium.
