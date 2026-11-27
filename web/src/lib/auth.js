// Auth web: JWT di localStorage (seperti dashboard admin, JWT backend
// berlaku 30 hari) + profil dari respons login Google.
import { writable } from 'svelte/store';
import { api, googleLogin as apiGoogleLogin, setJwt } from './api.js';

const JWT_KEY = 'ssweb.jwt';
const PROFILE_KEY = 'ssweb.profile';

function loadStored() {
  try {
    const token = localStorage.getItem(JWT_KEY);
    const profile = localStorage.getItem(PROFILE_KEY);
    if (token) setJwt(token);
    return { token, profile: profile ? JSON.parse(profile) : null };
  } catch {
    return { token: null, profile: null };
  }
}

const stored = typeof window !== 'undefined' ? loadStored() : { token: null, profile: null };

export const user = writable(stored.profile);
export const token = writable(stored.token);

export function isLoggedIn() {
  return !!stored.token;
}

export async function loginWithGoogle(idToken) {
  const res = await apiGoogleLogin(idToken);
  const t = res.token || res.jwt;
  const profile = res.profile || null;
  if (!t) throw new Error('Login gagal: server tidak mengembalikan token.');
  try {
    localStorage.setItem(JWT_KEY, t);
    if (profile) localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
  } catch {
    /* abaikan — mode privat */
  }
  setJwt(t);
  stored.token = t;
  stored.profile = profile;
  token.set(t);
  user.set(profile);
  return profile;
}

export function logout() {
  try {
    localStorage.removeItem(JWT_KEY);
    localStorage.removeItem(PROFILE_KEY);
  } catch {
    /* abaikan */
  }
  setJwt(null);
  stored.token = null;
  stored.profile = null;
  token.set(null);
  user.set(null);
  if (typeof window !== 'undefined' && window.google?.accounts?.id) {
    try {
      window.google.accounts.id.disableAutoSelect();
    } catch {
      /* abaikan */
    }
  }
}

export function isPremium(profile) {
  return !!profile?.is_premium;
}

// Segarkan profil tersimpan (localStorage + store) — dipakai saat status
// premium berubah di tengah sesi (tukar voucher / bayar / trial) supaya
// mahkota Premium di navbar + gate iklan langsung konsisten.
export function setProfile(profile) {
  try {
    if (profile) localStorage.setItem(PROFILE_KEY, JSON.stringify(profile));
  } catch {
    /* abaikan — mode privat */
  }
  stored.profile = profile;
  user.set(profile);
}
