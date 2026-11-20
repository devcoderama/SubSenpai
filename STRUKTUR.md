# STRUKTUR.md — SubSenpai

Rancangan struktur proyek: susunan folder, skema database, model akses, dan alur data. Dokumen ini adalah titik awal desain — boleh berubah selama pengembangan, asalkan perubahan besar dicatat di sini.

## 1. Prinsip Desain

- **Satu katalog, banyak tipe konten.** Anime, donghua, film, manga, manhua, manhwa hidup di tabel katalog yang sama dengan kolom `type`. Database TIDAK dipecah per genre maupun per tipe: genre dan tipe hanyalah kolom, karena pencarian lintas keduanya tidak bisa di-JOIN antar-database.
- **Pisah per jenis data.** Metadata di PostgreSQL, berkas (poster, halaman komik, subtitle) di object storage, video tidak disimpan sama sekali.
- **Video tidak pernah disimpan / tidak lewat server sendiri.** Player memutar stream HLS langsung dari sumber; server hanya menyimpan metadata dan URL.
- **Backend Go di VPS.** Backend ditulis dengan **Go** dan dijalankan di **VPS** sebagai binary mandiri di belakang reverse proxy — bukan serverless. Satu binary ringan (hemat RAM, tanpa runtime tambahan) melayani API; proses latar (antrean subtitle, polling pembayaran) berjalan sebagai goroutine/job terjadwal di layanan yang sama atau binary worker terpisah. **Tidak ada frontend web** — satu-satunya klien adalah **aplikasi Android (Kotlin)** yang berjalan di perangkat user dan berbicara ke API Go; semua logika, gerbang akses, dan data ada di backend.
- **Database & Auth lokal di VPS — tanpa Supabase.** PostgreSQL berjalan **lokal di VPS yang sama** agar ruang penyimpanan database mengikuti disk VPS (mis. 50 GB NVMe) — jauh di atas batas database paket gratis Supabase (500 MB) yang cepat mentok oleh pesan chat & riwayat user. Auth ditangani backend Go sendiri: backend memverifikasi token ID Google, lalu menerbitkan **JWT sesi buatannya sendiri** (kunci tanda tangan di env server) yang diverifikasi middleware di setiap request. Aplikasi tidak pernah menyentuh database secara langsung — semua akses data lewat API Go, dan aturan kepemilikan data ditegakkan di query backend (pengganti RLS).
- **Proses berat di job latar.** Antrean subtitle, terjemahan, dan pembangunan SRT dikerjakan job latar Go, tetapi transkripsi Whisper dititipkan ke API cloud (Groq/OpenAI) — VPS tidak punya GPU. Worker Python mandiri (faster-whisper di mesin GPU) hanya opsi bila nanti perlu full offline.
- **Gerbang akses ditegakkan di server.** Batas tamu/premium dicek di API Go, bukan sekadar menyembunyikan tombol di UI.
- **Cache untuk bacaan terbanyak.** Katalog (beranda, telusur) di-cache di memori backend Go (TTL 1–5 menit, dibuang saat admin mengubah katalog), dan domain API berjalan di belakang **Cloudflare** (cache tepi + proteksi dasar) — VPS kecil tidak perlu menjawab query yang sama berulang kali.
- **API hemat request.** Layar beranda dilayani satu endpoint gabungan (sorotan + per tipe + blok Lanjutkan), dan semua daftar besar memakai pagination cursor — bukan `OFFSET`.
- **Katalog tersinkron lokal di aplikasi.** Aplikasi menyimpan salinan katalog di penyimpanan lokal perangkat dan menyinkronkan hanya perubahannya memakai penanda `updated_at` (endpoint perubahan sejak waktu terakhir sinkron) — beranda tampil seketika tanpa menunggu jaringan, hemat kuota user, dan beban API turun.
- **Multi-bahasa sejak awal.** Antarmuka aplikasi tersedia dalam banyak bahasa (sumber daya bahasa Android per kode BCP-47; bawaan mengikuti bahasa perangkat, fallback Indonesia), subtitle bahasa asal diterjemahkan aplikasi ke bahasa pilihan user, dan teks bebas (sinopsis, komentar, pesan chat) bisa diterjemahkan sekali ketuk — **semua panggilan terjemahan dilakukan aplikasi dari perangkat masing-masing user** (IP tiap user berbeda-beda sehingga jarang kena limit — persis pola D TRADE); server tidak pernah memanggil layanan terjemahan. Bahasa pilihan user disimpan di `profiles.app_language` dan menjadi bahasa bawaan untuk subtitle, tombol Terjemahkan, dan templat notifikasi. Daftar bahasa awal: Indonesia (`id`), Inggris (`en`), Melayu (`ms`), Jepang (`ja`), Mandarin sederhana (`zh-CN`), Korea (`ko`), Thailand (`th`), Vietnam (`vi`), Spanyol (`es`) — daftar aktifnya dikelola dari panel admin.
- **Biaya tetap & terukur.** VPS (biaya tetap per bulan — sudah termasuk PostgreSQL lokal, tanpa langganan database terpisah) + Cloudflare R2 (free 10 GB); biaya variabel hanya API transkripsi (per menit audio) dan payment gateway (potongan per transaksi).

## 2. Struktur Folder

```
SubSenpai/
├── README.md               # Gambaran umum proyek
├── STRUKTUR.md             # Dokumen ini
├── STRUKTUR_ADMIN.md       # Rancangan panel admin
├── database/               # DATABASE.md (skema database: ERD + SQL) + database.sql (draf SQL)
├── android/                # Aplikasi Android (Kotlin) — SATU-SATUNYA klien, berjalan di perangkat user (rincian: android/README.md)
│   └── app/src/main/java/com/subsenpai/
│       ├── ui/                         # Layar: beranda, telusur, detail, player, reader, chat, dm, teman, grup, koleksi, pencapaian, rekap, keamanan akun, premium, watchlist, admin
│       ├── data/                       # Klien API Go (Retrofit/OkHttp) + dekripsi xdata/xtime, sesi JWT backend, model data, basis data lokal katalog (sinkronisasi delta), klien terjemahan Google gratis (endpoint `gtx` dari perangkat user) + cache terjemahan lokal
│       └── player/                     # Player HLS (Media3/ExoPlayer) + overlay subtitle + penerjemah subtitle di perangkat (endpoint `gtx` dari IP user)
├── backend/                # Backend Go — dijalankan di VPS
│   ├── cmd/
│   │   ├── server/                     # main.go — layanan HTTP API
│   │   └── worker/                     # main.go — job latar (antrean subtitle, polling GoPay, pembersih sesi)
│   ├── internal/
│   │   ├── httpapi/                    # Router (chi) + handler per domain: katalog, akses, subtitle, chat, premium, voucher, admin, webhook
│   │   ├── middleware/                 # Auth (verifikasi token Google saat login + JWT sesi backend), enkripsi response xdata/xtime, rate limit, CORS, logging
│   │   ├── db/                         # Akses PostgreSQL (pgx) + query per domain
│   │   ├── store/                      # Object storage R2 (poster, halaman komik, SRT, stiker katalog) + klien ImgBB (gambar chat & stiker pribadi user)
│   │   ├── jobs/                       # Worker subtitle (antrean → transkripsi API → terjemah → SRT → R2), polling pembayaran, dsb.
│   │   └── config/                     # Baca konfigurasi dari env
│   ├── go.mod
│   └── .env.example                    # DATABASE_URL, kredensial gateway & API transkripsi (contoh tanpa nilai asli)
├── web_admin/              # Panel admin web (website terpisah khusus admin; rincian: web_admin/README.md + STRUKTUR_ADMIN.md)
├── db/
│   ├── migrations/         # Migrasi skema PostgreSQL (lokal di VPS)
│   └── seed/               # Data awal (daftar genre, tipe, kanal chat, dsb.)
├── deploy/
│   ├── Caddyfile           # Reverse proxy + HTTPS otomatis (Let's Encrypt)
│   └── systemd/            # subsenpai-api.service, subsenpai-worker.service
└── docs/                   # Catatan tambahan (sumber data, riset iklan, dsb.)
```

Catatan: backend Go dibangun menjadi binary (`go build`) lalu dijalankan systemd di VPS — tidak perlu Docker, Node, maupun runtime lain; VPS hanya menjalankan layanan Go + Caddy. Aplikasi Android didistribusikan sebagai APK lewat website resmi & GitHub Releases (keputusan distribusi: lihat NOTE.md). Rincian deployment ada di Bagian 12.

## 3. Skema Database (PostgreSQL Lokal di VPS)

Diagram ERD, detail relasi, indeks, kontrol akses, dan draf SQL lengkap ada di **[DATABASE.md](database/DATABASE.md)** — bagian ini ringkasannya.

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
| rating | numeric | Skor rata-rata (agregat dari `title_ratings`) |
| is_adult | boolean | Konten 18+ — disembunyikan dari user yang filternya aktif |
| search_tsv | tsvector | Kolom pencarian full-text (dari title + title_alt, indeks GIN) |
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
| intro_start_sec / intro_end_sec | int | Penanda rentang intro (untuk tombol Lewati Opening) |
| outro_start_sec / outro_end_sec | int | Penanda rentang outro/kredit penutup |

Player mendukung **putar otomatis episode berikutnya** (saklar di pengaturan player) dan tombol **Lewati Opening/Outro** dari penanda di atas; penanda diisi admin dari panel katalog.

### `chapters` + `chapter_pages` — konten baca (manga/manhua/manhwa)
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | (chapters) |
| title_id | uuid (FK → titles) | |
| chapter_no | numeric | Nomor chapter (boleh desimal, mis. 10.5) |
| title | text | Judul chapter (opsional) |
| page_count | int | Jumlah halaman |

`chapter_pages`: id, chapter_id (FK), page_no, image_url (di R2).

Reader mendukung dua mode: **halaman** (per gambar, cocok untuk manga) dan **gulir vertikal** (webtoon, cocok untuk manhwa/manhua) — pilihan mode diingat per user.

### `subtitles` — khusus konten video
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | |
| episode_id | uuid (FK → episodes) | |
| lang | text | Bahasa ASAL episode (kode BCP-47, mis. `ja`/`zh`/`ko`/`en`) — tabel ini hanya menyimpan subtitle sumber (dari caption atau transkripsi). Terjemahan ke bahasa user TIDAK dibuat/disimpan di server: aplikasi menerjemahkannya di perangkat dari berkas sumber ini (Google Translate gratis endpoint `gtx`) dan menyimpan hasil terjemahannya di cache lokal perangkat |
| source | text | `caption` (dari CC sumber) / `whisper` (transkripsi API) |
| format | text | `srt` / `ass` |
| file_url | text | URL file subtitle (di R2) |
| status | text | `queued` / `processing` / `done` / `failed` |

### Data user & monetisasi (Auth oleh backend Go)
- Akun adalah tabel `profiles` itu sendiri — tidak ada layanan auth eksternal dan tidak ada tabel kredensial: backend tidak pernah menyimpan kata sandi karena login hanya lewat Google.
- **Login wajib Google (OAuth)** — hanya provider Google yang diaktifkan; pendaftaran email/password DIMATIKAN. Tujuannya menghindari bot: akun palsu harus lebih dulu memiliki akun Google yang lolos anti-abuse Google. Bukan anti-bot mutlak — lapisan cadangan bila masih kecolongan: Cloudflare Turnstile (captcha tak terlihat, gratis) di endpoint sensitif + rate limit API.
- Alur login di aplikasi: user masuk lewat **Google Sign-In** di Android → token ID Google dikirim ke endpoint login backend → backend memverifikasinya ke Google (tanda tangan, audience, masa berlaku) → backend membuat/memperbarui baris `profiles` (dicocokkan lewat `google_sub` yang unik) dan menerbitkan **JWT sesi buatannya sendiri** → JWT itu dikirim sebagai header `Authorization` pada setiap request berikutnya dan diverifikasi middleware backend.
- `profiles` — tabel user itu sendiri: `email` + `google_sub` (identitas Google, unik), nama tampil, avatar, `is_premium` (cache status), `premium_until`, `role` (`user`/`admin` — penanda akses panel admin), `friend_code` (kode teman yang juga berfungsi sebagai kode referral), `xp`/`level` (aktivitas, Bagian 9), `hide_adult` (saklar filter konten 18+, bawaan: sembunyi), `show_online`/`show_activity` (saklar privasi kehadiran sosial, Bagian 9), `trial_used_at` (penanda uji coba premium sudah dipakai, Bagian 4), `last_active_date`/`streak_days` (rentetan hari aktif, Bagian 9), `app_language` (bahasa aplikasi pilihan user — BCP-47; bahasa bawaan subtitle, tombol Terjemahkan, dan templat notifikasi).
- `subscriptions` — user_id, plan, status (`active`/`expired`/`cancelled`), current_period_end, provider (`midtrans`/`xendit`/`gopay`/`manual`/`voucher`/`trial`), provider_ref.
- `vouchers` — kode premium sekali pakai: dibuat admin (promo, bisa ditujukan ke user tertentu) atau dibeli user untuk dihadiahkan ke user lain; penukaran menambah hari premium penukar (detail di DATABASE.md).
- `watchlist` — user_id + title_id + `notify` (lonceng per judul: user mengaktifkan notifikasi hanya untuk judul favorit yang ia pilih, berlaku untuk semua tipe — anime, donghua, film, manga, manhua, manhwa).
- `watch_history` — user_id + episode_id + posisi terakhir (detik) + updated_at.
- `read_history` — user_id + chapter_id + halaman terakhir + updated_at.
- `guest_access` — jejak akses tamu: device_key (hash cookie/device), title_id, episode_id/chapter_id, used_at. Dipakai menegakkan batas 1 episode/chapter.
- `comments` — komentar user per episode/chapter (bisa dilaporkan & di-soft-delete, Bagian 10).
- `title_ratings` — rating 1–10 + ulasan user per judul (satu per user, bisa diubah); agregatnya mengisi `titles.rating`.
- `device_tokens` — token FCM per perangkat user untuk notifikasi rilis baru.
- `daily_stats` — agregat harian untuk dashboard admin, dihitung worker dari tabel riwayat.
- `notifications` — kotak masuk notifikasi tiap user (rilis baru / broadcast / sistem) + status terbaca; setiap push FCM juga disimpan di sini agar bisa dibaca ulang di aplikasi.
- `broadcasts` — pengumuman manual dari admin (judul, isi, audiens, jumlah terkirim) — sumber notifikasi berkind `broadcast`.
- `title_requests` + `title_request_votes` — permintaan judul yang belum ada di katalog + suara dukungan user; admin mengerjakannya berdasar jumlah suara (Bagian 10).
- `collections` + `collection_items` + `collection_follows` — koleksi kurasi buatan user (daftar judul bertema, publik/privat) yang bisa diikuti user lain (Bagian 10).
- `user_achievements` — pencapaian yang sudah dibuka user, di atas sistem XP/level (definisi pencapaian hidup di kode, Bagian 9).
- `user_devices` — registri perangkat per akun untuk batas perangkat premium & halaman keamanan akun (Bagian 4).
- **Terjemahan tidak punya tabel di server** — panggilan Google Translate gratis (endpoint `gtx`) dilakukan aplikasi dari perangkat masing-masing user dan hasil terjemahannya (subtitle maupun teks) disimpan sebagai **cache lokal di perangkat** (kunci: id konten + bahasa tujuan). Server tidak memanggil layanan terjemahan sama sekali.

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

Manfaat & jalur premium tambahan:

- **Download offline (khusus premium)** — episode (segmen HLS + file SRT) dan chapter bisa diunduh ke perangkat untuk ditonton/dibaca tanpa internet; berkas unduhan dienkripsi dengan kunci perangkat agar tidak disebar sebagai file mentah. Hanya berlaku untuk sumber yang segmennya memang bisa diunduh.
- **Referral** — `friend_code` juga berfungsi sebagai kode referral: pendaftar baru yang memasukkan kode referral teman saat pertama masuk memicu sistem menerbitkan voucher `referral` untuk **kedua pihak** (pengundang & pendaftar sama-sama dapat hari premium) lewat mekanisme voucher yang sudah ada.
- **Uji coba premium (trial)** — user baru boleh mencoba premium **gratis 3 hari, sekali per akun**: ditandai `profiles.trial_used_at` agar tidak diakali daftar ulang dengan akun yang sama, tercatat sebagai `subscriptions` berprovider `trial`, dan tidak memerlukan metode pembayaran apa pun.
- **Hadiah premium langsung ke teman** — pembelian voucher bisa langsung memilih penerima dari daftar teman: voucher terbit dengan `assigned_to` = teman itu dan terkirim otomatis sebagai notifikasi + entri kotak masuk (tanpa menyalin/menempel kode); penukarannya tetap mekanisme voucher yang sama.
- **Batas perangkat premium** — satu akun premium aktif di maks **2 perangkat** (`user_devices`); login dari perangkat ketiga meminta user mencabut salah satu perangkat lamanya lewat **halaman keamanan akun** (daftar perangkat akun + cabut sesi perangkat lain). Batas ditegakkan API saat perangkat baru mendaftar, bukan sekadar di UI.
- **Filter konten dewasa** — judul bertanda `is_adult` disembunyikan dari katalog & pencarian untuk user yang `hide_adult`-nya aktif (bawaan aktif); admin menandai tanda ini dari panel katalog.

## 5. Panel Admin

Pengelolaan & pemantauan berjalan lewat **panel admin berupa website terpisah khusus admin** — bukan bagian dari aplikasi Android (aplikasi user tidak memiliki layar admin sama sekali) — khusus akun ber-`role = 'admin'`, dan tetap dicek di backend Go pada setiap request API admin. Website admin di-deploy di VPS yang sama di belakang Caddy pada alamat khusus admin. Modulnya: dashboard ringkasan, manajemen katalog, manajemen subtitle (antrean + retry), manajemen user, langganan & pembayaran, voucher premium, laporan user, monitoring, dan pengaturan platform. Rancangan lengkapnya: **[STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md)**.

## 6. Alur Data

1. **Katalog masuk** — metadata judul & episode/chapter ditulis ke PostgreSQL lokal di VPS; poster & halaman komik diunggah ke R2 setelah dikonversi ke **WebP + thumbnail** (ukuran turun 30–60%, muat daftar jauh lebih ringan).
2. **User membuka halaman nonton/baca** — API Go mengecek gerbang akses (Bagian 4) dan membaca metadata dari PostgreSQL → player memutar `video_url` langsung dari sumber / reader memuat gambar halaman dari R2. Agar perpindahan terasa seketika (**prefetch**): reader memuat lebih dulu 3 halaman berikutnya, dan player memuat awal episode berikutnya saat progres episode berjalan mencapai ±80%.
3. **Subtitle diminta** (video saja) — player mengecek tabel `subtitles` (subtitle **bahasa asal** episode):
   - Sudah `done` → aplikasi memuat file dari R2. Bila bahasa user sama dengan bahasa asal, langsung dipakai; bila berbeda, aplikasi **menerjemahkannya di perangkat saat itu juga** — mengunduh SRT asal, menerjemahkan baris-barisnya ke bahasa user (`app_language` atau pilihan di player) memakai Google Translate gratis endpoint `gtx` langsung dari perangkat user (IP tiap user berbeda sehingga jarang kena limit — pola D TRADE; teks dipotong per ±1.200 karakter, timestamp utuh), lalu menyimpan hasil terjemahannya di **cache lokal perangkat** (kunci: episode + bahasa) agar pemutaran berikutnya seketika. Gagal terjemah → subtitle bahasa asal tetap ditampilkan.
   - Belum ada → buat baris `queued`; worker menyiapkan subtitle bahasa asalnya.
4. **Worker menyiapkan subtitle bahasa asal** — unduh audio → (ada caption bahasa asal? pakai langsung : transkripsi via API Whisper) → bangun .srt **bahasa asal** → unggah ke R2 → ubah status jadi `done`. Worker TIDAK menerjemahkan — penerjemahan adalah urusan aplikasi di perangkat user (langkah 3), agar panggilan ke endpoint terjemahan gratis berangkat dari IP user masing-masing, bukan IP server. Antrean **berprioritas**: episode baru langsung dibuatkan subtitle saat ditambahkan admin (tidak menunggu ada yang meminta), dan episode dari judul yang sedang banyak ditonton didahulukan.
5. **Progres** — player/reader melaporkan posisi terakhir ke API → `watch_history` / `read_history` (user login). Data ini mengisi blok **"Lanjutkan"** yang menjadi blok pertama di beranda aplikasi.
6. **Premium** — user membayar via Midtrans/Xendit (konfirmasi webhook) atau GoPay Merchant pihak ketiga (konfirmasi polling — Bagian 7) → `subscriptions` + `profiles.is_premium` diperbarui → iklan berhenti tampil. Premium juga bisa datang dari **voucher**: admin menerbitkan kode promo (atau user membeli voucher hadiah) → penerima login dan menukar kode → `premium_until` diperpanjang sejumlah hari voucher dan tercatat sebagai `subscriptions` berprovider `voucher`.
7. **Notifikasi rilis baru** — saat episode/chapter baru ditambahkan ke sebuah judul, worker mengirim push **FCM** ke user yang memfavoritkan judul itu DAN mengaktifkan loncengnya (`watchlist.notify = true`; token perangkat dari `device_tokens`), sekaligus menyimpan salinannya ke kotak masuk `notifications` di aplikasi.
8. **Broadcast admin** — admin menyusun pengumuman dari panel (mis. event promo voucher, maintenance, judul baru unggulan), memilih audiens (semua user / premium saja / gratis saja), lalu worker mengirimkannya **bertahap** — push FCM multicast per kelompok maks 500 token dengan jeda antar-kelompok (menghindari lonjakan beban VPS & batas laju FCM) + baris `notifications` untuk tiap penerima; `sent_count` terisi progresif selama pengiriman berjalan. Riwayat tercatat di `broadcasts` dan aksi adminnya di `admin_logs`.
9. **Sinkronisasi katalog ke aplikasi** — aplikasi menyimpan katalog lokal (prinsip di Bagian 1); saat dibuka, aplikasi meminta perubahan sejak sinkronisasi terakhir (berdasar `updated_at` pada judul/episode/chapter) dan hanya mengunduh bedanya.

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
- Teknologi: **WebSocket dari backend Go sendiri** (satu endpoint `/ws` di layanan API — Go menangani banyak koneksi ringan dengan mudah, jadi tidak perlu layanan realtime terpisah) + presence di memori server untuk menampilkan jumlah user online per kanal.
- Tampilan pesan seperti aplikasi chat komunitas: avatar, nickname, badge Premium/Admin, stempel waktu, dan banner pengumuman di bagian atas (diatur admin — mis. peringatan anti-penipuan). Pesan teks maks 500 karakter.
- **Kustomisasi khusus premium** (nilai jual langganan):
  - **Warna nickname** pilihan sendiri (`profiles.name_color`)
  - **Emoji pendamping nama** (`profiles.name_emoji`)
  - **Paket stiker premium** — pesan bisa berupa stiker gambar (dari R2, tabel `stickers`); user gratis hanya mendapat stiker dasar + emoji standar
  - Kustomisasi diperiksa di server. Bila langganan berakhir, tampilan kembali ke bawaan — pengaturan tetap tersimpan dan aktif lagi saat user berlangganan kembali.
- **Kirim gambar & stiker pribadi**: user bisa mengirim gambar di chat dan menyimpan stiker buatannya sendiri (`user_stickers`). Gambar TIDAK disimpan sebagai berkas mentah di server maupun database — aplikasi mengirim gambar ke backend Go; backend **memperkecilnya lebih dulu** (maks 1600px, konversi WebP, metadata EXIF dibuang — unggahan kamera 5–10 MB menjadi ringan), lalu mengunggahnya ke **ImgBB** (`POST https://api.imgbb.com/1/upload`; kunci API ImgBB hanya ada di server), lalu yang disimpan di database hanya **URL** hasilnya (`chat_messages.image_url`) beserta **URL thumbnail** dari ImgBB (`image_thumb_url`) — daftar chat menampilkan thumbnail, gambar penuh baru dimuat saat pesan diketuk. Batas ImgBB maks 32 MB per gambar dan tanpa expiration (gambar tidak kedaluwarsa). `delete_url` dari ImgBB disimpan internal di server untuk keperluan moderasi — tidak pernah dikirim ke aplikasi. Stiker katalog resmi tetap di R2.
- **Balas pesan (reply)**: user bisa membalas pesan tertentu — di chat global maupun kanal grup — dan balasan menampilkan kutipan pesan aslinya (nama penulis + potongan isi, tersimpan sebagai `reply_to_id`). Bila pesan asli dihapus moderator, kutipannya tampil sebagai "pesan telah dihapus" tetapi balasannya tetap ada. Komentar episode/chapter juga mendukung balasan (satu tingkat, Bagian 10) — komunitas bisa saling menanggapi seperti media sosial.
- **Tanpa tautan luar & tanpa promosi**: pesan chat dan komentar yang mengandung tautan luar (pola `http://`, `https://`, `www.`, atau domain umum) **ditolak di server sebelum tersimpan**; promosi juga disaring filter kata terlarang. User tidak bisa menempel tautan apa pun dari luar ke dalam aplikasi. Satu-satunya "tautan" yang beredar adalah yang dibuat aplikasi sendiri (kartu berbagi di bawah dan URL gambar hasil unggahan resmi).
- **Berbagi resmi dari aplikasi**: tombol Bagikan pada judul/episode/chapter menghasilkan dua bentuk saja — (a) **kartu berbagi di dalam aplikasi**: pesan khusus (`kind = share`) berisi kartu konten yang bisa dikirim ke chat global, kanal grup, atau teman; (b) **berbagi ke luar aplikasi** lewat menu berbagi Android, berisi teks + tautan resmi aplikasi yang dibuat oleh aplikasi. Di luar dua bentuk itu, berbagi tautan tidak ada.
- **Moderasi**: batas laju pengiriman, filter kata terlarang, admin/mod dapat menghapus pesan (soft-delete) dan membisukan user (`chat_muted_until`); user dapat melaporkan pesan (masuk `reports` bertarget `chat_message`). Modul adminnya dirinci di STRUKTUR_ADMIN.md.
- **Tombol Terjemahkan** — setiap pesan chat (dan komentar, Bagian 10) punya aksi **Terjemahkan**: aplikasi menerjemahkannya langsung dari perangkat user ke bahasa aplikasinya (`app_language`) memakai Google Translate gratis (endpoint `gtx`, dari IP perangkat user sendiri — pola D TRADE) dan menyimpan hasilnya di cache lokal perangkat — ketukan berikutnya untuk teks & bahasa yang sama tidak memanggil endpoint lagi. Server tidak terlibat dalam penerjemahan sama sekali. Teks asli selalu bisa ditampilkan kembali lewat "Lihat asli".

## 9. Teman & Grup

### Teman
- Setiap user punya **kode teman** unik (`profiles.friend_code`) — tambah teman dengan memasukkan kode itu, atau mencari nama pengguna.
- Alur: kirim permintaan (`friendships` berstatus `pending`) → penerima **menerima** (menjadi `accepted`) atau **menolak** (permintaan dihapus). Daftar teman tampil di profil masing-masing.
- **Blokir** (`user_blocks`): user yang diblokir tidak bisa mengirim permintaan teman maupun undangan grup, dan pertemanan yang sudah ada terputus.
- **Chat pribadi (DM)** — sesama teman bisa mengobrol 1:1 lewat kanal khusus (`chat_channels` berkind `dm`, tepat satu kanal per pasangan teman). DM memakai sistem pesan yang sama persis dengan Chat Global: reply, stiker, gambar ImgBB (dengan kompresi & thumbnail), kartu berbagi, dan larangan tautan luar ikut berlaku. DM hanya bisa dimulai dengan teman — bukan orang asing — dan berhenti bisa dipakai bila salah satu pihak memblokir.
- **Kehadiran sosial** — daftar teman menampilkan status online (presence WebSocket backend) dan aktivitas terakhir temannya ("sedang menonton X" / "baru membaca Y", dari riwayat terbaru). Keduanya bisa dimatikan per user lewat saklar privasi `show_online` / `show_activity` di profil.
- Khusus user login — tamu tidak memiliki fitur sosial sama sekali.

### Grup
- User login bisa **membuat grup** (`groups`): nama, deskripsi, avatar, dan sifat **publik** (bisa dicari & diikuti siapa saja) atau **privat** (hanya lewat undangan/persetujuan).
- Peran anggota (`group_members`): `owner` (pembuat grup; satu-satunya yang bisa membubarkan grup), `admin` (diangkat owner; ikut memoderasi), `member`.
- Setiap grup otomatis memiliki **satu kanal chat sendiri** (`chat_channels` berkind `group`) — memakai sistem chat yang sama dengan Chat Global (pesan teks, stiker, kustomisasi premium), tetapi hanya anggota grup yang bisa membaca & menulis di dalamnya.
- Bergabung: grup publik bisa langsung diikuti; grup privat lewat undangan dari daftar teman atau permintaan bergabung yang disetujui admin grup.
- **Tautan undangan grup** — setiap grup punya **kode undangan** unik (`groups.invite_code`) yang membentuk tautan resmi aplikasi (mis. `https://subsenpai.app/g/<kode>`). **Semua anggota grup** boleh membagikan tautan itu ke media sosial lewat menu berbagi Android agar banyak anggota baru bergabung — ini pengecualian resmi dari aturan tanpa tautan luar: tautannya dibuat oleh aplikasi (domain milik SubSenpai), dibagikan KE LUAR aplikasi, dan membuka halaman unduh aplikasi + undangan grupnya; user tetap tidak bisa menempel tautan sembarang ke dalam chat/komentar. Membuka tautan dengan aplikasi terpasang langsung masuk ke grup — untuk grup privat, tautan itu sendiri adalah undangannya (pemegang tautan bisa bergabung tanpa persetujuan). Pengaman: owner/admin grup bisa **mengatur ulang kode** (tautan lama langsung mati) atau mematikan penerimaan lewat tautan (`invite_enabled`) bila tautan menyebar ke tempat yang salah.
- **Nonton bareng (watch party)** — anggota grup (atau sekumpulan teman) bisa membuka ruang putar tersinkron dari sebuah episode: satu host mengendalikan play/pause/seek dan posisi semua peserta mengikutinya (sinkronisasi lewat kanal WebSocket sementara — sesi ini tidak disimpan sebagai data permanen), dengan chat di samping player. Ruang berakhir saat host menutupnya atau semua peserta keluar.
- Moderasi dua lapis: admin grup mengeluarkan anggota & menghapus pesan di kanal grupnya; grup yang dilaporkan user (`reports` bertarget `group`) ditindak admin platform (peringatan sampai pembubaran) — lihat STRUKTUR_ADMIN.md.

### Level & Lencana Aktivitas
- User mengumpulkan **XP** (`profiles.xp`) dari aktivitas: menonton episode, membaca chapter, mengirim pesan chat, dan memberi komentar — dihitung worker dari tabel riwayat (bukan dari klien, agar tidak bisa diakali).
- XP menaikkan **level** (`profiles.level`, cache yang dihitung ulang worker); level & lencana tampil di profil dan di samping nama pada chat — berdampingan dengan badge Premium/Admin.
- **Pencapaian** — di atas XP/level ada lencana tonggak (`user_achievements`): mis. episode ke-100 ditonton, rentetan 7/30 hari, komentar pertama, koleksi pertama yang diikuti 10 orang. Definisi pencapaian (nama, ikon, syarat) hidup di kode agar mudah ditambah tanpa migrasi; worker yang menganugerahkannya dari data aktivitas dan mengirim notifikasi `system` saat sebuah pencapaian terbuka.
- **Rentetan harian (streak)** — hari aktif (menonton/membaca/chat/komentar) dicatat di `profiles.last_active_date`; worker memperbarui `streak_days` (bertambah bila kemarin aktif, kembali ke 1 bila terputus) dan menampilkannya di profil — alasan kecil untuk membuka aplikasi setiap hari.

## 10. Komentar, Rating & Rekomendasi

- **Komentar per episode & chapter** (`comments`) — kolom komentar di halaman nonton/baca untuk user login; mendukung **balasan satu tingkat** (balasan atas balasan menempel ke komentar induk yang sama, tidak bersarang dalam); bisa dilaporkan (`reports` bertarget `comment`) dan di-soft-delete oleh pemilik/admin, sama seperti pesan chat. Aturan tanpa tautan luar & tanpa promosi yang sama dengan chat berlaku di sini. Tombol **Terjemahkan** (Google Translate gratis dari perangkat user + cache lokal perangkat) juga tersedia di komentar.
- **Rating & ulasan** (`title_ratings`) — user memberi skor 1–10 dan ulasan singkat per judul (satu per user, bisa diubah); agregat skor mengisi `titles.rating` dan ditampilkan di katalog.
- **Rekomendasi "karena kamu menonton ini"** — dihitung sederhana oleh worker dari genre yang sama + pola "user yang menonton X juga menonton Y" (agregat `watch_history`), disajikan sebagai satu blok di beranda lewat endpoint gabungan (Bagian 1) dengan cache — tanpa sistem ML terpisah.
- **Permintaan judul & voting** (`title_requests`) — user bisa meminta judul yang belum ada di katalog dan memberi suara pada permintaan user lain; admin melihat daftar terurut berdasar suara di panel katalog dan menandainya `approved` / `added` (tertaut ke judulnya) / `rejected`. Peminta & pendukung menerima notifikasi saat judul yang mereka dukung benar-benar masuk katalog.
- **Koleksi kurasi user** (`collections`) — user menyusun daftar judul bertema ("Donghua xianxia wajib tonton", "Anime sedih terbaik") sebagai koleksi publik/privat; user lain bisa mengikuti koleksi dan menemukan judul dari kurasi komunitas, bukan hanya algoritma. Koleksi yang melanggar aturan ditangani lewat laporan & moderasi yang sama seperti konten user lain.
- **Rekap Senpai (statistik pribadi)** — halaman statistik per user yang dihitung dari riwayatnya sendiri: total jam menonton, chapter dibaca, genre teratas, judul yang paling lama diikuti, rentetan & level saat ini. Rekap bisa dibagikan ke luar aplikasi sebagai kartu berbagi resmi (gambar rekap dibuat oleh aplikasi) — promosi dari mulut ke mulut tanpa melanggar aturan tautan di dalam aplikasi.

## 11. Keamanan API — Enkripsi Response (`xdata` / `xtime`)

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
- **Rate limit handshake**: endpoint `/api/v1/session` dibatasi per IP (mis. maks 30 handshake/menit) karena operasi ECDH memakan CPU — tanpa batas ini, endpoint bootstrap bisa dipakai menguras CPU VPS.

Kenapa bukan kunci dari signature URL atau kunci statis tertanam: path URL bersifat publik, jadi kunci semacam itu bertumpu pada satu secret induk yang tertanam di APK — sekali terbongkar, seluruh endpoint dan seluruh user terbuka selamanya. Dengan kunci sesi ECDH, membongkar APK tidak menghasilkan kunci apa pun, satu kunci yang bocor hanya membuka satu sesi, dan traffic lama yang terekam tidak bisa didekripsi belakangan karena kunci sementaranya sudah dibuang (*forward secrecy*).

Catatan jujur soal batas lapisan ini: penyerang yang sangat berniat tetap bisa meniru handshake aplikasi dan berbicara ke API seperti aplikasi asli. Lapisan ini berfungsi sebagai **anti-scraping massal**, **integritas**, **anti-replay**, dan **kerahasiaan per sesi** — BUKAN pengganti HTTPS, login, dan gerbang akses di server.

## 12. Deployment (VPS)

Satu VPS menjalankan seluruh aplikasi (spesifikasi awal 1 vCPU / 4 GB RAM — binary Go ringan, tetapi PostgreSQL lokal ikut memakai RAM & disk):

| Komponen | Bentuk | Port lokal |
| --- | --- | --- |
| Reverse proxy | Caddy (HTTPS otomatis Let's Encrypt) | 80/443 (publik) |
| Backend API | Binary Go (systemd `subsenpai-api`) | 8080 |
| Worker | Binary Go kedua dari basis kode yang sama (systemd `subsenpai-worker`) — antrean subtitle, polling GoPay, pembersih sesi | — |
| Database | PostgreSQL lokal di VPS — data di disk VPS (mis. 50 GB NVMe), hanya mendengarkan localhost | 5432 (lokal) |
| Auth | Di dalam backend Go (verifikasi token ID Google + JWT sesi buatan sendiri) | — |
| Berkas | Cloudflare R2 — di luar VPS | — |

- Caddy meneruskan seluruh request (API publik, mis. `api.subsenpai...`) ke backend Go; hanya Caddy yang terekspos ke internet, layanan Go hanya mendengarkan di localhost.
- Domain diarahkan lewat **Cloudflare** (proxy aktif): cache tepi untuk response katalog publik, IP asli VPS tersembunyi, dan proteksi DDoS dasar — gratis.
- Konfigurasi & rahasia backend di satu file env di VPS (mis. `/etc/subsenpai/backend.env`, izin 600): `DATABASE_URL`, kredensial payment gateway & GoPay Merchant, kunci API transkripsi, kunci API ImgBB, kredensial R2. Terjemahan tidak menyentuh server sama sekali (aplikasi memanggil endpoint publik gratis Google Translate langsung dari perangkat user), jadi tidak ada kredensial terjemahan di server. Tidak masuk repo (repo hanya berisi `.env.example`). Kunci sesi `xdata` TIDAK termasuk env/disk — dibuat per sesi dan hidup di memori saja (Bagian 11).
- Alur rilis: `go build` → unggah binary → `systemctl restart subsenpai-api subsenpai-worker`. Pembaruan aplikasi Android lewat rilis APK di website resmi & GitHub Releases (di luar VPS; lihat NOTE.md).
- Cadangan: `pg_dump` database berjalan terjadwal dari VPS (timer systemd, mis. tiap malam) dan hasilnya diunggah ke R2 — database lokal tidak mempunyai pemulihan titik-waktu bawaan, jadi dump ini penyelamat utamanya — dan dump TIDAK boleh hanya disimpan di VPS yang sama: unggahan ke R2 (di luar VPS) adalah bagian wajibnya, karena bila disk VPS rusak, database dan cadangannya tidak boleh hilang bersamaan. File env dan konfigurasi `deploy/` dicadangkan bersama dump. Setiap 3 bulan dilakukan **uji pemulihan**: dump terakhir dicoba dipulihkan ke database kosong dan hasilnya dicatat di modul Monitoring — cadangan yang belum pernah dipulihkan adalah harapan, bukan cadangan.
- Pemantauan: endpoint `GET /healthz` (jawaban polos `{ok: true}`, tanpa enkripsi) dipantau pemantau uptime eksternal gratis (mis. UptimeRobot) agar admin tahu dalam hitungan menit bila VPS atau layanan mati. Aplikasi Android juga mengirim **laporan crash** (Firebase Crashlytics gratis, atau GlitchTip yang di-host sendiri) bertanda versi rilis — error di perangkat user tidak terlihat dari server, jadi laporan crash adalah satu-satunya cara mengetahuinya; ringkasannya ditautkan dari modul Monitoring panel admin.

## 13. Batasan yang Disadari

- Satu VPS adalah titik tunggal kegagalan — bila VPS mati, seluruh layanan berhenti. Mitigasi awal: pantau uptime + restart otomatis systemd; pisahkan worker ke mesin lain bila antrean subtitle membesar.
- Aplikasi saja, tanpa versi web: penemuan konten lewat mesin pencari (SEO) tidak ada — distribusi & pertumbuhan mengandalkan unduhan APK dari website resmi & GitHub Releases serta promosi komunitas (Play Store ditunda — lihat NOTE.md). Sebagai gantinya VPS jauh lebih ringan (komponen terberat sebelumnya, Next.js, hilang).
- Database lokal berbagi nasib dengan VPS: bila VPS atau disknya mati, database ikut mati — dimitigasi cadangan `pg_dump` terjadwal yang wajib terunggah ke R2 (di luar VPS) dan pemantauan sisa disk dari panel admin. Ruang database juga berbagi disk dengan sistem & binary, jadi pertumbuhan tabel besar (pesan chat, riwayat) dipantau; arsip berkala disiapkan bila mulai sesak.
- Sumber konten resmi umumnya berkunci wilayah + DRM; daftar sumber yang benar-benar bisa dipakai dicatat terpisah di `docs/` setelah riset.
- Iklan: konten seperti ini umumnya ditolak AdSense — jaringan iklan alternatif diteliti di fase monetisasi.
- Menerjemahkan komik secara otomatis (OCR + menimpa teks pada gambar) di luar cakupan rancangan ini.
- Gambar kiriman user bergantung pada ImgBB, layanan hosting gambar gratis pihak ketiga tanpa SLA — gambar bisa saja hilang dari sisi mereka. Karena itu ImgBB hanya untuk gambar chat & stiker pribadi (risiko rendah); aset penting (poster, halaman komik, stiker katalog) tetap di R2 yang kita kendalikan.
- Terjemahan memakai endpoint publik **gratis** Google Translate (`client=gtx`, pola D TRADE) yang dipanggil **aplikasi dari perangkat masing-masing user** — server tidak pernah memanggilnya. Alasannya: IP tiap user berbeda (rumahan/seluler) sehingga jarang kena limit dan terbukti lancar di D TRADE, sedangkan IP datacenter VPS rawan throttle/blokir. Konsekuensi yang disadari: endpoint tetap tanpa SLA (gagal → teks/subtitle asli yang tampil), subtitle dalam bahasa baru perlu diterjemahkan dulu di perangkat saat pertama diputar (ada proses "menyiapkan subtitle"; sesudahnya tersimpan di cache lokal), dan hasil terjemahan **tidak terbagi antar-user** — tiap perangkat menerjemahkan salinannya sendiri. Itu harga yang diterima demi biaya nol dan bebas limit server.
