# STRUKTUR.md — SubSenpai

Rancangan struktur proyek: susunan folder, skema database, model akses, dan alur data. Dokumen ini adalah titik awal desain — boleh berubah selama pengembangan, asalkan perubahan besar dicatat di sini.

## 1. Prinsip Desain

- **Satu katalog, banyak tipe konten.** Anime, donghua, film, manga, manhua, manhwa hidup di tabel katalog yang sama dengan kolom `type`. Database TIDAK dipecah per genre maupun per tipe: genre dan tipe hanyalah kolom, karena pencarian lintas keduanya tidak bisa di-JOIN antar-database.
- **Pisah per jenis data.** Metadata di PostgreSQL, berkas (poster, halaman komik, subtitle) di object storage, video tidak disimpan sama sekali.
- **Video tidak pernah disimpan / tidak lewat server sendiri.** Player memutar stream HLS langsung dari sumber; server hanya menyimpan metadata dan URL.
- **Proses berat di luar Vercel.** Antrean subtitle, terjemahan, dan pembangunan SRT dikerjakan Next.js, tetapi transkripsi Whisper dititipkan ke API cloud (Groq/OpenAI) — serverless Vercel tidak punya GPU dan durasinya terbatas. Worker Python mandiri (faster-whisper di mesin GPU) hanya opsi bila nanti perlu full offline.
- **Gerbang akses ditegakkan di server.** Batas tamu/premium dicek di API, bukan sekadar menyembunyikan tombol di UI.
- **Semua serba gratis dulu.** Vercel (hobby) + Supabase (free) + Cloudflare R2 (free 10 GB); biaya variabel hanya API transkripsi (per menit audio) dan payment gateway (potongan per transaksi).

## 2. Struktur Folder

```
SubSenpai/
├── README.md               # Gambaran umum proyek
├── STRUKTUR.md             # Dokumen ini
├── apps/
│   └── web/                # Aplikasi Next.js (deploy ke Vercel)
│       ├── app/
│       │   ├── page.tsx                # Beranda: katalog & sorotan
│       │   ├── browse/[type]/          # Telusur per tipe (anime/donghua/film/manga/...)
│       │   ├── title/[slug]/           # Halaman detail judul + daftar episode/chapter
│       │   ├── watch/[episodeId]/      # Halaman nonton (player + subtitle)
│       │   ├── read/[chapterId]/       # Halaman baca komik (per halaman gambar)
│       │   ├── genre/[genre]/          # Telusur per genre
│       │   ├── search/                 # Pencarian
│       │   ├── watchlist/              # Watchlist user (login)
│       │   ├── premium/                # Halaman langganan & pembayaran
│       │   ├── admin/                  # Panel admin (detail: STRUKTUR_ADMIN.md)
│       │   └── api/                    # API tipis (katalog, akses, subtitle, progres, webhook bayar, admin)
│       ├── components/                 # Komponen UI (kartu judul, player, reader, dsb.)
│       └── lib/                        # Client Supabase, cek akses (gate), util, tipe data
├── worker/
│   └── subtitle/           # Worker subtitle (bagian dari Next.js / job terpisah)
│       ├── queue.ts        # Ambil antrean episode tanpa subtitle
│       ├── transcribe.ts   # Panggil API transkripsi (Groq/OpenAI Whisper)
│       ├── translate.ts    # Terjemahan per baris (Google/DeepL)
│       └── build-srt.ts    # Gabung teks + timing -> .srt/.ass, unggah ke R2
├── supabase/
│   ├── migrations/         # Migrasi skema PostgreSQL
│   └── seed/               # Data awal (daftar genre, tipe, dsb.)
└── docs/                   # Catatan tambahan (sumber data, riset iklan, dsb.)
```

Catatan: `worker/subtitle` versi awal berjalan sebagai job yang dipicu dari Next.js (cron/route internal). Versi Python (faster-whisper + Sugoi, full offline) disimpan sebagai opsi cadangan di folder yang sama bila diperlukan.

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
- `profiles` — 1:1 dengan user: nama tampil, avatar, `is_premium` (cache status), `premium_until`, `role` (`user`/`admin` — penanda akses panel admin).
- `subscriptions` — user_id, plan, status (`active`/`expired`/`cancelled`), current_period_end, provider (`midtrans`/`xendit`), provider_ref.
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

Pengelolaan & pemantauan berjalan di route `/admin` dalam aplikasi yang sama, khusus user ber-`role = 'admin'` (dicek di server, bukan sekadar menu disembunyikan). Modulnya: dashboard ringkasan, manajemen katalog, manajemen subtitle (antrean + retry), manajemen user, langganan & pembayaran, laporan user, monitoring, dan pengaturan platform. Rancangan lengkapnya: **[STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md)**.

## 6. Alur Data

1. **Katalog masuk** — metadata judul & episode/chapter ditulis ke Supabase; poster & halaman komik diunggah ke R2.
2. **User membuka halaman nonton/baca** — API mengecek gerbang akses (Bagian 4) → Next.js membaca metadata dari Supabase → player memutar `video_url` langsung dari sumber / reader memuat gambar halaman dari R2.
3. **Subtitle diminta** (video saja) — player mengecek tabel `subtitles`:
   - Sudah `done` → muat file dari R2, selesai.
   - Belum ada → buat baris `queued`; worker memprosesnya.
4. **Worker memproses antrean** — unduh audio → (ada caption bahasa asal? terjemahkan langsung : transkripsi via API Whisper) → terjemahkan → bangun .srt → unggah ke R2 → ubah status jadi `done`.
5. **Progres** — player/reader melaporkan posisi terakhir ke API → `watch_history` / `read_history` (user login).
6. **Premium** — user membayar via Midtrans/Xendit → webhook memperbarui `subscriptions` + `profiles.is_premium` → iklan berhenti tampil.

## 7. Batasan yang Disadari

- Fungsi serverless Vercel punya batas durasi & bandwidth — tidak dipakai untuk streaming video maupun menjalankan model Whisper.
- Project Supabase gratis bisa di-pause bila lama tidak aktif — aktivitas berkala menjaganya tetap bangun.
- Sumber konten resmi umumnya berkunci wilayah + DRM; daftar sumber yang benar-benar bisa dipakai dicatat terpisah di `docs/` setelah riset.
- Iklan: konten seperti ini umumnya ditolak AdSense — jaringan iklan alternatif diteliti di fase monetisasi.
- Menerjemahkan komik secara otomatis (OCR + menimpa teks pada gambar) di luar cakupan rancangan ini.
