-- 0016_payment_sessions_index.sql — indeks unik (provider, amount)
-- untuk sesi pending hanya relevan bagi GoPay (pencocokan nominal
-- unik saat polling). Gateway lain (Tokopay/Midtrans/Xendit) sah
-- memakai nominal yang sama untuk banyak sesi bersamaan. Indeks
-- lama memblokir checkout Tokopay kedua pada paket yang sama
-- selama sesi pertama masih pending — akar "gagal membuat sesi
-- pembayaran" yang ditemukan 2026-11-23.
drop index if exists payment_sessions_amount_pending_idx;
create unique index payment_sessions_amount_pending_idx
  on payment_sessions (provider, amount)
  where status = 'pending' and provider = 'gopay';
