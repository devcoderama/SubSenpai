// Aksi Svelte untuk animasi scroll ala aplikasi / Netflix / YouTube.
//
// reveal — elemen muncul satu per satu saat masuk viewport (fade + naik
// sedikit). Pakai: use:reveal={delayMs} — delay dipakai untuk jeda
// berurutan di grid (mis. (i % 12) * 45).
// infiniteScroll — panggil onLoad saat sentinel di ujung daftar terlihat
// (prefetch 400px sebelum ujung). Pakai:
//   use:infiniteScroll={{ enabled: !!nextCursor && !loadingMore, onLoad: loadMore }}
// Keduanya aman bila IntersectionObserver tidak ada (langsung tampil /
// tidak auto-load — halaman tetap menyediakan tombol "Muat lagi").

export function reveal(node, delay = 0) {
  node.classList.add('reveal');
  if (delay) node.style.setProperty('--rd', delay + 'ms');
  if (
    !('IntersectionObserver' in window) ||
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  ) {
    node.classList.add('in');
    return {};
  }
  const io = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (e.isIntersecting) {
          node.classList.add('in');
          io.disconnect();
        }
      }
    },
    { threshold: 0.08, rootMargin: '0px 0px 60px 0px' }
  );
  io.observe(node);
  return {
    destroy() {
      io.disconnect();
    }
  };
}

export function infiniteScroll(node, params) {
  let { enabled, onLoad } = params || {};
  let io = null;

  function teardown() {
    if (io) {
      io.disconnect();
      io = null;
    }
  }
  function setup() {
    teardown();
    if (!enabled || !('IntersectionObserver' in window)) return;
    io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) onLoad();
        }
      },
      { rootMargin: '0px 0px 400px 0px' }
    );
    io.observe(node);
  }

  setup();
  return {
    update(p) {
      ({ enabled, onLoad } = p || {});
      setup();
    },
    destroy() {
      teardown();
    }
  };
}
