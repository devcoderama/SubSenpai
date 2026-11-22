-- Data contoh untuk PENGEMBANGAN LOKAL saja (bukan data produksi).
-- Jalankan manual setelah migrasi: psql "$DATABASE_URL" -f db/seed/dev_seed.sql

insert into titles (id, slug, type, title, title_alt, synopsis, poster_url, year, status, genres, rating) values
  ('11111111-1111-1111-1111-111111111111', 'senja-di-kyoto', 'anime', 'Senja di Kyoto', 'Kyoto no Yuugure',
   'Drama romansa berlatar Kyoto modern — data contoh pengembangan.', 'https://picsum.photos/seed/senpai-poster1/480/640', 2026, 'ongoing', '{Romance,Drama}', 8.7),
  ('33333333-3333-3333-3333-333333333333', 'pedang-fajar', 'manhwa', 'Pedang Fajar', null,
   'Manhwa aksi fantasi — data contoh pengembangan.', 'https://picsum.photos/seed/senpai-poster2/480/640', 2025, 'ongoing', '{Action,Fantasy}', 8.2),
  ('55555555-5555-5555-5555-555555555555', 'kereta-terakhir', 'film', 'Kereta Terakhir', null,
   'Film thriller satu malam di kereta — data contoh pengembangan.', 'https://picsum.photos/seed/senpai-poster3/480/640', 2025, 'completed', '{Thriller}', 7.9)
on conflict (id) do nothing;

insert into episodes (id, title_id, episode_no, title, video_url, duration_sec, intro_start_sec, intro_end_sec) values
  ('21111111-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 1, 'Pertemuan di Stasiun',
   'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8', 1420, 30, 120),
  ('21111111-1111-1111-1111-111111111112', '11111111-1111-1111-1111-111111111111', 2, 'Hujan Pertama',
   'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8', 1410, 30, 118),
  ('65555555-5555-5555-5555-555555555555', '55555555-5555-5555-5555-555555555555', 1, null,
   'https://devstreaming-cdn.apple.com/videos/streaming/examples/img_bipbop_adv_example_ts/master.m3u8', 5400, null, null)
on conflict (id) do nothing;

insert into episode_sources (episode_id, url, label) values
  ('21111111-1111-1111-1111-111111111111', 'https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8', 'utama 720p'),
  ('21111111-1111-1111-1111-111111111111', 'https://demo.unified-streaming.com/k8s/features/stable/video/tears-of-steel/tears-of-steel.ism/.m3u8', 'cadangan')
on conflict do nothing;

insert into subtitles (episode_id, lang, source, format, file_url, status) values
  ('21111111-1111-1111-1111-111111111111', 'ja', 'caption', 'srt', 'https://example.com/subs/senja-ep1-ja.srt', 'done')
on conflict (episode_id, lang) do nothing;

insert into chapters (id, title_id, chapter_no, title, page_count) values
  ('43333333-3333-3333-3333-333333333333', '33333333-3333-3333-3333-333333333333', 1, 'Pedang yang Tidur', 3)
on conflict (id) do nothing;

insert into chapter_pages (chapter_id, page_no, image_url, light_url) values
  ('43333333-3333-3333-3333-333333333333', 1, 'https://picsum.photos/seed/senpai-page1/800/1200', 'https://picsum.photos/seed/senpai-page1/540/810'),
  ('43333333-3333-3333-3333-333333333333', 2, 'https://picsum.photos/seed/senpai-page2/800/1200', 'https://picsum.photos/seed/senpai-page2/540/810'),
  ('43333333-3333-3333-3333-333333333333', 3, 'https://picsum.photos/seed/senpai-page3/800/1200', 'https://picsum.photos/seed/senpai-page3/540/810')
on conflict (chapter_id, page_no) do nothing;
