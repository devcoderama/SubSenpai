-- Audit & perbaikan kelengkapan katalog oleh worker: penanda
-- percobaan perbaikan per judul agar judul yang sumbernya memang
-- rusak tidak dicoba terus-menerus setiap siklus.
alter table titles add column if not exists repair_attempts int not null default 0;
alter table titles add column if not exists last_repair_at timestamptz;
