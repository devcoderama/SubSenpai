// SEO — judul, deskripsi & JSON-LD per judul/episode (28 Nov 2026).
// Dipakai halaman Detail + Nonton (client-side <svelte:head>).
// Crawler tanpa JS dilayani halaman share backend (sharepage.go) yang
// memakai format yang SAMA — ubah di satu tempat, samakan di satunya.
export const SITE_URL = 'https://subsenpai.web.id';

export function typeLabel(t) {
  if (t === 'anime') return 'Anime';
  if (t === 'donghua') return 'Donghua';
  if (t === 'film') return 'Film';
  if (t === 'series') return 'Serial TV';
  return '';
}

/** Judul yang tampil di hasil pencarian Google: nama + tahun + tipe +
    kata kunci "Subtitle Indonesia". */
export function seoTitle(t) {
  if (!t) return 'SubSenpai — Nonton Anime, Donghua & Film Subtitle Indonesia';
  const year = t.year ? ` (${t.year})` : '';
  const type = typeLabel(t.type);
  return `${t.title}${year} — ${type ? type + ' ' : ''}Subtitle Indonesia | SubSenpai`;
}

/** Judul SEO untuk satu episode: "Judul — Episode N (tahun) — Tipe Subtitle Indonesia". */
export function seoEpisodeTitle(t, epNo) {
  if (!t) return 'SubSenpai — Nonton Anime, Donghua & Film Subtitle Indonesia';
  const base = `${t.title}${epNo != null ? ` — Episode ${epNo}` : ''}`;
  const year = t.year ? ` (${t.year})` : '';
  const type = typeLabel(t.type);
  return `${base}${year} — ${type ? type + ' ' : ''}Subtitle Indonesia | SubSenpai`;
}

/** Deskripsi: sinopsis bila ada; bila kosong, bangun dari metadata. */
export function seoDescription(t) {
  if (!t) return 'Nonton anime, donghua & film subtitle Indonesia di SubSenpai.';
  const s = (t.synopsis || '').trim();
  if (s) return s.length > 200 ? s.slice(0, 200) + '…' : s;
  const year = t.year ? ` (${t.year})` : '';
  const type = typeLabel(t.type);
  const genres = (t.genres || []).join(', ');
  return `Nonton ${t.title}${year}${type ? ' ' + type : ''} subtitle Indonesia di SubSenpai.${genres ? ' ' + genres + '.' : ''}`;
}

function baseJsonLd(t, type, name, url) {
  const obj = {
    '@context': 'https://schema.org',
    '@type': type,
    name,
    url,
    inLanguage: 'id'
  };
  if (t.poster_url) obj.image = t.poster_url;
  const d = seoDescription(t);
  if (d) obj.description = d;
  if (t.year) obj.datePublished = String(t.year);
  if (t.genres?.length) obj.genre = t.genres;
  // Rating skala 10 (TMDB / rating aplikasi) — bestRating eksplisit
  // supaya Google tidak mengira skala 5.
  if (t.rating && Number(t.rating) > 0) {
    obj.aggregateRating = {
      '@type': 'AggregateRating',
      ratingValue: String(Number(t.rating).toFixed(1)),
      bestRating: '10'
    };
  }
  return obj;
}

/** Bungkus JSON-LD menjadi tag <script> untuk {@html} di <svelte:head>.
    Dibangun DI SINI (file .js polos), bukan di blok <script> .svelte —
    string "</script>" di dalam blok <script> .svelte akan menutup blok itu
    dan merusak kompilasi. */
export function jsonLdScriptTag(json) {
  if (!json) return '';
  return '<script type="application/ld+json">' + json + '</script>';
}

/** JSON-LD Movie (film) / TVSeries (anime/donghua) untuk halaman Detail. */
export function seoJsonLd(t) {
  if (!t) return null;
  const type = t.type === 'film' ? 'Movie' : 'TVSeries';
  return JSON.stringify(baseJsonLd(t, type, t.title, `${SITE_URL}/judul/${t.id}`));
}

/** JSON-LD TVEpisode untuk halaman Nonton. */
export function seoEpisodeJsonLd(t, epNo, episodeId) {
  if (!t || epNo == null || !episodeId) return null;
  const obj = baseJsonLd(
    t,
    'TVEpisode',
    `${t.title} — Episode ${epNo}`,
    `${SITE_URL}/nonton/${episodeId}`
  );
  obj.episodeNumber = epNo;
  obj.partOfSeries = { '@type': 'TVSeries', name: t.title, url: `${SITE_URL}/judul/${t.id}` };
  return JSON.stringify(obj);
}
