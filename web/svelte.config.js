import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  kit: {
    // SPA statis: semua rute dilayani shell yang sama (fallback 200.html),
    // cocok untuk hosting statis Netlify seperti landing page.
    // ssr dimatikan di src/routes/+layout.js (kripto WebCrypto +
    // localStorage butuh peramban).
    adapter: adapter({ fallback: '200.html' })
  }
};

export default config;
