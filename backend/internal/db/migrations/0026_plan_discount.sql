-- 0026: harga asli + diskon per paket premium (permintaan Rams
-- 2026-11-28) untuk promo event, mis. diskon 40% saat film GJLS.
-- price tetap harga yang DITAGIH; original_price + discount_percent
-- hanya pajangan (harga coret + badge -X% di aplikasi/web).
-- Bila salah satunya 0, paket tampil tanpa diskon.
alter table subscription_plans
    add column if not exists original_price integer not null default 0 check (original_price >= 0),
    add column if not exists discount_percent integer not null default 0 check (discount_percent >= 0 and discount_percent <= 100);
