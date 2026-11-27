-- Cooldown percobaan enrich (diagnosis 2026-11-27): loadJobs memilih
-- 60 judul tertua tanpa pelacakan gagal, sehingga judul yang SeriesInfo-nya
-- selalu gagal diputar ulang tiap siklus dan 81 judul donghua yang
-- episodenya sudah lengkap tak kebagian giliran. Pola yang sama dengan
-- cooldown sync di migrasi 0023, hanya jangka yang lebih panjang (metadata
-- jarang muncul dalam semalam).
alter table titles add column if not exists last_enrich_at timestamptz;
alter table titles add column if not exists last_enrich_error text;
