#!/usr/bin/env bash
# install-vps.sh — Pemasang SubSenpai di VPS Ubuntu 22.04 yang SUDAH
# menjalankan proyek lain (MySQL/PHP/Laravel). Dirancang TIDAK bentrok:
#   - PostgreSQL cluster sendiri (port 5432, bawaan Ubuntu)
#   - API hanya mendengarkan 127.0.0.1:18080 (tidak terbuka ke internet)
#   - Web server yang ada TIDAK diubah kecuali DOMAIN_API diisi, dan
#     hanya MENAMBAH satu vhost baru (tidak menyentuh situs lain)
#   - User sistem, folder /opt/subsenpai, unit systemd sendiri
# Idempoten: aman dijalankan ulang untuk memperbarui ke versi terbaru.
#
# Pakai:
#   sudo bash install-vps.sh
#   sudo DOMAIN_API=api.contoh.com bash install-vps.sh
set -euo pipefail

REPO_URL="https://github.com/devcoderama/SubSenpai.git"
SRC_DIR="/opt/subsenpai/src"
APP_DIR="/opt/subsenpai"
ENV_DIR="/etc/subsenpai"
ENV_FILE="$ENV_DIR/backend.env"
GO_VER="1.24.4"
API_PORT="18080"
DOMAIN_API="${DOMAIN_API:-}"

say() { echo "[subsenpai] $*"; }
[ "$(id -u)" = "0" ] || { echo "Jalankan sebagai root (sudo)."; exit 1; }

echo "================ SURVEI SINGKAT ================"
ss -tlnp 2>/dev/null | awk 'NR==1 || /:(80|443|3306|5432|18080) /' || true
free -m | head -2
df -h / | tail -1
echo "================================================"

# --- 1. Paket dasar + PostgreSQL -------------------------------------
if ! command -v psql >/dev/null 2>&1; then
  say "Memasang PostgreSQL…"
  apt-get update -qq --allow-releaseinfo-change
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq postgresql git curl ca-certificates openssl >/dev/null
else
  say "PostgreSQL sudah ada — dilewati."
  apt-get install -y -qq git curl ca-certificates openssl >/dev/null 2>&1 || true
fi
systemctl enable --now postgresql >/dev/null 2>&1 || service postgresql start

# --- 2. User sistem ----------------------------------------------------
id subsenpai >/dev/null 2>&1 || useradd --system --home "$APP_DIR" --shell /usr/sbin/nologin subsenpai
mkdir -p "$APP_DIR" "$ENV_DIR"

# --- 3. Database + env (hanya sekali; rahasia dibuat DI VPS ini) -------
FRESH_INSTALL=0
if [ ! -f "$ENV_FILE" ]; then
  FRESH_INSTALL=1
  DBPASS="$(openssl rand -hex 24)"
  JWTSECRET="$(openssl rand -hex 32)"
  sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
do \$\$ begin
  if not exists (select from pg_roles where rolname = 'subsenpai') then
    create role subsenpai login password '$DBPASS';
  else
    alter role subsenpai password '$DBPASS';
  end if;
end \$\$;
SQL
  sudo -u postgres psql -tAc "select 1 from pg_database where datname='subsenpai'" | grep -q 1 \
    || sudo -u postgres createdb --owner=subsenpai subsenpai
  cat > "$ENV_FILE" <<EOF
HOST=127.0.0.1
PORT=$API_PORT
DATABASE_URL=postgres://subsenpai:$DBPASS@127.0.0.1:5432/subsenpai?sslmode=disable
JWT_SECRET=$JWTSECRET
GOOGLE_CLIENT_ID=
GOOGLE_CERTS_URL=
MIDTRANS_SERVER_KEY=
XENDIT_SECRET_KEY=
XENDIT_CALLBACK_TOKEN=
GOPAY_MERCHANT_URL=
GOPAY_USERNAME=
GOPAY_PASSWORD=
TOKOPAY_MERCHANT_ID=
TOKOPAY_SECRET_KEY=
TOKOPAY_API_URL=
DONGHUA_ID=https://anichin.moe
ANIME_ID=https://kuronime.sbs
FILM=https://hashflix.top
EOF
  chmod 600 "$ENV_FILE"
  say "Database + env dibuat (rahasia hanya tersimpan di $ENV_FILE)."
else
  say "Env sudah ada — database & rahasia lama dipertahankan."
fi

# --- 4. Go toolchain ----------------------------------------------------
if [ ! -x /usr/local/go/bin/go ]; then
  say "Memasang Go $GO_VER…"
  curl -fsSL "https://dl.google.com/go/go$GO_VER.linux-amd64.tar.gz" -o /tmp/go.tgz
  rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz && rm /tmp/go.tgz
fi
export PATH="/usr/local/go/bin:$PATH"

# --- 5. Sumber + build ---------------------------------------------------
if [ -d "$SRC_DIR/.git" ]; then
  say "Memperbarui sumber…"
  git config --global --add safe.directory "$SRC_DIR" 2>/dev/null || true
  # fetch + reset --hard (bukan pull --ff-only): klon dangkal
  # (--depth 50) terbukti bisa menjawab "Already up to date" padahal
  # origin/main sudah maju (kejadian 2026-11-22: worker lama terus
  # terpasang). Reset keras ke origin/main deterministik, dan commit
  # hasilnya dicetak agar bisa diverifikasi dari laporan.
  git -C "$SRC_DIR" fetch --depth 200 origin main
  git -C "$SRC_DIR" checkout -f main 2>/dev/null || git -C "$SRC_DIR" checkout -f -b main origin/main
  git -C "$SRC_DIR" reset --hard origin/main
  say "Sumber kini di commit: $(git -C "$SRC_DIR" log --oneline -1)"
else
  say "Mengkloning repo…"
  git clone --depth 50 "$REPO_URL" "$SRC_DIR"
fi
say "Membangun binary (server + worker)…"
(cd "$SRC_DIR/backend" && CGO_ENABLED=0 go build -buildvcs=false -trimpath -o "$APP_DIR/subsenpai-server" ./cmd/server \
  && CGO_ENABLED=0 go build -buildvcs=false -trimpath -o "$APP_DIR/subsenpai-worker" ./cmd/worker)
chown -R subsenpai:subsenpai "$APP_DIR"

# --- 6. systemd -----------------------------------------------------------
cp "$SRC_DIR/deploy/systemd/subsenpai-api.service" /etc/systemd/system/
cp "$SRC_DIR/deploy/systemd/subsenpai-worker.service" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now subsenpai-api subsenpai-worker >/dev/null 2>&1 || true
systemctl restart subsenpai-api subsenpai-worker

# --- 7. Seed contoh (HANYA pemasangan baru & katalog masih kosong) -------
# Pemasangan ulang tidak pernah menanam ulang data contoh: katalog yang
# sudah dikosongkan/diisi konten asli tidak ditimpa dummy lagi.
set -a; . "$ENV_FILE"; set +a
COUNT="$(psql "$DATABASE_URL" -tA -c 'select count(*) from titles' 2>/dev/null || echo x)"
if [ "$FRESH_INSTALL" != "1" ]; then
  say "Bukan pemasangan baru — seed contoh dilewati (katalog berisi $COUNT judul)."
elif [ "$COUNT" = "0" ]; then
  say "Mengisi data contoh…"
  psql "$DATABASE_URL" -q -f "$SRC_DIR/db/seed/dev_seed.sql" >/dev/null
elif [ "$COUNT" = "x" ]; then
  say "PERINGATAN: belum bisa membaca database untuk seed."
else
  say "Katalog sudah berisi $COUNT judul — seed dilewati."
fi

# --- 8. Vhost subdomain (opsional, hanya menambah — tidak mengubah situs lain)
if [ -n "$DOMAIN_API" ]; then
  if command -v nginx >/dev/null 2>&1 && [ -d /etc/nginx/sites-available ]; then
    if grep -rqs "server_name $DOMAIN_API" /etc/nginx/sites-enabled/; then
      say "Vhost $DOMAIN_API sudah ada — dilewati."
    else
      cat > /etc/nginx/sites-available/subsenpai-api <<EOF
server {
    listen 80;
    server_name $DOMAIN_API;
    location / {
        proxy_pass http://127.0.0.1:$API_PORT;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 300s;
    }
}
EOF
      ln -sf /etc/nginx/sites-available/subsenpai-api /etc/nginx/sites-enabled/subsenpai-api
      nginx -t && systemctl reload nginx
      say "Vhost nginx untuk $DOMAIN_API ditambahkan & nginx dimuat ulang."
    fi
  else
    say "DOMAIN_API diisi tapi nginx tidak terdeteksi — vhost dilewati (atur manual: proxy ke 127.0.0.1:$API_PORT)."
  fi
fi

# --- 9. Cek kesehatan -------------------------------------------------------
OK=0
for _ in $(seq 1 15); do
  if curl -fsS "http://127.0.0.1:$API_PORT/healthz" >/dev/null 2>&1; then OK=1; break; fi
  sleep 2
done
echo "================ LAPORAN ================"
systemctl is-active subsenpai-api subsenpai-worker postgresql || true
ss -tlnp 2>/dev/null | grep -E ':(5432|18080) ' || true
if [ "$OK" = "1" ]; then
  say "API HIDUP: http://127.0.0.1:$API_PORT/healthz menjawab OK."
else
  say "API BELUM MENJAWAB — 20 baris jurnal terakhir:"
  journalctl -u subsenpai-api -n 20 --no-pager || true
fi
echo "Selesai. Tempel seluruh keluaran ini kembali ke Ember."
