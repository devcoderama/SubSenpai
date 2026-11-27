// Iklan Monetag web — HANYA untuk non-premium (27 Nov 2026).
//
// Aturan Rams: akun Premium TIDAK ADA iklan sama sekali di halaman web.
// Karena itu skrip Monetag TIDAK dimuat di app.html — melainkan dari sini,
// sekali per muat halaman, sesudah status premium dipastikan:
//
//   - Profil tersimpan menyebut premium  -> jangan muat iklan.
//   - Login tapi profil belum menyebut premium -> tanyakan
//     GET /api/v1/premium/status (profil tersimpan bisa basi sesudah beli /
//     tukar voucher / klaim trial); premium -> jangan muat iklan + segarkan
//     profil tersimpan; bukan premium -> muat iklan.
//   - Tamu (tanpa JWT) -> bukan premium -> muat iklan.
//   - Status gagal ditanya (jaringan / sesi mati) -> muat iklan.
//
// Tag yang dimuat (persis kode dari Monetag) berlaku GLOBAL — semua halaman
// termasuk Detail (/judul/*) dan Nonton (/nonton/*) — karena dimuat sekali
// dari layout root. In-Page Push + Vignette adalah format yang me-render
// sendiri (tanpa slot penempatan), jadi tidak ada yang perlu ditambahkan
// per halaman; yang menentukan tampil tidaknya di suatu halaman hanyalah
// jadwal/frekuensi Monetag sendiri.
//
// Bila akun menjadi premium di tengah sesi (tukar voucher, pembayaran
// lunas, klaim trial), refreshPremium() menyegarkan profil tersimpan lalu
// memuat ulang halaman sesudah pesan sukses sempat terbaca — skrip iklan
// yang sudah dimuat tak bisa dicabut, jadi muat ulang adalah satu-satunya
// cara memastikan nol iklan sejak saat itu. Hal yang sama berlaku bila
// pengguna login dengan akun premium di tengah sesi (langganan user store
// di bawah): iklan yang terlanjur dimuat sebagai tamu hilang lewat muat
// ulang, dan sesudahnya gate memastikan iklan tidak dimuat lagi.
import { get } from 'svelte/store';
import { getPremiumStatus } from './api.js';
import { user, isPremium, isLoggedIn, setProfile } from './auth.js';

const TAGS = [
  { zone: '12365637', src: 'https://nap5k.com/tag.min.js' }, // In-Page Push (Banner)
  { zone: '12365639', src: 'https://n6wxm.com/vignette.min.js' }, // Vignette banner
];

let loaded = false;
let decided = false;
let subscribed = false;

function injectTag(zone, src) {
  // Sama persis dengan kode tag Monetag: buat <script>, isi data-zone +
  // src, tempel ke <html>/<body>.
  const s = document.createElement('script');
  s.dataset.zone = zone;
  s.src = src;
  [document.documentElement, document.body].filter(Boolean).pop().appendChild(s);
}

export function adsLoaded() {
  return loaded;
}

export function loadAds() {
  if (loaded || typeof document === 'undefined') return;
  loaded = true;
  for (const t of TAGS) injectTag(t.zone, t.src);
}

function applyPremiumStatus(st) {
  const profile = get(user) || {};
  setProfile({ ...profile, is_premium: true, premium_until: st.premium_until || profile.premium_until });
}

export async function maybeLoadAds() {
  if (decided || typeof window === 'undefined') return;
  decided = true;

  // Login di tengah sesi dengan akun premium -> iklan yang terlanjur
  // dimuat sebagai tamu harus hilang: muat ulang sekali. Sesudah muat
  // ulang, gate di bawah memastikan iklan tidak dimuat lagi.
  if (!subscribed) {
    subscribed = true;
    user.subscribe((p) => {
      if (loaded && isPremium(p)) window.location.reload();
    });
  }

  if (isPremium(get(user))) return; // premium: nol iklan
  if (!isLoggedIn()) {
    loadAds(); // tamu: bukan premium
    return;
  }
  try {
    const st = await getPremiumStatus();
    if (st?.is_premium) {
      applyPremiumStatus(st); // segarkan profil tersimpan, jangan muat iklan
      return;
    }
  } catch {
    // Gagal tanya server — anggap bukan premium.
  }
  loadAds();
}

// Dipanggil halaman Premium sesudah akun menjadi premium (tukar voucher,
// pembayaran lunas, klaim trial). Mengembalikan true bila kini premium.
// Bila iklan sudah terlanjur dimuat, halaman dimuat ulang 1,5 detik
// kemudian supaya pesan sukses sempat terbaca.
export async function refreshPremium() {
  try {
    const st = await getPremiumStatus();
    if (!st?.is_premium) return false;
    applyPremiumStatus(st);
    if (loaded) setTimeout(() => window.location.reload(), 1500);
    return true;
  } catch {
    return false;
  }
}
