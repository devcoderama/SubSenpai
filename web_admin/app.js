/* SubSenpai — Dashboard Admin (vanilla JS).
 *
 * Berbicara ke backend yang sama dengan aplikasi Android:
 * handshake ECDH (POST /api/v1/session) -> envelope {xdata, xtime}
 * AES-256-GCM, lalu JWT dari login Google. Hanya akun dengan
 * role 'admin' yang dibiarkan masuk; tidak ada pendaftaran di sini.
 */
'use strict';

const API_BASE = 'https://subsenpai-api.botlynk.id';
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

/* sessionStorage bisa MELEMPAR error di sebagian peramban (Brave
 * Shields / mode blokir cookie): setItem/removeItem gagal sementara
 * getItem jalan — login membeku diam bila tidak dibungkus. Semua
 * akses storage lewat pembungkus aman ini dengan cadangan memori. */
const memStore = {};
const storeGet = (k) => { try { return sessionStorage.getItem(k); } catch (e) { return memStore[k] || null; } };
const storeSet = (k, v) => { try { sessionStorage.setItem(k, v); } catch (e) { memStore[k] = v; } };
const storeDel = (k) => { try { sessionStorage.removeItem(k); } catch (e) { delete memStore[k]; } };

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
const TITLES = { ringkasan: 'Ringkasan', metode: 'Metode Pembayaran', paket: 'Paket Premium', pengguna: 'Pengguna', transaksi: 'Transaksi', voucher: 'Voucher', judul: 'Kelola Judul', crash: 'Laporan Crash', pengaturan: 'Pengaturan Aplikasi', notifikasi: 'Notifikasi', log: 'Log Admin', sistem: 'Info Sistem' };

function route() {
  const name = (location.hash.replace('#/', '') || 'ringkasan').split('?')[0];
  const page = TITLES[name] ? name : 'ringkasan';
  document.querySelectorAll('#mainNav a').forEach((a) =>
    a.classList.toggle('active', a.dataset.route === page));
  $('pageTitle').textContent = TITLES[page];
  ({ ringkasan: pageRingkasan, metode: pageMetode, paket: pagePaket, pengguna: pagePengguna, transaksi: pageTransaksi, voucher: pageVoucher, judul: pageJudul, crash: pageCrash, pengaturan: pagePengaturan, notifikasi: pageNotifikasi, log: pageLog, sistem: pageSistem })[page]();
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
    html += '<h2 class="group-title">' + GROUP_LABELS[g] + '</h2><table><tr><th>Metode</th><th>Kode</th><th>Aktif</th></tr>';
    for (const m of items) {
      html += '<tr><td>' + (m.icon_url ? '<img class="method-icon" src="' + esc(m.icon_url) + '" alt="">' : '') + esc(m.name) + '</td>' +
        '<td>' + esc(m.code) + '</td>' +
        '<td><label class="switch"><input type="checkbox" data-method="' + esc(m.code) + '"' + (m.is_active ? ' checked' : '') +
        '><span class="slider"></span></label></td></tr>';
    }
    html += '</table>';
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
    '<table><tr><th>Paket</th><th>Durasi</th><th>Harga (Rp)</th><th>Aktif</th><th></th></tr>';
  for (const p of r.body.plans || []) {
    html += '<tr><td>' + esc(p.label) + '<div class="muted small">' + esc(p.id) + '</div></td>' +
      '<td>' + p.days + ' hari</td>' +
      '<td><input type="number" min="0" step="500" value="' + p.price + '" id="price-' + esc(p.id) + '" style="width:130px"></td>' +
      '<td><label class="switch"><input type="checkbox" id="active-' + esc(p.id) + '"' + (p.is_active ? ' checked' : '') + '><span class="slider"></span></label></td>' +
      '<td><button class="btn small" data-save-plan="' + esc(p.id) + '">Simpan</button></td></tr>';
  }
  $('pageBody').innerHTML = html + '</table>';
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
    let html = '<table><tr><th>Pengguna</th><th>Role</th><th>Premium</th><th>Aksi</th></tr>';
    for (const u of r.body.users || []) {
      const isMe = me && u.id === me.id;
      html += '<tr><td>' + (u.avatar_url ? '<img class="avatar" src="' + esc(u.avatar_url) + '" referrerpolicy="no-referrer" alt="">' : '') +
        esc(u.display_name || '(tanpa nama)') + '<div class="muted small">' + esc(u.email) + '</div></td>' +
        '<td><span class="badge ' + esc(u.role) + '">' + esc(u.role) + '</span>' + (u.is_banned ? ' <span class="badge expired">banned</span>' : '') + '</td>' +
        '<td>' + (u.premium_aktif ? '<span class="badge premium">aktif s.d. ' + esc(fmtDate(u.premium_until)) + '</span>' : '<span class="muted">—</span>') + '</td>' +
        '<td>' + (isMe ? '<span class="muted small">(kamu)</span>' :
          (u.role === 'admin'
            ? '<button class="btn ghost small" data-role="' + esc(u.id) + '|user">Turunkan jadi User</button>'
            : '<button class="btn purple small" data-role="' + esc(u.id) + '|admin">Jadikan Admin</button>')) + '</td></tr>';
    }
    $('userList').innerHTML = html + '</table>';
    document.querySelectorAll('button[data-role]').forEach((el) => {
      el.addEventListener('click', async () => {
        const [id, role] = el.dataset.role.split('|');
        const msg = role === 'admin'
          ? 'Angkat pengguna ini menjadi ADMIN? Ia akan bisa membuka dashboard ini.'
          : 'Turunkan pengguna ini menjadi user biasa?';
        if (!confirm(msg)) return;
        const rr = await api('/api/v1/admin/users/' + encodeURIComponent(id), { method: 'PATCH', body: { role } });
        if (rr.status === 200) { toast('Role diubah menjadi ' + role); load(); }
        else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
      });
    });
  };
  $('userSearchBtn').addEventListener('click', load);
  $('userQ').addEventListener('keydown', (e) => { if (e.key === 'Enter') load(); });
  load();
}

/* ----- Transaksi ----- */
function trxTable(list) {
  if (!list.length) return '<p class="muted">Belum ada transaksi.</p>';
  let html = '<table><tr><th>Waktu</th><th>Pengguna</th><th>Keperluan</th><th>Provider</th><th>Jumlah</th><th>Status</th></tr>';
  for (const t of list) {
    html += '<tr><td>' + esc(fmtDate(t.created_at)) + '</td>' +
      '<td>' + esc(t.display_name || '(tanpa nama)') + '<div class="muted small">' + esc(t.email) + '</div></td>' +
      '<td>' + esc(t.purpose === 'voucher' ? 'Voucher' : 'Langganan') + (t.plan ? '<div class="muted small">' + esc(t.plan) + '</div>' : '') + '</td>' +
      '<td>' + esc(t.provider) + '</td><td>' + rp(t.amount) + '</td>' +
      '<td><span class="badge ' + esc(t.status) + '">' + esc(t.status) + '</span></td></tr>';
  }
  return html + '</table>';
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
    '<table><tr><th>Waktu</th><th>Pengguna</th><th>Keperluan</th><th>Provider</th><th>Jumlah</th><th>Status</th><th>Aksi</th></tr>';
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
  $('pageBody').innerHTML = html + '</table>';
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
  let html = '<table><tr><th>Kode</th><th>Durasi</th><th>Sumber</th><th>Untuk</th><th>Status</th><th>Ditukar oleh</th><th>Dibuat</th><th>Aksi</th></tr>';
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
  $('vc-list').innerHTML = html + '</table>';
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
async function pageJudul() {
  $('pageBody').innerHTML =
    '<p class="muted">Judul tersembunyi hilang dari aplikasi (beranda, katalog, pencarian, jadwal, detail, putar) tanpa menghapus datanya.</p>' +
    '<div class="toolbar"><input type="search" id="jd-q" placeholder="Cari judul…" style="flex:1">' +
    '<select id="jd-type"><option value="">Semua jenis</option><option value="anime">Anime</option><option value="donghua">Donghua</option><option value="film">Film</option></select>' +
    '<button class="btn" id="jd-search">Cari</button></div>' +
    '<div id="jd-list"><p class="muted">Memuat…</p></div>' +
    '<div style="margin-top:14px"><button class="btn ghost" id="jd-more">Muat lebih banyak</button></div>';
  judulOffset = 0;
  $('jd-search').addEventListener('click', () => { judulOffset = 0; loadJudul(true); });
  $('jd-q').addEventListener('keydown', (e) => { if (e.key === 'Enter') { judulOffset = 0; loadJudul(true); } });
  $('jd-more').addEventListener('click', () => loadJudul(false));
  loadJudul(true);
}
async function loadJudul(reset) {
  const q = encodeURIComponent($('jd-q').value.trim());
  const t = encodeURIComponent($('jd-type').value);
  const r = await api('/api/v1/admin/titles?q=' + q + '&type=' + t + '&limit=50&offset=' + judulOffset);
  if (r.status !== 200) { if (reset) $('jd-list').innerHTML = '<p class="error">Gagal memuat judul.</p>'; return; }
  const list = r.body.titles || [];
  judulOffset += list.length;
  let html = reset ? '<p class="muted">' + (r.body.total || 0) + ' judul</p><table><tr><th>Poster</th><th>Judul</th><th>Jenis</th><th>Tahun</th><th>Episode</th><th>Visibilitas</th><th>Aksi</th></tr>' : '';
  for (const x of list) {
    html += '<tr><td>' + (x.poster_url ? '<img src="' + esc(x.poster_url) + '" alt="" style="width:38px;height:54px;object-fit:cover;border-radius:6px" loading="lazy">' : '—') + '</td>' +
      '<td>' + esc(x.title) + '<div class="muted small">' + esc(x.slug) + '</div></td>' +
      '<td>' + esc(x.type) + '</td><td>' + (x.year || '—') + '</td><td>' + x.episode_count + '</td>' +
      '<td><span class="badge ' + (x.is_hidden ? 'expired' : 'premium') + '">' + (x.is_hidden ? 'Tersembunyi' : 'Tampil') + '</span></td>' +
      '<td><button class="btn ' + (x.is_hidden ? '' : 'ghost') + ' small" data-jd-toggle="' + esc(x.id) + '|' + (x.is_hidden ? 'show' : 'hide') + '">' +
      (x.is_hidden ? 'Tampilkan' : 'Sembunyikan') + '</button></td></tr>';
  }
  if (reset) { $('jd-list').innerHTML = html + '</table>'; }
  else { $('jd-list').querySelector('table').insertAdjacentHTML('beforeend', html); }
  document.querySelectorAll('button[data-jd-toggle]').forEach((el) => {
    if (el.dataset.wired) return;
    el.dataset.wired = '1';
    el.addEventListener('click', async () => {
      const [id, act] = el.dataset.jdToggle.split('|');
      const rr = await api('/api/v1/admin/titles/' + encodeURIComponent(id), {
        method: 'PATCH', body: { is_hidden: act === 'hide' },
      });
      if (rr.status === 200) { toast(act === 'hide' ? 'Judul disembunyikan.' : 'Judul ditampilkan.'); judulOffset = 0; loadJudul(true); }
      else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
    });
  });
}

/* ----- Laporan Crash ----- */
async function pageCrash() {
  $('pageBody').innerHTML =
    '<div class="toolbar"><p class="muted" style="flex:1">Laporan crash dari aplikasi (v0.22.0+). Ketuk satu laporan untuk melihat stack trace-nya.</p>' +
    '<button class="btn" id="cr-clear">Kosongkan Semua</button></div><div id="cr-list"><p class="muted">Memuat…</p></div>';
  $('cr-clear').addEventListener('click', async () => {
    if (!confirm('Hapus SEMUA laporan crash?')) return;
    const rr = await api('/api/v1/admin/crashes', { method: 'DELETE' });
    if (rr.status === 200) { toast('Dihapus: ' + rr.body.deleted + ' laporan.'); pageCrash(); }
    else toast('Gagal: ' + ((rr.body && rr.body.error) || rr.status));
  });
  const r = await api('/api/v1/admin/crashes?limit=100');
  if (r.status !== 200) { $('cr-list').innerHTML = '<p class="error">Gagal memuat laporan.</p>'; return; }
  const list = r.body.crashes || [];
  if (!list.length) { $('cr-list').innerHTML = '<p class="muted">Belum ada laporan crash. 🎉</p>'; return; }
  let html = '';
  for (const c of list) {
    const firstLine = (c.message || c.stack_trace || '').split('\n')[0];
    html += '<div class="card" style="margin-bottom:12px">' +
      '<div class="toolbar" style="align-items:flex-start"><div style="flex:1"><b>' + esc(firstLine || '(tanpa pesan)') + '</b>' +
      '<div class="muted small">' + esc(fmtDate(c.created_at)) + ' • v' + esc(c.app_version) + ' (' + c.version_code + ') • ' +
      esc(c.device) + ' • ' + esc(c.android_version) + (c.user_email ? ' • ' + esc(c.user_email) : '') + '</div></div>' +
      '<button class="btn ghost small" data-cr-toggle="' + esc(c.id) + '">Stack trace</button>' +
      '<button class="btn small" data-cr-del="' + esc(c.id) + '">Hapus</button></div>' +
      '<pre id="cr-stack-' + esc(c.id) + '" hidden style="white-space:pre-wrap;word-break:break-word;font-size:0.78rem;max-height:320px;overflow:auto;background:var(--panel-2);padding:12px;border-radius:9px">' + esc(c.stack_trace) + '</pre></div>';
  }
  $('cr-list').innerHTML = html;
  document.querySelectorAll('button[data-cr-toggle]').forEach((el) =>
    el.addEventListener('click', () => { const p = $('cr-stack-' + el.dataset.crToggle); p.hidden = !p.hidden; }));
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
    '<div class="card" style="margin-top:18px"><h2 style="margin-top:0">Pengumuman</h2>' +
    '<p class="muted">Teks ini tampil sebagai banner di Beranda aplikasi (tersimpan lewat tombol Simpan Pengaturan di bawah). Untuk mengirim notifikasi beneran ke halaman Notifikasi pengguna, pakai halaman <b>Notifikasi</b> di menu samping.</p>' +
    '<input type="text" id="st-ann" value="' + esc(s.announcement || '') + '" placeholder="cth: Server pindah — selamat menonton!" style="width:100%"></div>' +
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
      announcement: $('st-ann').value,
    };
    const rr = await api('/api/v1/admin/settings', { method: 'PATCH', body });
    if (rr.status === 200) toast('Pengaturan disimpan.');
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
  let html = '<table><tr><th>Waktu</th><th>Judul</th><th>Penerima</th><th>Terkirim</th><th>Dibaca</th><th></th></tr>';
  for (const b of list) {
    html += '<tr><td>' + esc(fmtDate(b.created_at)) + '</td>' +
      '<td><b>' + esc(b.title) + '</b>' + (b.body ? '<br><span class="small muted">' + esc(b.body) + '</span>' : '') + '</td>' +
      '<td>' + esc(audLbl[b.audience] || b.audience) + '</td>' +
      '<td>' + b.sent_count + '</td><td>' + b.read_count + '</td>' +
      '<td><button class="btn ghost small" data-del="' + esc(b.id) + '"><i class="fa-solid fa-trash"></i> Hapus</button></td></tr>';
  }
  $('nt-list').innerHTML = html + '</table>';
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
  let html = '<table><tr><th>Waktu</th><th>Admin</th><th>Aksi</th><th>Target</th><th>Rincian</th></tr>';
  for (const l of list) {
    let detail = '';
    try { detail = JSON.stringify(l.detail); } catch (e) { detail = ''; }
    if (detail.length > 90) detail = detail.slice(0, 90) + '…';
    html += '<tr><td>' + esc(fmtDate(l.created_at)) + '</td><td>' + esc(l.admin_email) + '</td>' +
      '<td><span class="badge info">' + esc(l.action) + '</span></td><td class="small">' + esc(l.target || '—') + '</td>' +
      '<td class="small muted">' + esc(detail) + '</td></tr>';
  }
  $('pageBody').innerHTML = html + '</table>';
}

/* ----- Info Sistem ----- */
async function pageSistem() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/system');
  if (r.status !== 200) return bodyError(r);
  const b = r.body, c = b.counts || {}, cfg = b.config || {};
  const flag = (v) => v ? '<span class="badge premium">terhubung</span>' : '<span class="badge expired">belum diatur</span>';
  let html = '<div class="cards">' +
    statCard('Pengguna', c.users) + statCard('Admin', c.admins) + statCard('Premium aktif', c.premium_users) +
    statCard('Judul', c.titles) + statCard('Judul tersembunyi', c.titles_hidden) + statCard('Episode', c.episodes) +
    statCard('Transaksi', c.transactions) + statCard('Voucher aktif', c.vouchers_active) +
    statCard('Laporan crash', c.crash_reports) + statCard('Postingan feed', c.feed_posts) + '</div>';
  html += '<div class="card"><h2 style="margin-top:0">Server</h2><table>' +
    '<tr><th>Go</th><td>' + esc(b.go_version) + '</td></tr>' +
    '<tr><th>Migrasi terakhir</th><td>' + esc(b.latest_migration || '—') + '</td></tr>' +
    '<tr><th>Waktu server</th><td>' + esc(fmtDate(b.server_time)) + '</td></tr></table></div>';
  html += '<div class="card" style="margin-top:18px"><h2 style="margin-top:0">Konfigurasi</h2><table>' +
    '<tr><th>Login Google</th><td>' + flag(cfg.google_login) + '</td></tr>' +
    '<tr><th>Tokopay</th><td>' + flag(cfg.tokopay) + '</td></tr>' +
    '<tr><th>ImgBB (gambar feed)</th><td>' + flag(cfg.imgbb) + '</td></tr></table>' +
    '<p class="muted">Konfigurasi diatur lewat berkas env di VPS (<code>/etc/subsenpai/backend.env</code>), bukan dari halaman ini.</p></div>';
  $('pageBody').innerHTML = html;
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
