/* SubSenpai — Dashboard Admin (vanilla JS).
 *
 * Berbicara ke backend yang sama dengan aplikasi Android:
 * handshake ECDH (POST /api/v1/session) -> envelope {xdata, xtime}
 * AES-256-GCM, lalu JWT dari login Google. Hanya akun dengan
 * role 'admin' yang dibiarkan masuk; tidak ada pendaftaran di sini.
 */
'use strict';

const API_BASE = 'https://api.subsenpai.web.id';
const GOOGLE_CLIENT_ID = '673952283037-h0qkk02cvlqdjgnv8f5k4os9ti281dta.apps.googleusercontent.com';
const JWT_KEY = 'ssadmin.jwt';

/* ================= Kripto sesi (xdata) =================
 * X25519 ditulis manual dengan BigInt (RFC 7748) karena dukungan
 * WebCrypto untuk X25519 belum merata antar peramban; HKDF dan
 * AES-GCM memakai WebCrypto. */
// [CRYPTO-BEGIN]
const _P = (1n << 255n) - 19n;
const _mod = (a) => ((a % _P) + _P) % _P;
function _leToBigInt(bytes) {
  let n = 0n;
  for (let i = bytes.length - 1; i >= 0; i--) n = (n << 8n) | BigInt(bytes[i]);
  return n;
}
function _bigIntToLe(n, len) {
  const out = new Uint8Array(len);
  for (let i = 0; i < len; i++) { out[i] = Number(n & 0xffn); n >>= 8n; }
  return out;
}
function _modInv(a) {
  let e = _P - 2n, base = _mod(a), r = 1n;
  while (e > 0n) { if (e & 1n) r = _mod(r * base); base = _mod(base * base); e >>= 1n; }
  return r;
}
function x25519(scalarBytes, uBytes) {
  const k = Uint8Array.from(scalarBytes);
  k[0] &= 248; k[31] &= 127; k[31] |= 64;
  const n = _leToBigInt(k);
  const u = Uint8Array.from(uBytes); u[31] &= 127;
  const x1 = _mod(_leToBigInt(u));
  let x2 = 1n, z2 = 0n, x3 = x1, z3 = 1n, swap = 0n;
  const a24 = 121665n;
  for (let t = 254; t >= 0; t--) {
    const kt = (n >> BigInt(t)) & 1n;
    swap ^= kt;
    if (swap === 1n) { [x2, x3] = [x3, x2]; [z2, z3] = [z3, z2]; }
    swap = kt;
    const A = _mod(x2 + z2), AA = _mod(A * A);
    const B = _mod(x2 - z2), BB = _mod(B * B);
    const E = _mod(AA - BB);
    const C = _mod(x3 + z3), D = _mod(x3 - z3);
    const DA = _mod(D * A), CB = _mod(C * B);
    x3 = _mod((DA + CB) * (DA + CB));
    z3 = _mod(x1 * _mod((DA - CB) * (DA - CB)));
    x2 = _mod(AA * BB);
    z2 = _mod(E * _mod(AA + _mod(a24 * E)));
  }
  if (swap === 1n) { [x2, x3] = [x3, x2]; [z2, z3] = [z3, z2]; }
  return _bigIntToLe(_mod(x2 * _modInv(z2)), 32);
}
const X25519_BASE = new Uint8Array(32); X25519_BASE[0] = 9;
// [CRYPTO-END]

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

/* localStorage bisa MELEMPAR error di sebagian peramban (Brave
 * Shields / mode blokir cookie): setItem/removeItem gagal sementara
 * getItem jalan — login membeku diam bila tidak dibungkus. Semua
 * akses storage lewat pembungkus aman ini dengan cadangan memori.
 *
 * Sejak v29 (permintaan Rams 2026-11-24): JWT disimpan di
 * localStorage, BUKAN sessionStorage, agar sesi login tetap ada
 * setelah peramban ditutup — tidak perlu login ulang tiap membuka
 * dashboard (JWT backend sendiri berlaku 30 hari). Token lama yang
 * masih tersimpan di sessionStorage dipindahkan sekali saat
 * ditemukan, jadi sesi yang sedang berjalan tidak terputus. */
const memStore = {};
const storeGet = (k) => {
  try {
    const v = localStorage.getItem(k);
    if (v != null) return v;
  } catch (e) { return memStore[k] || null; }
  try {
    const lama = sessionStorage.getItem(k);
    if (lama != null) {
      try { localStorage.setItem(k, lama); sessionStorage.removeItem(k); } catch (e) { /* abaikan */ }
      return lama;
    }
  } catch (e) { /* abaikan */ }
  return memStore[k] || null;
};
const storeSet = (k, v) => { try { localStorage.setItem(k, v); } catch (e) { memStore[k] = v; } };
const storeDel = (k) => {
  try { localStorage.removeItem(k); } catch (e) { /* abaikan */ }
  try { sessionStorage.removeItem(k); } catch (e) { /* abaikan */ }
  delete memStore[k];
};

let jwt = storeGet(JWT_KEY) || null;
let me = null;

/* fetch dengan batas waktu — tanpa ini, jaringan yang menggantung
 * (mis. VPN/aplikasi perekam di HP) membuat login diam selamanya. */
async function fetchTimeout(url, opts = {}, ms = 20000) {
  const ctl = new AbortController();
  const timer = setTimeout(() => ctl.abort(), ms);
  try {
    return await fetch(url, { ...opts, signal: ctl.signal });
  } catch (e) {
    if (e.name === 'AbortError') {
      throw new Error('Koneksi ke server timeout. Matikan dulu VPN/aplikasi ' +
        'perekam lalu lintas (mis. HttpCanary) bila sedang aktif, lalu coba lagi.');
    }
    throw e;
  } finally {
    clearTimeout(timer);
  }
}

/* Pelapor langkah login (diagnostik): diisi onGoogleCredential agar
 * halaman menunjukkan persis proses berhenti di langkah mana. */
let stepFn = null;
const step = (t) => { if (stepFn) stepFn(t); };

async function handshake() {
  step('Langkah 1/4: handshake enkripsi ke server…');
  const priv = crypto.getRandomValues(new Uint8Array(32));
  const pub = x25519(priv, X25519_BASE);
  const res = await fetchTimeout(API_BASE + '/api/v1/session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ client_public_key: b64urlEncode(pub) }),
  });
  if (!res.ok) throw new Error('Handshake sesi gagal (' + res.status + ')');
  const hs = await res.json();
  const shared = x25519(priv, b64urlDecode(hs.server_public_key));
  const hkdfKey = await crypto.subtle.importKey('raw', shared, 'HKDF', false, ['deriveKey']);
  aesKey = await crypto.subtle.deriveKey(
    { name: 'HKDF', hash: 'SHA-256', salt: new Uint8Array(0), info: new TextEncoder().encode('subsenpai-xdata-v1') },
    hkdfKey, { name: 'AES-GCM', length: 256 }, false, ['decrypt']);
  sessionId = hs.session_id;
  step('Langkah 2/4: handshake beres, mengirim permintaan…');
}

async function openEnvelope(env) {
  const payload = b64urlDecode(env.xdata);
  const plain = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: payload.slice(0, 12), additionalData: new TextEncoder().encode(String(env.xtime)) },
    aesKey, payload.slice(12));
  return JSON.parse(new TextDecoder().decode(plain));
}

/* Panggil API: dekripsi envelope; sesi kedaluwarsa -> handshake ulang sekali. */
async function api(path, opts = {}) {
  if (!sessionId) await handshake();
  const attempt = async () => {
    const headers = { 'X-Session-Id': sessionId, 'Content-Type': 'application/json' };
    if (jwt) headers['Authorization'] = 'Bearer ' + jwt;
    const res = await fetchTimeout(API_BASE + path, {
      method: opts.method || 'GET', headers,
      body: opts.body ? JSON.stringify(opts.body) : undefined,
    });
    step('Langkah 3/4: respons server diterima, membuka enkripsi…');
    const data = await res.json().catch(() => ({}));
    if (data && data.xdata) return { status: res.status, body: await openEnvelope(data) };
    return { status: res.status, body: data };
  };
  let r = await attempt();
  if (r.status === 401 && /sesi/i.test((r.body && r.body.error) || '')) {
    await handshake();
    r = await attempt();
  }
  return r;
}

/* ================= Util ================= */
const $ = (id) => document.getElementById(id);
const esc = (s) => String(s == null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
const rp = (n) => 'Rp' + Number(n || 0).toLocaleString('id-ID');
const fmtDate = (iso) => iso
  ? new Date(iso).toLocaleString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
  : '—';
const fmtBytes = (n) => {
  n = Number(n || 0);
  const u = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return (i === 0 ? String(n) : n.toFixed(1)) + ' ' + u[i];
};
const fmtUptime = (s) => {
  s = Number(s || 0);
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60);
  return (d > 0 ? d + ' hari ' : '') + h + ' jam ' + m + ' mnt';
};
function resBar(pct) {
  const p = Math.max(0, Math.min(100, pct || 0));
  const cls = p >= 90 ? 'crit' : (p >= 70 ? 'warn' : '');
  return '<div class="bar ' + cls + '"><div style="width:' + p.toFixed(0) + '%"></div></div>';
}
function resCard(lbl, valText, pct, sub) {
  return '<div class="card"><div class="lbl">' + esc(lbl) + '</div>' +
    '<div class="num" style="font-size:1.35rem">' + esc(valText) + '</div>' +
    resBar(pct) +
    (sub ? '<div class="lbl" style="margin-top:6px">' + sub + '</div>' : '') + '</div>';
}
function gaugeCard(label, valueText, unit, pct, color) {
  const p = Math.max(0, Math.min(100, pct || 0));
  return '<div class="card gauge-card"><div class="lbl">' + esc(label) + '</div>' +
    '<div class="gauge"><svg viewBox="0 0 120 120">' +
    '<circle class="g-track" cx="60" cy="60" r="52" pathLength="100"/>' +
    '<circle class="g-val" cx="60" cy="60" r="52" pathLength="100" stroke="' + color +
    '" stroke-dasharray="' + (p * 0.75).toFixed(1) + ' 100"/>' +
    '</svg><div class="g-num">' + esc(valueText) + '<span>' + esc(unit) + '</span></div></div></div>';
}
function sparkSVG(seriesList) {
  const w = 260, h = 44, pad = 2;
  let max = 0;
  seriesList.forEach((s) => s.data.forEach((v) => { if (v > max) max = v; }));
  if (max <= 0) max = 1;
  const lines = seriesList.map((s) => {
    if (s.data.length < 2) return '';
    const pts = s.data.map((v, i) =>
      (pad + i * (w - 2 * pad) / (s.data.length - 1)).toFixed(1) + ',' +
      (h - pad - (v / max) * (h - 2 * pad)).toFixed(1)).join(' ');
    return '<polyline points="' + pts + '" fill="none" stroke="' + s.color + '" stroke-width="1.6"/>';
  }).join('');
  if (!lines) return '';
  return '<svg class="spark" viewBox="0 0 ' + w + ' ' + h + '" preserveAspectRatio="none">' + lines + '</svg>';
}
function sparkCard(label, bigText, seriesList) {
  return '<div class="card"><div class="lbl">' + esc(label) + '</div>' +
    '<div class="num" style="font-size:1.3rem">' + esc(bigText) + '</div>' +
    sparkSVG(seriesList) + '</div>';
}
let toastTimer = null;
function toast(msg) {
  const t = $('toast');
  t.textContent = msg;
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, 3200);
}

/* ================= Login ================= */
/* Tampil/sembunyi dua tampilan utama memakai style.display eksplisit
 * — JANGAN atribut hidden: CSS .layout/.login-wrap sama-sama
 * display:flex dan di sebagian peramban mengalahkan [hidden],
 * membuat perpindahan tampilan tidak terjadi walau JS sudah jalan. */
function showView(which) {
  $('loginView').style.display = which === 'login' ? 'flex' : 'none';
  $('appView').style.display = which === 'app' ? 'flex' : 'none';
}

function showLogin(msg) {
  jwt = null; me = null; stepFn = null;
  storeDel(JWT_KEY);
  showView('login');
  $('loginHint').textContent = 'Gunakan akun Google admin kamu.';
  const err = $('loginError');
  if (msg) { err.textContent = msg; err.hidden = false; } else { err.hidden = true; }
}

/* Jaring pengaman diagnostik: kesalahan JS apa pun (termasuk di
 * fungsi async) ditampilkan di kartu login, bukan hilang diam-diam. */
function surfaceError(label, e) {
  try {
    const err = $('loginError');
    if (err) {
      err.textContent = label + ': ' + (e && e.message ? e.message : String(e));
      err.hidden = false;
    }
    const hint = $('loginHint');
    if (hint) hint.textContent = 'Terjadi kesalahan — lihat pesan merah di atas.';
  } catch (_) { /* abaikan */ }
}
window.addEventListener('error', (ev) => surfaceError('Kesalahan JS', ev.error || ev.message));
window.addEventListener('unhandledrejection', (ev) => surfaceError('Proses gagal', ev.reason));

async function onGoogleCredential(resp) {
  $('loginError').hidden = true;
  stepFn = (t) => { $('loginHint').textContent = t; };
  step('Memeriksa akun…');
  try {
    const r = await api('/api/v1/auth/google', { method: 'POST', body: { id_token: resp.credential } });
    const prof0 = (r.body && r.body.profile) || {};
    step('Langkah 4/4: respons HTTP ' + r.status + ', role="' + (prof0.role || '?') + '" — memeriksa…');
    if (r.status !== 200 || !r.body.token) {
      showLogin('Login gagal: ' + ((r.body && r.body.error) || 'respons tidak dikenal'));
      return;
    }
    const prof = r.body.profile || {};
    if (prof.role !== 'admin') {
      showLogin('Akun ' + (prof.email || '') + ' belum admin (role saat ini: "' +
        (prof.role || '?') + '"). Ubah role-nya menjadi admin di database, lalu login lagi.');
      return;
    }
    jwt = r.body.token;
    storeSet(JWT_KEY, jwt);
    me = prof;
    step('Langkah 5/5: role admin terkonfirmasi, membuka dashboard…');
    enterApp();
  } catch (e) {
    showLogin('Tidak dapat terhubung ke server: ' + e.message);
  }
}

function initGoogleButton() {
  let tries = 0;
  const tick = () => {
    if (window.google && google.accounts && google.accounts.id) {
      google.accounts.id.initialize({ client_id: GOOGLE_CLIENT_ID, callback: onGoogleCredential });
      google.accounts.id.renderButton($('googleBtn'), { theme: 'filled_black', size: 'large', text: 'signin_with' });
      $('loginHint').textContent = 'Gunakan akun Google admin kamu.';
    } else if (++tries < 40) {
      setTimeout(tick, 250);
    } else {
      $('loginHint').textContent = 'Tombol Google gagal dimuat. Periksa koneksi lalu muat ulang halaman.';
    }
  };
  tick();
}

/* ================= Aplikasi ================= */
function enterApp() {
  showView('app');
  stepFn = null;
  $('meName').textContent = (me && (me.display_name || me.email)) || 'Admin';
  $('meEmail').textContent = (me && me.email) || '';
  if (me && me.avatar_url) $('meAvatar').src = me.avatar_url;
  route();
}

const GROUP_LABELS = { va: 'Virtual Account', ewallet: 'E-Wallet', qris: 'QRIS', retail: 'Retail', pulsa: 'Pulsa' };
const TITLES = { ringkasan: 'Ringkasan', metode: 'Metode Pembayaran', paket: 'Paket Premium', pengguna: 'Pengguna', transaksi: 'Transaksi', voucher: 'Voucher', judul: 'Kelola Judul', crash: 'Laporan Crash', pengaturan: 'Pengaturan Aplikasi', pengumuman: 'Pengumuman Banner', notifikasi: 'Notifikasi', laporan: 'Laporan User', 'laporan-detail': 'Detail Laporan', log: 'Log Admin', 'log-detail': 'Detail Log', 'judul-detail': 'Detail Judul', 'pengguna-detail': 'Detail Pengguna', sistem: 'Info Sistem' };

function route() {
  if (window.__pageTimer) { clearInterval(window.__pageTimer); window.__pageTimer = null; }
  const name = (location.hash.replace('#/', '') || 'ringkasan').split('?')[0];
  const page = TITLES[name] ? name : 'ringkasan';
  document.querySelectorAll('#mainNav a').forEach((a) =>
    a.classList.toggle('active', a.dataset.route === page));
  // Submenu yang memuat halaman aktif selalu terbuka.
  document.querySelectorAll('#mainNav .nav-group').forEach((g) => {
    if (g.querySelector('a.active')) g.classList.add('open');
  });
  $('pageTitle').textContent = TITLES[page];
  ({ ringkasan: pageRingkasan, metode: pageMetode, paket: pagePaket, pengguna: pagePengguna, transaksi: pageTransaksi, voucher: pageVoucher, judul: pageJudul, crash: pageCrash, pengaturan: pagePengaturan, pengumuman: pagePengumuman, notifikasi: pageNotifikasi, laporan: pageLaporan, 'laporan-detail': pageLaporanDetail, log: pageLog, 'log-detail': pageLogDetail, 'judul-detail': pageJudulDetail, 'pengguna-detail': pagePenggunaDetail, sistem: pageSistem })[page]();
}

// Poster katalog lewat proxy server: CDN sumber menolak hotlink
// dari domain dashboard, server mengambilkan dengan Referer asal.
function posterSrc(u) {
  if (!u) return '';
  if (u.indexOf(API_BASE) === 0) return u;
  return API_BASE + '/api/v1/img-proxy?u=' + encodeURIComponent(u);
}

function bodyError(r) {
  $('pageBody').innerHTML = '<p class="error">Gagal memuat: ' + esc((r.body && r.body.error) || ('HTTP ' + r.status)) + '</p>';
}

/* ----- Ringkasan ----- */
async function pageRingkasan() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/stats');
  if (r.status !== 200) return bodyError(r);
  const s = r.body;
  const byType = Object.entries(s.titles_by_type || {})
    .map(([t, n]) => esc(t) + ' ' + n).join(' · ');
  $('pageBody').innerHTML =
    '<div class="cards">' +
    card('Total Pengguna', s.users) + card('Premium Aktif', s.premium_users) +
    card('Total Judul', s.titles, byType) + card('Transaksi Lunas', s.paid_transactions) +
    card('Pendapatan Tercatat', rp(s.revenue)) + card('Transaksi Pending', s.pending_transactions) +
    '</div><h2>Transaksi Terbaru</h2><div id="recentTrx"><p class="muted">Memuat…</p></div>';
  const t = await api('/api/v1/admin/transactions?limit=8');
  if (t.status === 200) $('recentTrx').innerHTML = trxTable(t.body.transactions || []);
}
function card(lbl, num, sub) {
  return '<div class="card"><div class="lbl">' + esc(lbl) + '</div><div class="num">' + esc(num) + '</div>' +
    (sub ? '<div class="lbl">' + sub + '</div>' : '') + '</div>';
}

/* ----- Metode Pembayaran ----- */
async function pageMetode() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/payment-methods');
  if (r.status !== 200) return bodyError(r);
  const methods = r.body.methods || [];
  let html = '<p class="muted">Aktif/nonaktifkan channel Tokopay yang tampil di aplikasi. ' +
    'Channel yang belum aktif di akun Tokopay sebaiknya dinonaktifkan di sini agar pengguna tidak gagal bayar.</p>';
  for (const g of Object.keys(GROUP_LABELS)) {
    const items = methods.filter((m) => m.group === g);
    if (!items.length) continue;
    html += '<h2 class="group-title">' + GROUP_LABELS[g] + '</h2><div class="tscroll"><table><tr><th>Metode</th><th>Kode</th><th>Aktif</th></tr>';
    for (const m of items) {
      html += '<tr><td>' + (m.icon_url ? '<img class="method-icon" src="' + esc(m.icon_url) + '" alt="">' : '') + esc(m.name) + '</td>' +
        '<td>' + esc(m.code) + '</td>' +
        '<td><label class="switch"><input type="checkbox" data-method="' + esc(m.code) + '"' + (m.is_active ? ' checked' : '') +
        '><span class="slider"></span></label></td></tr>';
    }
    html += '</table></div>';
  }
  $('pageBody').innerHTML = html;
  document.querySelectorAll('input[data-method]').forEach((el) => {
    el.addEventListener('change', async () => {
      const rr = await api('/api/v1/admin/payment-methods/' + encodeURIComponent(el.dataset.method), {
        method: 'PATCH', body: { is_active: el.checked },
      });
      if (rr.status === 200) toast(el.dataset.method + (el.checked ? ' diaktifkan' : ' dinonaktifkan'));
      else { el.checked = !el.checked; toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status)); }
    });
  });
}

/* ----- Paket Premium ----- */
async function pagePaket() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/plans');
  if (r.status !== 200) return bodyError(r);
  let html = '<p class="muted">Harga & status paket dibaca aplikasi dari tabel ini — perubahan langsung berlaku tanpa rilis ulang.</p>' +
    '<div class="tscroll"><table><tr><th>Paket</th><th>Durasi</th><th>Harga (Rp)</th><th>Aktif</th><th></th></tr>';
  for (const p of r.body.plans || []) {
    html += '<tr><td>' + esc(p.label) + '<div class="muted small">' + esc(p.id) + '</div></td>' +
      '<td>' + p.days + ' hari</td>' +
      '<td><input type="number" min="0" step="500" value="' + p.price + '" id="price-' + esc(p.id) + '" style="width:130px"></td>' +
      '<td><label class="switch"><input type="checkbox" id="active-' + esc(p.id) + '"' + (p.is_active ? ' checked' : '') + '><span class="slider"></span></label></td>' +
      '<td><button class="btn small" data-save-plan="' + esc(p.id) + '">Simpan</button></td></tr>';
  }
  $('pageBody').innerHTML = html + '</table></div>';
  document.querySelectorAll('button[data-save-plan]').forEach((el) => {
    el.addEventListener('click', async () => {
      const id = el.dataset.savePlan;
      const price = parseInt($('price-' + id).value, 10);
      const rr = await api('/api/v1/admin/plans/' + encodeURIComponent(id), {
        method: 'PATCH', body: { price: price, is_active: $('active-' + id).checked },
      });
      toast(rr.status === 200 ? 'Paket ' + id + ' disimpan' : 'Gagal: ' + ((rr.body && rr.body.error) || rr.status));
    });
  });
}

/* ----- Pengguna ----- */
async function pagePengguna() {
  $('pageBody').innerHTML =
    '<div class="toolbar"><input type="search" id="userQ" placeholder="Cari email atau nama…">' +
    '<button class="btn" id="userSearchBtn">Cari</button></div><div id="userList"><p class="muted">Memuat…</p></div>';
  const load = async () => {
    const q = encodeURIComponent($('userQ').value.trim());
    const r = await api('/api/v1/admin/users?limit=100&q=' + q);
    if (r.status !== 200) { $('userList').innerHTML = '<p class="error">Gagal memuat pengguna.</p>'; return; }
    let html = '<div class="tscroll"><table><tr><th>Pengguna</th><th>Role</th><th>Status</th><th>Premium</th><th>Detail</th></tr>';
    for (const u of r.body.users || []) {
      html += '<tr><td>' + (u.avatar_url ? '<img class="avatar" src="' + esc(u.avatar_url) + '" referrerpolicy="no-referrer" alt="">' : '') +
        esc(u.display_name || '(tanpa nama)') + '<div class="muted small">' + esc(u.email) + '</div></td>' +
        '<td><span class="badge ' + esc(u.role) + '">' + esc(u.role) + '</span></td>' +
        '<td>' + statusAkunBadge(u.status) + '</td>' +
        '<td>' + (u.premium_aktif ? '<span class="badge premium">aktif s.d. ' + esc(fmtDate(u.premium_until)) + '</span>' : '<span class="muted">—</span>') + '</td>' +
        '<td><a class="btn ghost small" href="#/pengguna-detail?id=' + esc(u.id) + '">Detail</a></td></tr>';
    }
    $('userList').innerHTML = html + '</table></div>';
  };
  $('userSearchBtn').addEventListener('click', load);
  $('userQ').addEventListener('keydown', (e) => { if (e.key === 'Enter') load(); });
  load();
}
function statusAkunBadge(s) {
  if (s === 'banned') return '<span class="badge expired">Ban</span>';
  if (s === 'suspended') return '<span class="badge info">Ditangguhkan</span>';
  return '<span class="badge premium">Aktif</span>';
}
// Halaman detail satu pengguna: role, status akun (aktif / ban /
// ditangguhkan) + catatan admin diatur dari sini. Akun nonaktif
// kehilangan akses aplikasi (login ditolak, sesi ditolak server).
async function pagePenggunaDetail() {
  const id = new URLSearchParams((location.hash.split('?')[1] || '')).get('id') || '';
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/users/' + encodeURIComponent(id));
  if (r.status !== 200 || !r.body.user) {
    $('pageBody').innerHTML = '<p class="error">Pengguna tidak ditemukan.</p>' +
      '<p><a class="btn ghost" href="#/pengguna">← Kembali ke Daftar Pengguna</a></p>';
    return;
  }
  const u = r.body.user;
  const isMe = me && u.id === me.id;
  const opt = (v, lbl) => '<option value="' + v + '"' + (u.status === v ? ' selected' : '') + '>' + lbl + '</option>';
  $('pageBody').innerHTML =
    '<p><a class="btn ghost small" href="#/pengguna">← Kembali ke Daftar Pengguna</a></p>' +
    '<div class="card"><div style="display:flex;gap:14px;align-items:center;flex-wrap:wrap">' +
    (u.avatar_url ? '<img class="avatar" src="' + esc(u.avatar_url) + '" referrerpolicy="no-referrer" alt="" style="width:56px;height:56px">' : '') +
    '<div><h2 style="margin:0">' + esc(u.display_name || '(tanpa nama)') + '</h2>' +
    '<div class="muted">' + esc(u.email) + '</div></div></div>' +
    '<dl class="kv" style="margin-top:16px">' +
    '<dt>Role</dt><dd><span class="badge ' + esc(u.role) + '">' + esc(u.role) + '</span></dd>' +
    '<dt>Premium</dt><dd>' + (u.premium_aktif ? 'Aktif s.d. ' + esc(fmtDate(u.premium_until)) : 'Tidak aktif') + '</dd>' +
    '<dt>Status akun</dt><dd>' + statusAkunBadge(u.status) + '</dd>' +
    '<dt>Catatan admin</dt><dd>' + esc(u.admin_note || '—') + '</dd>' +
    '</dl>' +
    (isMe ? '<p class="muted">Ini akunmu sendiri — role dan statusnya tidak bisa diubah dari sini.</p>' :
      '<div class="toolbar">' +
      (u.role === 'admin'
        ? '<button class="btn ghost" id="pg-role">Turunkan jadi User</button>'
        : '<button class="btn purple" id="pg-role">Jadikan Admin</button>') + '</div>' +
      '<h3 style="margin-bottom:8px">Ubah Status & Catatan</h3>' +
      '<div class="toolbar"><label>Status akun <select id="pg-status">' +
      opt('active', 'Aktif') + opt('suspended', 'Ditangguhkan') + opt('banned', 'Ban') +
      '</select></label></div>' +
      '<textarea id="pg-note" rows="3" maxlength="1000" placeholder="Catatan admin — ditampilkan ke user saat akunnya ditahan" style="width:100%">' + esc(u.admin_note || '') + '</textarea>' +
      '<p><button class="btn" id="pg-save">Simpan Status</button></p>') +
    '</div>';
  if (isMe) return;
  $('pg-role').addEventListener('click', async () => {
    const role = u.role === 'admin' ? 'user' : 'admin';
    const msg = role === 'admin'
      ? 'Angkat pengguna ini menjadi ADMIN? Ia akan bisa membuka dashboard ini.'
      : 'Turunkan pengguna ini menjadi user biasa?';
    if (!confirm(msg)) return;
    const rr = await api('/api/v1/admin/users/' + encodeURIComponent(id), { method: 'PATCH', body: { role } });
    if (rr.status === 200) { toast('Role diubah menjadi ' + role); pagePenggunaDetail(); }
    else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  });
  $('pg-save').addEventListener('click', async () => {
    const status = $('pg-status').value;
    const note = $('pg-note').value.trim();
    if (status !== 'active' && status !== u.status &&
      !confirm('Tahan akun ini? User langsung kehilangan akses ke aplikasi (sesi yang berjalan ikut ditolak).')) return;
    const rr = await api('/api/v1/admin/users/' + encodeURIComponent(id) + '/status', {
      method: 'PATCH', body: { status, note },
    });
    if (rr.status === 200) { toast('Status akun disimpan: ' + status); pagePenggunaDetail(); }
    else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  });
}

/* ----- Transaksi ----- */
function trxTable(list) {
  if (!list.length) return '<p class="muted">Belum ada transaksi.</p>';
  let html = '<div class="tscroll"><table><tr><th>Waktu</th><th>Pengguna</th><th>Keperluan</th><th>Provider</th><th>Jumlah</th><th>Status</th></tr>';
  for (const t of list) {
    html += '<tr><td>' + esc(fmtDate(t.created_at)) + '</td>' +
      '<td>' + esc(t.display_name || '(tanpa nama)') + '<div class="muted small">' + esc(t.email) + '</div></td>' +
      '<td>' + esc(t.purpose === 'voucher' ? 'Voucher' : 'Langganan') + (t.plan ? '<div class="muted small">' + esc(t.plan) + '</div>' : '') + '</td>' +
      '<td>' + esc(t.provider) + '</td><td>' + rp(t.amount) + '</td>' +
      '<td><span class="badge ' + esc(t.status) + '">' + esc(t.status) + '</span></td></tr>';
  }
  return html + '</table></div>';
}
/* ----- Transaksi (dengan edit & hapus) ----- */
let trxCache = [];
let planCache = [];
async function pageTransaksi() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const [r, rp2] = await Promise.all([
    api('/api/v1/admin/transactions?limit=100'),
    api('/api/v1/admin/plans'),
  ]);
  if (r.status !== 200) return bodyError(r);
  trxCache = r.body.transactions || [];
  planCache = rp2.status === 200 ? (rp2.body.plans || []) : [];
  renderTrx(null);
}
function renderTrx(editId) {
  if (!trxCache.length) { $('pageBody').innerHTML = '<p class="muted">Belum ada transaksi.</p>'; return; }
  let html = '<p class="muted">Mengubah status menjadi <b>paid</b> mengaktifkan hak pengguna ' +
    '(premium diperpanjang / voucher bisa ditukar), sama seperti callback gateway. ' +
    'Menghapus transaksi tidak mencabut premium yang sudah terlanjur aktif.</p>' +
    '<div class="tscroll"><table><tr><th>Waktu</th><th>Pengguna</th><th>Keperluan</th><th>Provider</th><th>Jumlah</th><th>Status</th><th>Aksi</th></tr>';
  for (const t of trxCache) {
    if (t.id === editId) {
      const planOpts = [''].concat(planCache.map((p) => p.id)).map((id) =>
        '<option value="' + esc(id) + '"' + (id === t.plan ? ' selected' : '') + '>' + esc(id || '(tanpa paket)') + '</option>').join('');
      const statusOpts = ['pending', 'paid', 'expired'].map((s) =>
        '<option value="' + s + '"' + (s === t.status ? ' selected' : '') + '>' + s + '</option>').join('');
      html += '<tr><td>' + esc(fmtDate(t.created_at)) + '</td>' +
        '<td>' + esc(t.display_name || '(tanpa nama)') + '<div class="muted small">' + esc(t.email) + '</div></td>' +
        '<td><select id="edit-plan">' + planOpts + '</select><div class="muted small">' + esc(t.purpose) + '</div></td>' +
        '<td>' + esc(t.provider) + '</td>' +
        '<td><input type="number" id="edit-amount" value="' + t.amount + '" min="1" step="500" style="width:110px"></td>' +
        '<td><select id="edit-status">' + statusOpts + '</select></td>' +
        '<td><div class="row-actions"><button class="btn small" data-trx-save="' + esc(t.id) + '">Simpan</button>' +
        '<button class="btn ghost small" data-trx-cancel="1">Batal</button></div></td></tr>';
    } else {
      html += '<tr><td>' + esc(fmtDate(t.created_at)) + '</td>' +
        '<td>' + esc(t.display_name || '(tanpa nama)') + '<div class="muted small">' + esc(t.email) + '</div></td>' +
        '<td>' + esc(t.purpose === 'voucher' ? 'Voucher' : 'Langganan') + (t.plan ? '<div class="muted small">' + esc(t.plan) + '</div>' : '') + '</td>' +
        '<td>' + esc(t.provider) + '</td><td>' + rp(t.amount) + '</td>' +
        '<td><span class="badge ' + esc(t.status) + '">' + esc(t.status) + '</span></td>' +
        '<td><div class="row-actions"><button class="btn ghost small" data-trx-edit="' + esc(t.id) + '">Edit</button>' +
        '<button class="btn small" data-trx-del="' + esc(t.id) + '">Hapus</button></div></td></tr>';
    }
  }
  $('pageBody').innerHTML = html + '</table></div>';
  document.querySelectorAll('button[data-trx-edit]').forEach((el) =>
    el.addEventListener('click', () => renderTrx(el.dataset.trxEdit)));
  document.querySelectorAll('button[data-trx-cancel]').forEach((el) =>
    el.addEventListener('click', () => renderTrx(null)));
  document.querySelectorAll('button[data-trx-save]').forEach((el) =>
    el.addEventListener('click', async () => {
      const id = el.dataset.trxSave;
      const body = {
        status: $('edit-status').value,
        amount: parseInt($('edit-amount').value, 10),
        plan: $('edit-plan').value,
      };
      const rr = await api('/api/v1/admin/transactions/' + encodeURIComponent(id), { method: 'PATCH', body });
      if (rr.status === 200) {
        toast('Transaksi disimpan.' + (rr.body.note ? ' ' + rr.body.note : ''));
        pageTransaksi();
      } else {
        toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
      }
    }));
  document.querySelectorAll('button[data-trx-del]').forEach((el) =>
    el.addEventListener('click', async () => {
      const t = trxCache.find((x) => x.id === el.dataset.trxDel);
      const msg = 'Hapus permanen transaksi ' + (t ? (t.email + ' — ' + rp(t.amount) + ' (' + t.status + ')') : 'ini') +
        '?\n\nPremium/voucher yang sudah terlanjur aktif dari transaksi ini TIDAK ikut dicabut.';
      if (!confirm(msg)) return;
      const rr = await api('/api/v1/admin/transactions/' + encodeURIComponent(el.dataset.trxDel), { method: 'DELETE' });
      if (rr.status === 200) { toast('Transaksi dihapus.'); pageTransaksi(); }
      else toast('Gagal menghapus: ' + ((rr.body && rr.body.error) || rr.status));
    }));
}

/* ----- Voucher ----- */
function copyText(text, msg) {
  const done = () => toast(msg || 'Disalin.');
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(done).catch(() => fallbackCopy(text, done));
  } else {
    fallbackCopy(text, done);
  }
}
function fallbackCopy(text, done) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.position = 'fixed'; ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  try { document.execCommand('copy'); done(); } catch (e) { toast('Gagal menyalin — salin manual ya.'); }
  ta.remove();
}

async function pageVoucher() {
  $('pageBody').innerHTML =
    '<div class="card" style="margin-bottom:20px"><h2 style="margin-top:0">Buat Voucher Hadiah</h2>' +
    '<p class="muted">Kode yang dibuat di sini ditukar pengguna lewat aplikasi ' +
    '(Premium › Beli Voucher › Tukar kode) dan langsung menambah hari premium penukar.</p>' +
    '<div class="toolbar">' +
    '<label>Durasi (hari)<br><input type="number" id="vc-days" value="30" min="1" max="3650" style="width:120px"></label>' +
    '<label>Jumlah kode<br><input type="number" id="vc-qty" value="1" min="1" max="50" style="width:110px"></label>' +
    '<label>Masa berlaku kode (hari, opsional)<br><input type="number" id="vc-valid" min="0" placeholder="tanpa batas" style="width:170px"></label>' +
    '</div>' +
    '<div class="toolbar"><label style="flex:1">Khusus email pengguna ini (opsional)<br>' +
    '<input type="text" id="vc-email" placeholder="kosongkan = siapa saja boleh menukar" style="width:100%"></label></div>' +
    '<button class="btn" id="vc-create">Buat Voucher</button>' +
    '<div id="vc-result" style="margin-top:16px"></div></div>' +
    '<h2>Daftar Voucher</h2><div id="vc-list"><p class="muted">Memuat…</p></div>';
  $('vc-create').addEventListener('click', async () => {
    const body = {
      duration_days: parseInt($('vc-days').value, 10),
      quantity: parseInt($('vc-qty').value, 10) || 1,
      assigned_email: $('vc-email').value.trim(),
      valid_days: parseInt($('vc-valid').value, 10) || 0,
    };
    const btn = $('vc-create');
    btn.disabled = true; btn.textContent = 'Membuat…';
    const r = await api('/api/v1/admin/vouchers', { method: 'POST', body });
    btn.disabled = false; btn.textContent = 'Buat Voucher';
    if (r.status !== 200) {
      $('vc-result').innerHTML = '<p class="error">Gagal: ' + esc((r.body && r.body.error) || r.status) + '</p>';
      return;
    }
    const codes = (r.body.vouchers || []).map((v) => v.code);
    let html = '<p><b>' + codes.length + ' kode dibuat</b> — bagikan ke pengguna:</p>';
    for (const c of codes) {
      html += '<div class="toolbar" style="align-items:center"><code style="font-size:1.05rem;letter-spacing:1px">' + esc(c) + '</code>' +
        '<button class="btn ghost small" data-copy="' + esc(c) + '">Salin</button></div>';
    }
    if (codes.length > 1) {
      html += '<button class="btn purple small" id="vc-copyall">Salin Semua Kode</button>';
    }
    $('vc-result').innerHTML = html;
    document.querySelectorAll('button[data-copy]').forEach((el) =>
      el.addEventListener('click', () => copyText(el.dataset.copy, 'Kode disalin.')));
    const all = $('vc-copyall');
    if (all) all.addEventListener('click', () => copyText(codes.join('\n'), 'Semua kode disalin.'));
    loadVoucherList();
  });
  loadVoucherList();
}

async function loadVoucherList() {
  const r = await api('/api/v1/admin/vouchers?limit=100');
  if (r.status !== 200) { $('vc-list').innerHTML = '<p class="error">Gagal memuat voucher.</p>'; return; }
  const list = r.body.vouchers || [];
  if (!list.length) { $('vc-list').innerHTML = '<p class="muted">Belum ada voucher.</p>'; return; }
  let html = '<div class="tscroll"><table><tr><th>Kode</th><th>Durasi</th><th>Sumber</th><th>Untuk</th><th>Status</th><th>Ditukar oleh</th><th>Dibuat</th><th>Aksi</th></tr>';
  for (const v of list) {
    html += '<tr><td><code>' + esc(v.code) + '</code></td>' +
      '<td>' + v.duration_days + ' hari</td><td>' + esc(v.source) + '</td>' +
      '<td>' + esc(v.assigned_to_email || 'siapa saja') + '</td>' +
      '<td><span class="badge ' + (v.status === 'active' ? 'premium' : v.status === 'redeemed' ? 'paid' : 'expired') + '">' + esc(v.status) + '</span>' +
      (v.expires_at && v.status === 'active' ? '<div class="muted small">s.d. ' + esc(fmtDate(v.expires_at)) + '</div>' : '') + '</td>' +
      '<td>' + esc(v.redeemed_by_email || '—') + '</td>' +
      '<td>' + esc(fmtDate(v.created_at)) + '</td>' +
      '<td><div class="row-actions"><button class="btn ghost small" data-vc-copy="' + esc(v.code) + '">Salin</button>' +
      (v.status !== 'redeemed' ? '<button class="btn small" data-vc-del="' + esc(v.id) + '|' + esc(v.code) + '">Hapus</button>' : '') +
      '</div></td></tr>';
  }
  $('vc-list').innerHTML = html + '</table></div>';
  document.querySelectorAll('button[data-vc-copy]').forEach((el) =>
    el.addEventListener('click', () => copyText(el.dataset.vcCopy, 'Kode disalin.')));
  document.querySelectorAll('button[data-vc-del]').forEach((el) =>
    el.addEventListener('click', async () => {
      const [id, code] = el.dataset.vcDel.split('|');
      if (!confirm('Hapus voucher ' + code + '? Kode ini tidak akan bisa ditukar lagi.')) return;
      const rr = await api('/api/v1/admin/vouchers/' + encodeURIComponent(id), { method: 'DELETE' });
      if (rr.status === 200) { toast('Voucher dihapus.'); loadVoucherList(); }
      else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
    }));
}

/* ----- Kelola Judul ----- */
let judulOffset = 0;
let judulFilter = '';
async function pageJudul() {
  $('pageBody').innerHTML =
    '<div class="card" id="jd-worker" style="margin-bottom:16px"><p class="muted">Memuat status worker…</p></div>' +
    '<div id="jd-bd"></div>' +
    '<p class="muted">Judul tersembunyi hilang dari aplikasi (beranda, katalog, pencarian, jadwal, detail, putar) tanpa menghapus datanya.</p>' +
    '<div class="toolbar"><input type="search" id="jd-q" placeholder="Cari judul…" style="flex:1">' +
    '<select id="jd-type"><option value="">Semua jenis</option><option value="anime">Anime</option><option value="donghua">Donghua</option><option value="film">Film</option></select>' +
    '<button class="btn" id="jd-search">Cari</button></div>' +
    '<div class="chiprow" id="jd-filters"></div>' +
    '<div id="jd-list"><p class="muted">Memuat…</p></div>' +
    '<div style="margin-top:14px"><button class="btn ghost" id="jd-more">Muat lebih banyak</button></div>';
  judulOffset = 0;
  judulFilter = '';
  $('jd-search').addEventListener('click', () => { judulOffset = 0; loadJudul(true); });
  // Memilih jenis di dropdown langsung memuat ulang daftar + chip
  // (sebelumnya dropdown diam sampai tombol Cari ditekan — sumber
  // kebingungan di screenshot Rams 2026-11-25: Donghua terpilih tapi
  // angka & daftar masih global).
  $('jd-type').addEventListener('change', () => { judulOffset = 0; loadJudul(true); });
  $('jd-q').addEventListener('keydown', (e) => { if (e.key === 'Enter') { judulOffset = 0; loadJudul(true); } });
  $('jd-more').addEventListener('click', () => loadJudul(false));
  loadWorkerStatus();
  if (window.__pageTimer) clearInterval(window.__pageTimer);
  window.__pageTimer = setInterval(loadWorkerStatus, 5000);
  loadJudul(true);
}
async function loadWorkerStatus() {
  const el = $('jd-worker');
  if (!el) return;
  const r = await api('/api/v1/admin/worker');
  if (r.status !== 200) { el.innerHTML = '<h2 style="margin-top:0">Proses Worker</h2><p class="muted">Status worker tidak tersedia.</p>'; return; }
  const st = r.body.status, q = r.body.queue || {};
  let html = '<h2 style="margin-top:0">Proses Worker</h2>';
  if (st) {
    html += '<p style="margin:4px 0"><b>' + esc(st.phase) + '</b>' +
      (st.current ? ' — sedang memproses: <b>' + esc(st.current) + '</b>' : '') + '</p>';
    if (st.total > 0) {
      const pct = Math.min(100, Math.round(st.done / st.total * 100));
      html += '<div class="bar"><div style="width:' + pct + '%"></div></div>' +
        '<p class="muted small">' + st.done + ' dari ' + st.total + ' judul pada fase ini</p>';
    }
    html += '<p class="muted small">Terakhir diperbarui ' + esc(fmtDate(st.updated_at)) + ' · kartu ini menyegar tiap 5 detik</p>';
  } else {
    html += '<p class="muted">Worker belum melaporkan status (mulai tercatat setelah backend diperbarui dan worker berjalan).</p>';
  }
  html += '<p class="muted" style="margin-bottom:0">Antrean kelengkapan: <b>' + (q.total || 0) + '</b> judul · ' +
    '<b>' + (q.komplit || 0) + '</b> komplit · <b>' + (q.belum || 0) + '</b> belum komplit · ' +
    '<b>' + (q.kosong || 0) + '</b> judul saja (antre dilengkapi worker)</p>';
  el.innerHTML = html;
}
function renderJudulFilters(counts) {
  const el = $('jd-filters');
  if (!el || !counts) return;
  const chip = (val, lbl, n) => '<button class="fchip' + (judulFilter === val ? ' on' : '') +
    '" data-jd-filter="' + val + '">' + lbl + ' (' + n + ')</button>';
  el.innerHTML = chip('', 'Semua', counts.total) + chip('komplit', 'Komplit', counts.komplit) +
    chip('belum', 'Belum Komplit', counts.belum) + chip('kosong', 'Judul Saja', counts.kosong);
  el.querySelectorAll('[data-jd-filter]').forEach((b) => b.addEventListener('click', () => {
    judulFilter = b.dataset.jdFilter;
    judulOffset = 0;
    loadJudul(true);
  }));
}
// Kartu rincian kelengkapan per jenis (Film / Anime / Donghua):
// berapa judul, komplit, belum komplit, dan judul saja untuk tiap
// jenis — dihitung backend dari seluruh katalog, tidak terpengaruh
// pencarian/pilihan jenis/filter yang sedang aktif. Backend lama
// belum mengirim counts_by_type: kartu tidak ditampilkan.
function renderJudulBreakdown(bt) {
  const el = $('jd-bd');
  if (!el) return;
  if (!bt) { el.innerHTML = ''; return; }
  const row = (lbl, c) => '<p class="muted" style="margin:3px 0"><b style="color:var(--text)">' + lbl + '</b> — ' +
    '<b>' + (c.total || 0) + '</b> judul · <b>' + (c.komplit || 0) + '</b> komplit · ' +
    '<b>' + (c.belum || 0) + '</b> belum komplit · <b>' + (c.kosong || 0) + '</b> judul saja</p>';
  el.innerHTML = '<div class="card" style="margin-bottom:16px"><h2 style="margin-top:0">Kelengkapan per Jenis</h2>' +
    row('Film', bt.film || {}) + row('Anime', bt.anime || {}) + row('Donghua', bt.donghua || {}) + '</div>';
}
async function loadJudul(reset) {
  const q = encodeURIComponent($('jd-q').value.trim());
  const t = encodeURIComponent($('jd-type').value);
  const r = await api('/api/v1/admin/titles?q=' + q + '&type=' + t + '&filter=' + encodeURIComponent(judulFilter) + '&limit=50&offset=' + judulOffset);
  if (r.status !== 200) { if (reset) $('jd-list').innerHTML = '<p class="error">Gagal memuat judul.</p>'; return; }
  if (reset) { renderJudulFilters(r.body.counts); renderJudulBreakdown(r.body.counts_by_type); }
  const list = r.body.titles || [];
  judulOffset += list.length;
  let html = reset ? '<p class="muted">' + (r.body.total || 0) + ' judul</p><div class="tscroll fit"><table><tr><th>Poster</th><th>Judul</th><th>Detail</th></tr>' : '';
  for (const x of list) {
    html += '<tr><td>' + (x.poster_url ? '<img src="' + esc(posterSrc(x.poster_url)) + '" alt="" style="width:38px;height:54px;object-fit:cover;border-radius:6px" loading="lazy">' : '—') + '</td>' +
      '<td class="ellipsis">' + esc(x.title) + '<div class="muted small">' + esc(x.slug) + '</div></td>' +
      '<td><a class="btn ghost small" href="#/judul-detail?id=' + esc(x.id) + '">Detail</a></td></tr>';
  }
  if (reset) { $('jd-list').innerHTML = html + '</table></div>'; }
  else { $('jd-list').querySelector('table').insertAdjacentHTML('beforeend', html); }
}
// Halaman detail satu judul: semua informasi (jenis, tahun,
// episode, kelengkapan, genre, sinopsis) tampil di sini; tabel
// Kelola Judul hanya poster + judul + Detail. Visibilitas diatur
// sakelar geser seperti tabel-tabel sebelumnya.
async function pageJudulDetail() {
  const id = new URLSearchParams((location.hash.split('?')[1] || '')).get('id') || '';
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/titles/' + encodeURIComponent(id));
  if (r.status !== 200 || !r.body.title) {
    $('pageBody').innerHTML = '<p class="error">Judul tidak ditemukan.</p>' +
      '<p><a class="btn ghost" href="#/judul">← Kembali ke Kelola Judul</a></p>';
    return;
  }
  const x = r.body.title;
  $('pageBody').innerHTML =
    '<p><a class="btn ghost small" href="#/judul">← Kembali ke Kelola Judul</a></p>' +
    '<div class="card"><div style="display:flex;gap:16px;align-items:flex-start;flex-wrap:wrap">' +
    (x.poster_url ? '<img src="' + esc(posterSrc(x.poster_url)) + '" alt="" style="width:110px;height:156px;object-fit:cover;border-radius:10px">' : '') +
    '<div style="flex:1;min-width:200px"><h2 style="margin:0 0 6px">' + esc(x.title) + '</h2>' +
    '<div class="muted small">' + esc(x.slug) + '</div>' +
    '<p style="margin:10px 0 0">' + (x.complete ? '<span class="badge premium">Metadata komplit</span>' :
      (x.missing || []).map((m) => '<span class="badge expired" style="margin:0 4px 4px 0">' + esc(m) + '</span>').join('')) + '</p></div></div>' +
    '<dl class="kv" style="margin-top:16px">' +
    '<dt>Jenis</dt><dd>' + esc(x.type) + '</dd>' +
    '<dt>Tahun</dt><dd>' + (x.year || '—') + '</dd>' +
    '<dt>Status rilis</dt><dd>' + esc(x.status || '—') + '</dd>' +
    '<dt>Episode</dt><dd>' + x.episode_count + ' episode</dd>' +
    '<dt>Genre</dt><dd>' + esc((x.genres || []).join(', ') || '—') + '</dd>' +
    '<dt>Sinopsis</dt><dd>' + esc(x.synopsis || '—') + '</dd>' +
    '<dt>Tampil di aplikasi</dt><dd><label class="switch"><input type="checkbox" id="jd-vis"' + (x.is_hidden ? '' : ' checked') + '><span class="slider"></span></label> ' +
    '<span class="muted small">' + (x.is_hidden ? 'sedang tersembunyi' : 'tampil') + '</span></dd>' +
    '</dl></div>';
  $('jd-vis').addEventListener('change', async (ev) => {
    const show = ev.target.checked;
    if (!show && !confirm('Sembunyikan judul ini dari aplikasi?')) { ev.target.checked = true; return; }
    const rr = await api('/api/v1/admin/titles/' + encodeURIComponent(id), {
      method: 'PATCH', body: { is_hidden: !show },
    });
    if (rr.status === 200) { toast(show ? 'Judul ditampilkan.' : 'Judul disembunyikan.'); pageJudulDetail(); }
    else { toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status)); ev.target.checked = !show; }
  });
}

/* ----- Laporan User ----- */
let laporanStatus = '';
const LAPORAN_ALASAN = {
  semua_server_gagal: 'Semua server gagal', link_rusak: 'Link rusak',
  video_salah: 'Video salah', subtitle: 'Subtitle bermasalah', lainnya: 'Lainnya',
};
const LAPORAN_STATUS = { baru: 'Baru', diproses: 'Diproses', selesai: 'Selesai', diabaikan: 'Diabaikan' };
async function pageLaporan() {
  $('pageBody').innerHTML =
    '<p class="muted">Laporan masalah dari pengguna (tombol Laporkan di pemutar). Tindak langsung dari sini: perbaiki link episode, atur tampil/sembunyi episode dan judulnya lewat sakelar geser, tandai selesai, atau abaikan. Tabel bisa digeser ke samping.</p>' +
    '<div class="chiprow" id="lp-filters"></div>' +
    '<div id="lp-list"><p class="muted">Memuat…</p></div>';
  loadLaporan();
}
async function loadLaporan() {
  const r = await api('/api/v1/admin/reports?status=' + encodeURIComponent(laporanStatus) + '&limit=50');
  if (r.status !== 200) { $('lp-list').innerHTML = '<p class="error">Gagal memuat laporan.</p>'; return; }
  const c = r.body.counts || {};
  const chip = (val, lbl, n) => '<button class="fchip' + (laporanStatus === val ? ' on' : '') +
    '" data-lp-filter="' + val + '">' + lbl + ' (' + n + ')</button>';
  $('lp-filters').innerHTML = chip('', 'Semua', c.total || 0) + chip('baru', 'Baru', c.baru || 0) +
    chip('diproses', 'Diproses', c.diproses || 0) + chip('selesai', 'Selesai', c.selesai || 0) +
    chip('diabaikan', 'Diabaikan', c.diabaikan || 0);
  $('lp-filters').querySelectorAll('[data-lp-filter]').forEach((b) => b.addEventListener('click', () => {
    laporanStatus = b.dataset.lpFilter;
    loadLaporan();
  }));
  const list = r.body.reports || [];
  if (!list.length) { $('lp-list').innerHTML = '<p class="muted">Tidak ada laporan.</p>'; return; }
  let html = '<div class="tscroll"><table class="ltable"><tr><th>Waktu</th><th>Judul / Episode</th><th>Alasan</th><th>Pelapor</th><th>Status</th><th>Aksi</th></tr>';
  for (const x of list) {
    html += '<tr><td class="small">' + esc(fmtDate(x.created_at)) + '</td>' +
      '<td class="ellipsis">' + esc(x.title) + (x.episode_no ? ' — Ep. ' + x.episode_no : '') + '</td>' +
      '<td>' + esc(LAPORAN_ALASAN[x.reason] || x.reason) + '</td>' +
      '<td class="small">' + esc(x.reporter) + '</td>' +
      '<td><span class="badge ' + (x.status === 'baru' ? 'premium' : 'expired') + '">' + esc(LAPORAN_STATUS[x.status] || x.status) + '</span></td>' +
      '<td><div class="lp-actions">' +
      (x.episode_id ? '<button class="btn small" data-lp-act="fix" data-lp-id="' + esc(x.id) + '">Perbaiki Link</button>' : '') +
      (x.episode_id ? '<span class="lp-vis">Episode <label class="switch"><input type="checkbox" data-lp-vis="episode" data-lp-id="' + esc(x.id) + '"' + (x.episode_hidden ? '' : ' checked') + '><span class="slider"></span></label></span>' : '') +
      (x.title_id ? '<span class="lp-vis">Judul <label class="switch"><input type="checkbox" data-lp-vis="title" data-lp-id="' + esc(x.id) + '"' + (x.title_hidden ? '' : ' checked') + '><span class="slider"></span></label></span>' : '') +
      (x.status !== 'selesai' ? '<button class="btn ghost small" data-lp-act="done" data-lp-id="' + esc(x.id) + '">Selesai</button>' : '') +
      (x.status !== 'diabaikan' ? '<button class="btn ghost small" data-lp-act="ignore" data-lp-id="' + esc(x.id) + '">Abaikan</button>' : '') +
      '<button class="btn ghost small" data-lp-act="del" data-lp-id="' + esc(x.id) + '">Hapus</button>' +
      '<a class="btn ghost small" href="#/laporan-detail?id=' + esc(x.id) + '">Detail</a>' +
      '</div></td></tr>';
  }
  $('lp-list').innerHTML = html + '</table></div>';
  pasangAksiLaporan();
}
// Pasang pendengar tombol/sakelar laporan — dipakai daftar &
// halaman detail.
function pasangAksiLaporan() {
  document.querySelectorAll('#pageBody [data-lp-act]').forEach((b) => b.addEventListener('click', () => laporanAksi(b.dataset.lpAct, b.dataset.lpId)));
  document.querySelectorAll('#pageBody [data-lp-vis]').forEach((el) => el.addEventListener('change', () => laporanVis(el)));
}
function refreshLaporan() {
  if ($('lp-detail')) pageLaporanDetail();
  else if ($('lp-list')) loadLaporan();
}
// Halaman detail satu laporan: judul & catatan UTUH tampil di
// sini (di tabel hanya ringkasannya), lengkap dengan sakelar
// tampil/sembunyi dan semua tombol tindakannya.
async function pageLaporanDetail() {
  const id = new URLSearchParams((location.hash.split('?')[1] || '')).get('id') || '';
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/reports/' + encodeURIComponent(id));
  if (r.status !== 200 || !r.body.report) {
    $('pageBody').innerHTML = '<p class="error">Laporan tidak ditemukan.</p>' +
      '<p><a class="btn ghost" href="#/laporan">← Kembali ke Laporan User</a></p>';
    return;
  }
  const x = r.body.report;
  $('pageBody').innerHTML =
    '<p><a class="btn ghost small" href="#/laporan">← Kembali ke Laporan User</a></p>' +
    '<div class="card" id="lp-detail"><h2 style="margin-top:0">' + esc(x.title) + '</h2>' +
    '<dl class="kv">' +
    '<dt>Episode</dt><dd>' + (x.episode_no ? 'Episode ' + x.episode_no : '—') + '</dd>' +
    '<dt>Alasan</dt><dd>' + esc(LAPORAN_ALASAN[x.reason] || x.reason) + '</dd>' +
    '<dt>Catatan user</dt><dd>' + esc(x.note || '—') + '</dd>' +
    '<dt>Pelapor</dt><dd>' + esc(x.reporter) + '</dd>' +
    '<dt>Status</dt><dd><span class="badge ' + (x.status === 'baru' ? 'premium' : 'expired') + '">' + esc(LAPORAN_STATUS[x.status] || x.status) + '</span></dd>' +
    '<dt>Waktu lapor</dt><dd>' + esc(fmtDate(x.created_at)) + '</dd>' +
    (x.title_id ? '<dt>Judul tampil</dt><dd><label class="switch"><input type="checkbox" data-lp-vis="title" data-lp-id="' + esc(x.id) + '"' + (x.title_hidden ? '' : ' checked') + '><span class="slider"></span></label> <span class="muted small">' + (x.title_hidden ? 'sedang tersembunyi' : 'tampil di aplikasi') + '</span></dd>' : '') +
    (x.episode_id ? '<dt>Episode tampil</dt><dd><label class="switch"><input type="checkbox" data-lp-vis="episode" data-lp-id="' + esc(x.id) + '"' + (x.episode_hidden ? '' : ' checked') + '><span class="slider"></span></label> <span class="muted small">' + (x.episode_hidden ? 'sedang tersembunyi' : 'tampil di aplikasi') + '</span></dd>' : '') +
    '</dl>' +
    '<div class="lp-actions" style="justify-content:flex-start;margin-top:18px">' +
    (x.episode_id ? '<button class="btn small" data-lp-act="fix" data-lp-id="' + esc(x.id) + '">Perbaiki Link</button>' : '') +
    (x.status !== 'selesai' ? '<button class="btn ghost small" data-lp-act="done" data-lp-id="' + esc(x.id) + '">Selesai</button>' : '') +
    (x.status !== 'diabaikan' ? '<button class="btn ghost small" data-lp-act="ignore" data-lp-id="' + esc(x.id) + '">Abaikan</button>' : '') +
    '<button class="btn ghost small" data-lp-act="del" data-lp-id="' + esc(x.id) + '">Hapus</button>' +
    '</div></div>';
  pasangAksiLaporan();
}
// Sakelar geser tampil/sembunyi episode & judul (seperti sakelar
// metode pembayaran Tokopay): ON = tampil di aplikasi, OFF =
// tersembunyi. Menyembunyikan tetap dikonfirmasi dulu.
async function laporanVis(el) {
  const id = el.dataset.lpId, kind = el.dataset.lpVis, show = el.checked;
  if (!show && !confirm(kind === 'episode' ? 'Sembunyikan episode ini dari aplikasi?' : 'Sembunyikan SELURUH judul ini dari aplikasi?')) {
    el.checked = true;
    return;
  }
  const path = kind === 'episode'
    ? (show ? '/show-episode' : '/hide-episode')
    : (show ? '/show-title' : '/hide-title');
  const rr = await api('/api/v1/admin/reports/' + encodeURIComponent(id) + path, { method: 'POST' });
  if (rr.status === 200) toast(show ? 'Ditampilkan lagi di aplikasi.' : 'Disembunyikan dari aplikasi.');
  else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  refreshLaporan();
}
async function laporanAksi(act, id) {
  const base = '/api/v1/admin/reports/' + encodeURIComponent(id);
  let rr = null;
  if (act === 'fix') {
    rr = await api(base + '/fix-links', { method: 'POST' });
    if (rr.status === 200) toast('Sumber lama dibuang; episode sedang di-resolve ulang dari situs sumber.');
  } else if (act === 'done') {
    rr = await api(base, { method: 'PATCH', body: { status: 'selesai' } });
    if (rr.status === 200) toast('Laporan ditandai selesai.');
  } else if (act === 'ignore') {
    rr = await api(base, { method: 'PATCH', body: { status: 'diabaikan' } });
    if (rr.status === 200) toast('Laporan diabaikan.');
  } else if (act === 'del') {
    if (!confirm('Hapus laporan ini permanen?')) return;
    rr = await api(base, { method: 'DELETE' });
    if (rr.status === 200) toast('Laporan dihapus.');
  }
  if (rr && rr.status !== 200) toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  if (act === 'del' && rr && rr.status === 200 && $('lp-detail')) { location.hash = '#/laporan'; return; }
  refreshLaporan();
}

/* ----- Laporan Crash ----- */
let crashFilter = '';
async function pageCrash() {
  $('pageBody').innerHTML =
    '<div class="toolbar"><p class="muted" style="flex:1">Laporan crash dari aplikasi (v0.22.0+). Ketuk "Stack trace" untuk detail teknisnya, lalu tandai sudah diperbaiki setelah ditangani.</p>' +
    '<button class="btn" id="cr-clear">Kosongkan Semua</button></div>' +
    '<div class="chiprow" id="cr-filters"></div><div id="cr-list"><p class="muted">Memuat…</p></div>';
  $('cr-clear').addEventListener('click', async () => {
    if (!confirm('Hapus SEMUA laporan crash?')) return;
    const rr = await api('/api/v1/admin/crashes', { method: 'DELETE' });
    if (rr.status === 200) { toast('Dihapus: ' + rr.body.deleted + ' laporan.'); pageCrash(); }
    else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  });
  loadCrashes();
}
async function loadCrashes() {
  const r = await api('/api/v1/admin/crashes?limit=100&fixed=' + encodeURIComponent(crashFilter));
  if (r.status !== 200) { $('cr-list').innerHTML = '<p class="error">Gagal memuat laporan.</p>'; return; }
  const c = r.body.counts || {};
  const chip = (val, lbl, n) => '<button class="fchip' + (crashFilter === val ? ' on' : '') +
    '" data-cr-filter="' + val + '">' + lbl + ' (' + n + ')</button>';
  $('cr-filters').innerHTML = chip('', 'Semua', c.total || 0) + chip('belum', 'Belum Diperbaiki', c.belum || 0) +
    chip('sudah', 'Sudah Diperbaiki', c.sudah || 0);
  $('cr-filters').querySelectorAll('[data-cr-filter]').forEach((b) => b.addEventListener('click', () => {
    crashFilter = b.dataset.crFilter;
    loadCrashes();
  }));
  const list = r.body.crashes || [];
  if (!list.length) { $('cr-list').innerHTML = '<p class="muted">Tidak ada laporan crash pada filter ini.</p>'; return; }
  let html = '';
  for (const c of list) {
    const nama = c.user_name || c.user_email || 'Pengguna anonim';
    html += '<div class="card" style="margin-bottom:12px">' +
      '<div class="toolbar" style="align-items:flex-start"><div style="flex:1"><b>' + esc(nama) + '</b> ' +
      '<span class="badge ' + (c.is_fixed ? 'premium' : 'expired') + '">' + (c.is_fixed ? 'Sudah diperbaiki' : 'Belum diperbaiki') + '</span>' +
      '<div class="muted small">' + (c.user_name && c.user_email ? esc(c.user_email) + ' • ' : '') +
      esc(fmtDate(c.created_at)) + ' • v' + esc(c.app_version) + ' (' + c.version_code + ') • ' +
      esc(c.device) + ' • ' + esc(c.android_version) +
      (c.is_fixed && c.fixed_at ? ' • ditandai selesai ' + esc(fmtDate(c.fixed_at)) : '') + '</div></div>' +
      '<button class="btn ghost small" data-cr-toggle="' + esc(c.id) + '">Stack trace</button>' +
      '<button class="btn ' + (c.is_fixed ? 'ghost' : '') + ' small" data-cr-fix="' + esc(c.id) + '|' + (c.is_fixed ? '0' : '1') + '">' +
      (c.is_fixed ? 'Tandai Belum' : 'Tandai Sudah Diperbaiki') + '</button>' +
      '<button class="btn small" data-cr-del="' + esc(c.id) + '">Hapus</button></div>' +
      '<div id="cr-stackwrap-' + esc(c.id) + '" hidden style="position:relative">' +
      '<button class="btn ghost small" data-cr-copy="' + esc(c.id) + '" style="position:absolute;top:8px;right:8px;z-index:1">Salin</button>' +
      '<pre id="cr-stack-' + esc(c.id) + '" style="white-space:pre-wrap;word-break:break-word;font-size:0.78rem;max-height:320px;overflow:auto;background:var(--panel-2);padding:12px;border-radius:9px;margin:0">' + esc(c.stack_trace) + '</pre></div></div>';
  }
  $('cr-list').innerHTML = html;
  document.querySelectorAll('button[data-cr-toggle]').forEach((el) =>
    el.addEventListener('click', () => { const p = $('cr-stackwrap-' + el.dataset.crToggle); p.hidden = !p.hidden; }));
  document.querySelectorAll('button[data-cr-copy]').forEach((el) =>
    el.addEventListener('click', async () => {
      const teks = ($('cr-stack-' + el.dataset.crCopy) || {}).textContent || '';
      try {
        await navigator.clipboard.writeText(teks);
        toast('Log error disalin.');
      } catch (e) {
        const ta = document.createElement('textarea');
        ta.value = teks;
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); toast('Log error disalin.'); }
        catch (e2) { toast('Gagal menyalin otomatis — buka stack trace lalu salin manual.'); }
        ta.remove();
      }
    }));
  document.querySelectorAll('button[data-cr-fix]').forEach((el) =>
    el.addEventListener('click', async () => {
      const [id, to] = el.dataset.crFix.split('|');
      const rr = await api('/api/v1/admin/crashes/' + encodeURIComponent(id), {
        method: 'PATCH', body: { is_fixed: to === '1' },
      });
      if (rr.status === 200) { toast(to === '1' ? 'Ditandai sudah diperbaiki.' : 'Ditandai belum diperbaiki.'); loadCrashes(); }
      else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
    }));
  document.querySelectorAll('button[data-cr-del]').forEach((el) =>
    el.addEventListener('click', async () => {
      const rr = await api('/api/v1/admin/crashes/' + encodeURIComponent(el.dataset.crDel), { method: 'DELETE' });
      if (rr.status === 200) { toast('Laporan dihapus.'); pageCrash(); }
      else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
    }));
}

/* ----- Pengaturan Aplikasi ----- */
async function pagePengaturan() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/settings');
  if (r.status !== 200) return bodyError(r);
  const s = r.body.settings || {};
  $('pageBody').innerHTML =
    '<div class="card"><h2 style="margin-top:0">Mode Perawatan</h2>' +
    '<p class="muted">Geser tombol untuk langsung mengaktifkan/mematikan — tanpa perlu menekan Simpan. Saat aktif, aplikasi pengguna (v0.22.2+) otomatis menampilkan halaman perawatan dalam waktu maks 60 detik (aplikasi memeriksa pengaturan tiap menit), dan halaman itu lepas sendiri begitu mode dimatikan. Versi lama menerima perubahan saat aplikasi dibuka lagi.</p>' +
    '<div class="toolbar"><label class="switch"><input type="checkbox" id="st-maint"' + (s.maintenance_mode === 'true' ? ' checked' : '') + '><span class="slider"></span></label><span id="st-maint-lbl">' + (s.maintenance_mode === 'true' ? 'Mode perawatan AKTIF' : 'Mode perawatan nonaktif') + '</span></div>' +
    '<label>Pesan perawatan<br><input type="text" id="st-maint-msg" value="' + esc(s.maintenance_message || '') + '" style="width:100%"></label></div>' +
    '<div class="card" style="margin-top:18px"><h2 style="margin-top:0">Paksa Perbarui</h2>' +
    '<p class="muted">Tulis versi seperti biasa (contoh: 0.22.2). Pengguna dengan versi aplikasi di bawah itu wajib memperbarui. Kosongkan untuk menonaktifkan.</p>' +
    '<div class="toolbar"><label>Versi minimum<br><input type="text" id="st-minver" value="' + esc(s.min_version_name || '') + '" placeholder="cth: 0.22.2" inputmode="decimal" style="width:160px"></label>' +
    '<label style="flex:1">Tautan unduh versi baru<br><input type="text" id="st-url" value="' + esc(s.update_url || '') + '" style="width:100%"></label></div></div>' +
    '<div style="margin-top:18px"><button class="btn" id="st-save">Simpan Pengaturan</button></div>';
  // Toggle mode perawatan: simpan LANGSUNG saat digeser (seperti
  // halaman Metode Pembayaran) + kabari admin efeknya ke aplikasi.
  $('st-maint').addEventListener('change', async (e) => {
    const on = e.target.checked;
    $('st-maint-lbl').textContent = on ? 'Mengaktifkan…' : 'Mematikan…';
    const rr = await api('/api/v1/admin/settings', {
      method: 'PATCH',
      body: { maintenance_mode: on, maintenance_message: $('st-maint-msg').value },
    });
    if (rr.status === 200) {
      $('st-maint-lbl').textContent = on ? 'Mode perawatan AKTIF' : 'Mode perawatan nonaktif';
      toast(on
        ? '🔧 Mode perawatan AKTIF — aplikasi pengguna menampilkan halaman perawatan dalam maks 60 detik.'
        : 'Mode perawatan dimatikan — aplikasi kembali normal dalam maks 60 detik.');
    } else {
      e.target.checked = !on;
      $('st-maint-lbl').textContent = !on ? 'Mode perawatan AKTIF' : 'Mode perawatan nonaktif';
      toast('Gagal mengubah mode perawatan: ' + ((rr.body && rr.body.error) || rr.status));
    }
  });
  $('st-save').addEventListener('click', async () => {
    const body = {
      maintenance_mode: $('st-maint').checked,
      maintenance_message: $('st-maint-msg').value,
      min_version_name: $('st-minver').value.trim(),
      min_version_code: 0,
      update_url: $('st-url').value.trim(),
    };
    const rr = await api('/api/v1/admin/settings', { method: 'PATCH', body });
    if (rr.status === 200) toast('Pengaturan disimpan.');
    else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  });
}

/* ----- Pengumuman Banner (halaman khusus, terpisah dari
   Notifikasi — permintaan Rams: dua hal, jangan digabung) ----- */
async function pagePengumuman() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/settings');
  if (r.status !== 200) return bodyError(r);
  const s = r.body.settings || {};
  $('pageBody').innerHTML =
    '<div class="card"><h2 style="margin-top:0"><i class="fa-solid fa-bullhorn"></i> Pengumuman Banner</h2>' +
    '<p class="muted">Satu-satunya fungsi halaman ini: mengatur teks <b>banner</b> yang tampil di Beranda aplikasi. Ini BUKAN notifikasi — untuk mengirim notifikasi ke halaman Notifikasi pengguna, pakai halaman <b>Notifikasi</b>. Banner tampil di aplikasi (v0.22.2+) dalam maks 60 detik setelah disimpan; kosongkan teks untuk menyembunyikan banner.</p>' +
    '<label>Teks banner<br><input type="text" id="pg-ann" value="' + esc(s.announcement || '') + '" placeholder="cth: Server pindah — selamat menonton!" style="width:100%"></label>' +
    '<p class="small muted" style="margin-top:10px">Status saat ini: ' + (s.announcement ? '<b style="color:var(--green)">banner AKTIF</b>' : '<b>banner kosong (tidak tampil)</b>') + '</p>' +
    '<div style="margin-top:12px"><button class="btn" id="pg-save"><i class="fa-solid fa-floppy-disk"></i> Simpan Banner</button></div></div>';
  $('pg-save').addEventListener('click', async () => {
    const rr = await api('/api/v1/admin/settings', {
      method: 'PATCH', body: { announcement: $('pg-ann').value },
    });
    if (rr.status === 200) { toast('Banner disimpan.'); pagePengumuman(); }
    else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  });
}

/* ----- Notifikasi (siaran ke aplikasi) ----- */
async function pageNotifikasi() {
  $('pageBody').innerHTML =
    '<div class="card"><h2 style="margin-top:0"><i class="fa-solid fa-bell"></i> Kirim Notifikasi</h2>' +
    '<p class="muted">Notifikasi terkirim ke <b>halaman Notifikasi</b> di aplikasi semua pengguna (lencana belum dibaca + notifikasi sistem Android muncul di aplikasi v0.22.4+ yang sedang berjalan, maks 60 detik). Beda dengan Pengumuman yang hanya berupa banner.</p>' +
    '<label>Judul notifikasi<br><input type="text" id="nt-title" maxlength="140" placeholder="cth: Episode baru Wukong sudah rilis!" style="width:100%"></label>' +
    '<label style="display:block;margin-top:10px">Isi<br><input type="text" id="nt-body" maxlength="1000" placeholder="cth: Tonton sekarang sebelum ketinggalan." style="width:100%"></label>' +
    '<div class="toolbar" style="margin-top:12px"><label>Penerima<br><select id="nt-aud"><option value="all">Semua pengguna</option><option value="premium">Premium saja</option><option value="free">Gratis saja</option></select></label>' +
    '<button class="btn purple" id="nt-send" style="align-self:flex-end"><i class="fa-solid fa-paper-plane"></i> Kirim Notifikasi</button></div></div>' +
    '<h2 class="group-title">Riwayat Siaran</h2><div id="nt-list"><p class="muted">Memuat…</p></div>';
  $('nt-send').addEventListener('click', async () => {
    const title = $('nt-title').value.trim();
    if (!title) { toast('Judul notifikasi wajib diisi.'); return; }
    const rr = await api('/api/v1/admin/notifications', {
      method: 'POST',
      body: { title, body: $('nt-body').value.trim(), audience: $('nt-aud').value },
    });
    if (rr.status === 200) {
      toast('📣 Notifikasi terkirim ke ' + rr.body.recipients + ' pengguna.');
      $('nt-title').value = '';
      $('nt-body').value = '';
      loadBroadcasts();
    } else {
      toast('Gagal mengirim: ' + ((rr.body && rr.body.error) || rr.status));
    }
  });
  loadBroadcasts();
}

async function loadBroadcasts() {
  const r = await api('/api/v1/admin/notifications');
  if (r.status !== 200) { $('nt-list').innerHTML = '<p class="error">Gagal memuat riwayat.</p>'; return; }
  const list = r.body.broadcasts || [];
  if (!list.length) { $('nt-list').innerHTML = '<p class="muted">Belum ada notifikasi terkirim.</p>'; return; }
  const audLbl = { all: 'Semua', premium: 'Premium', free: 'Gratis' };
  let html = '<div class="tscroll"><table><tr><th>Waktu</th><th>Judul</th><th>Penerima</th><th>Terkirim</th><th>Dibaca</th><th></th></tr>';
  for (const b of list) {
    html += '<tr><td>' + esc(fmtDate(b.created_at)) + '</td>' +
      '<td><b>' + esc(b.title) + '</b>' + (b.body ? '<br><span class="small muted">' + esc(b.body) + '</span>' : '') + '</td>' +
      '<td>' + esc(audLbl[b.audience] || b.audience) + '</td>' +
      '<td>' + b.sent_count + '</td><td>' + b.read_count + '</td>' +
      '<td><button class="btn ghost small" data-del="' + esc(b.id) + '"><i class="fa-solid fa-trash"></i> Hapus</button></td></tr>';
  }
  $('nt-list').innerHTML = html + '</table></div>';
  document.querySelectorAll('[data-del]').forEach((btn) =>
    btn.addEventListener('click', async () => {
      if (!confirm('Hapus siaran ini? Salinannya di aplikasi pengguna ikut terhapus.')) return;
      const rr = await api('/api/v1/admin/notifications/' + btn.dataset.del, { method: 'DELETE' });
      if (rr.status === 200) { toast('Siaran dihapus.'); loadBroadcasts(); }
      else toast('Gagal menghapus.');
    }));
}

/* ----- Log Admin ----- */
async function pageLog() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/logs?limit=100');
  if (r.status !== 200) return bodyError(r);
  const list = r.body.logs || [];
  if (!list.length) { $('pageBody').innerHTML = '<p class="muted">Belum ada aktivitas admin tercatat.</p>'; return; }
  let html = '<div class="tscroll"><table><tr><th>Waktu</th><th>Admin</th><th>Aksi</th><th>Target</th><th>Rincian</th><th></th></tr>';
  for (const l of list) {
    let detail = '';
    try { detail = JSON.stringify(l.detail); } catch (e) { detail = ''; }
    if (detail.length > 90) detail = detail.slice(0, 90) + '…';
    html += '<tr><td>' + esc(fmtDate(l.created_at)) + '</td><td>' + esc(l.admin_email) + '</td>' +
      '<td><span class="badge info">' + esc(l.action) + '</span></td><td class="small">' + esc(l.target || '—') + '</td>' +
      '<td class="small muted">' + esc(detail) + '</td>' +
      '<td><a class="btn ghost small" href="#/log-detail?id=' + esc(l.id) + '">Detail</a></td></tr>';
  }
  $('pageBody').innerHTML = html + '</table></div>';
}
// Halaman detail satu catatan log: rincian utuh tampil di sini,
// bukan dipotong di tabel.
async function pageLogDetail() {
  const id = new URLSearchParams((location.hash.split('?')[1] || '')).get('id') || '';
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/logs?limit=100');
  if (r.status !== 200) return bodyError(r);
  const l = (r.body.logs || []).find((x) => x.id === id);
  if (!l) {
    $('pageBody').innerHTML = '<p class="error">Catatan log tidak ditemukan (mungkin sudah lewat 100 catatan terakhir).</p>' +
      '<p><a class="btn ghost" href="#/log">← Kembali ke Log Admin</a></p>';
    return;
  }
  let detail = '';
  try { detail = JSON.stringify(l.detail, null, 2); } catch (e) { detail = String(l.detail || ''); }
  $('pageBody').innerHTML =
    '<p><a class="btn ghost small" href="#/log">← Kembali ke Log Admin</a></p>' +
    '<div class="card"><dl class="kv">' +
    '<dt>Waktu</dt><dd>' + esc(fmtDate(l.created_at)) + '</dd>' +
    '<dt>Admin</dt><dd>' + esc(l.admin_email) + '</dd>' +
    '<dt>Aksi</dt><dd><span class="badge info">' + esc(l.action) + '</span></dd>' +
    '<dt>Target</dt><dd>' + esc(l.target || '—') + '</dd>' +
    '</dl><h3 style="margin-bottom:8px">Rincian</h3>' +
    '<pre style="white-space:pre-wrap;word-break:break-word;font-size:0.82rem;background:var(--panel-2);padding:12px;border-radius:9px;max-height:420px;overflow:auto;margin:0">' + esc(detail) + '</pre></div>';
}

/* ----- Info Sistem ----- */
async function pageSistem(keep) {
  if (!keep) $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/system');
  if (r.status !== 200) return bodyError(r);
  const b = r.body, c = b.counts || {}, cfg = b.config || {}, sv = b.server || {};
  const flag = (v) => v ? '<span class="badge premium">terhubung</span>' : '<span class="badge expired">belum diatur</span>';
  const loadTxt = (sv.load || []).map((x) => Number(x).toFixed(2)).join(' · ');
  const hist = b.history || [];
  const hCpu = hist.map((p) => p.cpu || 0), hMem = hist.map((p) => p.mem || 0);
  const hLoad = hist.map((p) => p.load1 || 0);
  const hRx = hist.map((p) => (p.rx_rate || 0) * 8 / 1e6), hTx = hist.map((p) => (p.tx_rate || 0) * 8 / 1e6);
  const hDR = hist.map((p) => (p.disk_read_rate || 0) / 1048576), hDW = hist.map((p) => (p.disk_write_rate || 0) / 1048576);
  const peak = (arr, floor) => Math.max.apply(null, [floor].concat(arr)) * 1.25;
  const rxMbps = (sv.net_rx_rate || 0) * 8 / 1e6, txMbps = (sv.net_tx_rate || 0) * 8 / 1e6;
  const drMBs = (sv.disk_read_rate || 0) / 1048576, dwMBs = (sv.disk_write_rate || 0) / 1048576;
  let html = '<h2 style="margin-top:4px">Sumber Daya Server</h2><div class="gauges">' +
    gaugeCard('CPU', (Number(sv.cpu_percent || 0)).toFixed(1), '%', sv.cpu_percent, '#4da3ff') +
    gaugeCard('RAM Terpakai', (Number(sv.mem_percent || 0)).toFixed(1), '%', sv.mem_percent, '#e8b93e') +
    gaugeCard('Jaringan Masuk', rxMbps.toFixed(2), 'Mbps', rxMbps / peak(hRx, 8) * 100, '#4da3ff') +
    gaugeCard('Jaringan Keluar', txMbps.toFixed(2), 'Mbps', txMbps / peak(hTx, 8) * 100, '#ff5a3c') +
    gaugeCard('Disk Baca', drMBs.toFixed(2), 'MB/dtk', drMBs / peak(hDR, 1) * 100, '#4da3ff') +
    gaugeCard('Disk Tulis', dwMBs.toFixed(2), 'MB/dtk', dwMBs / peak(hDW, 1) * 100, '#ff5a3c') +
    '</div>';
  html += '<div class="cards">' +
    sparkCard('CPU %', (Number(sv.cpu_percent || 0)).toFixed(1) + '%', [{ data: hCpu, color: '#4da3ff' }]) +
    sparkCard('RAM %', (Number(sv.mem_percent || 0)).toFixed(1) + '%', [{ data: hMem, color: '#e8b93e' }]) +
    sparkCard('Load (1 mnt) · ' + esc(sv.num_cpu || 0) + ' core', loadTxt || '—',
      [{ data: hLoad, color: '#46d369' }]) +
    sparkCard('Jaringan Mbps (masuk / keluar)', rxMbps.toFixed(2) + ' / ' + txMbps.toFixed(2),
      [{ data: hRx, color: '#4da3ff' }, { data: hTx, color: '#ff5a3c' }]) +
    sparkCard('Disk MB/dtk (baca / tulis)', drMBs.toFixed(2) + ' / ' + dwMBs.toFixed(2),
      [{ data: hDR, color: '#4da3ff' }, { data: hDW, color: '#ff5a3c' }]) +
    sparkCard('Disk Terpakai', fmtBytes(sv.disk_used_bytes) + ' / ' + fmtBytes(sv.disk_total_bytes), []) +
    '</div>';
  html += '<div class="card" style="margin-bottom:18px"><table>' +
    '<tr><th>Hostname</th><td>' + esc(sv.hostname || '—') + ' (' + esc(sv.os || '—') + ')</td></tr>' +
    '<tr><th>Uptime server</th><td>' + esc(fmtUptime(sv.uptime_seconds)) + '</td></tr>' +
    '<tr><th>RAM</th><td>' + fmtBytes(sv.mem_used_bytes) + ' terpakai dari ' + fmtBytes(sv.mem_total_bytes) +
      ' · sisa ' + fmtBytes((sv.mem_total_bytes || 0) - (sv.mem_used_bytes || 0)) + '</td></tr>' +
    '<tr><th>Bandwidth total</th><td>↓ ' + fmtBytes(sv.net_rx_bytes) + ' · ↑ ' + fmtBytes(sv.net_tx_bytes) + ' (sejak server menyala)</td></tr>' +
    '<tr><th>Memori proses API</th><td>' + fmtBytes(sv.process_mem_bytes) + ' · ' + esc(sv.goroutines || 0) + ' goroutine</td></tr>' +
    '<tr><th>Ukuran database</th><td>' + fmtBytes(b.db_size_bytes) + '</td></tr></table>' +
    '<p class="muted" style="margin-bottom:0">Halaman ini menyegarkan otomatis tiap 10 detik. Grafik = riwayat 30 menit terakhir (sampel tiap 5 detik, tersimpan di memori server — mulai dari nol tiap server restart). Total bandwidth dihitung OS sejak server menyala; kuota bulanan hanya terlihat di panel penyedia VPS.</p></div>';
  html += '<div class="cards">' +
    card('Pengguna', c.users) + card('Admin', c.admins) + card('Premium aktif', c.premium_users) +
    card('Judul', c.titles) + card('Judul tersembunyi', c.titles_hidden) + card('Episode', c.episodes) +
    card('Transaksi', c.transactions) + card('Voucher aktif', c.vouchers_active) +
    card('Laporan crash', c.crash_reports, c.crash_unfixed ? c.crash_unfixed + ' belum diperbaiki' : '') + card('Postingan feed', c.feed_posts) + '</div>';
  html += '<div class="card"><h2 style="margin-top:0">Server</h2><table>' +
    '<tr><th>Go</th><td>' + esc(b.go_version) + '</td></tr>' +
    '<tr><th>Migrasi terakhir</th><td>' + esc(b.latest_migration || '—') + '</td></tr>' +
    '<tr><th>Waktu server</th><td>' + esc(fmtDate(b.server_time)) + '</td></tr></table></div>';
  html += '<div class="card" style="margin-top:18px"><h2 style="margin-top:0">Konfigurasi</h2><table>' +
    '<tr><th>Login Google</th><td>' + flag(cfg.google_login) + '</td></tr>' +
    '<tr><th>Tokopay</th><td>' + flag(cfg.tokopay) + '</td></tr>' +
    '<tr><th>Discord (bot komunitas)</th><td>' + flag(cfg.discord) + '</td></tr>' +
    '<tr><th>Telegram (bot grup)</th><td>' + flag(cfg.telegram) + '</td></tr>' +
    '<tr><th>ImgBB (gambar feed)</th><td>' + flag(cfg.imgbb) + '</td></tr></table>' +
    '<p class="muted">Konfigurasi diatur lewat berkas env di VPS (<code>/etc/subsenpai/backend.env</code>), bukan dari halaman ini.</p></div>';
  $('pageBody').innerHTML = html;
  if (window.__pageTimer) clearInterval(window.__pageTimer);
  window.__pageTimer = setInterval(() => {
    if ((location.hash || '').indexOf('sistem') >= 0) pageSistem(true);
  }, 10000);
}

/* ----- Sidebar geser (drawer) ----- */
function openSidebar() {
  $('sidebar').classList.add('open');
  $('sideBackdrop').classList.add('show');
}
function closeSidebar() {
  $('sidebar').classList.remove('open');
  $('sideBackdrop').classList.remove('show');
}
function wireDrawer() {
  $('burger').addEventListener('click', openSidebar);
  $('sideClose').addEventListener('click', closeSidebar);
  $('sideBackdrop').addEventListener('click', closeSidebar);
  document.querySelectorAll('#mainNav a').forEach((a) =>
    a.addEventListener('click', closeSidebar));
  // Kepala submenu: klik melipat/membuka grupnya.
  document.querySelectorAll('#mainNav .nav-group-head').forEach((h) =>
    h.addEventListener('click', () => h.parentElement.classList.toggle('open')));
}

/* ================= Boot ================= */
wireDrawer();
$('logoutBtn').addEventListener('click', () => {
  if (window.google && google.accounts && google.accounts.id) google.accounts.id.disableAutoSelect();
  showLogin();
  initGoogleButton();
});
window.addEventListener('hashchange', () => { if (jwt) route(); });

(async function boot() {
  if (jwt) {
    try {
      const r = await api('/api/v1/me');
      const prof = (r.body && (r.body.profile || r.body)) || {};
      if (r.status === 200 && prof.role === 'admin') { me = prof; enterApp(); return; }
    } catch (e) { /* jatuh ke login */ }
    showLogin();
  } else {
    showLogin();
  }
  initGoogleButton();
})();
