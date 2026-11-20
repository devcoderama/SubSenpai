# STRUKTUR.md — SubSenpai

Rancangan struktur proyek: susunan folder, skema database, dan alur data. Dokumen ini adalah titik awal desain — boleh berubah selama pengembangan, asalkan perubahan besar dicatat di sini.

## 1. Prinsip Desain

- **Pisah per jenis data, bukan per genre.** Genre hanyalah kolom di tabel anime. Database tidak dipecah per genre karena satu anime bisa punya banyak genre dan pencarian lintas genre tidak bisa di-JOIN antar-database.
- **Video tidak pernah disimpan / tidak lewat server sendiri.** Player memutar stream HLS langsung dari sumber; server hanya menyimpan metadata dan URL.
- **Proses berat di luar Vercel.** Transkripsi Whisper + terjemahan berjalan di worker terpisah (mesin GPU); Vercel hanya melayani frontend + API tipis.
- **Semua serba gratis dulu.** Vercel (hobby) + Supabase (free) + Cloudflare R2 (free 10 GB).

## 2. Struktur Folder

```
SubSenpai/
├── README.md               # Gambaran umum proyek
├── STRUKTUR.md             # Dokumen ini
├── apps/
│   └── web/                # Aplikasi Next.js (deploy ke Vercel)
│       ├── app/
│       │   ├── page.tsx            # Beranda: katalog & sorotan
│       │   ├── anime/[slug]/       # Halaman detail anime + daftar episode
│       │   ├── watch/[episodeId]/  # Halaman nonton (player + subtitle)
│       │   ├── genre/[genre]/      # Telusur per genre
│       │   ├── search/             # Pencarian
│       │   ├── watchlist/          # Watchlist user (login)
│       │   └── api/                # API tipis (katalog, subtitle, progres)
│       ├── components/             # Komponen UI (kartu anime, player, dsb.)
│       └── lib/                    # Client Supabase, util, tipe data
├── worker/
│   └── subtitle/           # Worker subtitle otomatis (Python, mesin GPU)
│       ├── transcribe.py   # faster-whisper: audio -> teks Jepang + timestamp
│       ├── translate.py    # Terjemahan per baris (Google/DeepL/Sugoi)
│       ├── build_srt.py    # Gabung teks + timing -> .srt/.ass
│       └── pipeline.py     # Antrean episode: ambil -> proses -> unggah -> catat DB
├── supabase/
│   ├── migrations/         # Migrasi skema PostgreSQL
│   └── seed/               # Data awal (daftar genre, dsb.)
└── docs/                   # Catatan tambahan (sumber data, riset, dsb.)
```

## 3. Skema Database (Supabase / PostgreSQL)

### `anime`
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | |
| slug | text (unik) | Untuk URL: `/anime/<slug>` |
| title | text | Judul romaji/Inggris |
| title_jp | text | Judul asli Jepang |
| synopsis | text | Sinopsis (Indonesia bila ada) |
| poster_url | text | URL poster (di R2) |
| year | int | Tahun rilis |
| season | text | `winter` / `spring` / `summer` / `fall` |
| status | text | `ongoing` / `completed` |
| genres | text[] | Daftar genre — kolom biasa, BUKAN database terpisah |
| rating | numeric | Skor rata-rata (opsional) |
| created_at | timestamptz | |

### `episodes`
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | |
| anime_id | uuid (FK → anime) | |
| episode_no | int | Nomor episode |
| title | text | Judul episode (opsional) |
| video_url | text | URL stream HLS dari sumber |
| duration_sec | int | Durasi (opsional) |
| aired_at | date | Tanggal tayang di Jepang |

### `subtitles`
| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| id | uuid (PK) | |
| episode_id | uuid (FK → episodes) | |
| lang | text | Kode bahasa, mis. `id` |
| source | text | `caption` (dari CC Jepang) / `whisper` (transkripsi) |
| format | text | `srt` / `ass` |
| file_url | text | URL file subtitle (di R2) |
| status | text | `queued` / `processing` / `done` / `failed` |

### Data user (Supabase Auth)
- Akun dikelola **Supabase Auth** (`auth.users`) — tidak buat tabel kredensial sendiri.
- `profiles` — 1:1 dengan user: nama tampil, avatar.
- `watchlist` — user_id + anime_id.
- `watch_history` — user_id + episode_id + posisi terakhir (detik) + updated_at.

## 4. Alur Data

1. **Katalog masuk** — data anime & episode (metadata + URL HLS) ditulis ke Supabase; poster diunggah ke R2.
2. **User membuka halaman nonton** — Next.js membaca metadata episode dari Supabase; player memutar `video_url` langsung dari sumber.
3. **Subtitle diminta** — player mengecek tabel `subtitles`:
   - Sudah `done` → muat file dari R2, selesai.
   - Belum ada → buat baris `queued`; worker yang memprosesnya.
4. **Worker memproses antrean** — unduh audio → (ada caption Jepang? terjemahkan langsung : transkripsi Whisper) → terjemahkan → bangun .srt → unggah ke R2 → ubah status jadi `done`.
5. **Progres tontonan** — player melaporkan posisi terakhir ke API → `watch_history` (hanya user login).

## 5. Batasan yang Disadari

- Fungsi serverless Vercel punya batas durasi & bandwidth — tidak dipakai untuk streaming video maupun proses Whisper.
- Project Supabase gratis bisa di-pause bila lama tidak aktif — worker/aktivitas berkala menjaganya tetap bangun.
- Sumber video resmi Jepang umumnya berkunci wilayah + DRM; daftar sumber yang benar-benar bisa dipakai dicatat terpisah di `docs/` setelah riset.
