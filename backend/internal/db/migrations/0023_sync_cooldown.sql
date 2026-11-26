-- Cooldown kegagalan sinkronisasi katalog (diagnosis Rams 2026-11-26):
-- judul yang gagal sync dicatat agar dilewati 6 jam berikutnya, supaya
-- judul yang gagal terus (mis. wp-json Anichin 403, HiAnime timeout)
-- tidak memonopoli slot tiap siklus. Sekaligus membersihkan base URL
-- Anichin (.my kloningan berbahaya -> .moe resmi; fetch sendiri sudah
-- dinormalisasi moeBase, ini hanya merapikan data registri).
alter table titles add column if not exists last_sync_at timestamptz;
alter table titles add column if not exists last_sync_error text;
update content_sources set base_url = 'https://anichin.moe'
where resolver = 'anichin' and base_url = 'https://anichin.my';
