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
    // JANGAN kirim X-Device-Id dulu: backend lama menolaknya di
    // preflight CORS (Allow-Headers tak memuatnya) sehingga SEMUA
    // panggilan API diblokir peramban (27 Nov 2026, a380001).
    // Backend sudah diperbaiki (cors.go) — header ini diaktifkan
    // lagi sesudah Rams menjalankan install-vps.sh berikutnya.
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
  if (r.status === 401 && /sesi/i.test((r.body && r.body.error) || '')) {
    sessionId = null;
    aesKey = null;
    await ensureSession();
    r = await attempt();
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

export const getLiveBrowse = (type, genre, page = 1) =>
  api(
    `/api/v1/live-browse?type=${encodeURIComponent(type)}&genre=${encodeURIComponent(genre)}&page=${page}`
  );

export const getPlans = () => api('/api/v1/plans');
export const getPaymentMethods = () => api('/api/v1/payment-methods');
export const getAppConfig = () => api('/api/v1/app-config');
// Status premium terbaru dari server (sumber kebenaran: premium_until di
// DB) — dipakai gate iklan: profil tersimpan bisa basi sesudah beli /
// tukar voucher / klaim trial.
export const getPremiumStatus = () => api('/api/v1/premium/status');

export const googleLogin = (idToken) =>
  api('/api/v1/auth/google', { method: 'POST', body: { id_token: idToken } });

export const checkout = (body) =>
  api('/api/v1/payments/checkout', { method: 'POST', body });

export const redeemVoucher = (code) =>
  api('/api/v1/vouchers/redeem', { method: 'POST', body: { code } });

export const getMyVouchers = () => api('/api/v1/vouchers/mine');

export const paymentStatus = (sessionId) =>
  api('/api/v1/payments/status/' + encodeURIComponent(sessionId));

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

export const getFeed = ({ tab = 'teman', category = '', q = '', cursor = '' } = {}) => {
  const params = new URLSearchParams();
  if (tab) params.set('tab', tab);
  if (category) params.set('category', category);
  if (q) params.set('q', q);
  if (cursor) params.set('cursor', cursor);
  return api('/api/v1/feed?' + params.toString());
};

export const createPost = ({ body, image_url = '', category = 'umum', title_id = null }) =>
  api('/api/v1/feed/posts', {
    method: 'POST',
    body: { body, image_url, category, visibility: 'publik', ...(title_id ? { title_id } : {}) }
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

export const getComments = (id) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/comments`);

export const createComment = (id, body) =>
  api(`/api/v1/feed/posts/${encodeURIComponent(id)}/comments`, {
    method: 'POST',
    body: { body }
  });

/** Unggah gambar/video feed (multipart — tidak lewat envelope JSON). */
export async function uploadFeedImage(file) {
  await ensureSession();
  const fd = new FormData();
  fd.append('image', file);
  const headers = { 'X-Session-Id': sessionId };
  if (jwt) headers['Authorization'] = 'Bearer ' + jwt;
  // X-Device-Id tidak dikirim dulu (alasan sama seperti di api()).
  const res = await fetchTimeout(API_BASE + '/api/v1/feed/upload', {
    method: 'POST',
    headers,
    body: fd
  }, 60000);
  const data = await res.json().catch(() => ({}));
  const body = data && data.xdata ? await openEnvelope(data) : data;
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
