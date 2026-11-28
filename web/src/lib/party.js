// Klien WebSocket Nonton Bareng — cermin ChatSocket.kt (aplikasi).
// Auth lewat ?token= (handleWS menerima token query — peramban tak bisa
// mengirim header Authorization di WebSocket). Ruang bersifat efemer
// (hanya sinyal di memori server): tak ada daftar anggota, tak ada chat,
// tak ada ikatan episode di server — episode dibawa klien (tautan undangan).
import { API_BASE } from './api.js';

const WS_BASE = API_BASE.replace(/^http/, 'ws');

export class PartySocket {
  constructor(token, onEvent) {
    this.token = token;
    this.onEvent = onEvent;
    this.ws = null;
    this.code = null; // kode ruang aktif (untuk sambung ulang)
    this.queue = [];
    this.retries = 0;
    this.closed = false; // close() sengaja dipanggil — jangan sambung ulang
  }

  connect() {
    this.closed = false;
    const ws = new WebSocket(WS_BASE + '/ws?token=' + encodeURIComponent(this.token));
    this.ws = ws;
    ws.onopen = () => {
      this.retries = 0;
      for (const msg of this.queue) ws.send(msg);
      this.queue = [];
      // Sambung ulang: masuk lagi ke ruang yang sama.
      if (this.code) this.join(this.code);
    };
    ws.onmessage = (ev) => {
      try {
        this.onEvent(JSON.parse(ev.data));
      } catch {
        /* abaikan bingkai tak terbaca */
      }
    };
    ws.onclose = () => {
      if (this.closed) return;
      if (this.code && this.retries < 3) {
        this.retries += 1;
        setTimeout(() => {
          if (!this.closed) this.connect();
        }, 2000);
      } else {
        this.onEvent({ type: 'party_disconnected' });
      }
    };
    ws.onerror = () => {
      /* onclose yang menangani */
    };
  }

  send(obj) {
    const msg = JSON.stringify(obj);
    if (this.ws && this.ws.readyState === WebSocket.OPEN) this.ws.send(msg);
    else this.queue.push(msg);
  }

  create(episodeId) {
    this.send({ type: 'party_create', episode_id: episodeId });
  }

  join(code) {
    this.send({ type: 'party_join', code });
  }

  sync(code, action, positionSec) {
    this.send({ type: 'party_sync', code, action, position_sec: positionSec });
  }

  leave(code) {
    this.send({ type: 'party_leave', code });
  }

  close() {
    this.closed = true;
    try {
      this.ws?.close(1000);
    } catch {
      /* abaikan */
    }
    this.ws = null;
  }
}
