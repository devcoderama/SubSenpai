-- 0005: metadata seri tambahan untuk halaman detail (network, studio,
-- durasi per episode, negara) — diambil dari halaman seri sumber
-- (mis. blok info Anichin) oleh job EnrichTitles.

alter table titles add column if not exists network text;
alter table titles add column if not exists studio text;
alter table titles add column if not exists duration_min int;
alter table titles add column if not exists country text;
