# SubSenpai 🎌

**Anime, donghua, film, dan komik dalam satu aplikasi — konten video dengan subtitle Indonesia otomatis.**

SubSenpai adalah aplikasi streaming & baca: anime dan donghua dari sumber asalnya (raw, tanpa subtitle) dengan subtitle yang dibuat otomatis lewat pipeline Whisper + terjemahan mesin, film, serta manga/manhua/manhwa untuk dibaca. Tamu bisa mencicipi 1 episode/chapter; sisanya dibuka lewat login — gratis dengan iklan, atau premium tanpa iklan.

> 🚧 **MASIH DALAM TAHAP PENGEMBANGAN** — repo ini baru berisi rancangan dan struktur awal. Belum ada versi yang bisa dipakai.

## ✨ Fitur yang Direncanakan

- 📺 **Katalog multi-konten** — anime, donghua, film, manga, manhua, manhwa; telusur & cari berdasarkan judul, tipe, genre, musim, dan tahun
- ▶️ **Streaming langsung** — video diputar langsung dari sumbernya (HLS) ke player
- 📖 **Mode baca** — chapter komik per halaman gambar, nyaman di HP
- 🗣️ **Subtitle otomatis → Indonesia** — untuk konten video, dibuat oleh pipeline Whisper + terjemahan mesin
- ⚡ **Jalur cepat caption** — kalau sumbernya punya closed caption bahasa asal, caption langsung diterjemahkan (timing presisi, tanpa transkripsi)
- 👤 **Akun pengguna** — masuk **wajib dengan Google** (anti-bot; tanpa form email/password), watchlist, dan riwayat tontonan/bacaan (Supabase Auth)
- 🎟️ **Freemium** — tamu: 1 episode/chapter; login gratis: lanjut dengan iklan; premium: tanpa iklan, semua terbuka
- 🎁 **Voucher premium** — admin membagikan kode promo; user juga bisa membeli voucher untuk dihadiahkan ke user lain. Kode sekali pakai, ditukar menjadi hari premium
- 💬 **Chat Global** — obrolan realtime per kanal topik (Umum, Anime, Donghua, Manga, Rekomendasi) untuk user login. User **premium** bisa mengustomisasi warna nickname, memasang emoji di nama, dan memakai stiker eksklusif. Semua user bisa kirim gambar & stiker pribadi — dihosting via ImgBB, yang disimpan di database hanya URL-nya. Ada balasan pesan (reply) ala media sosial, berbagi hanya lewat kartu berbagi resmi aplikasi, dan tautan luar/promosi dilarang di chat & komentar
- 👥 **Teman & Grup** — tambah teman lewat kode teman, buat grup publik/privat dengan kanal chatnya sendiri, undang teman, dan kelola anggota dengan peran owner/admin/member
- 🔔 **Notifikasi pintar** — user mengaktifkan lonceng di judul favoritnya (anime/donghua/film/manga/manhua/manhwa) agar tidak ketinggalan episode/chapter baru (push FCM + kotak masuk di aplikasi); admin juga bisa mengirim pengumuman & promo manual, mis. event voucher
- ⭐ **Komentar, rating & rekomendasi** — komentar per episode/chapter, rating 1–10 dari user, dan blok rekomendasi "karena kamu menonton ini"
- ⬇️ **Download offline (premium)** — unduh episode/chapter terenkripsi ke perangkat untuk dinikmati tanpa internet; referral teman juga memberi hari premium untuk kedua pihak
- 🔐 **Response API terenkripsi** — semua response JSON backend dibungkus envelope `xdata`/`xtime` (AES-256-GCM dengan **kunci sesi sekali pakai hasil tukar kunci ECDH** — tidak ada kunci yang tertanam di aplikasi) dan baru dibuka di aplikasi — anti-scraping, anti-tamper, anti-replay (detail di STRUKTUR.md Bagian 11)
- 🌙 **Mode gelap** — tema bawaan untuk nonton malam

## 🗣️ Cara Kerja Subtitle Otomatis

```
Video raw ──► ekstrak audio ──► Whisper (transkripsi + timestamp)
                                       │
                                       ▼
                 teks bahasa asal ──► terjemahan mesin ──► file .srt/.ass
                                       │
                                       ▼
                           tampil sebagai subtitle di player
```

1. **Transkripsi** — audio ditranskripsi oleh Whisper (large-v3; WhisperX untuk timing per kata) menjadi teks bertimestamp.
2. **Terjemahan** — teks diterjemahkan per baris ke bahasa Indonesia (Google Translate API / DeepL / Sugoi Translator untuk jalur offline).
3. **Penyajian** — teks terjemahan + timestamp digabung menjadi file `.srt`/`.ass`, disimpan di object storage, lalu dimuat player sebagai track subtitle.

Catatan jujur: kualitas subtitle mesin adalah "cukup untuk mengikuti cerita", bukan setara fansub manusia — nama karakter, istilah khas, dan lagu pembuka/penutup adalah titik lemahnya. Subtitle otomatis hanya untuk konten video; menerjemahkan komik (OCR + menimpa teks pada gambar) adalah persoalan terpisah yang jauh lebih berat.

## 🎟️ Model Akses (Freemium)

| Pengguna | Akses |
| --- | --- |
| Tamu (belum login) | Maksimal **1 episode / 1 chapter** |
| Login gratis | Semua katalog, **dengan iklan** |
| Premium (langganan) | Semua katalog, **tanpa iklan** |

Gerbang akses ditegakkan di server (API), bukan sekadar disembunyikan di tampilan. Pembayaran premium direncanakan lewat payment gateway lokal (Midtrans/Xendit — bisa QRIS). Alternatif bootstrap: GoPay Merchant lewat API pihak ketiga (QRIS dinamis + polling mutasi, tanpa webhook — alur lengkap & risikonya di STRUKTUR.md Bagian 7). Premium juga bisa diperoleh dari **voucher**: kode promo dari admin, atau voucher yang dibeli user untuk dihadiahkan ke user lain. Jaringan iklan untuk konten seperti ini umumnya bukan AdSense; pilihannya diteliti di fase monetisasi.

## 🏗️ Arsitektur

Pemisahan berdasarkan **jenis data**, bukan per genre:

| Bagian | Teknologi | Fungsi |
| --- | --- | --- |
| Klien | Aplikasi Android (Kotlin) di perangkat user | Katalog, player, reader, Chat Global, panel admin — semua tampilan & interaksi; tidak ada versi web |
| Backend API | **Go** di VPS (binary + systemd, di belakang Caddy) | Semua logika: katalog, gerbang akses, subtitle, chat, premium, voucher, admin + enkripsi response `xdata`/`xtime` |
| Database + Auth | Supabase (PostgreSQL) | Katalog multi-konten, episode/chapter, subtitle, user, langganan |
| File storage | Cloudflare R2 / Supabase Storage | Poster, halaman komik & file subtitle (.srt/.ass) |
| Video | Stream langsung dari sumber (HLS) | Tidak lewat server sendiri — hemat bandwidth |
| Worker subtitle | Job latar Go (antrean + terjemah + SRT) + API transkripsi cloud | Transkripsi dititipkan ke API Whisper (Groq/OpenAI); worker Python mandiri hanya bila perlu full offline |
| Pembayaran | Midtrans / Xendit + GoPay Merchant API pihak ketiga | Langganan premium & voucher (QRIS, e-wallet, transfer). Jalur GoPay: QRIS dinamis + konfirmasi polling mutasi (tanpa webhook) — [Api-Gopar-Merch](https://github.com/ZeroByte000/Api-Gopar-Merch) |
| Panel admin | Route `/admin` (role admin) | Kelola katalog, subtitle, user, langganan + monitoring — [STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md) |

Detail folder, skema database, model akses, dan alur data: lihat **[STRUKTUR.md](STRUKTUR.md)**. Diagram ERD dan draf SQL database: lihat **[DATABASE.md](DATABASE.md)**. Panel pengelolaan & pemantauan: lihat **[STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md)**. Rekomendasi putaran 2 (belum keputusan): lihat **[REKOMENDASI.md](REKOMENDASI.md)**.

## 🗺️ Roadmap

- [ ] Fase 0 — Rancangan & struktur repo *(sedang berjalan)*
- [ ] Fase 1 — Katalog multi-konten + halaman detail + player HLS
- [ ] Fase 2 — Worker subtitle otomatis (transkripsi → terjemah → SRT)
- [ ] Fase 3 — Akun, watchlist, riwayat
- [ ] Fase 4 — Gerbang freemium (tamu 1 episode) + iklan + premium + Chat Global, teman & grup
- [ ] Fase 5 — Mode baca manga/manhua/manhwa (termasuk mode gulir vertikal untuk manhwa/manhua)
- [ ] Fase 6 — Pencarian lanjutan, rekomendasi, dan pemolesan UI

## ⚠️ Catatan

Proyek hobi/eksperimen untuk belajar. Konten berasal dari sumber pihak ketiga; sebagian besar sumber resmi memakai kunci wilayah dan DRM, sehingga ketersediaan sumber adalah tantangan tersendiri. Subtitle dihasilkan mesin dan dapat mengandung kesalahan.
