# SubSenpai 🎌

**Nonton anime langsung dari sumber Jepang, subtitle Indonesia otomatis.**

SubSenpai adalah aplikasi streaming anime yang mengambil video dari sumber Jepang (raw, tanpa subtitle) lalu membuat subtitle-nya secara otomatis: audio ditranskripsi oleh Whisper, diterjemahkan dengan terjemahan mesin, dan ditampilkan sebagai subtitle di player — tanpa menunggu fansub.

> 🚧 **MASIH DALAM TAHAP PENGEMBANGAN** — repo ini baru berisi rancangan dan struktur awal. Belum ada versi yang bisa dipakai.

## ✨ Fitur yang Direncanakan

- 📺 **Katalog anime** — telusur & cari berdasarkan judul, genre, musim, dan tahun
- ▶️ **Streaming langsung** — video diputar langsung dari sumbernya (HLS) ke player
- 🗣️ **Subtitle otomatis Jepang → Indonesia** — dibuat oleh pipeline Whisper + terjemahan mesin
- ⚡ **Jalur cepat caption** — kalau sumbernya punya closed caption bahasa Jepang, caption langsung diterjemahkan (timing presisi, tanpa transkripsi)
- 👤 **Akun pengguna** — daftar/masuk, watchlist, dan riwayat tontonan (Supabase Auth)
- 🌙 **Mode gelap** — tema bawaan untuk nonton malam

## 🗣️ Cara Kerja Subtitle Otomatis

```
Video raw (Jepang) ──► ekstrak audio ──► Whisper (transkripsi + timestamp)
                                              │
                                              ▼
                        teks Jepang ──► terjemahan mesin ──► file .srt/.ass
                                              │
                                              ▼
                                  tampil sebagai subtitle di player
```

1. **Transkripsi** — audio episode ditranskripsi oleh Whisper (large-v3; WhisperX untuk timing per kata) menjadi teks Jepang bertimestamp.
2. **Terjemahan** — teks diterjemahkan per baris ke bahasa Indonesia (Google Translate API / DeepL / Sugoi Translator untuk jalur offline).
3. **Penyajian** — teks terjemahan + timestamp digabung menjadi file `.srt`/`.ass`, disimpan di object storage, lalu dimuat player sebagai track subtitle.

Catatan jujur: kualitas subtitle mesin adalah "cukup untuk mengikuti cerita", bukan setara fansub manusia — nama karakter, istilah khas, dan lagu pembuka/penutup adalah titik lemahnya.

## 🏗️ Arsitektur

Pemisahan berdasarkan **jenis data**, bukan per genre:

| Bagian | Teknologi | Fungsi |
| --- | --- | --- |
| Frontend + API | Next.js di Vercel | Halaman katalog, player, API tipis |
| Database + Auth | Supabase (PostgreSQL) | Katalog anime, episode, subtitle, data user |
| File storage | Cloudflare R2 / Supabase Storage | Poster & file subtitle (.srt/.ass) |
| Video | Stream langsung dari sumber (HLS) | Tidak lewat server sendiri — hemat bandwidth & durasi |
| Worker subtitle | Python (faster-whisper) di mesin GPU | Transkripsi + terjemahan, menulis hasil ke database/storage |

Detail folder, skema database, dan alur data: lihat **[STRUKTUR.md](STRUKTUR.md)**.

## 🗺️ Roadmap

- [ ] Fase 0 — Rancangan & struktur repo *(sedang berjalan)*
- [ ] Fase 1 — Katalog + halaman detail + player HLS
- [ ] Fase 2 — Worker subtitle otomatis (Whisper → terjemah → SRT)
- [ ] Fase 3 — Akun, watchlist, riwayat tontonan
- [ ] Fase 4 — Pencarian lanjutan, rekomendasi, dan pemolesan UI

## ⚠️ Catatan

Proyek hobi/eksperimen untuk belajar. Video berasal dari sumber pihak ketiga di Jepang; sebagian besar sumber resmi memakai kunci wilayah dan DRM, sehingga ketersediaan sumber adalah tantangan tersendiri. Subtitle dihasilkan mesin dan dapat mengandung kesalahan.
