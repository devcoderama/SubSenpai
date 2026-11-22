-- 0014: katalog paket premium pindah ke database (permintaan Rams
-- 2026-11-22) agar harga/durasi bisa diubah tanpa rilis ulang.
-- Harga mulai Rp8.000/bulan; paket lebih panjang lebih murah per
-- bulannya. Id mengikuti pola premium-{hari} yang sudah dipakai
-- tabel subscriptions.
create table if not exists subscription_plans (
    id         text primary key,
    days       integer not null check (days > 0),
    price      integer not null check (price >= 0),
    label      text not null,
    is_active  boolean not null default true,
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

insert into subscription_plans (id, days, price, label, sort_order) values
    ('premium-30',   30,   8000,   'Premium 1 Bulan',  1),
    ('premium-90',   90,   21000,  'Premium 3 Bulan',  2),
    ('premium-180',  180,  39000,  'Premium 6 Bulan',  3),
    ('premium-365',  365,  72000,  'Premium 12 Bulan', 4),
    ('premium-1095', 1095, 180000, 'Premium 3 Tahun',  5)
on conflict (id) do update
    set days = excluded.days,
        price = excluded.price,
        label = excluded.label,
        sort_order = excluded.sort_order,
        is_active = true,
        updated_at = now();
