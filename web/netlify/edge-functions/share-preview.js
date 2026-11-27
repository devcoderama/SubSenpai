// Share-preview untuk crawler — Netlify Edge Function (Deno).
//
// Masalah yang dipecahkan: web frontend ini SPA statis, jadi crawler
// (Telegram, WhatsApp, Discord, X, Facebook, Googlebot) yang membuka
// tautan share /nonton/{episodeId} atau /judul/{id} hanya akan melihat
// shell SPA tanpa poster/judul. Halaman share di backend
// (GET /share/{id} dan /share-episode/{episodeId}) sudah menyiapkan
// preview lengkap (poster, judul, sinopsis, tombol buka aplikasi).
//
// Fungsi ini: bila User-Agent terlihat seperti crawler, teruskan
// permintaannya ke halaman share backend dan kembalikan HTML itu.
// Peramban biasa dilewatkan ke SPA (context.next()) sehingga
// pemutar web tetap jalan. App Links Android tidak terpengaruh
// (assetlinks.json dilayani langsung dari static/.well-known/).
//
// Catatan: edge function berjalan SEBELUM redirect SPA /* -> /200.html,
// jadi pola path di bawah tidak bentrok dengan fallback SPA.

const BOT_RE =
  /bot|crawler|spider|preview|facebookexternalhit|whatsapp|telegram|discord|slack|linkedin|twitter|pinterest|googlebot|bingbot|yandex|baidu|sogou|exabot|facebot|ia_archiver/i;

export default async (request, context) => {
  const ua = request.headers.get('user-agent') || '';
  if (!BOT_RE.test(ua)) return context.next();

  const url = new URL(request.url);
  const m = url.pathname.match(/^\/(nonton|judul)\/([^/]+)/);
  if (!m) return context.next();

  const kind = m[1] === 'nonton' ? 'share-episode' : 'share';
  const target = `https://api.subsenpai.web.id/${kind}/${encodeURIComponent(m[2])}`;
  try {
    const res = await fetch(target, {
      headers: { 'user-agent': ua, accept: 'text/html' }
    });
    return new Response(res.body, {
      status: res.status,
      headers: {
        'content-type': res.headers.get('content-type') || 'text/html; charset=utf-8',
        'cache-control': res.headers.get('cache-control') || 'public, max-age=3600'
      }
    });
  } catch {
    // Backend share sedang tidak bisa dijangkau — biarkan SPA yang tampil.
    return context.next();
  }
};

export const config = { path: ['/nonton/*', '/judul/*'] };
