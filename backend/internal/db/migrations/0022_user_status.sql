-- Status akun yang dikelola admin (permintaan Rams 2026-11-23
-- malam): aktif / ban / ditangguhkan + catatan admin. Akun yang
-- tidak aktif tidak bisa login dan sesi berjalan-nya ditolak
-- middleware auth. is_banned lama dipetakan ke status 'banned'
-- dan selanjutnya dijaga selaras (banned <=> is_banned).
alter table profiles add column if not exists status text not null default 'active';
alter table profiles add column if not exists admin_note text not null default '';
alter table profiles add column if not exists status_updated_at timestamptz;
update profiles set status = 'banned' where is_banned and status = 'active';
alter table profiles drop constraint if exists profiles_status_check;
alter table profiles add constraint profiles_status_check
  check (status in ('active', 'banned', 'suspended'));
