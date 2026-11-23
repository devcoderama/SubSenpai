-- 0017: peralatan admin & developer (permintaan Rams 2026-11-23):
-- 1) titles.is_hidden — admin bisa menyembunyikan judul dari aplikasi
--    tanpa menghapus datanya (katalog, beranda, pencarian, jadwal,
--    detail, dan playback menyaring judul tersembunyi).
-- 2) crash_reports — laporan crash dari aplikasi Android; aplikasi
--    menyimpan laporan saat crash dan mengunggahnya pada peluncuran
--    berikutnya; admin membacanya di dashboard.
-- 3) app_settings — pengaturan aplikasi yang bisa diubah admin dari
--    dashboard: mode perawatan, versi minimum (paksa perbarui), dan
--    pengumuman. Disajikan ke aplikasi lewat GET /api/v1/app-config.

alter table titles add column if not exists is_hidden boolean not null default false;

create table if not exists crash_reports (
  id uuid primary key default gen_random_uuid(),
  user_id uuid references profiles(id) on delete set null,
  app_version text not null default '',
  version_code int not null default 0,
  android_version text not null default '',
  device text not null default '',
  message text not null default '',
  stack_trace text not null default '',
  created_at timestamptz not null default now()
);
create index if not exists crash_reports_created_idx on crash_reports (created_at desc);

create table if not exists app_settings (
  key text primary key,
  value text not null default '',
  updated_at timestamptz not null default now()
);
insert into app_settings (key, value) values
  ('maintenance_mode', 'false'),
  ('maintenance_message', 'SubSenpai sedang dalam perawatan. Coba lagi sebentar lagi ya.'),
  ('min_version_code', '0'),
  ('update_url', 'https://github.com/devcoderama/SubSenpai/releases'),
  ('announcement', '')
on conflict (key) do nothing;
