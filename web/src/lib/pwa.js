// PWA: service worker + prompt install (item #10, 28 Nov 2026).
import { writable } from 'svelte/store';

/** Event beforeinstallprompt dari browser (bila ada) — disimpan agar
    tombol "Install aplikasi" bisa memunculkannya saat diketuk. */
/** @type {import('svelte/store').Writable<{ prompt: () => void; userChoice: Promise<{ outcome: string }> } | null>} */
export const installPrompt = writable(null);
/** true bila browser sudah menawarkan install (prompt tersedia). */
export const canInstall = writable(false);
/** true bila dibuka sebagai aplikasi terinstall (standalone). */
export const isStandalone = writable(false);
/** iPhone/iPad — tidak ada beforeinstallprompt; install lewat
    Safari: Bagikan → Tambah ke Layar Utama. */
export const isIOS =
  typeof navigator !== 'undefined' && /iphone|ipad|ipod/i.test(navigator.userAgent || '');

function checkStandalone() {
  if (typeof window === 'undefined') return false;
  return (
    window.matchMedia('(display-mode: standalone)').matches ||
    /** @type {any} */ (window.navigator).standalone === true
  );
}

export function initPWA() {
  if (typeof window === 'undefined') return;
  isStandalone.set(checkStandalone());
  if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('/sw.js').catch(() => {
        /* SW gagal — situs tetap jalan normal, cuma tak installable. */
      });
    });
  }
  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault();
    installPrompt.set(/** @type {any} */ (e));
    canInstall.set(true);
  });
  window.addEventListener('appinstalled', () => {
    installPrompt.set(null);
    canInstall.set(false);
    isStandalone.set(true);
  });
}

/** Tampilkan prompt install bawaan browser. false bila tak ada prompt. */
export async function promptInstall() {
  /** @type {{ prompt: () => void; userChoice: Promise<{ outcome: string }> } | null} */
  let e = null;
  installPrompt.subscribe((v) => (e = v))();
  if (!e) return false;
  e.prompt();
  const choice = await e.userChoice;
  if (choice && choice.outcome === 'accepted') {
    installPrompt.set(null);
    canInstall.set(false);
    return true;
  }
  return false;
}
