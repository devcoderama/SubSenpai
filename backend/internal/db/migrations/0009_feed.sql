-- 0009_feed.sql — Feed komunitas sosial (redesain halaman Chat,
-- permintaan Rams 2026-11-22, referensi Threads/Facebook).
--
-- Audit sebelumnya: chat kanal (chat_channels/chat_messages) adalah
-- model ruang obrolan, bukan postingan — tidak ada like/komentar
-- berjenjang/bookmark. Tabel notifikasi sudah ada tetapi jenisnya
-- hanya release/broadcast/system. Jadi feed dibuat sebagai model
-- baru di samping chat (ruang chat grup/DM tetap dipakai apa adanya):
--
--   feed_posts          postingan (teks/gambar, kategori, grup opsional)
--   feed_post_likes     like postingan (PK gabungan = anti dobel)
--   feed_comments       komentar + balasan (parent_id menunjuk komentar akar/balas)
--   feed_comment_likes  like komentar/balasan
--   feed_bookmarks      bookmark postingan
--
-- Tab "Teman" memakai friendships status accepted yang sudah ada;
-- tab "Grup" memakai group_members yang sudah ada. Notifikasi
-- diperluas: jenis 'social' + kolom ref_post_id & actor_id.

create table feed_posts (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  body text check (char_length(body) <= 1000),
  image_url text,
  category text not null default 'umum'
    check (category in ('umum', 'anime', 'donghua', 'film', 'manga', 'rekomendasi')),
  group_id uuid references groups(id) on delete cascade,
  share_count int not null default 0,
  created_at timestamptz not null default now(),
  deleted_at timestamptz,
  check (body is not null or image_url is not null)
);
create index feed_posts_created_idx on feed_posts (created_at desc)
  where deleted_at is null;
create index feed_posts_group_idx on feed_posts (group_id, created_at desc)
  where deleted_at is null;
create index feed_posts_user_idx on feed_posts (user_id, created_at desc);

create table feed_post_likes (
  post_id uuid not null references feed_posts(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (post_id, user_id)
);

create table feed_comments (
  id uuid primary key default gen_random_uuid(),
  post_id uuid not null references feed_posts(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  parent_id uuid references feed_comments(id) on delete cascade,
  body text not null check (char_length(body) <= 500),
  created_at timestamptz not null default now(),
  deleted_at timestamptz
);
create index feed_comments_post_idx on feed_comments (post_id, created_at);

create table feed_comment_likes (
  comment_id uuid not null references feed_comments(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (comment_id, user_id)
);

create table feed_bookmarks (
  post_id uuid not null references feed_posts(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (post_id, user_id)
);

alter table notifications drop constraint notifications_kind_check;
alter table notifications add constraint notifications_kind_check
  check (kind in ('release', 'broadcast', 'system', 'social'));
alter table notifications add column ref_post_id uuid
  references feed_posts(id) on delete set null;
alter table notifications add column actor_id uuid
  references profiles(id) on delete set null;
