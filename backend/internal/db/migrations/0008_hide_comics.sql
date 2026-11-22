-- 0008_hide_comics.sql — Cabut manga/manhua/manhwa dari katalog.
--
-- Latar (2026-11-22, keputusan Rams): fitur komik dihapus dari
-- aplikasi untuk sekarang — fokus anime, donghua, film. Aplikasi
-- v0.8.2 sudah membuang seluruh UI komik, tetapi judul komik yang
-- telanjur ditemukan worker dari API manhuarmtl (23 judul: 2 manga,
-- 1 manhua, 20 manhwa — termasuk "Fallen to Paradise") masih muncul
-- di Beranda lewat umpan tren/terbaru backend karena feed itu tidak
-- membedakan jenis.
--
-- Migrasi ini: (1) menonaktifkan sumber komik agar worker berhenti
-- menemukan/menyinkronkan/memerkaya judul komik; (2) menghapus
-- baris judul komik dari katalog — semua FK ke titles berstatus
-- on delete cascade / set null, jadi episode, pemetaan sumber,
-- watchlist, dan cache tren terkait ikut bersih. Bila komik
-- dihidupkan lagi nanti, aktifkan sumbernya dan penemuan worker
-- akan mengisi ulang katalog dari API sumber.

update content_sources
set is_active = false
where content_type in ('manga', 'manhua', 'manhwa');

delete from titles
where type in ('manga', 'manhua', 'manhwa');
