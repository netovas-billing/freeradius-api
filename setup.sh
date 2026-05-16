#!/usr/bin/env bash
# Bootstrap: copy default raddb config dan schema SQL dari image FreeRADIUS,
# lalu apply overlay (dynamic_clients, SQL→MySQL, perms) supaya stack langsung
# jalan tanpa edit manual. Idempotent — aman dijalankan ulang.
set -euo pipefail

cd "$(dirname "$0")"

IMAGE="freeradius/freeradius-server:latest"
TMP="fr-bootstrap-$$"

SRC_CONFIG="/etc/freeradius"
SRC_SCHEMA="/etc/freeradius/mods-config/sql/main/mysql/schema.sql"

# DB credential default (sinkron dengan docker-compose.yml).
# Kalau ubah DB_PASSWORD di .env, jalankan setup.sh ulang dengan flag --force
# atau edit manual baris `password = ...` di mods-available/sql.
DB_PASSWORD_DEFAULT="radiuspass_ganti"

# ---- Cleanup symlink dangling dari run gagal ----
if [ -L freeradius/raddb ] && [ ! -e freeradius/raddb ]; then
  echo "Menghapus symlink dangling freeradius/raddb ..."
  rm freeradius/raddb
fi

# ---- 1. Copy raddb config + schema dari image ----
if [ -d freeradius/raddb ] && [ -n "$(ls -A freeradius/raddb 2>/dev/null)" ]; then
  echo "freeradius/raddb sudah berisi config. Skip bootstrap raddb."
else
  echo "Pulling $IMAGE ..."
  docker pull "$IMAGE"
  docker create --name "$TMP" "$IMAGE" >/dev/null
  trap 'docker rm "$TMP" >/dev/null 2>&1 || true' EXIT
  mkdir -p freeradius
  rm -f freeradius/raddb
  echo "Copying $SRC_CONFIG ke ./freeradius/raddb (follow symlinks) ..."
  mkdir -p freeradius/raddb
  docker cp -L -a "$TMP":"$SRC_CONFIG"/. ./freeradius/raddb/

  if [ ! -f initdb/01-schema.sql ]; then
    echo "Copying MariaDB schema ke ./initdb/01-schema.sql ..."
    mkdir -p initdb
    docker cp -L "$TMP":"$SRC_SCHEMA" ./initdb/01-schema.sql
  fi

  if [ ! -f initdb/03-ippool-schema.sql ]; then
    echo "Copying ippool schema ke ./initdb/03-ippool-schema.sql ..."
    docker cp -L "$TMP":/etc/freeradius/mods-config/sql/ippool/mysql/schema.sql ./initdb/03-ippool-schema.sql 2>/dev/null || true
  fi
fi

# ---- 2. Fix perms: freerad user di container UID != owner host file ----
echo "Setting world-read permissions di freeradius/raddb/ ..."
chmod -R go+rX freeradius/raddb/

# ---- 3. Enable modul SQL ----
if [ -d freeradius/raddb/mods-available ] && [ ! -e freeradius/raddb/mods-enabled/sql ]; then
  echo "Enable mods-enabled/sql ..."
  ln -sf ../mods-available/sql freeradius/raddb/mods-enabled/sql
fi

# ---- 4. Patch mods-available/sql ke MySQL ----
SQL_CONF="freeradius/raddb/mods-available/sql"
if [ -f "$SQL_CONF" ]; then
  echo "Patch $SQL_CONF: driver/dialect/server/login/password ..."
  # dialect dan driver
  sed -i 's|^	dialect = "sqlite"|	dialect = "mysql"|' "$SQL_CONF"
  sed -i 's|^	driver = "rlm_sql_null"|	driver = "rlm_sql_${dialect}"|' "$SQL_CONF"
  # Uncomment & set server/port/login/password (asumsi baris 167-170 di template)
  sed -i \
    -e 's|^#\?\s*server\s*=\s*"localhost"|	server = "db"|' \
    -e 's|^#\?\s*port\s*=\s*3306|	port = 3306|' \
    -e 's|^#\?\s*login\s*=\s*"radius"|	login = "radius"|' \
    -e 's|^#\?\s*password\s*=\s*"radpass"|	password = "'"$DB_PASSWORD_DEFAULT"'"|' \
    "$SQL_CONF"

  # Comment-out TLS block di dalam mysql {...} — koneksi internal Docker
  if grep -q '^		tls {$' "$SQL_CONF"; then
    echo "Comment-out TLS block di mysql {} ..."
    # Pakai awk: di antara `mysql {` dan close-brace pertama, comment tls block
    awk '
      /^	mysql \{/ { in_mysql=1 }
      in_mysql && /^		tls \{/ { in_tls=1 }
      in_tls { print "#" $0; if (/^		\}/) in_tls=0; next }
      in_mysql && /^	\}/ { in_mysql=0 }
      { print }
    ' "$SQL_CONF" > "$SQL_CONF.tmp" && mv "$SQL_CONF.tmp" "$SQL_CONF"
  fi
fi

# ---- 5. Apply overlay: dynamic_clients site + clients.conf ----
OVERLAY_SITE="freeradius/overlay/sites-available/dynamic-clients"
DEST_SITE="freeradius/raddb/sites-available/dynamic-clients"
if [ -f "$OVERLAY_SITE" ]; then
  echo "Apply overlay: sites-available/dynamic-clients ..."
  cp "$OVERLAY_SITE" "$DEST_SITE"
fi

if [ ! -e freeradius/raddb/sites-enabled/dynamic-clients ]; then
  echo "Enable sites-enabled/dynamic-clients ..."
  ln -sf ../sites-available/dynamic-clients freeradius/raddb/sites-enabled/dynamic-clients
fi

OVERLAY_CLIENTS="freeradius/overlay/clients.conf.append"
CLIENTS="freeradius/raddb/clients.conf"
if [ -f "$OVERLAY_CLIENTS" ] && [ -f "$CLIENTS" ]; then
  if ! grep -q '^client dynamic {' "$CLIENTS"; then
    echo "Append dynamic client block ke clients.conf ..."
    cat "$OVERLAY_CLIENTS" >> "$CLIENTS"
  fi
fi

# ---- 6. Generate .env kalau belum ada ----
if [ ! -f .env ]; then
  echo "Creating .env dari .env.example ..."
  cp .env.example .env
  if command -v openssl >/dev/null; then
    KEY="$(openssl rand -hex 32)"
    sed -i.bak "s|^API_KEY=.*|API_KEY=${KEY}|" .env && rm -f .env.bak
    echo "Generated random API_KEY di .env"
  fi
fi

cat <<'EOF'

Bootstrap selesai.

Yang otomatis diaplikasikan:
  - raddb/ + initdb/ schema dari image FreeRADIUS
  - chmod go+rX di raddb/ (freerad UID di container bisa baca)
  - mods-enabled/sql symlink, dialect=mysql, server=db, login=radius
  - TLS block ke DB di-comment (koneksi internal Docker)
  - sites-enabled/dynamic-clients aktif (overlay dari freeradius/overlay/)
  - clients.conf: tambah blok `client dynamic` (0.0.0.0/0 — narrow di prod!)
  - .env dengan API_KEY random 32-byte

Sebelum jalan:
  - Periksa freeradius/raddb/clients.conf — ganti subnet `client dynamic`
    dari 0.0.0.0/0 ke range mitra Anda (mis. VPN tunnel) untuk production.
  - Kalau ubah DB_PASSWORD di .env, edit juga password di
    freeradius/raddb/mods-available/sql.

Lanjut:
  docker compose up -d
  docker compose logs -f freeradius
  # API: http://localhost:8000/docs/index.html
EOF
