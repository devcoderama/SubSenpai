#!/usr/bin/env bash
# Backup database PostgreSQL SubSenpai (DB "subsenpai" di VPS ini).
#
# Hasil: /root/backups/subsenpai-YYYYmmdd-HHMMSS.sql.gz
# Rotasi: hanya 14 berkas terbaru yang disimpan.
# Bila rclone punya remote "r2" (Cloudflare R2) atau "gdrive"
# (Google Drive), salinan terbaru ikut diunggah ke folder
# "subsenpai-backups" di sana (salinan di luar VPS — pelindung
# bila VPS-nya yang mati).
#
# Pasang (sebagai root di VPS):
#   curl -fsSL https://raw.githubusercontent.com/devcoderama/SubSenpai/main/deploy/backup-db.sh -o /usr/local/bin/subsenpai-backup
#   chmod +x /usr/local/bin/subsenpai-backup
#   /usr/local/bin/subsenpai-backup          # uji sekali
#   (crontab -l 2>/dev/null; echo '30 3 * * * /usr/local/bin/subsenpai-backup >> /var/log/subsenpai-backup.log 2>&1') | crontab -
#
# Pulihkan (restore) ke database kosong:
#   su postgres -c "dropdb --if-exists subsenpai && createdb subsenpai"
#   gunzip -c /root/backups/<berkas>.sql.gz | su postgres -c "psql -q subsenpai"
set -euo pipefail

# Berapa backup terbaru yang disimpan (yang lebih tua dihapus
# otomatis): di VPS sendiri dan di salinan luar VPS.
SIMPAN_LOKAL=14
SIMPAN_ONLINE=5

DIR=/root/backups
mkdir -p "$DIR"
STAMP=$(date +%Y%m%d-%H%M%S)
OUT="$DIR/subsenpai-$STAMP.sql.gz"

(cd /tmp && su postgres -c "pg_dump subsenpai") | gzip -9 > "$OUT"
echo "backup selesai: $OUT ($(du -h "$OUT" | cut -f1))"

# Rotasi lokal: berkas sesudah urutan terbaru ke-SIMPAN_LOKAL
# (yang tertua) dihapus.
ls -1t "$DIR"/subsenpai-*.sql.gz | tail -n +$((SIMPAN_LOKAL + 1)) | xargs -r rm -f

# Salinan luar VPS (opsional, aktif sendiri bila rclone remote
# "r2" atau "gdrive" ada).
REMOTE=""
if command -v rclone >/dev/null 2>&1; then
  if rclone listremotes 2>/dev/null | grep -qx 'r2:'; then
    REMOTE="r2:subsenpai-backups/"
  elif rclone listremotes 2>/dev/null | grep -qx 'gdrive:'; then
    REMOTE="gdrive:subsenpai-backups/"
  fi
fi
if [ -n "$REMOTE" ]; then
  if rclone copy "$OUT" "$REMOTE" --quiet; then
    echo "tersalin ke luar VPS: $REMOTE"
    # Rotasi salinan luar VPS: simpan SIMPAN_ONLINE berkas
    # terbaru (nama berkas berpola tanggal, urutan nama = urutan
    # waktu), sisanya dihapus. Google Drive: hapus permanen,
    # jangan masuk tong sampah dulu.
    DELFLAGS=()
    case "$REMOTE" in
      gdrive:*) DELFLAGS=(--drive-use-trash=false) ;;
    esac
    mapfile -t BERKAS_REMOTE < <(rclone lsf "$REMOTE" --files-only 2>/dev/null | grep '^subsenpai-.*\.sql\.gz$' | sort || true)
    JUMLAH=${#BERKAS_REMOTE[@]}
    if [ "$JUMLAH" -gt "$SIMPAN_ONLINE" ]; then
      for ((i = 0; i < JUMLAH - SIMPAN_ONLINE; i++)); do
        if rclone deletefile "${DELFLAGS[@]}" "$REMOTE${BERKAS_REMOTE[i]}" 2>/dev/null; then
          echo "rotasi: salinan lama terhapus: ${BERKAS_REMOTE[i]}"
        fi
      done
    fi
  fi
fi
