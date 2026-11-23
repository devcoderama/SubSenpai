-- Unlock episode lewat iklan (permintaan Rams 2026-11-23): akun
-- gratis & tamu bisa membuka SATU episode terkunci dengan menonton
-- iklan (Monetag). Satu baris = satu episode yang sudah dibuka,
-- dimiliki user login (user_id) ATAU perangkat tamu (device_key =
-- hash X-Device-Id, sama seperti guest_access). Permanen per episode.
create table if not exists ad_unlocks (
    id         uuid primary key default gen_random_uuid(),
    user_id    uuid references profiles(id) on delete cascade,
    device_key text,
    episode_id uuid not null references episodes(id) on delete cascade,
    created_at timestamptz not null default now(),
    check ((user_id is not null) <> (device_key is not null))
);
create unique index if not exists ad_unlocks_user_ep_idx
    on ad_unlocks (user_id, episode_id) where user_id is not null;
create unique index if not exists ad_unlocks_device_ep_idx
    on ad_unlocks (device_key, episode_id) where device_key is not null;
