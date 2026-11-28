-- 0029_tv_channels.sql — TV Langsung / IPTV (permintaan Rams 2026-11-28):
-- Kanal TV live (HLS .m3u8) yang diputar langsung dari HP pengguna ke
-- sumber stream — VPS tidak men-proxy video, jadi bandwidth VPS aman.
-- Label stabilitas dikurasi dari pengecekan langsung 2026-11-28:
--   'bagus'        = stream resmi (domain pemilik channel) + HTTPS +
--                    menjawab 200 + playlist #EXTM3U saat dicek.
--   'tidak stabil' = restream dari IP mentah (satu kotak 202.150.172.59
--                    melayani RCTI/SCTV/GTV/TVRI-HD sekaligus — mati satu
--                    mati semua; HTTP pula, jadi di web HTTPS ke-block
--                    mixed content), atau belum terverifikasi dari VM
--                    (BeritaSatu, Al Jazeera, France 24: koneksi gagal —
--                    mungkin geo-block / blokir IP datacenter; health-check
--                    di VPS Indonesia yang akan menentukan).
-- Health-check worker (jobs.CheckTVChannels, tiap 30 mnt) mem-probe
-- tiap kanal: 3x gagal beruntun → is_active=false (sembunyi otomatis);
-- begitu hidup lagi → aktif kembali sendiri. Tanpa panel admin v1.
create table if not exists tv_channels (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  slug text not null unique,
  logo_url text,
  stream_url text not null,
  country text not null default 'ID',
  category text not null default 'Nasional',
  quality text,
  stability text not null default 'bagus'
    check (stability in ('bagus', 'tidak stabil')),
  source text not null default 'resmi'
    check (source in ('resmi', 'restream', 'belum terverifikasi')),
  http_referer text,
  http_user_agent text,
  is_active boolean not null default true,
  sort_order integer not null default 0,
  last_ok_at timestamptz,
  last_checked_at timestamptz,
  fail_count integer not null default 0,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index if not exists tv_channels_active_idx
  on tv_channels (is_active, sort_order);

-- Seed kurasi v1 (23 kanal). ON CONFLICT DO NOTHING: seed aman
-- diulang (installer ulang / migrasi ulang tidak menduplikasi).
insert into tv_channels
  (name, slug, logo_url, stream_url, country, category, quality,
   stability, source, http_referer, http_user_agent, sort_order)
values
  -- -- -- BAGUS: stream resmi, HTTPS, 200 + #EXTM3U terverifikasi -- -- --
  ('Trans TV', 'transtv',
   'https://upload.wikimedia.org/wikipedia/en/thumb/6/62/Trans_TV_2013.svg/960px-Trans_TV_2013.svg.png',
   'https://live.cnbcindonesia.com/transtv/smil:transtv.smil/playlist.m3u8',
   'ID', 'Nasional', '720p', 'bagus', 'resmi', null, null, 10),
  ('Trans7', 'trans7',
   'https://i.imgur.com/fAbGImS.png',
   'https://live.cnbcindonesia.com/trans7/smil:trans7.smil/playlist.m3u8',
   'ID', 'Nasional', '720p', 'bagus', 'resmi', null, null, 20),
  ('CNN Indonesia', 'cnn-indonesia',
   'https://i.imgur.com/l0JyOrm.png',
   'https://video.detik.com/livecnn/smil:cnntv/playlist.m3u8',
   'ID', 'Berita', null, 'bagus', 'resmi', null,
   'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36', 30),
  ('CNBC Indonesia', 'cnbc-indonesia',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/5/51/CNBC_Indonesia_2025.svg/960px-CNBC_Indonesia_2025.svg.png',
   'https://video.detik.com/livecnbc/smil:cnbctv/playlist.m3u8',
   'ID', 'Berita', null, 'bagus', 'resmi', null,
   'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36', 40),
  ('BTV', 'btv',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/2/22/BTV_%28Indonesia%29_logo_%282025%29.svg/960px-BTV_%28Indonesia%29_logo_%282025%29.svg.png',
   'https://lnd0t3b922.tenbytecdn.com/ta-sg1/90369cf5-6ac2-411d-b3d0-15fae10a2e2c/master.m3u8',
   'ID', 'Berita', '1080p', 'bagus', 'resmi', null, null, 50),
  ('TVRI World', 'tvri-world',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/9/9e/TVRIWorld.png/960px-TVRIWorld.png',
   'https://ott-balancer.tvri.go.id/live/eds/TVRIWorld/hls/TVRIWorld.m3u8',
   'ID', 'Nasional', '1080p', 'bagus', 'resmi', null, null, 60),
  ('TVRI Sport HD', 'tvri-sport',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/9/9e/TVRI_Sport_2022.svg/960px-TVRI_Sport_2022.svg.png',
   'https://ott-balancer.tvri.go.id/live/eds/SportHD/hls/SportHD.m3u8',
   'ID', 'Olahraga', '1080p', 'bagus', 'resmi', null, null, 70),
  ('TVRI Jakarta', 'tvri-jakarta',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/e/eb/TVRILogo2019.svg/960px-TVRILogo2019.svg.png',
   'https://ott-balancer.tvri.go.id/live/eds/DKI/hls/DKI.m3u8',
   'ID', 'Daerah', '576i', 'bagus', 'resmi', null, null, 80),
  ('TVRI Jawa Barat', 'tvri-jawa-barat',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/e/eb/TVRILogo2019.svg/960px-TVRILogo2019.svg.png',
   'https://ott-balancer.tvri.go.id/live/eds/Jabar/hls/Jabar.m3u8',
   'ID', 'Daerah', '480p', 'bagus', 'resmi', null, null, 90),
  ('TVRI Jawa Tengah', 'tvri-jawa-tengah',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/e/eb/TVRILogo2019.svg/960px-TVRILogo2019.svg.png',
   'https://ott-balancer.tvri.go.id/live/eds/Jateng/hls/Jateng.m3u8',
   'ID', 'Daerah', '720p', 'bagus', 'resmi', null, null, 100),
  ('TVRI Jawa Timur', 'tvri-jawa-timur',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/e/eb/TVRILogo2019.svg/960px-TVRILogo2019.svg.png',
   'https://ott-balancer.tvri.go.id/live/eds/Jatim/hls/Jatim.m3u8',
   'ID', 'Daerah', '720p', 'bagus', 'resmi', null, null, 110),
  ('Dhoho TV', 'dhoho-tv',
   'https://i.imgur.com/4Khyyfh.png',
   'https://dhohotv.siar.us/dhohotv/live/playlist.m3u8',
   'ID', 'Daerah', '720p', 'bagus', 'resmi', null, null, 120),
  ('Astro Blitar TV', 'astro-blitar-tv',
   'https://i.imgur.com/rORKbGw.png',
   'https://5bf7b725107e5.streamlock.net/abtv/abtv/playlist.m3u8',
   'ID', 'Daerah', '720p', 'bagus', 'resmi', null, null, 130),
  ('Banjar TV', 'banjar-tv',
   'https://i.imgur.com/TA76a10.png',
   'https://banjartv.siar.us/banjartv/live/playlist.m3u8',
   'ID', 'Daerah', '720p', 'bagus', 'resmi', null, null, 140),
  ('DW English', 'dw-english', null,
   'https://dwamdstream102.akamaized.net/hls/live/2015525/dwstream102/index.m3u8',
   'DE', 'Internasional', null, 'bagus', 'resmi', null, null, 150),
  -- -- -- TIDAK STABIL: restream IP mentah / belum terverifikasi -- -- --
  ('RCTI', 'rcti',
   'https://i.imgur.com/iLFx8ia.png',
   'http://202.150.172.59/RCTIHD/index.m3u8',
   'ID', 'Nasional', '1080p', 'tidak stabil', 'restream', null, null, 200),
  ('SCTV', 'sctv',
   'https://i.imgur.com/EMmOvnQ.png',
   'http://202.150.172.59/SCTV/tracks-v1a1/mono.m3u8',
   'ID', 'Nasional', null, 'tidak stabil', 'restream', null, null, 210),
  ('GTV', 'gtv',
   'https://i.imgur.com/jjuKnYs.png',
   'http://202.150.172.59/GTVHD/index.m3u8',
   'ID', 'Nasional', '1080p', 'tidak stabil', 'restream', null, null, 220),
  ('ANTV', 'antv',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/2/2e/Antv_logo.svg/960px-Antv_logo.svg.png',
   'http://103.58.160.157:8278/720-ANTV/playlist.m3u8',
   'ID', 'Nasional', '720p', 'tidak stabil', 'restream', null,
   'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/65.0.3325.181 Safari/537.36', 230),
  ('TVRI Nasional', 'tvri-nasional',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/e/eb/TVRILogo2019.svg/960px-TVRILogo2019.svg.png',
   'http://202.150.172.59/TVRI/index.m3u8',
   'ID', 'Nasional', '1080p', 'tidak stabil', 'restream', null, null, 240),
  ('BeritaSatu', 'beritasatu',
   'https://i.postimg.cc/6pycF7Vx/300px-IDTV.png',
   'https://xtdslboppkkv-pull.bpmedialive.com/live/beritasatu/abr.m3u8',
   'ID', 'Berita', '1080p', 'tidak stabil', 'belum terverifikasi',
   'https://www.beritasatu.com/beritasatu-live-streaming', null, 250),
  ('Al Jazeera English', 'al-jazeera-english', null,
   'https://live-hls-web-aje.getaj.net/AJE/index.m3u8',
   'QA', 'Internasional', null, 'tidak stabil', 'belum terverifikasi', null, null, 260),
  ('France 24 English', 'france-24-english', null,
   'https://static.france24.com/live/cdn/web/hls/france24-en.m3u8',
   'FR', 'Internasional', null, 'tidak stabil', 'belum terverifikasi', null, null, 270)
on conflict (slug) do nothing;
