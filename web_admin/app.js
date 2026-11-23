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
const TITLES = { ringkasan: 'Ringkasan', metode: 'Metode Pembayaran', paket: 'Paket Premium', pengguna: 'Pengguna', transaksi: 'Transaksi' };

function route() {
  const name = (location.hash.replace('#/', '') || 'ringkasan').split('?')[0];
  const page = TITLES[name] ? name : 'ringkasan';
  document.querySelectorAll('#mainNav a').forEach((a) =>
    a.classList.toggle('active', a.dataset.route === page));
  $('pageTitle').textContent = TITLES[page];
  ({ ringkasan: pageRingkasan, metode: pageMetode, paket: pagePaket, pengguna: pagePengguna, transaksi: pageTransaksi })[page]();
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
async function pageTransaksi() {
  $('pageBody').innerHTML = '<p class="muted">Memuat…</p>';
  const r = await api('/api/v1/admin/transactions?limit=100');
  if (r.status !== 200) return bodyError(r);
  $('pageBody').innerHTML = trxTable(r.body.transactions || []);
}

/* ================= Boot ================= */
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
