// Riwayat tontonan lokal (tamu) — cadangan bila belum masuk.
// Pengguna yang masuk memakai riwayat server (GET /api/v1/history/watch).
const KEY = 'ssweb.history';
const MAX = 50;

export function getLocalHistory() {
  try {
    const raw = localStorage.getItem(KEY);
    const arr = raw ? JSON.parse(raw) : [];
    return Array.isArray(arr) ? arr : [];
  } catch {
    return [];
  }
}

function save(list) {
  try {
    localStorage.setItem(KEY, JSON.stringify(list.slice(0, MAX)));
  } catch {
    /* penyimpanan penuh / privat — abaikan */
  }
}

// item: {episode_id, title_id, title, poster_url, episode_no, position_sec}
export function addLocalHistory(item) {
  if (!item?.episode_id) return;
  const list = getLocalHistory().filter((x) => x.episode_id !== item.episode_id);
  list.unshift({
    ...item,
    position_sec: Math.floor(item.position_sec || 0),
    updated_at: new Date().toISOString()
  });
  save(list);
}

export function updateLocalPosition(episodeId, positionSec) {
  const list = getLocalHistory();
  const it = list.find((x) => x.episode_id === episodeId);
  if (!it) return;
  it.position_sec = Math.floor(positionSec);
  it.updated_at = new Date().toISOString();
  save(list);
}

export function removeLocalHistory(episodeId) {
  save(getLocalHistory().filter((x) => x.episode_id !== episodeId));
}

export function clearLocalHistory() {
  try {
    localStorage.removeItem(KEY);
  } catch {
    /* abaikan */
  }
}

export function fmtPosition(sec) {
  const s = Math.floor(sec || 0);
  const m = Math.floor(s / 60);
  const r = s % 60;
  return m + ':' + String(r).padStart(2, '0');
}

export function fmtWhen(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  const diff = Date.now() - d.getTime();
  if (diff < 60 * 1000) return 'baru saja';
  if (diff < 60 * 60 * 1000) return Math.floor(diff / 60000) + ' mnt lalu';
  if (diff < 24 * 60 * 60 * 1000) return Math.floor(diff / 3600000) + ' jam lalu';
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
}
