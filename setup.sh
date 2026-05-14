#!/usr/bin/env bash
# Bootstrap: copy default raddb config dan schema SQL dari image FreeRADIUS.
# Jalankan sekali sebelum `docker compose up`.
set -euo pipefail

cd "$(dirname "$0")"

IMAGE="freeradius/freeradius-server:latest"
TMP="fr-bootstrap-$$"

# Di image FreeRADIUS, /etc/raddb adalah symlink ke /etc/freeradius.
# Path config yang sebenarnya dipakai container adalah /etc/freeradius.
SRC_CONFIG="/etc/freeradius"
SRC_SCHEMA="/etc/freeradius/mods-config/sql/main/mysql/schema.sql"

# Hapus state symlink dangling kalau ada (sisa dari run gagal sebelumnya).
if [ -L freeradius/raddb ] && [ ! -e freeradius/raddb ]; then
  echo "Menghapus symlink dangling freeradius/raddb dari run sebelumnya ..."
  rm freeradius/raddb
fi

if [ -d freeradius/raddb ] && [ -n "$(ls -A freeradius/raddb 2>/dev/null)" ]; then
  echo "freeradius/raddb sudah berisi config. Skip bootstrap raddb."
else
  echo "Pulling $IMAGE ..."
  docker pull "$IMAGE"
  docker create --name "$TMP" "$IMAGE" >/dev/null
  trap 'docker rm "$TMP" >/dev/null 2>&1 || true' EXIT
  mkdir -p freeradius
  # Pastikan tidak ada symlink sisa
  rm -f freeradius/raddb
  echo "Copying $SRC_CONFIG ke ./freeradius/raddb (follow symlinks) ..."
  # -L: follow symlink di SRC_PATH (raddb -> freeradius).
  # Pakai trailing /. supaya isi folder yang disalin, bukan foldernya.
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

# Aktifkan modul SQL (symlink mods-available/sql -> mods-enabled/sql).
if [ -d freeradius/raddb/mods-available ] && [ ! -e freeradius/raddb/mods-enabled/sql ]; then
  echo "Enable modul SQL (mods-enabled/sql -> ../mods-available/sql) ..."
  ln -sf ../mods-available/sql freeradius/raddb/mods-enabled/sql
fi

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

Langkah berikutnya:
  1. Edit freeradius/raddb/mods-available/sql:
       driver  = "rlm_sql_mysql"
       dialect = "mysql"
       server  = "db"
       port    = 3306
       login   = "radius"
       password = "<sesuai DB_PASSWORD di .env>"
       radius_db = "radius"
  2. Edit freeradius/raddb/sites-available/default — aktifkan baris 'sql'
     di section authorize {} dan accounting {}.
  3. (Opsional) freeradius/raddb/sites-available/inner-tunnel — aktifkan
     'sql' di section authorize {} untuk PEAP/TTLS.
  4. Edit freeradius/raddb/clients.conf — tambahkan client/NAS.
  5. docker compose up -d
  6. docker compose logs -f freeradius
  7. API tersedia di http://localhost:8000/docs (Swagger UI).
EOF
