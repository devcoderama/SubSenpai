-- Laporan user (tombol Laporkan di pemutar) + sembunyikan per
-- episode. Permintaan Rams 2026-11-23 malam: user melaporkan link
-- streaming yang mati (mis. semua pilihan server gagal); admin
-- menindak dari dashboard — perbaiki link (resolve ulang episode),
-- sembunyikan episode itu saja, atau sembunyikan judulnya.
alter table episodes add column if not exists is_hidden boolean not null default false;

create table if not exists user_reports (
    id uuid primary key default gen_random_uuid(),
    user_id uuid references profiles(id) on delete set null,
    device_key text not null default '',
    reporter_key text not null default '',
    title_id uuid references titles(id) on delete cascade,
    episode_id uuid references episodes(id) on delete cascade,
    reason text not null default 'link_rusak',
    note text not null default '',
    status text not null default 'baru'
        check (status in ('baru','diproses','selesai','diabaikan')),
    handled_by uuid references profiles(id) on delete set null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
create index if not exists user_reports_status_idx on user_reports (status, created_at desc);
