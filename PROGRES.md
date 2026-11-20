# PROGRES.md — Alur Progres SubSenpai

✓ = selesai sebagai rancangan resmi
- = belum dikerjakan

## Alur Dokumen

✓ [README.md](README.md) → visi, fitur ringkas, model akses, arsitektur ringkas, roadmap, dan donasi
✓ [STRUKTUR.md](STRUKTUR.md) → arsitektur final: aplikasi Android Kotlin + backend Go + PostgreSQL lokal di VPS + worker
✓ [DATABASE.md](DATABASE.md) → skema dasar 36 tabel, ERD, relasi, enum, indeks, dan draf SQL
✓ [STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md) → panel admin di dalam aplikasi Android
✓ [STRUKTUR.md](STRUKTUR.md) + [DATABASE.md](DATABASE.md) + [STRUKTUR_ADMIN.md](STRUKTUR_ADMIN.md) → rekomendasi putaran 1–2 terintegrasi resmi
✓ [FITUR.md](FITUR.md) → fitur resmi F13–F39
✓ [KEAMANAN.md](KEAMANAN.md) → keamanan resmi, Play Integrity O8, gerbang versi minimum O17, dan token integrasi F35
✓ [OPTIMASI.md](OPTIMASI.md) → optimasi resmi O9–O27
✓ [NOTE.md](NOTE.md) → distribusi aplikasi hanya melalui website resmi & GitHub Releases
✓ [PROGRES.md](PROGRES.md) → peta alur progres ini
- [DATABASE.md](DATABASE.md) → sinkronisasi final seluruh perubahan skema F13–F39 dan O8–O27 ke ERD, relasi, enum, indeks, dan DDL
- [DATABASE.md](DATABASE.md) → migrasi awal final dari DDL hasil sinkronisasi

## Alur Pembangunan

- Fase 0 → selesaikan sinkronisasi final DATABASE.md sebelum pembangunan dimulai
- Fase 1 → katalog multi-konten + halaman detail + player HLS
- Fase 2 → worker subtitle bahasa asal (caption/Whisper → SRT asal) + terjemahan subtitle di perangkat user
- Fase 3 → login Google + JWT backend + watchlist + riwayat
- Fase 4 → gerbang freemium + pembayaran + voucher + chat global + teman & grup + DM + nonton bareng
- Fase 5 → mode baca + reader vertikal + sumber perangkat ber-Cloudflare F39 + OCR & terjemahan balon komik F31
- Fase 6 → fitur tambahan F13–F39 + optimasi O9–O27 sesuai prioritas di dokumen masing-masing
- Pengujian menyeluruh → keamanan, pembayaran, subtitle, player, reader, notifikasi, cadangan, dan pemulihan
- Deployment VPS → backend Go + worker + PostgreSQL lokal + Caddy
- Rilis APK bertahap → website resmi + GitHub Releases sesuai O27
- Pasca-rilis → pantau crash, metrik, cadangan, dan uji pemulihan berkala
