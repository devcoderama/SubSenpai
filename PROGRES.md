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
- Fase 2 → worker subtitle bahasa asal (caption/Whisper → SRT asal) + terjemahan subtitle di perangkat user
- Fase 3 → login Google + JWT backend + watchlist + riwayat
- Fase 4 → gerbang freemium + pembayaran + voucher + chat global + teman & grup + DM + nonton bareng
- Fase 5 → mode baca + reader vertikal + sumber perangkat ber-Cloudflare F39 + OCR & terjemahan balon komik F31
- Fase 6 → fitur tambahan F13–F39 + optimasi O9–O27 sesuai prioritas di dokumen masing-masing
- Pengujian menyeluruh → keamanan, pembayaran, subtitle, player, reader, notifikasi, cadangan, dan pemulihan
- Deployment VPS → backend Go + worker + PostgreSQL lokal + Caddy
- Rilis APK bertahap → website resmi + GitHub Releases sesuai O27
- Pasca-rilis → pantau crash, metrik, cadangan, dan uji pemulihan berkala
