-- 0004_tokopay.sql — Fase 4 (lanjutan): Tokopay menjadi gateway utama
-- menggantikan Midtrans/Xendit (permintaan Rams 2026-11-22: biaya admin
-- & fee lebih murah). Enum provider langganan ditambah 'tokopay'.

alter table subscriptions drop constraint if exists subscriptions_provider_check;
alter table subscriptions add constraint subscriptions_provider_check
  check (provider in ('midtrans','xendit','gopay','tokopay','manual','voucher','trial'));
