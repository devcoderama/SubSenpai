-- SubSenpai - Draf SQL skema database (PostgreSQL, lokal di VPS)
-- Diekstrak dari database/DATABASE.md Bagian 7 (skema dasar 36 tabel).
-- CATATAN: perubahan skema gelombang 3-5 (FITUR.md / OPTIMASI.md / KEAMANAN.md)
-- belum tergabung di berkas ini; sinkronisasi final adalah langkah Fase 0 (lihat PROGRES.md).

create table titles (
  id uuid primary key default gen_random_uuid(),
  slug text not null unique,
  type text not null check (type in ('anime','donghua','film','manga','manhua','manhwa')),
  title text not null,
  title_alt text,
  synopsis text,
  poster_url text,
  year int,
  season text check (season in ('winter','spring','summer','fall')),
  status text not null default 'ongoing' check (status in ('ongoing','completed')),
  genres text[] not null default '{}',
  rating numeric,
  is_adult boolean not null default false,
  search_tsv tsvector generated always as (
    to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(title_alt, ''))
  ) stored,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
create index titles_search_idx on titles using gin (search_tsv);
create index titles_type_idx on titles (type, created_at desc);
create index titles_genres_idx on titles using gin (genres);
create index titles_updated_idx on titles (updated_at);

create table episodes (
  id uuid primary key default gen_random_uuid(),
  title_id uuid not null references titles(id) on delete cascade,
  episode_no int not null,
  title text,
  video_url text not null,
  duration_sec int,
  aired_at date,
  intro_start_sec int,
  intro_end_sec int,
  outro_start_sec int,
  outro_end_sec int,
  updated_at timestamptz not null default now(),
  unique (title_id, episode_no),
  check (intro_start_sec is null or intro_end_sec is null or intro_end_sec > intro_start_sec),
  check (outro_start_sec is null or outro_end_sec is null or outro_end_sec > outro_start_sec)
);
create index episodes_updated_idx on episodes (updated_at);

create table chapters (
  id uuid primary key default gen_random_uuid(),
  title_id uuid not null references titles(id) on delete cascade,
  chapter_no numeric not null,
  title text,
  page_count int not null default 0,
  updated_at timestamptz not null default now(),
  unique (title_id, chapter_no)
);
create index chapters_updated_idx on chapters (updated_at);

create table chapter_pages (
  id uuid primary key default gen_random_uuid(),
  chapter_id uuid not null references chapters(id) on delete cascade,
  page_no int not null,
  image_url text not null,
  unique (chapter_id, page_no)
);

create table subtitles (
  id uuid primary key default gen_random_uuid(),
  episode_id uuid not null references episodes(id) on delete cascade,
  lang text not null default 'id', -- bahasa ASAL episode (kode BCP-47); terjemahan dibuat di aplikasi user
  source text not null check (source in ('caption','whisper')),
  format text not null default 'srt' check (format in ('srt','ass')),
  file_url text,
  status text not null default 'queued' check (status in ('queued','processing','done','failed')),
  unique (episode_id, lang)
);

create table profiles (
  id uuid primary key default gen_random_uuid(),
  email text unique,
  google_sub text unique,
  display_name text,
  avatar_url text,
  is_premium boolean not null default false,
  premium_until timestamptz,
  role text not null default 'user' check (role in ('user','admin')),
  name_color text,
  name_emoji text,
  chat_muted_until timestamptz,
  friend_code text unique,
  xp int not null default 0,
  level int not null default 1,
  hide_adult boolean not null default true,
  show_online boolean not null default true,
  show_activity boolean not null default true,
  trial_used_at timestamptz,
  last_active_date date,
  streak_days int not null default 0,
  is_banned boolean not null default false,
  app_language text not null default 'id'
);

create table subscriptions (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  plan text not null,
  status text not null default 'active' check (status in ('active','expired','cancelled')),
  current_period_end timestamptz,
  provider text check (provider in ('midtrans','xendit','gopay','manual','voucher','trial')),
  provider_ref text
);

create table watchlist (
  user_id uuid not null references profiles(id) on delete cascade,
  title_id uuid not null references titles(id) on delete cascade,
  notify boolean not null default false,
  created_at timestamptz not null default now(),
  primary key (user_id, title_id)
);

create table watch_history (
  user_id uuid not null references profiles(id) on delete cascade,
  episode_id uuid not null references episodes(id) on delete cascade,
  position_sec int not null default 0,
  updated_at timestamptz not null default now(),
  primary key (user_id, episode_id)
);
create index watch_history_user_idx on watch_history (user_id, updated_at desc);

create table read_history (
  user_id uuid not null references profiles(id) on delete cascade,
  chapter_id uuid not null references chapters(id) on delete cascade,
  last_page int not null default 1,
  updated_at timestamptz not null default now(),
  primary key (user_id, chapter_id)
);
create index read_history_user_idx on read_history (user_id, updated_at desc);

create table guest_access (
  id uuid primary key default gen_random_uuid(),
  device_key text not null,
  title_id uuid not null references titles(id) on delete cascade,
  episode_id uuid references episodes(id) on delete cascade,
  chapter_id uuid references chapters(id) on delete cascade,
  used_at timestamptz not null default now(),
  check ((episode_id is not null) <> (chapter_id is not null))
);
create index guest_access_device_idx on guest_access (device_key);

create table reports (
  id uuid primary key default gen_random_uuid(),
  user_id uuid references profiles(id) on delete set null,
  target_kind text not null check (target_kind in ('episode','chapter','subtitle','chat_message','group','comment')),
  target_id uuid not null,
  reason text not null,
  status text not null default 'open' check (status in ('open','resolved','dismissed')),
  created_at timestamptz not null default now()
);
create index reports_status_idx on reports (status, created_at desc);

create table admin_logs (
  id uuid primary key default gen_random_uuid(),
  admin_id uuid not null references profiles(id) on delete cascade,
  action text not null,
  target text,
  detail jsonb,
  created_at timestamptz not null default now()
);
create index admin_logs_admin_idx on admin_logs (admin_id, created_at desc);

create table vouchers (
  id uuid primary key default gen_random_uuid(),
  code text not null unique,
  source text not null check (source in ('admin','purchase','referral')),
  duration_days int not null check (duration_days > 0),
  status text not null default 'active' check (status in ('active','redeemed','expired','revoked')),
  created_by uuid not null references profiles(id) on delete cascade,
  assigned_to uuid references profiles(id) on delete set null,
  redeemed_by uuid references profiles(id) on delete set null,
  redeemed_at timestamptz,
  expires_at timestamptz,
  payment_ref text,
  created_at timestamptz not null default now()
);
create index vouchers_status_idx on vouchers (status, created_at desc);

create table payment_sessions (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  purpose text not null check (purpose in ('subscription','voucher')),
  provider text not null default 'gopay',
  amount int not null check (amount > 0),
  unique_code int not null check (unique_code between 1 and 999),
  status text not null default 'pending' check (status in ('pending','paid','expired')),
  matched_ref text,
  expires_at timestamptz not null,
  created_at timestamptz not null default now()
);
-- nominal unik tidak boleh dipakai dua sesi aktif sekaligus
create unique index payment_sessions_amount_pending_idx
  on payment_sessions (provider, amount) where status = 'pending';

create table chat_channels (
  id uuid primary key default gen_random_uuid(),
  slug text not null unique,
  name text not null,
  description text,
  sort_order int not null default 0,
  kind text not null default 'global' check (kind in ('global','group','dm')),
  group_id uuid,
  dm_user_a_id uuid references profiles(id) on delete cascade,
  dm_user_b_id uuid references profiles(id) on delete cascade,
  unique (group_id),
  check (
    (kind = 'group' and group_id is not null and dm_user_a_id is null and dm_user_b_id is null)
    or (kind = 'dm' and group_id is null and dm_user_a_id is not null and dm_user_b_id is not null and dm_user_a_id < dm_user_b_id)
    or (kind = 'global' and group_id is null and dm_user_a_id is null and dm_user_b_id is null)
  )
);
create unique index chat_channels_dm_pair_idx on chat_channels (dm_user_a_id, dm_user_b_id) where kind = 'dm';

create table stickers (
  id uuid primary key default gen_random_uuid(),
  pack text not null,
  name text not null,
  image_url text not null,
  is_premium boolean not null default false
);

create table chat_messages (
  id uuid primary key default gen_random_uuid(),
  channel_id uuid not null references chat_channels(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  kind text not null default 'text' check (kind in ('text','sticker','image','share')),
  body text check (char_length(body) <= 500),
  sticker_id uuid references stickers(id) on delete set null,
  image_url text,
  image_thumb_url text,
  image_delete_url text,
  reply_to_id uuid references chat_messages(id) on delete set null,
  ref_title_id uuid references titles(id) on delete set null,
  ref_episode_id uuid references episodes(id) on delete set null,
  created_at timestamptz not null default now(),
  deleted_at timestamptz,
  check (
    (kind = 'text' and body is not null)
    or (kind = 'sticker' and (sticker_id is not null or image_url is not null))
    or (kind = 'image' and image_url is not null)
    or (kind = 'share' and ref_title_id is not null)
  )
);
create index chat_messages_channel_idx on chat_messages (channel_id, created_at desc);

create table friendships (
  requester_id uuid not null references profiles(id) on delete cascade,
  addressee_id uuid not null references profiles(id) on delete cascade,
  status text not null default 'pending' check (status in ('pending','accepted')),
  created_at timestamptz not null default now(),
  primary key (requester_id, addressee_id),
  check (requester_id <> addressee_id)
);

create table user_blocks (
  blocker_id uuid not null references profiles(id) on delete cascade,
  blocked_id uuid not null references profiles(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (blocker_id, blocked_id),
  check (blocker_id <> blocked_id)
);

create table groups (
  id uuid primary key default gen_random_uuid(),
  name text not null,
  description text,
  avatar_url text,
  owner_id uuid not null references profiles(id) on delete cascade,
  is_public boolean not null default true,
  invite_code text unique,
  invite_enabled boolean not null default true,
  created_at timestamptz not null default now()
);
create index groups_name_idx on groups (name);

create table group_members (
  group_id uuid not null references groups(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  role text not null default 'member' check (role in ('owner','admin','member')),
  joined_at timestamptz not null default now(),
  primary key (group_id, user_id)
);
create index group_members_user_idx on group_members (user_id);

-- FK kanal chat grup (tabel groups baru ada setelah chat_channels dibuat)
alter table chat_channels
  add constraint chat_channels_group_fk
  foreign key (group_id) references groups(id) on delete cascade;

create table user_stickers (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  image_url text not null,
  thumb_url text,
  delete_url text,
  created_at timestamptz not null default now()
);
create index user_stickers_user_idx on user_stickers (user_id, created_at desc);

create table comments (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  episode_id uuid references episodes(id) on delete cascade,
  chapter_id uuid references chapters(id) on delete cascade,
  parent_id uuid references comments(id) on delete cascade,
  body text not null,
  created_at timestamptz not null default now(),
  deleted_at timestamptz,
  check ((episode_id is not null) <> (chapter_id is not null))
);
create index comments_episode_idx on comments (episode_id, created_at desc);
create index comments_chapter_idx on comments (chapter_id, created_at desc);

create table title_ratings (
  user_id uuid not null references profiles(id) on delete cascade,
  title_id uuid not null references titles(id) on delete cascade,
  score int not null check (score between 1 and 10),
  review text,
  created_at timestamptz not null default now(),
  primary key (user_id, title_id)
);

create table device_tokens (
  user_id uuid not null references profiles(id) on delete cascade,
  fcm_token text not null,
  platform text not null default 'android',
  updated_at timestamptz not null default now(),
  primary key (user_id, fcm_token)
);

create table daily_stats (
  day date primary key,
  views int not null default 0,
  reads int not null default 0,
  new_users int not null default 0,
  chat_messages int not null default 0
);

create table broadcasts (
  id uuid primary key default gen_random_uuid(),
  admin_id uuid not null references profiles(id) on delete cascade,
  audience text not null default 'all' check (audience in ('all','premium','free')),
  title text not null,
  body text not null,
  ref_title_id uuid references titles(id) on delete set null,
  sent_count int not null default 0,
  created_at timestamptz not null default now()
);

create table notifications (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  kind text not null check (kind in ('release','broadcast','system')),
  title text not null,
  body text not null,
  ref_title_id uuid references titles(id) on delete set null,
  broadcast_id uuid references broadcasts(id) on delete set null,
  read_at timestamptz,
  created_at timestamptz not null default now()
);
create index notifications_user_idx on notifications (user_id, created_at desc);

create table title_requests (
  id uuid primary key default gen_random_uuid(),
  requester_id uuid not null references profiles(id) on delete cascade,
  title_text text not null,
  type text not null check (type in ('anime','donghua','film','manga','manhua','manhwa')),
  note text,
  status text not null default 'pending' check (status in ('pending','approved','added','rejected')),
  matched_title_id uuid references titles(id) on delete set null,
  created_at timestamptz not null default now()
);
create index title_requests_status_idx on title_requests (status, created_at desc);

create table title_request_votes (
  request_id uuid not null references title_requests(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (request_id, user_id)
);

create table collections (
  id uuid primary key default gen_random_uuid(),
  owner_id uuid not null references profiles(id) on delete cascade,
  name text not null,
  description text,
  is_public boolean not null default true,
  created_at timestamptz not null default now()
);
create index collections_owner_idx on collections (owner_id);

create table collection_items (
  collection_id uuid not null references collections(id) on delete cascade,
  title_id uuid not null references titles(id) on delete cascade,
  position int not null default 0,
  primary key (collection_id, title_id)
);
create index collection_items_order_idx on collection_items (collection_id, position);

create table collection_follows (
  collection_id uuid not null references collections(id) on delete cascade,
  user_id uuid not null references profiles(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (collection_id, user_id)
);

create table user_achievements (
  user_id uuid not null references profiles(id) on delete cascade,
  achievement_key text not null,
  unlocked_at timestamptz not null default now(),
  primary key (user_id, achievement_key)
);

create table user_devices (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references profiles(id) on delete cascade,
  device_id text not null,
  device_name text,
  last_seen_at timestamptz not null default now(),
  revoked_at timestamptz,
  created_at timestamptz not null default now(),
  unique (user_id, device_id)
);
create index user_devices_user_idx on user_devices (user_id);
