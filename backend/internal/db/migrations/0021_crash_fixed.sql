-- Penanda perbaikan laporan crash (permintaan Rams 2026-11-23
-- malam): setelah melihat laporan crash di dashboard, admin bisa
-- menandainya sudah diperbaiki atau belum.
alter table crash_reports add column if not exists is_fixed boolean not null default false;
alter table crash_reports add column if not exists fixed_at timestamptz;
