# web_admin/ — Panel Admin Web SubSenpai

Folder ini adalah rumah **panel admin SubSenpai: sebuah website terpisah khusus admin** — bukan bagian dari aplikasi Android (aplikasi user tidak memiliki layar maupun menu admin sama sekali), dan bukan versi web untuk user.

Status saat ini: folder ini baru berisi README rancangan ini; kode panelnya dibuat pada fase pembangunan (lihat [PROGRES.md](../PROGRES.md)). Rincian fungsi tiap modul ada di [STRUKTUR_ADMIN.md](../STRUKTUR_ADMIN.md).

## Akses & Keamanan (Ringkas)

- Login memakai **Google OAuth**; hanya akun dengan `role = 'admin'` di tabel `profiles` yang bisa masuk — akun lain ditolak.
- Setiap request diperiksa ulang di **backend Go** (grup API `/api/v1/admin/*`): sesi + role dicek di server, bukan mengandalkan tampilan.
- Website admin di-deploy di **VPS yang sama di belakang Caddy**, pada alamat khusus admin yang tidak dipublikasikan ke user.
- Panel **tidak memegang akses database** dan tidak menyimpan rahasia server (token bot, webhook, kredensial gateway tetap di server); semua aksi penting admin tercatat di tabel `admin_logs`.

## Struktur Folder (Rancangan)

```
web_admin/
├── README.md                  # Dokumen ini
├── (konfigurasi proyek web)   # Teknologi frontend web ringan — diputuskan saat pembangunan
└── src/
    ├── pages/                 # Satu halaman per modul panel
    │   ├── DashboardPage          # Ringkasan
    │   ├── TitlesPage             # Kelola katalog (judul, episode, chapter, permintaan judul)
    │   ├── SubtitlesPage          # Antrean & hasil subtitle
    │   ├── UsersPage              # Kelola user, premium manual & perangkat akun
    │   ├── SubscriptionsPage      # Langganan & transaksi
    │   ├── VouchersPage           # Voucher premium (buat, bagikan, pantau klaim)
    │   ├── ChatModerationPage     # Moderasi Chat Global (pesan, kanal, stiker)
    │   ├── ReportsPage            # Laporan user (link mati, subtitle salah)
    │   ├── MonitoringPage         # Kesehatan sistem & metrik
    │   ├── NotificationsPage      # Broadcast notifikasi manual (promo/event)
    │   └── SettingsPage           # Pengaturan platform
    ├── components/            # Komponen tampilan bersama (tabel, formulir, grafik)
    └── api/                   # Klien API admin — memanggil backend Go /api/v1/admin/*
                               # dengan sesi login admin; tidak ada logika akses di sini
```

## Cara Kerja

1. Admin membuka alamat panel dan login dengan Google.
2. Backend memverifikasi akun dan role-nya, lalu menerbitkan sesi admin.
3. Semua data halaman diminta dari API admin backend; semua penulisan (ubah katalog, premium, retry subtitle, broadcast) dijalankan backend dan tercatat di `admin_logs`.
4. Bila sesi berakhir atau role dicabut, akses panel tertutup pada request berikutnya.
