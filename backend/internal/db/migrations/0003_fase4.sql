-- 0003_fase4.sql — Fase 4: kolom paket pada sesi pembayaran + kanal chat
-- topik bawaan (STRUKTUR: Umum, Anime, Donghua, Film, Manga, Rekomendasi).

alter table payment_sessions add column if not exists plan text;

insert into chat_channels (slug, name, kind, sort_order) values
  ('umum','Umum','global',1),
  ('anime','Anime','global',2),
  ('donghua','Donghua','global',3),
  ('film','Film','global',4),
  ('manga','Manga','global',5),
  ('rekomendasi','Rekomendasi','global',6)
on conflict (slug) do nothing;
