-- 0013: postingan feed boleh merujuk judul katalog (Bagikan ke Post
-- dari halaman Detail/Player) agar judul di postingan bisa diklik
-- menuju halaman detail judulnya.
alter table feed_posts
    add column if not exists title_id uuid references titles(id) on delete set null;
