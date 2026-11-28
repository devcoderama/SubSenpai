-- Penanda kirim jadwal rilis per judul per kanal (perbaikan 2026-11-28):
-- pengirim lama menandai per hari per kanal, jadi kirim yang gagal di
-- tengah (429 Telegram) diulang sehari penuh dari awal — judul awal
-- kekirim berkali-kali. Sekarang tiap judul ditandai sendiri begitu
-- terkirim, dan siklus berikutnya hanya mengulang yang belum terkirim.
create table if not exists jadwal_notified (
  channel text not null,
  day date not null,
  slug text not null,
  sent_at timestamptz not null default now(),
  primary key (channel, day, slug)
);
create index if not exists jadwal_notified_day_idx on jadwal_notified (day);
