-- 0006_kuronime_server.sql — Anime (Kuronime) menjadi sumber SERVER
-- penuh seperti donghua (permintaan Rams 2026-11-22).
--
-- Latar: baris Kuronime di content_sources semula fetch_mode
-- 'device' (F39) karena kuronime.sbs dikira tidak bisa di-scrape
-- dari server (Cloudflare). Setelah scraping Kuronime dibangun
-- ulang ala donghua (arsip HTML + SeriesInfo + episode HTML +
-- Resolve iframedc) dan terbukti VPS bisa mengambil HTML-nya
-- (discovery arsip mengisi katalog dengan poster), anime harus
-- berperilaku persis seperti donghua: detail server berisi
-- metadata + daftar episode, dan pemutar video server di aplikasi
-- — bukan lagi "Buka dari Sumber" di perangkat.
--
-- Isi migrasi (idempoten secara alami — semua dibatasi kondisi
-- fetch_mode='device' yang sesudah migrasi tidak ada lagi):
-- 1. Baris sumber Kuronime: fetch_mode 'device' → 'server', agar
--    penemuan berikutnya memasukkan judul sebagai judul server
--    lengkap dengan pemetaan title_sources.
-- 2. Judul anime perangkat yang sudah ada: buat pemetaan
--    title_sources dari source_url_template-nya (URL halaman seri
--    Kuronime; bentuk lama /series/<slug>/ dialihkan situs ke
--    /anime/<slug>/ dan tetap sah).
-- 3. Judul anime yang pemetaannya berhasil dibuat: fetch_mode
--    'device' → 'server'. Judul tanpa template URL (tidak bisa
--    dipetakan) dibiarkan perangkat agar tetap bisa dibuka (F39).

update content_sources
set fetch_mode = 'server', updated_at = now()
where resolver = 'kuronime' and fetch_mode = 'device';

insert into title_sources (title_id, source_id, source_url)
select t.id, cs.id, t.source_url_template
from titles t
join content_sources cs
  on cs.resolver = 'kuronime' and cs.content_type = 'anime'
where t.type = 'anime'
  and t.fetch_mode = 'device'
  and t.source_url_template is not null
  and t.source_url_template <> ''
on conflict (title_id, source_id) do nothing;

update titles t
set fetch_mode = 'server'
where t.type = 'anime'
  and t.fetch_mode = 'device'
  and exists (select 1 from title_sources ts where ts.title_id = t.id);
