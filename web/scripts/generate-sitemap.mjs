// Sitemap dinamis — SEO (28 Nov 2026).
//
// Dijalankan sebagai `prebuild` (npm run build): mengambil SEMUA judul
// dari API publik lalu menulis web/static/sitemap.xml — halaman statis
// + satu /judul/{id} per judul, supaya Google bisa menemukan tiap
// halaman judul secara sistematis (sitemap lama cuma 9 halaman statis,
// tanpa satu pun judul).
//
// Bila API gagal dijangkau, sitemap lama DIBIARKAN dan build tetap
// jalan (exit 0) — sitemap basi lebih baik daripada build gagal.
import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { getTitles } from '../src/lib/api.js';

const SITE = 'https://subsenpai.web.id';
const STATIC_PAGES = [
  '',
  'jadwal',
  'cari',
  'premium',
  'masuk',
  'post',
  'riwayat',
  'lainnya',
  'unduh',
  'terms',
  'privacy'
];
const OUT = join(dirname(fileURLToPath(import.meta.url)), '..', 'static', 'sitemap.xml');

try {
  const seen = new Set();
  const urls = [];
  for (const p of STATIC_PAGES) {
    const u = `${SITE}/${p}`;
    seen.add(u);
    urls.push(u);
  }
  let cursor = null;
  let pages = 0;
  let total = 0;
  do {
    const res = await getTitles({ limit: 100, cursor });
    for (const t of res.items || []) {
      if (!t?.id) continue;
      const u = `${SITE}/judul/${t.id}`;
      if (seen.has(u)) continue;
      seen.add(u);
      urls.push(u);
    }
    total += (res.items || []).length;
    cursor = res.next_cursor || null;
    pages += 1;
    if (pages > 500) break; // pengaman: 50 ribu judul
  } while (cursor);

  const xml =
    `<?xml version="1.0" encoding="UTF-8"?>\n` +
    `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n` +
    urls.map((u) => `  <url><loc>${u}</loc></url>`).join('\n') +
    `\n</urlset>\n`;
  writeFileSync(OUT, xml);
  console.log(`sitemap: ${urls.length} URL (${total} judul) -> ${OUT}`);
} catch (e) {
  console.warn('sitemap: gagal mengambil judul dari API, sitemap lama dibiarkan:', e?.message || e);
}
