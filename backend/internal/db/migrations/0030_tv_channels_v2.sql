-- 0030_tv_channels_v2.sql — TV Langsung v2 (permintaan Rams 2026-11-28 18:03):
-- "iptv nya gabisa semua dari link yang kamu pake bre coba yang dari
-- github ini https://github.com/PemudaNegri/iptv-indonesia"
--
-- APA YANG TERJADI (diagnosis 28 Nov 2026, probe mendalam 3 ronde):
-- 1. Cek pertama v0.24.40 (17:17) HANYA memastikan 200 + "#EXTM3U" di
--    awal playlist. Itu TIDAK cukup: playlist HLS bisa 200 + #EXTM3U
--    tapi berisi 0 segmen (tidak pernah bisa diputar). 8 dari 10 link
--    dens.tv repo (BeritaSatu/BTV/Metro/RTV/TVRI/BN/Magna/Nusantara)
--    persis begitu — playlist kosong. Mereka TIDAK di-seed di sini.
-- 2. Repo PemudaNegri ternyata sebagian besar MATI / tidak bisa
--    diverifikasi dari VM: beetv.my.id (RCTI/GTV/MNCTV/ANTV) 404 semua;
--    beIN Sports terenkripsi DRM (cenc, butuh license server); SPOTV
--    400; Garuda TV sempat mati lalu hidup (di-seed, bagus).
--    Yang HIDUP dari repo dan di-seed: MDTV, Moji (dens.tv),
--    DAAI TV, Garuda TV + 11 kanal tak-terverifikasi (di bawah).
-- 3. 013tv.com (Trans TV/Trans7/ANTV/CNBC/Indosiar, http :8080) dan
--    cdnbal1.indihometv.com (SCTV/Kompas/tvOne/daerah/anak, DASH .mpd)
--    TIDAK BISA diverifikasi dari VM ini (proxy egress 403 / koneksi
--    ditutup) — mereka di-seed sebagai 'tidak stabil' + source
--    'belum terverifikasi'. Health-check worker di VPS Indonesia yang
--    menentukan hidup/matinya (3x gagal -> sembunyi otomatis).
-- 4. Link LAMA ternyata sebagian besar HIDUP (dicek ulang dengan probe
--    mendalam: playlist HARUS punya segmen + 1 segmen HARUS terunduh):
--    Trans TV/Trans7, CNN/CNBC (master smil), TVRI x6, BTV, Dhoho,
--    Astro, Banjar, DW (playlist ok, segmen gagal 3x dari VM -> turun
--    ke 'tidak stabil'). Kegagalan massal 18:03 adalah egress VM yang
--    sempat buruk (timeout TLS di banyak host sekaligus), BUKAN
--    semua origin mati. CNN: pakai master smil (chunklist repo
--    w1188349826 sudah basi — ID chunklist berotasi).
-- 5. Pelajaran: probe worker (jobs/probeStream) diperketat di commit
--    yang sama — sekarang wajib ada segmen (ikut variant bila master),
--    dan kenal DASH .mpd (tolak yang terenkripsi).
--
-- Total seed v2: 38 kanal (18 bagus + 20 tidak stabil). Upsert per slug:
-- baris lama yang URL-nya berubah (SCTV -> .mpd IndiHome, ANTV ->
-- 013tv, DW bagus -> tidak stabil) ikut diperbarui; sisanya sama.
--
-- Kolom web_ok (baru di migrasi ini): true = pemutar web (hls.js)
-- punya peluang memutar kanal ini; false = HANYA aplikasi.
-- Alasan false bersifat struktural, bukan soal stabilitas:
--   * stream http:// (RCTI/GTV/TVRI-Nasional/ANTV/Indosiar):
--     mixed content — peramban memblokir http di situs https.
--   * stream MPEG-DASH .mpd (SCTV/Kompas/tvOne/RTV/Metro/Bali/
--     JakTV/JawaPos/JTV/CBeebies/Nickelodeon): peramban tidak bisa
--     memutar DASH tanpa pustaka khusus; hanya ExoPlayer (aplikasi).
-- Kanal HLS https lainnya web_ok=true — TERMASUK Trans TV/Trans7/
-- CNN/CNBC: origin mereka menolak Referer asing (403) TAPI mengizinkan
-- CORS (*, bila tanpa Referer). Halaman /tv memakai meta referrer
-- no-referrer (lihat web/src/routes/tv/+page.svelte) supaya peramban
-- tidak mengirim Referer -> origin melayani (200) dan hls.js memutar.
-- Tanpa meta itu, keempatnya gagal di web persis seperti laporan Rams.
alter table if exists tv_channels
  add column if not exists web_ok boolean not null default true;

insert into tv_channels
  (name, slug, logo_url, stream_url, country, category, quality,
   stability, source, http_referer, http_user_agent, sort_order)
values
  -- -- -- BAGUS (18): segmen terverifikasi terunduh 28 Nov 2026 -- -- --
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
  -- BARU dari repo PemudaNegri (segmen terverifikasi):
  ('MDTV', 'mdtv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/imageCir_MDTV-180x180-CL_2025_10_08_13_01_28.jpg',
   'https://op-flashcon-digdayahd-1.dens.tv/h/h223/index.m3u8',
   'ID', 'Nasional', null, 'bagus', 'resmi', null, null, 160),
  ('Moji', 'moji',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_moji.png',
   'https://op-flashcon-digdayahd-1.dens.tv/h/h207/index.m3u8',
   'ID', 'Nasional', null, 'bagus', 'resmi', null, null, 170),
  ('DAAI TV', 'daai-tv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_daaitv.png',
   'https://pull.daaiplus.com/live-DAAIPLUS/live-DAAIPLUS_HD.m3u8',
   'ID', 'Nasional', '720p', 'bagus', 'resmi', null, null, 180),
  ('Garuda TV', 'garuda-tv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/garudatv.jpg',
   'https://hgmtv.com:19360/garudatvlivestreaming/720p.m3u8',
   'ID', 'Nasional', '720p', 'bagus', 'resmi', null, null, 190),
  -- -- -- TIDAK STABIL (20): restream / belum terverifikasi, worker VPS yang menilai -- -- --
  ('RCTI', 'rcti',
   'https://i.imgur.com/iLFx8ia.png',
   'http://202.150.172.59/RCTIHD/index.m3u8',
   'ID', 'Nasional', '1080p', 'tidak stabil', 'restream', null, null, 200),
  -- SCTV pindah ke DASH IndiHome (repo): restream box 202.150 mati dari VM 2x;
  -- .mpd hanya jalan di aplikasi (web tampil panel app-only).
  ('SCTV', 'sctv',
   'https://i.imgur.com/EMmOvnQ.png',
   'https://cdnbal1.indihometv.com/atm/DASH/sctv/sctv-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Nasional', null, 'tidak stabil', 'belum terverifikasi', null, null, 210),
  ('GTV', 'gtv',
   'https://i.imgur.com/jjuKnYs.png',
   'http://202.150.172.59/GTVHD/index.m3u8',
   'ID', 'Nasional', '1080p', 'tidak stabil', 'restream', null, null, 220),
  -- ANTV pindah ke 013tv (repo): URL lama 103.58.160.157 mengembalikan
  -- 401 pasti (butuh kredensial) — tidak bisa diputar apa adanya.
  ('ANTV', 'antv',
   'https://upload.wikimedia.org/wikipedia/commons/thumb/2/2e/Antv_logo.svg/960px-Antv_logo.svg.png',
   'http://013tv.com:8080/F33848/cbd9779b/165072',
   'ID', 'Nasional', '720p', 'tidak stabil', 'belum terverifikasi', null, null, 230),
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
   'FR', 'Internasional', null, 'tidak stabil', 'belum terverifikasi', null, null, 270),
  -- DW turun dari bagus: playlist ok, tapi segmen .ts gagal diunduh
  -- 3x dari VM (worker VPS yang memutuskan).
  ('DW English', 'dw-english', null,
   'https://dwamdstream102.akamaized.net/hls/live/2015525/dwstream102/index.m3u8',
   'DE', 'Internasional', null, 'tidak stabil', 'resmi', null, null, 275),
  -- BARU dari repo PemudaNegri (tak-terverifikasi dari VM):
  ('Indosiar', 'indosiar',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_indosiar.png',
   'http://013tv.com:8080/F33848/cbd9779b/162138',
   'ID', 'Nasional', null, 'tidak stabil', 'belum terverifikasi', null, null, 280),
  ('Kompas TV', 'kompas-tv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_kompastv.png',
   'https://cdnbal1.indihometv.com/atm/DASH/KOMPAS_TV/KOMPAS_TV-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Berita', null, 'tidak stabil', 'belum terverifikasi', null, null, 290),
  ('tvOne', 'tvone',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_tvone.png',
   'https://cdnbal1.indihometv.com/atm/DASH/tvone/tvone-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Berita', null, 'tidak stabil', 'belum terverifikasi', null, null, 300),
  ('RTV', 'rtv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_rtv.png',
   'https://cdnbal1.indihometv.com/atm/DASH/RAJAWALI_TV/RAJAWALI_TV-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Nasional', null, 'tidak stabil', 'belum terverifikasi', null, null, 310),
  ('Metro TV', 'metro-tv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_metrotv.png',
   'https://tvratu.my.id/vid/index.mpd?id=777',
   'ID', 'Berita', null, 'tidak stabil', 'belum terverifikasi', null, null, 320),
  ('Bali TV', 'bali-tv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_balitv.png',
   'https://cdnbal1.indihometv.com/atm/DASH/balitv/balitv-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Daerah', null, 'tidak stabil', 'belum terverifikasi', null, null, 330),
  ('JakTV', 'jaktv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_jaktv.png',
   'https://cdnbal1.indihometv.com/atm/DASH/JAK_TV/JAK_TV-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Daerah', null, 'tidak stabil', 'belum terverifikasi', null, null, 340),
  ('JawaPos TV', 'jawapos-tv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_jawapos.png',
   'https://cdnbal1.indihometv.com/atm/DASH/jawapos/jawapos-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Daerah', null, 'tidak stabil', 'belum terverifikasi', null, null, 350),
  ('JTV', 'jtv',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_jtv.png',
   'https://cdnbal1.indihometv.com/atm/DASH/jtv/jtv-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Daerah', null, 'tidak stabil', 'belum terverifikasi', null, null, 360),
  ('CBeebies', 'cbeebies',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/imageCir_180x180_CBeebies_2025_12_03_10_31_32.jpg',
   'https://cdnbal1.indihometv.com/atm/DASH/cbeebies/cbeebies-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Anak', null, 'tidak stabil', 'belum terverifikasi', null, null, 370),
  ('Nickelodeon', 'nickelodeon',
   'https://raw.githubusercontent.com/apistech/project/refs/heads/main/logo/image_ch_nick.png',
   'https://cdnbal1.indihometv.com/atm/DASH/nickelodeon/nickelodeon-avc1_2500000=7-3277707030000000.mpd',
   'ID', 'Anak', null, 'tidak stabil', 'belum terverifikasi', null, null, 380)
on conflict (slug) do update set
  name = excluded.name,
  logo_url = excluded.logo_url,
  stream_url = excluded.stream_url,
  country = excluded.country,
  category = excluded.category,
  quality = excluded.quality,
  stability = excluded.stability,
  source = excluded.source,
  http_referer = excluded.http_referer,
  http_user_agent = excluded.http_user_agent,
  sort_order = excluded.sort_order,
  is_active = true,
  fail_count = 0,
  updated_at = now();

-- Kanal yang secara struktural hanya bisa diputar di aplikasi
-- (http mixed content / DASH .mpd) — lihat komentar web_ok di atas.
update tv_channels set web_ok = false where slug in (
  'rcti', 'gtv', 'tvri-nasional', 'antv', 'indosiar',
  'sctv', 'kompas-tv', 'tvone', 'rtv', 'metro-tv', 'bali-tv',
  'jaktv', 'jawapos-tv', 'jtv', 'cbeebies', 'nickelodeon'
);
