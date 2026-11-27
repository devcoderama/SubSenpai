# SubSenpai Web (SvelteKit)

Versi web SubSenpai — tampilan seperti aplikasi Android, tema Netflix.
Memakai API backend Go yang sama dengan aplikasi
(`https://api.subsenpai.web.id`), termasuk enkripsi envelope `{xdata,
xtime}` (handshake ECDH, AES-256-GCM) dan login Google.

## Jalankan lokal

```sh
cd web
npm install
npm run dev        # http://localhost:5173
```

Build produksi (keluaran statis di `build/`):

```sh
npm run build
npm run preview
```

## Struktur

- `src/lib/api.js` — klien API + kripto sesi (di-port dari
  `web_admin/app.js`, sudah terverifikasi terhadap backend Go).
- `src/lib/auth.js` — JWT di localStorage + profil (login Google).
- `src/lib/components/` — Navbar, Hero, Rail, TitleCard, EpisodeList, Player.
- `src/routes/` — Beranda `/`, Detail `/judul/[id]`, Nonton
  `/nonton/[episodeId]`, Jadwal `/jadwal`, Cari `/cari`, Premium
  `/premium`, Masuk `/masuk`.
- SPA penuh (`ssr: false` di `src/routes/+layout.js`) + adapter-static
  dengan fallback `200.html`, jadi semua rute dilayani shell yang sama.

## Deploy (Netlify)

Buat satu site Netlify baru dari repo ini:

- Base directory: `web`
- Build command: `npm run build`
- Publish directory: `web/build`

`netlify.toml` di folder ini sudah berisi pengaturan itu + redirect SPA
(`/* -> /200.html 200`). CORS backend sudah mengizinkan
`https://*.netlify.app`, jadi tidak perlu ubah backend. Login Google
memakai client ID web yang sama dengan dashboard admin; tambahkan origin
site Netlify baru ke "Authorized JavaScript origins" di Google Cloud
Console bila Google menolak tombol login.

## Catatan

- Aturan premium/watch ditegakkan server (episode pertama gratis,
  sisanya Premium; film khusus Premium). Web hanya menampilkan state
  terkunci — tidak ada bypass.
- Buka-iklan (Monetag, 15 detik) tersedia di halaman Nonton untuk episode
  terkunci, alurnya sama dengan aplikasi (start -> tonton -> complete,
  waktu ditegakkan server).
- Pembayaran Premium di web memakai Tokopay QRIS via endpoint checkout
  yang sama dengan aplikasi; status dicek langsung ke Tokopay.
