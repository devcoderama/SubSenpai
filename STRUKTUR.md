# STRUKTUR.md — SubSenpai

Rancangan struktur proyek: susunan folder, skema database, model akses, dan alur data. Dokumen ini adalah titik awal desain — boleh berubah selama pengembangan, asalkan perubahan besar dicatat di sini.

## 1. Prinsip Desain

- **Satu katalog, banyak tipe konten.** Anime, donghua, film, manga, manhua, manhwa hidup di tabel katalog yang sama dengan kolom `type`. Database TIDAK dipecah per genre maupun per tipe: genre dan tipe hanyalah kolom, karena pencarian lintas keduanya tidak bisa di-JOIN antar-database.
- **Pisah per jenis data.** Metadata di PostgreSQL, berkas (poster, halaman komik, subtitle) di object storage, video tidak disimpan sama sekali.
- **Video tidak pernah disimpan / tidak lewat server sendiri.** Player memutar stream HLS langsung dari sumber; server hanya menyimpan metadata dan URL.
- **Backend Go di VPS.** Backend ditulis dengan **Go** dan dijalankan di **VPS** sebagai binary mandiri di belakang reverse proxy — bukan serverless. Satu binary ringan (hemat RAM, tanpa runtime tambahan) melayani API; proses latar (antrean subtitle, polling pembayaran) berjalan sebagai goroutine/job terjadwal di layanan yang sama atau binary worker terpisah. **Tidak ada frontend web** — satu-satunya klien adalah **aplikasi Android (Kotlin)** yang berjalan di perangkat user dan berbicara ke API Go; semua logika, gerbang akses, dan data ada di backend.
- **Database & Auth tetap Supabase.** PostgreSQL + Supabase Auth (login Google) tetap dipakai; backend Go mengakses PostgreSQL secara langsung (driver pgx) dan memverifikasi token JWT Supabase Auth di middleware. Yang berubah hanya tempat komputasi berjalan (VPS), bukan skema datanya.
- **Proses berat di job latar.** Antrean subtitle, terjemahan, dan pembangunan SRT dikerjakan job latar Go, tetapi transkripsi Whisper dititipkan ke API cloud (Groq/OpenAI) — VPS tidak punya GPU. Worker Python mandiri (faster-whisper di mesin GPU) hanya opsi bila nanti perlu full offline.
- **Gerbang akses ditegakkan di server.** Batas tamu/premium dicek di API Go, bukan sekadar menyembunyikan tombol di UI.
- **Biaya tetap & terukur.** VPS (biaya tetap per bulan) + Supabase (free) + Cloudflare R2 (free 10 GB); biaya variabel hanya API transkripsi (per menit audio) dan payment gateway (potongan per transaksi).

## 2. Struktur Folder

```
SubSenpai/
├── README.md               # Gambaran umum proyek
├── STRUKTUR.md             # Dokumen ini
├── STRUKTUR_ADMIN.md       # Rancangan panel admin
├── DATABASE.md             # Skema database (ERD + SQL)
├── apps/
│   └── android/            # Aplikasi Android (Kotlin) — SATU-SATUNYA klien, berjalan di perangkat user
│       └── app/src/main/java/com/subsenpai/
│           ├── ui/                     # Layar: beranda, telusur, detail, player, reader, chat, premium, watchlist, admin
│           ├── data/                   # Klien API Go (Retrofit/OkHttp) + dekripsi xdata/xtime, sesi Supabase Auth, model data
│           └── player/                 # Player HLS (Media3/ExoPlayer) + overlay subtitle
├── backend/                # Backend Go — dijalankan di VPS
│   ├── cmd/
│   │   ├── server/                     # main.go — layanan HTTP API
│   │   └── worker/                     # main.go — job latar (antrean subtitle, polling GoPay, pembersih sesi)
│   ├── internal/
│   │   ├── httpapi/                    # Router (chi) + handler per domain: katalog, akses, subtitle, chat, premium, voucher, admin, webhook
│   │   ├── middleware/                 # Auth (verifikasi JWT Supabase), enkripsi response xdata/xtime, rate limit, CORS, logging
│   │   ├── db/                         # Akses PostgreSQL (pgx) + query per domain
│   │   ├── store/                      # Object storage R2 (poster, halaman komik, SRT, stiker)
│   │   ├── jobs/                       # Worker subtitle (antrean → transkripsi API → terjemah → SRT → R2), polling pembayaran, dsb.
│   │   └── config/                     # Baca konfigurasi dari env
│   ├── go.mod
│   └── .env.example                    # DATABASE_URL, kredensial gateway & API transkripsi (contoh tanpa nilai asli)
├── supabase/
│   ├── migrations/         # Migrasi skema PostgreSQL
│   └── seed/               # Data awal (daftar genre, tipe, kanal chat, dsb.)
├── deploy/
│   ├── Caddyfile           # Reverse proxy + HTTPS otomatis (Let's Encrypt)
│   └── systemd/            # subsenpai-api.service, subsenpai-worker.service
└── docs/                   # Catatan tambahan (sumber data, riset iklan, dsb.)
```

Catatan: backend Go dibangun menjadi binary (`go build`) lalu dijalankan systemd di VPS — tidak perlu Docker, Node, maupun runtime lain; VPS hanya menjalankan layanan Go + Caddy. Aplikasi Android didistribusikan sebagai APK / lewat Play Store. Rincian deployment ada di Bagian 10.

## 3. Skema Database (Supabase / PostgreSQL)

Diagram ERD, detail relasi, indeks, RLS, dan draf SQL lengkap ada di **[DATABASE.md](DATABASE.md)** — bagian ini ringkasannya.

### `titles` — katalog semua konten
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | |
| slug | text (unik) | Untuk URL: `/title/<slug>` |
| type | text | `anime` / `donghua` / `film` / `manga` / `manhua` / `manhwa` |
| title | text | Judul romaji/Inggris |
| title_alt | text | Judul asli (Jepang/Cina/Korea) |
| synopsis | text | Sinopsis (Indonesia bila ada) |
| poster_url | text | URL poster (di R2) |
| year | int | Tahun rilis |
| season | text | `winter` / `spring` / `summer` / `fall` (khusus anime; boleh kosong) |
| status | text | `ongoing` / `completed` |
| genres | text[] | Daftar genre — kolom biasa, BUKAN database terpisah |
| rating | numeric | Skor rata-rata (opsional) |
| created_at | timestamptz | |

### `episodes` — konten video (anime/donghua/film)
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | |
| title_id | uuid (FK → titles) | |
| episode_no | int | Nomor episode (film: 1) |
| title | text | Judul episode (opsional) |
| video_url | text | URL stream HLS dari sumber |
| duration_sec | int | Durasi (opsional) |
| aired_at | date | Tanggal tayang |

### `chapters` + `chapter_pages` — konten baca (manga/manhua/manhwa)
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | (chapters) |
| title_id | uuid (FK → titles) | |
| chapter_no | numeric | Nomor chapter (boleh desimal, mis. 10.5) |
| title | text | Judul chapter (opsional) |
| page_count | int | Jumlah halaman |

`chapter_pages`: id, chapter_id (FK), page_no, image_url (di R2).

### `subtitles` — khusus konten video
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | |
| episode_id | uuid (FK → episodes) | |
| lang | text | Kode bahasa, mis. `id` |
| source | text | `caption` (dari CC sumber) / `whisper` (transkripsi API) |
| format | text | `srt` / `ass` |
| file_url | text | URL file subtitle (di R2) |
| status | text | `queued` / `processing` / `done` / `failed` |

### Data user & monetisasi (Supabase Auth)
- Akun dikelola **Supabase Auth** (`auth.users`) — tidak buat tabel kredensial sendiri.
- **Login wajib Google (OAuth)** — hanya provider Google yang diaktifkan; pendaftaran email/password DIMATIKAN. Tujuannya menghindari bot: akun palsu harus lebih dulu memiliki akun Google yang lolos anti-abuse Google. Bukan anti-bot mutlak — lapisan cadangan bila masih kecolongan: Cloudflare Turnstile (captcha tak terlihat, gratis) di endpoint sensitif + rate limit API.
- Alur login di aplikasi: user masuk lewat **Google Sign-In** di Android → token ID Google ditukar menjadi sesi Supabase Auth (`signInWithIdToken`) → JWT sesi dikirim sebagai header `Authorization` ke API Go pada setiap request, dan backend memverifikasinya.
- `profiles` — 1:1 dengan user: nama tampil, avatar, `is_premium` (cache status), `premium_until`, `role` (`user`/`admin` — penanda akses panel admin).
- `subscriptions` — user_id, plan, status (`active`/`expired`/`cancelled`), current_period_end, provider (`midtrans`/`xendit`/`manual`/`voucher`), provider_ref.
- `vouchers` — kode premium sekali pakai: dibuat admin (promo, bisa ditujukan ke user tertentu) atau dibeli user untuk dihadiahkan ke user lain; penukaran menambah hari premium penukar (detail di DATABASE.md).
- `watchlist` — user_id + title_id.
- `watch_history` — user_id + episode_id + posisi terakhir (detik) + updated_at.
- `read_history` — user_id + chapter_id + halaman terakhir + updated_at.
- `guest_access` — jejak akses tamu: device_key (hash cookie/device), title_id, episode_id/chapter_id, used_at. Dipakai menegakkan batas 1 episode/chapter.

## 4. Model Akses (Freemium)

| Pengguna | Batas | Iklan |
| --- | --- | --- |
| Tamu (belum login) | Maks. **1 episode / 1 chapter** (dicatat di `guest_access`) | Ya |
| Login gratis | Semua katalog | Ya |
| Premium | Semua katalog | **Tidak** |

Penegakan: setiap permintaan memutar episode / membuka chapter melewati cek akses di API —
1. User premium / login → lolos (premium tanpa iklan).
2. Tamu → cek `guest_access`: belum pernah membuka episode/chapter apa pun → lolos dan catat; sudah → tolak dengan ajakan login.
3. Status premium bersumber dari `subscriptions` (diperbarui webhook payment gateway), `profiles.is_premium` hanya cache bacaan cepat.

## 5. Panel Admin

Pengelolaan & pemantauan berjalan sebagai **layar admin di dalam aplikasi yang sama**, khusus user ber-`role = 'admin'` (menu admin disembunyikan untuk user biasa DAN tetap dicek di backend Go pada setiap request API admin). Modulnya: dashboard ringkasan, manajemen katalog, manajemen subtitle (antrean + retry), manajemen user, langganan & pembayaran, voucher premium, laporan user, monitoring, dan pengaturan platform. Rancangan lengkapnya: **[STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md)**.

## 6. Alur Data

1. **Katalog masuk** — metadata judul & episode/chapter ditulis ke Supabase; poster & halaman komik diunggah ke R2.
2. **User membuka halaman nonton/baca** — API Go mengecek gerbang akses (Bagian 4) dan membaca metadata dari PostgreSQL → player memutar `video_url` langsung dari sumber / reader memuat gambar halaman dari R2.
3. **Subtitle diminta** (video saja) — player mengecek tabel `subtitles`:
   - Sudah `done` → muat file dari R2, selesai.
   - Belum ada → buat baris `queued`; worker memprosesnya.
4. **Worker memproses antrean** — unduh audio → (ada caption bahasa asal? terjemahkan langsung : transkripsi via API Whisper) → terjemahkan → bangun .srt → unggah ke R2 → ubah status jadi `done`.
5. **Progres** — player/reader melaporkan posisi terakhir ke API → `watch_history` / `read_history` (user login).
6. **Premium** — user membayar via Midtrans/Xendit (konfirmasi webhook) atau GoPay Merchant pihak ketiga (konfirmasi polling — Bagian 7) → `subscriptions` + `profiles.is_premium` diperbarui → iklan berhenti tampil. Premium juga bisa datang dari **voucher**: admin menerbitkan kode promo (atau user membeli voucher hadiah) → penerima login dan menukar kode → `premium_until` diperpanjang sejumlah hari voucher dan tercatat sebagai `subscriptions` berprovider `voucher`.

## 7. Alur Pembayaran

### 7.1 Gateway resmi (jalur utama)
Midtrans / Xendit: user diarahkan ke halaman pembayaran (QRIS, e-wallet, transfer bank) → gateway memanggil **webhook** server saat pembayaran berhasil → `subscriptions` diaktifkan / voucher pembelian diterbitkan. Ini jalur yang disarankan untuk jangka panjang (resmi, ada refund & rekonsiliasi).

### 7.2 GoPay Merchant via API pihak ketiga (alternatif bootstrap)
Berdasarkan repo [ZeroByte000/Api-Gopar-Merch](https://github.com/ZeroByte000/Api-Gopar-Merch): API **tidak resmi** yang membungkus akun GoPay Merchant (GoBiz), berjalan sebagai Vercel Serverless Functions. Endpoint yang dipakai:

| Endpoint | Fungsi |
| --- | --- |
| `POST /api/login` | Login email + password merchant → `access_token` |
| `POST /api/create-payment` | Mengubah QRIS statis merchant menjadi QRIS **dinamis** berisi jumlah tertentu → string QRIS + URL gambar QR |
| `POST /api/cek-transaksi` | Daftar transaksi masuk terbaru (header `Authorization: Bearer <token>`), bisa difilter per jumlah |

Alurnya (berlaku untuk langganan premium maupun pembelian voucher):

1. User memilih paket → server membuat baris `payment_sessions` ber-provider `gopay` dengan **nominal unik** (harga + kode unik acak 1–999, mis. Rp30.000 → Rp30.417) dan masa berlaku ±15 menit.
2. Server memanggil `/api/create-payment` dengan nominal itu → QR dinamis ditampilkan ke user.
3. User membayar dengan aplikasi apa pun (GoPay, DANA, OVO, m-banking — QRIS bersifat lintas aplikasi).
4. **Tidak ada webhook** — job polling di backend Go (goroutine berkala pada layanan worker) memanggil `/api/cek-transaksi` memakai token login merchant (token disimpan & diperbarui di server) dan mencocokkan transaksi berstatus `success` yang jumlahnya **persis** nominal unik, dalam jendela waktu sesi.
5. Cocok → sesi menjadi `paid`, `subscriptions.provider_ref` / `vouchers.payment_ref` diisi id referensi GoPay, lalu premium diaktifkan / voucher diterbitkan (sama seperti jalur webhook). Lewat batas waktu tanpa kecocokan → sesi `expired` dan nominal uniknya bebas dipakai sesi lain.

Risiko & catatan (wajib dipahami sebelum memakai jalur ini):

- **Tidak resmi** — bergantung pada akses akun merchant yang sewaktu-waktu bisa berubah/dibatasi pihak GoTo; pola akses yang tidak wajar berisiko membuat akun merchant dibatasi.
- Kredensial merchant (email + password) hidup sebagai *environment variable* server — keamanan server sama dengan keamanan uang masuk; tidak boleh tampil di frontend, log, atau repo.
- Pencocokan berbasis jumlah → nominal unik **wajib**; dua sesi aktif tidak boleh berbagi nominal yang sama.
- Tidak ada mekanisme refund/perselisihan resmi; selisih diselesaikan manual lewat modul Langganan di panel admin.
- Karena itu posisinya **alternatif murah untuk memulai**, bukan pengganti gateway resmi.

## 8. Chat Global

- Ruang obrolan publik **realtime** khusus user yang sudah login (tamu tidak ikut membaca/menulis — selaras dengan login Google wajib, sekaligus anti-spam).
- **Kanal per topik**: Umum, Anime, Donghua, Film, Manga, Rekomendasi — dikelola admin lewat tabel `chat_channels`.
- Teknologi: **Supabase Realtime** (tanpa server websocket terpisah) + Presence untuk menampilkan jumlah user online per kanal.
- Tampilan pesan seperti aplikasi chat komunitas: avatar, nickname, badge Premium/Admin, stempel waktu, dan banner pengumuman di bagian atas (diatur admin — mis. peringatan anti-penipuan). Pesan teks maks 500 karakter.
- **Kustomisasi khusus premium** (nilai jual langganan):
  - **Warna nickname** pilihan sendiri (`profiles.name_color`)
  - **Emoji pendamping nama** (`profiles.name_emoji`)
  - **Paket stiker premium** — pesan bisa berupa stiker gambar (dari R2, tabel `stickers`); user gratis hanya mendapat stiker dasar + emoji standar
  - Kustomisasi diperiksa di server. Bila langganan berakhir, tampilan kembali ke bawaan — pengaturan tetap tersimpan dan aktif lagi saat user berlangganan kembali.
- **Moderasi**: batas laju pengiriman, filter kata terlarang, admin/mod dapat menghapus pesan (soft-delete) dan membisukan user (`chat_muted_until`); user dapat melaporkan pesan (masuk `reports` bertarget `chat_message`). Modul adminnya dirinci di STRUKTUR_ADMIN.md.

## 9. Keamanan API — Enkripsi Response (`xdata` / `xtime`)

Semua response JSON dari backend TIDAK dikirim mentah. Backend membungkusnya dalam **envelope terenkripsi**, dan aplikasi di sisi klien membukanya kembali menjadi response asal sebelum ditampilkan:

```json
{
  "xdata": "m9lDub1JKs9VPPt4kHh7bC0_iehgM0IPWOgOBDBJ3gufKaK_KetuKjzGXnrTUNWdbhc84XP1_ifEJYb8AnYswqcZA2gR7VSZVXxNeuDqYy6d9RGo4ofz0cfbRwi0eZLoGuGDuMAvlMUr693R56OH4h-",
  "xtime": 1726843096
}
```

Spesifikasi:

- **Kunci sesi sekali pakai (ECDH)** — TIDAK ada kunci enkripsi yang tertanam di aplikasi maupun disimpan permanen di server. Setiap sesi membuat kuncinya sendiri:
  1. Saat aplikasi dibuka, aplikasi membuat pasangan kunci sementara **X25519** dan mengirim kunci publiknya ke endpoint handshake `POST /api/v1/session` (boleh anonim untuk tamu; membawa JWT bila sudah login).
  2. Backend membuat pasangan kunci sementaranya sendiri, membalas kunci publiknya, dan kedua sisi menurunkan kunci sesi yang sama lewat **ECDH + HKDF-SHA256** (info: `subsenpai-xdata-v1`). Kunci rahasia bersama tidak pernah dikirim lewat jaringan.
  3. Backend menyimpan kunci sesi di memori (peta `session_id → kunci`; `session_id` acak dikembalikan ke aplikasi dan dikirim balik sebagai header `X-Session-Id`); aplikasi menyimpan kuncinya di memori saja. Sesi berakhir (logout / aplikasi ditutup / kedaluwarsa maks 24 jam) → kunci dibuang dan handshake diulang.
- **Enkripsi**: AES-256-GCM dengan kunci sesi. `xdata` = `base64url( IV acak 12 byte ‖ ciphertext ‖ tag GCM )` dari response JSON asli (UTF-8). `xtime` (detik Unix saat response dibuat) dipakai sebagai *additional authenticated data* (AAD) — mengubah `xtime` membuat dekripsi gagal.
- **Alur di aplikasi**: terima envelope → periksa `xtime` (ditolak bila selisih dengan waktu perangkat > 300 detik — anti-replay) → dekripsi dengan kunci sesi → JSON asal → tampilkan. Bila tag GCM tidak cocok, response dianggap dimanipulasi dan dibuang; bila server menjawab sesi tidak dikenal/kedaluwarsa, aplikasi melakukan handshake ulang lalu mengulang request-nya.
- **Penerapan terpusat**: enkripsi dilakukan satu middleware pembungkus response di backend Go (`internal/middleware`, memakai `crypto/aes` + `crypto/cipher` bawaan Go), jadi tiap handler tidak mengimplementasikan sendiri-sendiri. Sisi aplikasi memakai modul crypto klien Android (X25519 + AES/GCM bawaan platform).
- **Berlaku untuk**: semua endpoint JSON backend Go di `/api/v1/*` (katalog, gerbang akses, subtitle, chat, premium, admin) — kecuali endpoint handshake itu sendiri.
- **Pengecualian**: endpoint handshake `/api/v1/session` (bootstrap kunci; dilindungi HTTPS), webhook pembayaran (server-ke-server, punya tanda tangan gateway sendiri), dan file media/HLS/SRT (bukan JSON).

Kenapa bukan kunci dari signature URL atau kunci statis tertanam: path URL bersifat publik, jadi kunci semacam itu bertumpu pada satu secret induk yang tertanam di APK — sekali terbongkar, seluruh endpoint dan seluruh user terbuka selamanya. Dengan kunci sesi ECDH, membongkar APK tidak menghasilkan kunci apa pun, satu kunci yang bocor hanya membuka satu sesi, dan traffic lama yang terekam tidak bisa didekripsi belakangan karena kunci sementaranya sudah dibuang (*forward secrecy*).

Catatan jujur soal batas lapisan ini: penyerang yang sangat berniat tetap bisa meniru handshake aplikasi dan berbicara ke API seperti aplikasi asli. Lapisan ini berfungsi sebagai **anti-scraping massal**, **integritas**, **anti-replay**, dan **kerahasiaan per sesi** — BUKAN pengganti HTTPS, login, dan gerbang akses di server.

## 10. Deployment (VPS)

Satu VPS menjalankan seluruh aplikasi (spesifikasi awal cukup 1–2 vCPU / 1–2 GB RAM karena binary Go ringan):

| Komponen | Bentuk | Port lokal |
| --- | --- | --- |
| Reverse proxy | Caddy (HTTPS otomatis Let's Encrypt) | 80/443 (publik) |
| Backend API | Binary Go (systemd `subsenpai-api`) | 8080 |
| Worker | Binary Go kedua dari basis kode yang sama (systemd `subsenpai-worker`) — antrean subtitle, polling GoPay, pembersih sesi | — |
| Database & Auth | Supabase (PostgreSQL + Google OAuth) — di luar VPS | — |
| Berkas | Cloudflare R2 — di luar VPS | — |

- Caddy meneruskan seluruh request (API publik, mis. `api.subsenpai...`) ke backend Go; hanya Caddy yang terekspos ke internet, layanan Go hanya mendengarkan di localhost.
- Konfigurasi & rahasia backend di satu file env di VPS (mis. `/etc/subsenpai/backend.env`, izin 600): `DATABASE_URL`, kredensial payment gateway & GoPay Merchant, kunci API transkripsi/terjemahan, kredensial R2. Tidak masuk repo (repo hanya berisi `.env.example`). Kunci sesi `xdata` TIDAK termasuk env/disk — dibuat per sesi dan hidup di memori saja (Bagian 9).
- Alur rilis: `go build` → unggah binary → `systemctl restart subsenpai-api subsenpai-worker`. Pembaruan aplikasi Android lewat rilis APK/Play Store seperti biasa (di luar VPS).
- Cadangan: database diurus Supabase; yang perlu dicadangkan dari VPS hanya file env dan konfigurasi `deploy/`.

## 11. Batasan yang Disadari

- Satu VPS adalah titik tunggal kegagalan — bila VPS mati, seluruh layanan berhenti. Mitigasi awal: pantau uptime + restart otomatis systemd; pisahkan worker ke mesin lain bila antrean subtitle membesar.
- Aplikasi saja, tanpa versi web: penemuan konten lewat mesin pencari (SEO) tidak ada — distribusi & pertumbuhan mengandalkan APK/Play Store dan promosi komunitas. Sebagai gantinya VPS jauh lebih ringan (komponen terberat sebelumnya, Next.js, hilang).
- Project Supabase gratis bisa di-pause bila lama tidak aktif — aktivitas berkala menjaganya tetap bangun.
- Sumber konten resmi umumnya berkunci wilayah + DRM; daftar sumber yang benar-benar bisa dipakai dicatat terpisah di `docs/` setelah riset.
- Iklan: konten seperti ini umumnya ditolak AdSense — jaringan iklan alternatif diteliti di fase monetisasi.
- Menerjemahkan komik secara otomatis (OCR + menimpa teks pada gambar) di luar cakupan rancangan ini.
