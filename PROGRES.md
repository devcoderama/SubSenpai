# PROGRES.md — Alur Progres SubSenpai

✓ = selesai sebagai rancangan resmi
- = belum dikerjakan

## Alur Dokumen

✓ [README.md](README.md) → visi, fitur ringkas, model akses, arsitektur ringkas, roadmap, dan donasi
✓ [STRUKTUR.md](STRUKTUR.md) → arsitektur final: aplikasi Android Kotlin + backend Go + PostgreSQL lokal di VPS + worker
✓ [database/DATABASE.md](database/DATABASE.md) → skema dasar 36 tabel, ERD, relasi, enum, indeks, dan draf SQL
✓ [STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md) → panel admin website terpisah khusus admin
✓ [STRUKTUR.md](STRUKTUR.md) + [database/DATABASE.md](database/DATABASE.md) + [STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md) → rekomendasi putaran 1–2 terintegrasi resmi
✓ [FITUR.md](FITUR.md) → fitur resmi F13–F39
✓ [FITUR.md](FITUR.md) + [STRUKTUR.md](STRUKTUR.md) → F40 sumber subtitle siap pakai per bahasa (registri sumber; transkripsi Whisper & terjemahan mesin bukan lagi jalur utama)
✓ [KEAMANAN.md](KEAMANAN.md) → keamanan resmi, Play Integrity O8, gerbang versi minimum O17, dan token integrasi F35
✓ [OPTIMASI.md](OPTIMASI.md) → optimasi resmi O9–O27
✓ [NOTE.md](NOTE.md) → distribusi aplikasi hanya melalui website resmi & GitHub Releases
✓ [PROGRES.md](PROGRES.md) → peta alur progres ini
✓ [android/README.md](android/README.md) → struktur aplikasi Android (folder `android/` di akar repo)
✓ [web_admin/README.md](web_admin/README.md) → struktur panel admin web (folder `web_admin/` di akar repo)
✓ [database/DATABASE.md](database/DATABASE.md) → sinkronisasi final seluruh perubahan skema F13–F39 dan O8–O27 ke ERD, relasi, enum, indeks, dan DDL (52 tabel)
✓ [database/database.sql](database/database.sql) → migrasi awal final dari DDL hasil sinkronisasi (tervalidasi parser PostgreSQL)
✓ [database/database.sql](database/database.sql) → SQL skema final 52 tabel, sinkron dengan DATABASE.md Bagian 7

## Alur Pembangunan

✓ Fase 0 → sinkronisasi final DATABASE.md selesai (52 tabel) — pembangunan bisa dimulai
✓ Fase 1 → katalog multi-konten + halaman detail + player HLS (backend Go + aplikasi Android terbangun & terverifikasi: migrasi 52 tabel jalan, endpoint katalog terenkripsi xdata terbukti hidup, APK debug terkompilasi)
✓ Fase 2 → registri sumber per bahasa + resolver per situs (F40) + migrasi 0002 (content_sources, title_sources, episode_sources.lang) + cadangan terjemahan gtx di perangkat — TERBANGUN & terverifikasi hidup 2026-11-22: migrasi 0002 jalan (13 sumber teregistri), resolver Anichin lulus uji ujung-ke-ujung (judul nyata: 219 episode terpetakan, 20 episode terselesaikan = 79 aliran ber-lang id, terbaca API lewat envelope xdata), cek kesehatan sumber jalan di worker; resolver HiAnime tertulis (alur ajax MegaCloud) tapi domainnya belum terjangkau dari jaringan uji; Kuronime ber-mode device (Cloudflare menolak IP server); aliran Anichin berbentuk embed (OK.ru/Dailymotion/dll.) — ekstraksi aliran langsung per host / pemutar embed adalah penyempurnaan berikutnya; APK 0.2.0 terkompilasi (pemilih bahasa soft/hard + chip Terjemahkan gtx)
✓ Fase 3 → login Google + JWT backend + watchlist + riwayat — TERBANGUN & terverifikasi hidup 2026-11-22: verifikasi token ID Google via JWKS RS256 (uji unit + uji hidup dengan JWKS lokal sebagai pengganti Google), JWT backend HS256, endpoint /me + watchlist (tambah/cek/daftar/hapus) + riwayat tonton/baca per profil tontonan — semua lulus uji ujung-ke-ujung lewat envelope xdata dan endpoint user menolak tanpa login (401); APK 0.3.0 terkompilasi (layar Akun Google Sign-In via Credential Manager, tombol watchlist di detail, blok Lanjutkan + lapor progres dari player). Syarat go-live: GOOGLE_CLIENT_ID di server + GOOGLE_WEB_CLIENT_ID saat build aplikasi diisi dari proyek Google Cloud Rams (tombol masuk nonaktif selama kosong)
✓ Fase 4 → gerbang freemium + pembayaran + voucher + chat global + teman & grup + DM + nonton bareng — TERBANGUN & terverifikasi hidup 2026-11-22: gerbang tamu maks 1 episode + 1 chapter per perangkat (guest_access, header X-Device-Id) lulus uji (200/403/200 ulang); trial premium 3 hari sekali per akun; checkout GoPay nominal unik (harga + kode 1–999) + klien merchant Api-Gopar-Merch & job polling pencocokan di worker (matcher teruji unit; aktivasi sesi bersama di paket billing); webhook Midtrans (tanda tangan SHA512 diverifikasi — salah ditolak, benar mengaktifkan & menumpuk masa premium) dan Xendit (token callback); voucher redeem (hadiah purchase terkunci sampai sesi lunas, penukaran menumpuk premium, sekali pakai), voucher referral 3 hari untuk pengundang + pendaftar baru, batas 2 perangkat premium saat login + cabut perangkat; chat WebSocket /ws (6 kanal topik dari migrasi 0003, grup, DM) dengan filter tautan luar di server, balasan + pratinjau, kehadiran, kustomisasi nama premium; teman via kode teman (minta/terima/blokir), grup publik/privat + gabung via kode undangan, watch party (kode 6 karakter + sinkron play/pause/seek — satu bug panic indeks negatif pada pembangkit kode ditemukan lewat uji hidup dan sudah diperbaiki dengan crypto/rand); APK 0.4.0 terkompilasi (layar Premium, Chat, Teman, Grup, kunci freemium di player, nonton bareng di player). Catatan jujur: checkout Midtrans/Xendit & QRIS/polling GoPay butuh kunci/kredensial asli di env server (tanpa itu endpoint menjawab belum dikonfigurasi / sesi nominal unik tetap dibuat); iklan untuk non-premium saat ini baru berupa penanda access.ads dari API — SDK iklan belum dipasang
- Fase 5 → mode baca + reader vertikal + sumber perangkat ber-Cloudflare F39 + OCR & terjemahan balon komik F31
- Fase 6 → fitur tambahan F13–F39 + optimasi O9–O27 sesuai prioritas di dokumen masing-masing
- Pengujian menyeluruh → keamanan, pembayaran, subtitle, player, reader, notifikasi, cadangan, dan pemulihan
- Deployment VPS → backend Go + worker + PostgreSQL lokal + Caddy
- Rilis APK bertahap → website resmi + GitHub Releases sesuai O27
- Pasca-rilis → pantau crash, metrik, cadangan, dan uji pemulihan berkala
