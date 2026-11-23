# web_admin/ — Dashboard Admin Web SubSenpai

Dashboard admin SubSenpai: **website terpisah khusus admin** (HTML + CSS +
JavaScript murni, tanpa framework) — bukan bagian dari aplikasi Android,
dan bukan versi web untuk user. Backend-nya **tetap backend Go yang sama**
dengan aplikasi (`https://subsenpai-api.botlynk.id`, grup `/api/v1/admin/*`).

## Isi

- `index.html` — kerangka halaman (login + layout dashboard).
- `style.css` — tema mengikuti situs dokumentasi (latar `#0B0F1A`, aksen
  ungu `#a78bfa`, merah Netflix untuk aksi utama).
- `app.js` — seluruh logika: handshake ECDH + dekripsi envelope xdata
  (X25519 ditulis manual dengan BigInt karena WebCrypto X25519 belum
  merata; HKDF-SHA256 + AES-256-GCM memakai WebCrypto), login Google,
  dan 5 halaman: Ringkasan, Metode Pembayaran, Paket Premium, Pengguna,
  Transaksi.

## Akses & keamanan

- Login hanya **Google** (Google Identity Services) dan hanya akun yang
  `role`-nya **`admin`** di tabel `profiles` yang dibiarkan masuk; akun
  lain ditolak di halaman login, dan semua endpoint admin tetap memeriksa
  JWT + role di server (403 untuk non-admin).
- **Tidak ada pendaftaran** di web ini. Admin baru diangkat oleh admin
  yang sudah ada dari halaman **Pengguna** ("Jadikan Admin"), atau dengan
  mengubah `role` langsung di database.
- JWT disimpan di `sessionStorage` (hilang saat tab ditutup).

## Deploy (Netlify)

Situs Netlify terpisah dari situs dokumentasi:

- Base/publish directory: `web_admin` (tanpa build command).
- Backend mengizinkan CORS untuk origin `https://*.netlify.app`
  (atau daftar eksplisit lewat env `WEB_ORIGINS` di server).

Langkah wajib sekali di **Google Cloud Console** (project yang sama
dengan login aplikasi): OAuth client ID web
`673952283037-…` → **Authorized JavaScript origins** → tambahkan origin
situs admin ini (mis. `https://subsenpai-admin.netlify.app`). Tanpa itu
tombol login Google tidak akan berfungsi dari domain ini.
