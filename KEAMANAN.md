# KEAMANAN.md — Rancangan Keamanan SubSenpai

Dokumen keamanan **resmi** SubSenpai: merangkum lapisan keamanan yang sudah dirancang di dokumen lain (dengan rujukannya) dan merinci keamanan gelombang ketiga — terutama **O8 Play Integrity** dari REKOMENDASI.md putaran 3 yang disetujui 20 Nov 2026. Pendamping [STRUKTUR.md](STRUKTUR.md), [DATABASE.md](DATABASE.md), [FITUR.md](FITUR.md), dan [OPTIMASI.md](OPTIMASI.md).

---

## 1. Lapisan yang Sudah Dirancang (Ringkasan + Rujukan)

- **HTTPS di semua jalur** — Caddy di depan backend Go (STRUKTUR.md, Deployment).
- **Respons API terenkripsi `xdata`/`xtime`** — AES-256-GCM dengan kunci sesi ECDH sekali pakai (handshake `POST /api/v1/session`, tanpa kunci tertanam di aplikasi), anti-scraping/integritas/anti-replay; batas jujurnya tercatat di STRUKTUR.md Bagian 11: ini bukan pengganti HTTPS, login, dan gerbang server.
- **Login hanya Google** — tanpa pendaftaran email/kata sandi (menaikkan biaya akun bot); backend memverifikasi token ID Google lalu menerbitkan **JWT sesi backend** sendiri.
- **Database lokal VPS hanya mendengarkan localhost** — hanya backend & worker yang memegang kredensial DB; kepemilikan data ditegakkan di kueri backend dari JWT terverifikasi (DATABASE.md Bagian 6).
- **Rahasia hanya di env server** (`/etc/subsenpai/backend.env`, izin 600): kredensial payment gateway & GoPay Merchant, kunci API transkripsi, kunci API ImgBB, kredensial R2. Kunci ImgBB tidak pernah dikirim ke aplikasi; `delete_url` ImgBB disimpan server saja.
- **Rate limit** pada API dan pada handshake sesi ECDH; login & aksi sensitif dibatasi lebih ketat.
- **Gerbang premium selalu di server** — aplikasi tidak pernah menjadi penentu akses premium, voucher, atau unduhan.
- **Larangan tautan eksternal & promosi** di chat dan komentar (ditolak server sebelum disimpan) + filter kata; satu-satunya tautan keluar adalah mekanisme resmi aplikasi (kartu bagikan, tautan undangan grup).
- **Panel admin**: menu hanya tampil untuk `role='admin'`, setiap aksi diperiksa ulang di backend, dan semua tindakan tercatat di `admin_logs`.
- **Enkripsi cadangan**: cadangan database di R2 hanya berguna dengan kredensial R2 yang juga server-side; uji pemulihan berkala memastikan cadangan benar-benar bisa dipakai.

## 2. O8 — Play Integrity untuk Aksi Sensitif

Memastikan permintaan aksi bernilai datang dari **aplikasi resmi yang tidak dimodifikasi** di perangkat yang lolos pemeriksaan Google — menekan kloningan APK & penyalahgunaan API.

**Alur**
1. Sebelum aksi sensitif, aplikasi meminta token Play Integrity dengan *nonce* yang diterbitkan server untuk sesi itu (token terikat nonce, sekali pakai, berumur pendek).
2. Aplikasi menyertakan token pada permintaan aksi; backend memverifikasi token ke Google (keputusan integritas aplikasi + perangkat) sebelum memproses.
3. Hasil verifikasi dicatat seperlunya untuk audit (bukan token mentahnya).

**Cakupan aksi (yang wajib lolos)**
- Klaim/penukaran voucher & penerimaan hadiah premium.
- Pembuatan sesi unduhan premium (termasuk unduhan otomatis F21).
- Kontribusi terjemahan (F13) dan impor daftar (F16) — aksi unggah bernilai yang rawan spam.

**Kebijakan gagal**
- Penjelajahan & pemutaran biasa **tidak** diganggu pemeriksaan ini.
- Aksi sensitif yang gagal verifikasi ditolak dengan pesan ramah di aplikasi ("perangkat/aplikasi tidak lolos pemeriksaan keamanan") — bukan pesan kesalahan teknis.
- **Saklar darurat admin**: bila layanan Google bermasalah massal, admin bisa melonggarkan pemeriksaan untuk sementara dari pengaturan platform; pelonggaran selalu tercatat di `admin_logs` dan otomatis berakhir (ada batas waktu).

## 3. Keamanan Unggahan & Berkas dari User

- **Kontribusi subtitle (F13)** — berkas diperlakukan sebagai data, tidak pernah dieksekusi/dijalankan: pemeriksaan format SRT ketat (jumlah blok & timestamp harus sama dengan asal, tanpa tautan/HTML), batas ukuran, dan varian komunitas bisa ditarik admin kapan pun. Hak kontribusi bisa dicabut per user.
- **Impor daftar MAL/AniList (F16)** — parser XML berjalan dengan **entitas eksternal dimatikan** (anti-XXE), berkas JSON divalidasi terhadap skema yang diharapkan, ada batas ukuran berkas & jumlah entri, dan hasil parsing tidak pernah menulis di luar tabel milik user yang mengimpor.
- **Gambar chat & stiker pribadi** — tetap lewat backend (kompresi + buang EXIF) lalu ImgBB, seperti dirancang di STRUKTUR.md; aplikasi tidak pernah menerima kunci API ImgBB.
- **Semua unggahan** tunduk pada rate limit per user dan pencatatan pelaku.

## 4. Moderasi & Pencegahan Penyalahgunaan

- **Moderator komunitas (F22)** — hak minimal (hapus pesan & bisukan sementara, hanya kanal global), semua tindakan tercatat di `admin_logs` dengan pelaku moderator, bisa dicabut admin kapan pun; penyalahgunaan = pencabutan + sanksi akun.
- **Batas-batas anti-spam** yang sudah dirancang tetap berlaku dan diperluas ke fitur baru: batas kontribusi & laporan per user per hari (F13/F18), batas pembuatan grup/permintaan judul, dan filter kata + larangan tautan di semua permukaan teks.
- **Jam tenang & broadcast bertahap** (F20 + pengiriman FCM bertahap) juga berfungsi sebagai rem penyalahgunaan notifikasi dari sisi platform sendiri.

## 5. Batasan Jujur

- Play Integrity **menaikkan biaya serangan, bukan tembok mutlak**: perangkat yang di-root atau ditambal secara khusus bisa saja lolos sebagian pemeriksaan. Karena itu pemeriksaan ini adalah lapisan tambahan — gerbang nilai (premium, voucher, pembayaran) tetap selalu diverifikasi di server dan tidak pernah bergantung pada integritas klien saja.
- Enkripsi `xdata` membatasi scraping massal dan replay, tetapi penyerang yang tekun tetap bisa meniru handshake dari aplikasi hasil rekayasa balik; pertahanan utama terhadap penyalahgunaan tetap rate limit, gerbang login/premium di server, dan Play Integrity di atas.
