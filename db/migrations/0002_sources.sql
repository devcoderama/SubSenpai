-- 0002_sources.sql — F40: registri sumber konten per bahasa (Fase 2).
-- Menambah tabel content_sources + title_sources, kolom lang di
-- episode_sources, dan nilai 'sumber'/'vtt' pada CHECK subtitles.
-- Skema 0001 tidak diubah mundur (disiplin O23).

create table content_sources (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  content_type text not null check (content_type in ('anime','donghua','film','manga','manhua','manhwa')),
  lang text not null, -- kode BCP-47 bahasa subtitle sumber ini; 'multi' = softsub multi-trek
  base_url text, -- bisa diganti admin dari panel; situs sumber sering pindah domain
  resolver text not null, -- kunci resolver di kode worker/aplikasi
  fetch_mode text not null default 'server' check (fetch_mode in ('server','device')),
  subtitle_kind text not null default 'hard' check (subtitle_kind in ('soft','hard','none')),
  priority int not null default 100,
  is_active boolean not null default true,
  health text not null default 'unknown' check (health in ('unknown','ok','down')),
  last_checked_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (resolver, content_type, lang)
);

create table title_sources (
  id uuid primary key default gen_random_uuid(),
  title_id uuid not null references titles(id) on delete cascade,
  source_id uuid not null references content_sources(id) on delete cascade,
  source_url text not null, -- halaman judul ini di situs sumber
  source_ref text, -- id/slug internal situs sumber bila ada
  created_at timestamptz not null default now(),
  unique (title_id, source_id)
);
create index title_sources_title_idx on title_sources (title_id);

alter table episode_sources add column lang text;
comment on column episode_sources.lang is 'Bahasa subtitle aliran ini (hardsub); kosong untuk aliran softsub yang treknya di tabel subtitles (F40)';

alter table subtitles drop constraint subtitles_source_check;
alter table subtitles add constraint subtitles_source_check
  check (source in ('caption','whisper','community','sumber'));
alter table subtitles drop constraint subtitles_format_check;
alter table subtitles add constraint subtitles_format_check
  check (format in ('srt','ass','vtt'));
comment on column subtitles.lang is 'Bahasa subtitle ini (BCP-47); sejak F40 berisi trek siap pakai hasil ambilan dari sumber softsub + komunitas (F13)';

-- Registri awal: peta sumber F40 (terverifikasi aktif 2026-11-22). Idempoten.
-- Kuronime ber-fetch_mode 'device': Cloudflare-nya menolak IP server
-- (HTTP 403 terverifikasi dari IP datacenter) — diselesaikan di perangkat
-- user dengan pola F39, bukan oleh worker.
insert into content_sources (name, content_type, lang, base_url, resolver, fetch_mode, subtitle_kind, priority, is_active) values
  ('Kuronime','anime','id','https://kuronime.sbs','kuronime','device','hard',10,true),
  ('HiAnime','anime','multi','https://hianime.to','hianime','server','soft',10,true),
  ('AnimeFLV','anime','es','https://www3.animeflv.net','animeflv','server','hard',10,true),
  ('AnimeVietsub','anime','vi','https://animevietsub.nl','animevietsub','server','hard',10,true),
  ('AGE动漫','anime','zh-CN','https://www.agedm.io','agedm','server','hard',10,true),
  ('AnimeRuka','anime','th','https://animeruka.net','animeruka','server','hard',10,true),
  ('Anilife','anime','ko',null,'anilife','server','hard',10,false),
  ('Anichin','donghua','id','https://anichin.my','anichin','server','hard',10,true),
  ('AnimeXin','donghua','multi','https://animexin.cloud','animexin','server','hard',10,true),
  ('Hashflix','film','multi','https://hashflix.top','hashflix','server','soft',10,true),
  ('ManhuaRMTL','manga','en','https://manhuarmtl.com','manhuarmtl','device','none',10,true),
  ('ManhuaRMTL','manhua','en','https://manhuarmtl.com','manhuarmtl','device','none',10,true),
  ('ManhuaRMTL','manhwa','en','https://manhuarmtl.com','manhuarmtl','device','none',10,true)
on conflict (resolver, content_type, lang) do nothing;
