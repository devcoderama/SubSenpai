// Service worker PWA SubSenpai (item #10, 28 Nov 2026).
// Minimal dan aman: hanya mengurus origin sendiri; API
// (api.subsenpai.web.id), CDN, dan poster situs luar dilewatkan
// begitu saja (tidak pernah di-cache).
//
// - Halaman (navigasi): network-first, jatuh ke shell cache '/' saat
//   offline — aplikasi tetap terbuka, datanya yang gagal dimuat.
// - Aset berversi /_app/immutable/: cache-first (nama file membawa
//   hash, jadi tak pernah basi).
// - File statis lain (ikon, manifest): stale-while-revalidate.
const VERSION = 'v1';
const SHELL = 'subsenpai-shell-' + VERSION;
const RUNTIME = 'subsenpai-runtime-' + VERSION;
const PRECACHE = ['/', '/manifest.webmanifest', '/icon-192.png', '/icon-512.png'];

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches
      .open(SHELL)
      .then((c) => c.addAll(PRECACHE))
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys.filter((k) => k !== SHELL && k !== RUNTIME).map((k) => caches.delete(k))
        )
      )
      .then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (e) => {
  if (e.request.method !== 'GET') return;
  const url = new URL(e.request.url);
  // Bukan origin sendiri (API, CDN, poster) — jangan sentuh.
  if (url.origin !== self.location.origin) return;

  if (url.pathname.startsWith('/_app/immutable/')) {
    e.respondWith(
      caches.open(RUNTIME).then((c) =>
        c.match(e.request).then(
          (hit) =>
            hit ||
            fetch(e.request).then((res) => {
              if (res.ok) c.put(e.request, res.clone());
              return res;
            })
        )
      )
    );
    return;
  }

  if (e.request.mode === 'navigate') {
    e.respondWith(
      fetch(e.request).catch(() =>
        caches.open(SHELL).then((c) => c.match('/'))
      )
    );
    return;
  }

  e.respondWith(
    caches.open(RUNTIME).then((c) =>
      c.match(e.request).then((hit) => {
        const net = fetch(e.request)
          .then((res) => {
            if (res.ok) c.put(e.request, res.clone());
            return res;
          })
          .catch(() => hit);
        return hit || net;
      })
    )
  );
});
