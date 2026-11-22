-- 0015: daftar metode pembayaran Tokopay disimpan di database
-- (permintaan Rams 2026-11-22) lengkap dengan URL ikonnya, agar
-- admin bisa mengaktifkan/menonaktifkan metode dari panel admin
-- (mis. saat salah satu channel gangguan) tanpa rilis ulang.
-- Daftar channel sesuai dokumentasi resmi Tokopay
-- (docs.tokopay.id/persiapan-awal/metode-pembayaran, dicek
-- 2026-11-22): 23 channel dalam 5 grup. Tokopay TIDAK punya OVO.
-- icon_url memakai layanan favicon Google (sz=128) dari domain
-- resmi tiap merek — terverifikasi hidup 2026-11-22; LINKAJA &
-- ALFAMART kosong (favicon mereknya tidak tersedia) dan aplikasi
-- menampilkan lingkaran huruf awal sebagai pengganti.
create table if not exists payment_methods (
    code       text primary key,
    name       text not null,
    grup       text not null check (grup in ('va', 'ewallet', 'qris', 'retail', 'pulsa')),
    icon_url   text not null default '',
    is_active  boolean not null default true,
    sort_order integer not null default 0,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

insert into payment_methods (code, name, grup, icon_url, sort_order) values
    ('BRIVA',            'BRI Virtual Account',      'va',      'https://www.google.com/s2/favicons?domain=bri.co.id&sz=128',          1),
    ('BCAVA',            'BCA Virtual Account',      'va',      'https://www.google.com/s2/favicons?domain=bca.co.id&sz=128',          2),
    ('BNIVA',            'BNI Virtual Account',      'va',      'https://www.google.com/s2/favicons?domain=bni.co.id&sz=128',          3),
    ('MANDIRIVA',        'Mandiri Virtual Account',  'va',      'https://www.google.com/s2/favicons?domain=bankmandiri.co.id&sz=128', 4),
    ('PERMATAVA',        'Permata Virtual Account', 'va',      'https://www.google.com/s2/favicons?domain=permatabank.com&sz=128',   5),
    ('PERMATAVAA',       'Permata Virtual Account 2','va',      'https://www.google.com/s2/favicons?domain=permatabank.com&sz=128',   6),
    ('CIMBVA',           'CIMB Virtual Account',     'va',      'https://www.google.com/s2/favicons?domain=cimbniaga.co.id&sz=128',   7),
    ('DANAMONVA',        'Danamon Virtual Account',  'va',      'https://www.google.com/s2/favicons?domain=danamon.co.id&sz=128',     8),
    ('BSIVA',            'BSI Virtual Account',      'va',      'https://www.google.com/s2/favicons?domain=bankbsi.co.id&sz=128',     9),
    ('BNCVA',            'BNC Virtual Account',      'va',      'https://www.google.com/s2/favicons?domain=neobank.id&sz=128',        10),
    ('SHOPEEPAY',        'ShopeePay',                'ewallet', 'https://www.google.com/s2/favicons?domain=shopee.co.id&sz=128',    11),
    ('GOPAY',            'GoPay',                    'ewallet', 'https://www.google.com/s2/favicons?domain=gopay.co.id&sz=128',     12),
    ('DANA',             'DANA',                     'ewallet', 'https://www.google.com/s2/favicons?domain=dana.id&sz=128',         13),
    ('LINKAJA',          'LinkAja',                  'ewallet', '',                                                                       14),
    ('QRIS',             'QRIS',                     'qris',    'https://www.google.com/s2/favicons?domain=qris.id&sz=128',         15),
    ('QRISREALTIME',     'QRIS Realtime',            'qris',    'https://www.google.com/s2/favicons?domain=qris.id&sz=128',         16),
    ('QRIS_REALTIME_NOBU','QRIS Realtime Nobu',      'qris',    'https://www.google.com/s2/favicons?domain=qris.id&sz=128',         17),
    ('ALFAMART',         'Alfamart',                 'retail',  '',                                                                       18),
    ('INDOMARET',        'Indomaret',                'retail',  'https://www.google.com/s2/favicons?domain=klikindomaret.com&sz=128', 19),
    ('TELKOMSEL',        'Pulsa Telkomsel',          'pulsa',   'https://www.google.com/s2/favicons?domain=telkomsel.com&sz=128',   20),
    ('AXIS',             'Pulsa AXIS',               'pulsa',   'https://www.google.com/s2/favicons?domain=axis.co.id&sz=128',      21),
    ('XL',               'Pulsa XL',                 'pulsa',   'https://www.google.com/s2/favicons?domain=xl.co.id&sz=128',        22),
    ('TRI',              'Pulsa Tri',                'pulsa',   'https://www.google.com/s2/favicons?domain=tri.co.id&sz=128',       23)
on conflict (code) do update
    set name = excluded.name,
        grup = excluded.grup,
        icon_url = excluded.icon_url,
        sort_order = excluded.sort_order,
        updated_at = now();
