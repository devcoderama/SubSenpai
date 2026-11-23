#!/usr/bin/env bash
# Backup database PostgreSQL SubSenpai (DB "subsenpai" di VPS ini).
#
# Hasil: /root/backups/subsenpai-YYYYmmdd-HHMMSS.sql.gz
# Rotasi: hanya 14 berkas terbaru yang disimpan.
# Bila rclone dengan remote bernama "r2" sudah terkonfigurasi,
# salinan terbaru ikut diunggah ke bucket "subsenpai-backups"
# (salinan di luar VPS — pelindung bila VPS-nya yang mati).
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

DIR=/root/backups
mkdir -p "$DIR"
STAMP=$(date +%Y%m%d-%H%M%S)
OUT="$DIR/subsenpai-$STAMP.sql.gz"

su postgres -c "pg_dump subsenpai" | gzip -9 > "$OUT"
echo "backup selesai: $OUT ($(du -h "$OUT" | cut -f1))"

# Rotasi: berkas ke-15 dan seterusnya (yang tertua) dihapus.
ls -1t "$DIR"/subsenpai-*.sql.gz | tail -n +15 | xargs -r rm -f

# Salinan luar VPS (opsional, aktif sendiri bila rclone remote
# "r2" ada): rclone copy ke Cloudflare R2.
if command -v rclone >/dev/null 2>&1 && rclone listremotes 2>/dev/null | grep -qx 'r2:'; then
  rclone copy "$OUT" r2:subsenpai-backups/ --quiet && echo "tersalin ke R2: subsenpai-backups/"
fi
