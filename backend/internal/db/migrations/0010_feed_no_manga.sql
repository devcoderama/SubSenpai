-- 0010_feed_no_manga.sql — Kategori Manga dihapus dari feed
-- komunitas (keputusan Rams 2026-11-22, menyusul penghapusan komik
-- dari aplikasi): kategori postingan hanya umum, rekomendasi,
-- anime, donghua, film. Postingan berkategori manga yang sudah ada
-- dipindah ke umum agar lolos batasan baru. Nama constraint dicari
-- dari katalog (ilike — pg_get_constraintdef tidak menjamin huruf
-- kecil) agar migrasi tidak bergantung pada nama bawaan.

update feed_posts set category = 'umum' where category = 'manga';

do $$
declare c record;
begin
  for c in
    select conname from pg_constraint
    where conrelid = 'feed_posts'::regclass and contype = 'c'
      and conname <> 'feed_posts_category_check'
      and pg_get_constraintdef(oid) ilike '%category%'
  loop
    execute 'alter table feed_posts drop constraint ' || quote_ident(c.conname);
  end loop;
end $$;

alter table feed_posts drop constraint if exists feed_posts_category_check;
alter table feed_posts add constraint feed_posts_category_check
  check (category in ('umum', 'rekomendasi', 'anime', 'donghua', 'film'));
