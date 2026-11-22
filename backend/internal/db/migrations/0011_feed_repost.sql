-- 0011_feed_repost.sql — Repost (posting ulang) untuk feed
-- komunitas (permintaan Rams 2026-11-22, mengikuti referensi:
-- ikon 🔁 dengan jumlah). Repost adalah baris feed_posts baru yang
-- menunjuk postingan asli lewat repost_of_id; konten, like,
-- komentar, dan bookmark selalu menunjuk postingan asli. Baris
-- repost tidak punya body/image sendiri, jadi batasan konten
-- dilonggarkan untuk baris yang punya repost_of_id.
--
-- Catatan: pg_get_constraintdef menulis "IS NOT NULL" huruf besar,
-- jadi pencarian constraint lama WAJIB ilike (bug yang tertangkap
-- uji asap 2026-11-22: versi like-sensitif tidak menemukan
-- constraint lama dan repost tertolak check "feed_posts_check").

alter table feed_posts add column if not exists repost_of_id uuid
  references feed_posts(id) on delete cascade;
create index if not exists feed_posts_repost_idx on feed_posts (repost_of_id)
  where repost_of_id is not null;

do $$
declare c record;
begin
  for c in
    select conname from pg_constraint
    where conrelid = 'feed_posts'::regclass and contype = 'c'
      and conname <> 'feed_posts_content_check'
      and pg_get_constraintdef(oid) ilike '%body is not null%'
  loop
    execute 'alter table feed_posts drop constraint ' || quote_ident(c.conname);
  end loop;
end $$;

alter table feed_posts drop constraint if exists feed_posts_content_check;
alter table feed_posts add constraint feed_posts_content_check
  check (body is not null or image_url is not null or repost_of_id is not null);
