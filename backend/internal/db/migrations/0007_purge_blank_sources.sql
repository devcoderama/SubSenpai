-- 0007_purge_blank_sources.sql — Bersihkan aliran placeholder
-- "about:blank" dari episode_sources.
--
-- Latar (2026-11-22): Resolve Kuronime versi awal membaca src
-- iframe#iframedc yang ternyata hanya placeholder lazy-load
-- ("about:blank") dan menyimpannya sebagai aliran "Animeku",
-- sehingga pemutar di aplikasi menampilkan halaman putih kosong.
-- Pengurai sudah diperbaiki (URL player.animeku.org dicari di
-- seluruh halaman + placeholder ditolak di semua jalur), tetapi
-- baris yang sudah tersimpan tidak akan diselesaikan ulang oleh
-- worker karena episodenya dianggap sudah punya sumber. Baris
-- placeholder ini dihapus agar sinkronisasi berikutnya
-- menyelesaikan ulang episode terkait dengan pengurai baru.

delete from episode_sources
where url is null
   or url = ''
   or url = 'about:blank'
   or url like 'javascript:%';
