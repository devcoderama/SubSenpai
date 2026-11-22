# android/ — Aplikasi Android SubSenpai

Folder ini adalah rumah **aplikasi Android SubSenpai (Kotlin)** — satu-satunya klien SubSenpai, berjalan di perangkat user (tidak ada versi web). Status: **Fase 3 terbangun** (v0.3.0) — Fase 2 (pemilih bahasa F40 di player: chip aliran hardsub per bahasa, chip trek subtitle softsub, chip Terjemahkan gtx) ditambah akun: layar Akun dengan Google Sign-In (Credential Manager) yang menukar token ID Google menjadi JWT backend (disimpan di perangkat), tombol watchlist di halaman detail, layar Watchlist Saya, blok Lanjutkan di beranda dari riwayat tonton, dan pelaporan progres putar dari player; APK debug terkompilasi (lihat [PROGRES.md](../PROGRES.md)).

Aplikasi didistribusikan sebagai APK lewat **website resmi & GitHub Releases** (keputusan distribusi: [NOTE.md](../NOTE.md)). Backend-nya adalah layanan Go di VPS — aplikasi hanya berbicara ke API backend, tidak pernah langsung ke database.

## Struktur Folder (Rancangan)

```
android/
├── README.md                       # Dokumen ini
├── settings.gradle.kts             # Pengaturan proyek Gradle
├── build.gradle.kts                # Konfigurasi build tingkat proyek
└── app/                            # Modul aplikasi
    ├── build.gradle.kts            # Konfigurasi modul (SDK, dependensi, penandatanganan rilis)
    └── src/main/
        ├── AndroidManifest.xml
        ├── java/com/subsenpai/     # Kode Kotlin, paket aplikasi com.subsenpai
        │   ├── ui/                 # Semua layar (Jetpack Compose)
        │   │   ├── beranda, telusur, detail judul
        │   │   ├── player (video) & reader (komik)
        │   │   ├── chat global, dm, teman, grup, koleksi
        │   │   ├── watchlist, riwayat, pencapaian, rekap
        │   │   └── premium, keamanan akun, pengaturan
        │   ├── data/               # Lapisan data
        │   │   ├── Klien API backend Go (Retrofit/OkHttp)
        │   │   ├── Dekripsi response xdata/xtime + handshake sesi ECDH
        │   │   ├── Sesi login: Google Sign-In → token → JWT backend
        │   │   ├── Model data & basis data lokal katalog (sinkronisasi delta)
        │   │   ├── Klien terjemahan gtx dari perangkat + cache terjemahan lokal
        │   │   └── Penyimpanan terenkripsi perangkat (cookie clearance sumber, token sesi)
        │   └── player/             # Player HLS Media3/ExoPlayer
        │       ├── Overlay subtitle + penerjemah subtitle di perangkat
        │       ├── Lewati intro/outro, PiP, mode audio, Cast
        │       └── WebView verifikasi sumber ber-Cloudflare (F39)
        └── res/                    # Sumber daya Android
            ├── values-<bahasa>/    # String antarmuka multi-bahasa (BCP-47: id, en, ms, ja, zh-CN, ko, th, vi, es)
            └── tema, ikon, & aset tampilan lainnya
```

## Tanggung Jawab Aplikasi

- **Login Google saja**: aplikasi mengirim token Google ke backend; backend memverifikasi dan menerbitkan JWT sesi — aplikasi menyimpan sesi itu, bukan kata sandi.
- **Membuka enkripsi API**: semua response backend dibungkus `xdata`/`xtime`; aplikasi melakukan handshake sesi ECDH (kunci hanya di memori) dan mendekripsinya sebelum ditampilkan (rincian: [STRUKTUR.md](../STRUKTUR.md) Bagian 11).
- **Menerjemahkan di perangkat**: subtitle dan tombol Terjemahkan diterjemahkan aplikasi dari IP user sendiri (endpoint `gtx`) dan di-cache lokal — server tidak menerjemahkan sama sekali.
- **Mengambil sumber ber-Cloudflare di perangkat**: untuk judul `fetch_mode='device'` (F39), aplikasi membuka situs sumber di WebView, melewati verifikasi Cloudflare sekali, lalu memakai cookie `cf_clearance` yang tersimpan terenkripsi di perangkat untuk mengambil chapter & halaman — pola yang sama dengan Mihon.
- **Bukan tempat panel admin**: aplikasi ini tidak memiliki layar/menu admin sama sekali — panel admin adalah website terpisah khusus admin (rincian: [STRUKTUR_ADMIN.md](../STRUKTUR_ADMIN.md)).
- Sisanya mengikuti dokumen rancangan: fitur di [FITUR.md](../FITUR.md), keamanan di [KEAMANAN.md](../KEAMANAN.md), optimasi di [OPTIMASI.md](../OPTIMASI.md).

## Yang Tidak Dilakukan Aplikasi

- Tidak menyimpan rahasia server (kunci API, kredensial database, token bot) dalam bentuk apa pun.
- Tidak menegakkan aturan akses sendirian — gerbang freemium/premium selalu diputuskan backend.
- Tidak menyimpan database utama; katalog lokal hanyalah cache hasil sinkronisasi untuk pencarian instan & hemat data.
