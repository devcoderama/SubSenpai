-- 0012_feed_visibility.sql — Visibilitas postingan feed
-- (permintaan Rams 2026-11-22): publik (semua pengguna login),
-- teman (hanya penulis + teman yang diterima), pribadi (hanya
-- penulis). Postingan lama bawaan 'publik'. Gerbang visibilitas
-- menilai penulis postingan ASLI (op), jadi repost tidak bisa
-- membocorkan postingan teman/pribadi ke yang bukan haknya.

alter table feed_posts add column visibility text not null default 'publik'
  check (visibility in ('publik', 'teman', 'pribadi'));
