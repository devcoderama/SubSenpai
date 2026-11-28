// SubSenpai Web — klien API backend Go.
//
// Semua respons JSON ter-enkripsi envelope {xdata, xtime} (ECDH sesi,
// AES-256-GCM). Pola kripto ini di-port dari web_admin/app.js yang sudah
// terverifikasi byte-identik dengan backend Go: X25519 ditulis manual
// dengan BigInt (RFC 7748) karena dukungan WebCrypto untuk X25519 belum
// merata antar peramban; HKDF dan AES-GCM memakai WebCrypto.

export const API_BASE = 'https://api.subsenpai.web.id';
export const GOOGLE_CLIENT_ID =
  '673952283037-h0qkk02cvlqdjgnv8f5k4os9ti281dta.apps.googleusercontent.com';

/* ================= Kripto sesi (xdata) ================= */
const _P = (1n << 255n) - 19n;
const _mod = (a) => ((a % _P) + _P) % _P;
function _leToBigInt(bytes) {
  let n = 0n;
  for (let i = bytes.length - 1; i >= 0; i--) n = (n << 8n) | BigInt(bytes[i]);
  return n;
}
function _bigIntToLe(n, len) {
  const out = new Uint8Array(len);
  for (let i = 0; i < len; i++) {
    out[i] = Number(n & 0xffn);
    n >>= 8n;
  }
  return out;
}
function _modInv(a) {
  let e = _P - 2n,
    base = _mod(a),
    r = 1n;
  while (e > 0n) {
    if (e & 1n) r = _mod(r * base);
    base = _mod(base * base);
    e >>= 1n;
  }
  return r;
}
function x25519(scalarBytes, uBytes) {
  const k = Uint8Array.from(scalarBytes);
  k[0] &= 248;
  k[31] &= 127;
  k[31] |= 64;
  const n = _leToBigInt(k);
  const u = Uint8Array.from(uBytes);
  u[31] &= 127;
  const x1 = _mod(_leToBigInt(u));
  let x2 = 1n,
    z2 = 0n,
    x3 = x1,
    z3 = 1n,
    swap = 0n;
  const a24 = 121665n;
  for (let t = 254; t >= 0; t--) {
    const kt = (n >> BigInt(t)) & 1n;
    swap ^= kt;
    if (swap === 1n) {
      [x2, x3] = [x3, x2];
      [z2, z3] = [z3, z2];
    }
    swap = kt;
    const A = _mod(x2 + z2),
      AA = _mod(A * A);
    const B = _mod(x2 - z2),
      BB = _mod(B * B);
    const E = _mod(AA - BB);
    const C = _mod(x3 + z3),
      D = _mod(x3 - z3);
    const DA = _mod(D * A),
      CB = _mod(C * B);
    x3 = _mod((DA + CB) * (DA + CB));
    z3 = _mod(x1 * _mod((DA - CB) * (DA - CB)));
    x2 = _mod(AA * BB);
    z2 = _mod(E * _mod(AA + _mod(a24 * E)));
  }
  if (swap === 1n) {
    [x2, x3] = [x3, x2];
    [z2, z3] = [z3, z2];
  }
  return _bigIntToLe(_mod(x2 * _modInv(z2)), 32);
}
const X25519_BASE = new Uint8Array(32);
X25519_BASE[0] = 9;

function b64urlEncode(bytes) {
  let s = '';
  for (const b of bytes) s += String.fromCharCode(b);
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}
function b64urlDecode(str) {
  const s = str.replace(/-/g, '+').replace(/_/g, '/');
  const bin = atob(s + '='.repeat((4 - (s.length % 4)) % 4));
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}

let sessionId = null;
let aesKey = null;
let handshakePromise = null;

// Satu handshake yang sedang berjalan dipakai bersama oleh semua
// panggilan paralel. Tanpa ini, tiap panggilan pertama memulai
// handshake sendiri dan handshake yang selesai terakhir menimpa
// aesKey, sehingga respons panggilan yang lebih dulu gagal didekripsi
// (race ini yang membuat Beranda menampilkan "Gagal memuat beranda"
// di produksi 27 Nov 2026, sementara halaman yang panggilannya
// tunggal seperti Jadwal normal-normal saja).
async function ensureSession() {
  if (sessionId) return;
  if (!handshakePromise) {
    handshakePromise = handshake().finally(() => {
      handshakePromise = null;
    });
  }
  await handshakePromise;
}
let jwt = null;

export function setJwt(token) {
  jwt = token || null;
}

// ID perangkat tamu yang persisten (seperti aplikasi): dikirim sebagai
// header X-Device-Id — backend memakainya untuk laporan tamu, ad-unlock
// tamu, dsb. Tanpa header, backend memakai hash IP (cadangan kasar).
export function getDeviceId() {
  if (typeof localStorage === 'undefined') return '';
  try {
    let id = localStorage.getItem('subsenpai_device_id');
    if (!id) {
      id =
        (crypto.randomUUID && crypto.randomUUID()) ||
        'web-' + Date.now() + '-' + Math.random().toString(36).slice(2);
      localStorage.setItem('subsenpai_device_id', id);
    }
    return id;
  } catch {
    return '';
  }
}

async function fetchTimeout(url, opts = {}, ms = 20000) {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), ms);
  try {
    return await fetch(url, { ...opts, signal: ctl.signal });
  } catch (e) {
    if (e.name === 'AbortError') throw new Error('Koneksi ke server timeout. Coba lagi.');
    throw e;
  } finally {
    clearTimeout(timer);
  }
}

async function handshake() {
  const priv = crypto.getRandomValues(new Uint8Array(32));
  const pub = x25519(priv, X25519_BASE);
  const res = await fetchTimeout(API_BASE + '/api/v1/session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ client_public_key: b64urlEncode(pub) })
  });
  if (!res.ok) throw new Error('Handshake sesi gagal (' + res.status + ')');
  const hs = await res.json();
  const shared = x25519(priv, b64urlDecode(hs.server_public_key));
  const hkdfKey = await crypto.subtle.importKey('raw', shared, 'HKDF', false, ['deriveKey']);
  aesKey = await crypto.subtle.deriveKey(
    {
      name: 'HKDF',
      hash: 'SHA-256',
      salt: new Uint8Array(0),
      info: new TextEncoder().encode('subsenpai-xdata-v1')
    },
    hkdfKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['decrypt']
  );
  sessionId = hs.session_id;
}

async function openEnvelope(env) {
  const payload = b64urlDecode(env.xdata);
  const plain = await crypto.subtle.decrypt(
    {
      name: 'AES-GCM',
      iv: payload.slice(0, 12),
      additionalData: new TextEncoder().encode(String(env.xtime))
    },
    aesKey,
    payload.slice(12)
  );
  return JSON.parse(new TextDecoder().decode(plain));
}

// 401 "perangkat_dicabut:<nama>": lempar error yang sama sesudah
// memberitahu layout lewat event 'session-revoked' (detail = nama
// perangkat yang dicabut). Bukan error — diam bila bukan 401 ini.
function throwIfRevoked(r) {
  const errText = (r.body && (r.body.error || r.body.message)) || '';
  if (r.status === 401 && errText.startsWith('perangkat_dicabut:')) {
    const name = errText.slice('perangkat_dicabut:'.length).trim();
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('session-revoked', { detail: name }));
    }
    throw Object.assign(new Error(errText), { status: r.status, body: r.body });
  }
}

/**
 * Panggil API: dekripsi envelope otomatis; sesi kedaluwarsa ->
 * handshake ulang sekali. Mengembalikan { status, body }.
 * Lempar Error bila body berisi {error} dengan status >= 400.
 */
export async function api(path, opts = {}) {
  await ensureSession();
  const attempt = async () => {
    const headers = { 'X-Session-Id': sessionId, 'Content-Type': 'application/json' };
    if (jwt) headers['Authorization'] = 'Bearer ' + jwt;
    // X-Device-Id AKTIF LAGI (27 Nov 20:16): backend sudah mengizinkan
    // header ini di preflight CORS (terverifikasi live: Allow-Headers
    // memuat X-Device-Id) — installer sudah jalan, jadi identitas tamu
    // kembali stabil lintas ganti IP (laporan + ad-unlock tamu).
    const deviceId = getDeviceId();
    if (deviceId) headers['X-Device-Id'] = deviceId;
    const res = await fetchTimeout(API_BASE + path, {
      method: opts.method || 'GET',
      headers,
      body: opts.body ? JSON.stringify(opts.body) : undefined
    });
    const data = await res.json().catch(() => ({}));
    if (data && data.xdata) return { status: res.status, body: await openEnvelope(data) };
    return { status: res.status, body: data };
  };
  let r = await attempt();
  // 401 perangkat_dicabut: sesi ini dicabut permanen (login di
  // perangkat lain melewati batas — premium 2, gratis 1 — atau dicabut
  // manual di halaman Perangkat). JANGAN handshake ulang (sesi ECDH
  // bukan masalahnya): beri tahu layout lewat event supaya token
  // dihapus dan pengguna diarahkan ke /masuk, lalu lempar error
  // yang sama. Dicek SEBELUM logika retry sesi, dan sekali lagi
  // sesudah retry (bila percobaan ulang yang mengembalikan 401 ini).
  throwIfRevoked(r);
  if (r.status === 401 && /sesi/i.test((r.body && r.body.error) || '')) {
    sessionId = null;
    aesKey = null;
    await ensureSession();
    r = await attempt();
    throwIfRevoked(r);
  }
  // 401 padahal token JWT dikirim (dan bukan perangkat_dicabut / sesi
  // ECDH — keduanya sudah ditangani di atas): token tersimpan sudah
  // mati (kedaluwarsa 30 hari / tidak valid / akun dihapus). Beri tahu
  // layout lewat event 'session-expired' supaya token dihapus dan
  // pengguna diarahkan ke /masuk — di sana One Tap bisa masuk lagi
  // otomatis tanpa klik bila sesi Google masih ada.
  if (r.status === 401 && jwt) {
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('session-expired'));
    }
  }
  if (r.status >= 400) {
    throw Object.assign(
      new Error((r.body && (r.body.error || r.body.message)) || 'Kesalahan server (' + r.status + ')'),
      { status: r.status, body: r.body }
    );
  }
  return r.body;
}

/** True bila error adalah penolakan gerbang premium (403 premium_required). */
export function isPremiumRequired(e) {
  return e?.status === 403 || /premium_required/i.test(e?.message || '');
}

/* ================= Endpoint yang dipakai web ================= */

export const getHome = () => api('/api/v1/home');

export const getTitles = (params = {}) => {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== '' && v != null) q.set(k, String(v));
  return api('/api/v1/titles?' + q.toString());
};

export const getTitle = (id) => api('/api/v1/titles/' + encodeURIComponent(id));

export const getPlayback = (episodeId) =>
  api('/api/v1/episodes/' + encodeURIComponent(episodeId) + '/playback');

export const getSchedule = (type) => api('/api/v1/schedule?type=' + encodeURIComponent(type));

export const getGenres = (type = '') =>
  api('/api/v1/genres' + (type ? '?type=' + encodeURIComponent(type) : ''));

export const getFilters = (type) =>
  api('/api/v1/filters?type=' + encodeURIComponent(type));

export const getLiveBrowse = (type, genre, page = 1, opts = {}) => {
  const p = new URLSearchParams({ type, page: String(page) });
  if (genre) p.set('genre', genre);
  const genres = opts.genres?.length ? opts.genres : [];
  if (genres.length) p.set('genres', genres.join(','));
  if (opts.year) p.set('year', String(opts.year));
  if (opts.country) p.set('country', opts.country);
  if (opts.sort) p.set('sort', opts.sort);
  return api(`/api/v1/live-browse?${p.toString()}`);
};

/* ================= TV Langsung (IPTV, 28 Nov 2026) ================= */
// GET /api/v1/tv/channels?category=&country=&stability= → {items:
// [{id, name, slug, logo_url, stream_url, country, category, quality,
// stability, source, http_referer?, http_user_agent?}]} — publik,
// gratis semua tier. stability 'bagus' = stream resmi HTTPS
// terverifikasi; 'tidak stabil' = restream IP / belum terverifikasi.
export const getTVChannels = (params = {}) => {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== '' && v != null) q.set(k, String(v));
  const s = q.toString();
  return api('/api/v1/tv/channels' + (s ? '?' + s : ''));
};

export const getTVChannel = (id) => api('/api/v1/tv/channels/' + encodeURIComponent(id));

// Impor satu judul hasil telusur live ke katalog (cermin
// Models.importTitle aplikasi); mengembalikan {id, slug, created}.
export const importTitle = (type, item, genre = '') =>
  api('/api/v1/titles/import', {
    method: 'POST',
    body: {
      type,
      url: item.url,
      title: item.title,
      poster_url: item.poster_url || '',
      year: item.year || 0,
      genres: item.genres || [],
      ...(genre ? { genre } : {})
    }
  });

export const getPlans = () => api('/api/v1/plans');
export const getPaymentMethods = () => api('/api/v1/payment-methods');
export const getAppConfig = () => api('/api/v1/app-config');
// Status premium terbaru dari server (sumber kebenaran: premium_until di
// DB) — dipakai gate iklan: profil tersimpan bisa basi sesudah beli /
// tukar voucher / klaim trial.
export const getPremiumStatus = () => api('/api/v1/premium/status');

// Nama perangkat web yang mudah dikenali di halaman Perangkat
// ("Web · Chrome · Windows") — dikirim saat login supaya backend
// mencatat perangkat ini; tanpa device_id, registerDevice backend
// melewatkan pencatatan dan batas sesi tidak pernah berlaku di web.
function webDeviceName() {
  try {
    const ua = (typeof navigator !== 'undefined' && navigator.userAgent) || '';
    const browser = /Edg\//.test(ua)
      ? 'Edge'
      : /Chrome\//.test(ua)
        ? 'Chrome'
        : /Firefox\//.test(ua)
          ? 'Firefox'
          : /Safari\//.test(ua)
            ? 'Safari'
            : 'Browser';
    const os = /Android/.test(ua)
      ? 'Android'
      : /iPhone|iPad/.test(ua)
        ? 'iOS'
        : /Windows/.test(ua)
          ? 'Windows'
          : /Mac OS/.test(ua)
            ? 'macOS'
            : /Linux/.test(ua)
              ? 'Linux'
              : '';
    return 'Web · ' + browser + (os ? ' · ' + os : '');
  } catch {
    return 'Web';
  }
}

// Login Google; referralCode (kode teman pengundang) hanya dikirim bila
// ada — backend hanya memakainya untuk akun BARU (voucher referral
// 3 hari ke pengundang + pendaftar, auth.go).
export const googleLogin = (idToken, referralCode) => {
  const body = { id_token: idToken, device_id: getDeviceId(), device_name: webDeviceName() };
  if (referralCode) body.referral_code = referralCode;
  return api('/api/v1/auth/google', { method: 'POST', body });
};

/* ================= Perangkat login (batas sesi) ================= */
// GET /api/v1/devices → {devices: [{device_id, device_name?,
// last_seen_at, created_at}]} — perangkat aktif akun ini.
export const getDevices = () => api('/api/v1/devices');

// Cabut satu perangkat milik sendiri; perangkat itu langsung 401
// perangkat_dicabut pada request berikutnya.
export const revokeDevice = (deviceId) =>
  api('/api/v1/devices/revoke', { method: 'POST', body: { device_id: deviceId } });

export const checkout = (body) =>
  api('/api/v1/payments/checkout', { method: 'POST', body });

export const redeemVoucher = (code) =>
  api('/api/v1/vouchers/redeem', { method: 'POST', body: { code } });

export const getMyVouchers = () => api('/api/v1/vouchers/mine');

export const paymentStatus = (sessionId) =>
  api('/api/v1/payments/status/' + encodeURIComponent(sessionId));

// Riwayat transaksi (sesi pembayaran) milik user — GET
// /api/v1/transactions → {transactions: [{id, purpose, provider,
// plan_id, plan_label, amount, status, created_at, expires_at}]}
// (maks 50, terbaru dulu).
export const getTransactions = () => api('/api/v1/transactions');

export const adUnlockStart = (episodeId) =>
  api(`/api/v1/episodes/${encodeURIComponent(episodeId)}/ad-unlock/start`, { method: 'POST' });

export const adUnlockComplete = (episodeId, body) =>
  api(`/api/v1/episodes/${encodeURIComponent(episodeId)}/ad-unlock/complete`, {
    method: 'POST',
    body
  });

/** Poster sumber dilewatkan img-proxy backend (Referer benar, anti 403 hotlink). */
export function posterUrl(u) {
  if (!u) return '';
  if (!/^https?:\/\//i.test(u)) return u;
  return API_BASE + '/api/v1/img-proxy?u=' + encodeURIComponent(u);
}

/* ================= Feed komunitas (butuh login) ================= */

export const getFeed = ({ tab = 'teman', category = '', q = '', cursor = '', group_id = '' } = {}) => {
  const params = new URLSearchParams();
  if (tab) params.set('tab', tab);
  if (category) params.set('category', category);
  if (q) params.set('q', q);
  if (cursor) params.set('cursor', cursor);
  if (group_id) params.set('group_id', group_id);
  return api('/api/v1/feed?' + params.toString());
};

export const createPost = ({ body, image_url = '', category = 'umum', title_id = null, visibility = 'publik' }) =>
  api('/api/v1/feed/posts', {
    method: 'POST',
    body: { body, image_url, category, visibility, ...(title_id ? { title_id } : {}) }
  });

/** Daftar grup saya + grup publik (handleGroupsList → {groups: [...]}). */
export const getGroups = () => api('/api/v1/groups');

/* ================= Notifikasi (butuh login) ================= */
// GET /api/v1/feed/notifications[?kind=social|broadcast|release]
// → {items: [{id, kind, title, body, ref_post_id?, ref_title_id?,
//    actor_id?, actor_avatar_url, read_at?, created_at}], unread}
export const getNotifications = (kind = '') =>
  api('/api/v1/feed/notifications' + (kind ? '?kind=' + encodeURIComponent(kind) : ''));

// Tandai dibaca; kind membatasi hanya jenis itu, tanpa kind = semua.
export const markNotificationsRead = (kind = '') =>
  api('/api/v1/feed/notifications/read', {
    method: 'POST',
    body: kind ? { kind } : {}
  });

export const likePost = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/like`, { method: 'POST' });

export const repostPost = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/repost`, { method: 'POST' });

export const bookmarkPost = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/bookmark`, { method: 'POST' });

export const sharePost = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/share`, { method: 'POST' });

export const deletePost = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}`, { method: 'DELETE' });

export const getPost = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}`);

export const getComments = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/comments`);

export const createComment = (id, body, parentId = null) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/comments`, {
    method: 'POST',
    body: parentId ? { body, parent_id: parentId } : { body }
  });

export const likeComment = (id) =>
  api(`/api/v1/feed/comments/${encodeURIComponent(id)}/like`, { method: 'POST' });

export const deleteComment = (id) =>
  api(`/api/v1/feed/comments/${encodeURIComponent(id)}`, { method: 'DELETE' });

/** Unggah gambar/video feed (multipart — tidak lewat envelope JSON). */
export async function uploadFeedImage(file) {
  await ensureSession();
  const fd = new FormData();
  fd.append('image', file);
  const headers = { 'X-Session-Id': sessionId };
  if (jwt) headers['Authorization'] = 'Bearer ' + jwt;
  const deviceId = getDeviceId();
  if (deviceId) headers['X-Device-Id'] = deviceId;
  const res = await fetchTimeout(API_BASE + '/api/v1/feed/upload', {
    method: 'POST',
    headers,
    body: fd
  }, 60000);
  const data = await res.json().catch(() => ({}));
  const body = data && data.xdata ? await openEnvelope(data) : data;
  // Perangkat dicabut: beri tahu layout seperti api() (upload ini
  // tidak lewat api() sehingga perlu pengecekan sendiri).
  throwIfRevoked({ status: res.status, body });
  if (!res.ok || body?.error) {
    throw new Error(body?.error || 'Gagal mengunggah media (' + res.status + ')');
  }
  return body; // {url}
}

/* ================= Riwayat tontonan (butuh login) ================= */

/* ================= Daftar Saya / watchlist (butuh login) ================= */

export const getWatchlist = () => api('/api/v1/watchlist');

export const checkWatchlist = (id) =>
  api('/api/v1/watchlist/' + encodeURIComponent(id));

export const addWatchlist = (id) =>
  api('/api/v1/watchlist/' + encodeURIComponent(id), { method: 'POST', body: {} });

export const removeWatchlist = (id) =>
  api('/api/v1/watchlist/' + encodeURIComponent(id), { method: 'DELETE' });

/* ================= Laporan masalah episode (tamu boleh, via device id) ================= */

export const createReport = (episodeId, reason, note = '') =>
  api('/api/v1/reports', { method: 'POST', body: { episode_id: episodeId, reason, note } });

export const getHistory = (limit = 20) => api(`/api/v1/history/watch?limit=${limit}`);

export const reportHistory = (episodeId, positionSec = 0) =>
  api('/api/v1/history/watch', {
    method: 'POST',
    body: { episode_id: episodeId, position_sec: Math.floor(positionSec) }
  });
