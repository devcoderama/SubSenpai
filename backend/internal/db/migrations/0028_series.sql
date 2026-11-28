-- 0028_series.sql — Serial TV (permintaan Rams 2026-11-28):
-- API /database Hashflix yang dipakai film ternyata juga melayani
-- TV (trending/tv, discover/tv, search/tv, details/tv), dan embed
-- TV jalan di semua server (vidstuck/zxcstream/vidbolt/vidfast).
-- 1) CHECK tipe judul + content_type: tambah 'series'.
-- 2) episodes.season_no: episode serial dipisah per season; unik
--    baru (title_id, season_no, episode_no) — tanpa ini S1E1 dan
--    S2E1 bertabrakan di unique (title_id, episode_no) lama.
--    Episode lama (anime/donghua/film) dapat season_no = 1.
-- 3) Registri: baris Hashflix kedua untuk series (resolver sama,
--    content_type 'series') — penemuan/katalog TV jalan lewat sini.
-- Nama constraint dicari lewat definisinya (bukan ditebak) agar
-- migrasi tetap jalan walau nama auto-generated-nya berbeda.
do $$
declare
  cname text;
begin
  select conname into cname from pg_constraint
  where conrelid = 'titles'::regclass and contype = 'c'
    and pg_get_constraintdef(oid) ilike '%(type = any%';
  if cname is not null then
    execute 'alter table titles drop constraint ' || quote_ident(cname);
  end if;
  select conname into cname from pg_constraint
  where conrelid = 'content_sources'::regclass and contype = 'c'
    and pg_get_constraintdef(oid) ilike '%(content_type = any%';
  if cname is not null then
    execute 'alter table content_sources drop constraint ' || quote_ident(cname);
  end if;
  select conname into cname from pg_constraint
  where conrelid = 'episodes'::regclass and contype = 'u'
    and pg_get_constraintdef(oid) like '%(title_id, episode_no)%';
  if cname is not null then
    execute 'alter table episodes drop constraint ' || quote_ident(cname);
  end if;
end $$;

alter table titles add constraint titles_type_check
  check (type in ('anime','donghua','film','series','manga','manhua','manhwa'));

alter table content_sources add constraint content_sources_content_type_check
  check (content_type in ('anime','donghua','film','series','manga','manhua','manhwa'));

alter table episodes add column if not exists season_no int not null default 1;
alter table episodes add constraint episodes_title_season_episode_key
  unique (title_id, season_no, episode_no);

insert into content_sources (name, content_type, lang, base_url, resolver, fetch_mode, subtitle_kind, priority, is_active) values
  ('Hashflix','series','multi','https://hashflix.top','hashflix','server','soft',10,true)
on conflict (resolver, content_type, lang) do nothing;
