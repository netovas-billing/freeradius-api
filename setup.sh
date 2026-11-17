#!/usr/bin/env bash
# Bootstrap: copy default raddb config dan schema SQL dari image FreeRADIUS,
# lalu apply overlay (dynamic_clients, SQL→MySQL, perms) supaya stack langsung
# jalan tanpa edit manual. Idempotent — aman dijalankan ulang.
set -euo pipefail

cd "$(dirname "$0")"

# Tag dibaca dari .env (FR_TAG) dengan bawaan yang SAMA seperti
# docker-compose.yml. raddb/ disalin dari image ini, jadi kalau dua tempat
# menyebut versi berbeda, server berjalan dengan config yang bukan miliknya.
TMP="fr-bootstrap-$$"

SRC_CONFIG="/etc/freeradius"
SRC_SCHEMA="/etc/freeradius/mods-config/sql/main/mysql/schema.sql"

# ---- 0. Generate .env kalau belum ada ----------------------------------------
#
# DI AWAL, bukan di akhir: RADIUS_HEALTH_SECRET dibangkitkan KE .env lalu
# ditanam ke raddb di langkah 5 — .env harus sudah ada sebelum itu. Dulu blok
# ini langkah terakhir; nilainya sama dengan bawaan di bawah, jadi urutan baru
# tidak mengubah apa yang ditanam.
acakHex() {
  if command -v openssl >/dev/null; then
    openssl rand -hex 32
  else
    od -An -N32 -tx1 /dev/urandom | tr -d ' \n'
  fi
}
if [ ! -f .env ]; then
  echo "Creating .env dari .env.example ..."
  cp .env.example .env
  KEY="$(acakHex)"
  sed -i.bak "s|^API_KEY=.*|API_KEY=${KEY}|" .env && rm -f .env.bak
  echo "Generated random API_KEY di .env"
fi

# ---- 0b. Muat .env -----------------------------------------------------------
#
# Sampai 13 Okt 2026 berkas ini TIDAK PERNAH membaca .env: sandi DB ditulis ke
# raddb dari konstanta, dan sed-nya hanya mencocokkan baris template asli
# (`password = "radpass"`). Akibatnya begitu DB_PASSWORD di .env diganti —
# dan di appliance mitra ia WAJIB diganti, kalau tidak semua mitra kembar —
# raddb tetap memegang sandi lama dan FreeRADIUS gagal konek DB. Auth mati,
# tanpa satu pun pesan yang menunjuk sebabnya.
#
# Komentar lama di sini menyuruh menjalankan ulang dengan `--force`. Skrip ini
# TIDAK punya parsing argumen sama sekali, jadi flag itu diabaikan diam-diam —
# instruksi yang salah, di dalam repo sendiri.
#
# Hanya baris ber-^KEY=VALUE yang diambil: `set -a; . .env` membajak variabel
# lain di skrip ini (pernah terjadi di radius-manager: PORT_REGISTRY dan
# DB_HOST ikut tertimpa).
muatEnv() {
  [ -f .env ] || return 0
  local baris kunci nilai
  while IFS= read -r baris; do
    case "$baris" in
      FR_TAG=*|DB_NAME=*|DB_USER=*|DB_PASSWORD=*|RADIUS_HEALTH_SECRET=*)
        kunci="${baris%%=*}"; nilai="${baris#*=}"
        # `if`, bukan `[ … ] && …`: di bawah `set -e` bentuk kedua membuat
        # skrip KELUAR begitu ada baris berkunci cocok tapi nilainya kosong
        # (mis. `DB_PASSWORD=` di .env yang belum di-enroll) — dan keluarnya
        # tanpa pesan, dengan status 1, di tengah bootstrap.
        if [ -n "$nilai" ]; then
          eval "$kunci=\$nilai"
        fi
        ;;
    esac
  done < .env
}
muatEnv

FR_TAG="${FR_TAG:-3.2.10}"
IMAGE="freeradius/freeradius-server:${FR_TAG}"
DB_PASSWORD="${DB_PASSWORD:-radiuspass_ganti}"
# User DB SAMA dengan yang dibaca docker-compose.yml (MARIADB_USER & api:
# ${DB_USER:-radius}). Sampai 17 Nov 2026 login di raddb DITULIS MATI "radius":
# server yang .env-nya DB_USER=radiuspali (Pali) membuat MariaDB & api memakai
# radiuspali, FreeRADIUS tetap login "radius" → "Access denied", container
# restart terus dan `docker compose logs` KOSONG (galat masuk berkas log).
DB_USER="${DB_USER:-radius}"
# Nilai ini masuk ke pola sed dan konfigurasi FreeRADIUS — tolak karakter yang
# bisa memecah keduanya daripada menanam login yang rusak diam-diam.
case "$DB_USER" in
  *[!A-Za-z0-9_.-]*)
    echo "DB_USER di .env hanya boleh huruf, angka, titik, garis bawah, strip: '$DB_USER'" >&2
    exit 1 ;;
esac
# Nama database — SAMA dengan MARIADB_DATABASE & api (${DB_NAME:-radius}).
# Sampai 17 Nov 2026 radius_db di raddb tetap bawaan template ("radius"): Pali
# ber-DB_NAME=radiuspali → MariaDB membuat radiuspali, FreeRADIUS meminta
# "radius" → "Access denied ... to database 'radius'", restart terus, log
# compose kosong. Titik tidak diizinkan (bukan nama database MariaDB yang sah).
DB_NAME="${DB_NAME:-radius}"
case "$DB_NAME" in
  *[!A-Za-z0-9_-]*)
    echo "DB_NAME di .env hanya boleh huruf, angka, garis bawah, strip: '$DB_NAME'" >&2
    exit 1 ;;
esac

# Secret pemeriksa kesehatan FreeRADIUS (GET /api/v1/radius/health). Kosong =
# dibangkitkan dan DITULIS ke .env — api membacanya dari sana lewat compose,
# raddb mendapatkannya di langkah 5. Keduanya HARUS sama; kalau tidak, ERP
# membaca FreeRADIUS sebagai mati dan menolak memindahkan router.
if [ -z "${RADIUS_HEALTH_SECRET:-}" ]; then
  RADIUS_HEALTH_SECRET="$(acakHex)"
  if grep -q '^RADIUS_HEALTH_SECRET=' .env; then
    sed -i "s|^RADIUS_HEALTH_SECRET=.*|RADIUS_HEALTH_SECRET=${RADIUS_HEALTH_SECRET}|" .env
  else
    printf '\n# Dibangkitkan setup.sh — secret pemeriksa kesehatan FreeRADIUS (lihat .env.example).\nRADIUS_HEALTH_SECRET=%s\n' \
      "$RADIUS_HEALTH_SECRET" >> .env
  fi
  grep -qx "RADIUS_HEALTH_SECRET=${RADIUS_HEALTH_SECRET}" .env || {
    echo "GAGAL menulis RADIUS_HEALTH_SECRET ke .env — berhenti" >&2
    exit 1
  }
  echo "Generated random RADIUS_HEALTH_SECRET di .env"
fi
# Masuk ke pola sed dan ke tanda kutip konfigurasi FreeRADIUS.
case "$RADIUS_HEALTH_SECRET" in
  *[!A-Za-z0-9_.-]*)
    echo "RADIUS_HEALTH_SECRET di .env hanya boleh huruf, angka, titik, garis bawah, strip" >&2
    exit 1 ;;
esac
if [ "${#RADIUS_HEALTH_SECRET}" -lt 16 ]; then
  echo "RADIUS_HEALTH_SECRET di .env terlalu pendek (minimal 16 karakter) — kosongkan supaya dibangkitkan" >&2
  exit 1
fi

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
  echo "Patch $SQL_CONF: driver/dialect/server/login/password/radius_db ..."
  # dialect dan driver
  sed -i 's|^	dialect = "sqlite"|	dialect = "mysql"|' "$SQL_CONF"
  sed -i 's|^	driver = "rlm_sql_null"|	driver = "rlm_sql_${dialect}"|' "$SQL_CONF"
  # Uncomment & set server/port/login/password (asumsi baris 167-170 di template)
  sed -i \
    -e 's|^#\?\s*server\s*=\s*"localhost"|	server = "db"|' \
    -e 's|^#\?\s*port\s*=\s*3306|	port = 3306|' \
    "$SQL_CONF"

  # Login: dari DB_USER, cocokkan NILAI APA PUN (idempoten & ikut .env yang
  # diganti) — pola yang sama dengan sandi di bawah.
  sed -i -E 's|^#?([[:space:]]*)login[[:space:]]*=[[:space:]]*".*"|\1login = "'"$DB_USER"'"|' \
    "$SQL_CONF"
  grep -q "login = \"$DB_USER\"" "$SQL_CONF" || {
    echo "GAGAL menanam user DB ($DB_USER) ke $SQL_CONF — berhenti, auth akan mati senyap" >&2
    exit 1
  }

  # Sandi: cocokkan NILAI APA PUN, bukan hanya nilai template. Itu yang
  # membuat langkah ini idempoten DAN benar saat .env sudah diganti.
  #
  # `^#?` WAJIB: di template 3.2.10 barisnya BERKOMENTAR dengan `#` di kolom 0
  # sebelum tab — `#\tpassword = "radpass"`. Tiga sed di atas (server/port/
  # login) ikut membuang `#`-nya; versi pertama sed ini tidak, jadi ia tidak
  # mencocokkan apa pun dan bootstrap berhenti di penjaga di bawah.
  # `certificate_password` tidak ikut tercocok karena `password` harus berada
  # langsung sesudah spasi awal.
  sed -i -E 's|^#?([[:space:]]*)password[[:space:]]*=[[:space:]]*".*"|\1password = "'"$DB_PASSWORD"'"|' \
    "$SQL_CONF"

  # DIVERIFIKASI, tidak diasumsikan. Kegagalan sed di sini adalah kegagalan
  # paling mahal di seluruh berkas: ia tidak menghasilkan galat, hanya
  # FreeRADIUS yang tak bisa konek DB berjam-jam kemudian.
  grep -q "password = \"$DB_PASSWORD\"" "$SQL_CONF" || {
    echo "GAGAL menanam sandi DB ke $SQL_CONF — berhenti, auth akan mati senyap" >&2
    exit 1
  }

  # Nama database: hanya baris AKTIF (tanpa `#`). BEDA dengan login/sandi:
  # template memuat tiga contoh radius_db BERKOMENTAR (Oracle, PostgreSQL) —
  # `^#?` akan menghidupkan ketiganya. Baris aktifnya `\tradius_db = "radius"`.
  sed -i -E 's|^([[:space:]]*)radius_db[[:space:]]*=[[:space:]]*".*"|\1radius_db = "'"$DB_NAME"'"|' \
    "$SQL_CONF"
  # Tepat SATU baris aktif dan nilainya DB_NAME.
  if [ "$(grep -cE '^[[:space:]]*radius_db[[:space:]]*=' "$SQL_CONF")" != 1 ] ||
     ! grep -qE '^[[:space:]]*radius_db = "'"$DB_NAME"'"$' "$SQL_CONF"; then
    echo "GAGAL menanam nama database ($DB_NAME) ke $SQL_CONF — berhenti, FreeRADIUS akan ditolak MariaDB" >&2
    exit 1
  fi

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

# ---- 5b. Sasaran pemeriksa kesehatan (GET /api/v1/radius/health) ----
#
# Socket INTERNAL 18190 dengan client-nya sendiri — alasannya di kepala berkas
# overlay. DISALIN ULANG setiap kali (idempoten, ikut secret .env yang diganti),
# secret ditanam lalu DIVERIFIKASI seperti sandi DB: secret yang tak cocok
# membuat ERP membaca FreeRADIUS sebagai mati dan menolak memindahkan router.
OVERLAY_HEALTH="freeradius/overlay/sites-available/netovas-health"
DEST_HEALTH="freeradius/raddb/sites-available/netovas-health"
if [ ! -f "$OVERLAY_HEALTH" ]; then
  echo "GAGAL: $OVERLAY_HEALTH tidak ada — repo tidak utuh" >&2
  exit 1
fi
echo "Apply overlay: sites-available/netovas-health (secret dari .env) ..."
cp "$OVERLAY_HEALTH" "$DEST_HEALTH"
sed -i "s|__RADIUS_HEALTH_SECRET__|${RADIUS_HEALTH_SECRET}|" "$DEST_HEALTH"
if grep -q '__RADIUS_HEALTH_SECRET__' "$DEST_HEALTH" ||
   ! grep -qF "secret = \"${RADIUS_HEALTH_SECRET}\"" "$DEST_HEALTH"; then
  echo "GAGAL menanam RADIUS_HEALTH_SECRET ke $DEST_HEALTH — berhenti" >&2
  exit 1
fi
chmod go+r "$DEST_HEALTH"
if [ ! -e freeradius/raddb/sites-enabled/netovas-health ]; then
  echo "Enable sites-enabled/netovas-health ..."
  ln -sf ../sites-available/netovas-health freeradius/raddb/sites-enabled/netovas-health
fi

cat <<'EOF'

Bootstrap selesai.

Yang otomatis diaplikasikan:
  - raddb/ + initdb/ schema dari image FreeRADIUS
  - chmod go+rX di raddb/ (freerad UID di container bisa baca)
  - mods-enabled/sql symlink, dialect=mysql, server=db
  - TLS block ke DB di-comment (koneksi internal Docker)
  - sites-enabled/dynamic-clients aktif (overlay dari freeradius/overlay/)
  - clients.conf: tambah blok `client dynamic` (0.0.0.0/0 — narrow di prod!)
  - user (DB_USER), sandi (DB_PASSWORD) & nama database (DB_NAME) dari .env
    DITANAM ke mods-available/sql, lalu DIVERIFIKASI
  - sites-enabled/netovas-health: socket internal 18190 untuk pemeriksa
    kesehatan, secret RADIUS_HEALTH_SECRET dari .env (dibangkitkan bila kosong)
  - .env dengan API_KEY random 32-byte (bila .env belum ada)

Sebelum jalan:
  - Periksa freeradius/raddb/clients.conf — ganti subnet `client dynamic`
    dari 0.0.0.0/0 ke range mitra Anda (mis. VPN tunnel) untuk production.
  - Ubah DB_NAME/DB_USER/DB_PASSWORD di .env lalu jalankan ULANG berkas ini; ketiganya
    ditanam ke raddb dari .env, bukan dari konstanta, dan hasilnya diperiksa.
    (Sampai 13 Okt 2026 itu TIDAK terjadi: raddb memegang sandi lama dan
    FreeRADIUS gagal konek DB tanpa pesan yang menunjuk sebabnya.)

Lanjut:
  docker compose up -d
  # Server LAMA yang baru diperbarui: raddb berubah, FreeRADIUS harus restart
  docker compose restart freeradius
  # WAJIB sebelum server ini dipakai — FreeRADIUS-nya sendiri, bukan hanya API:
  curl -s -H "X-API-Key: <API_KEY dari .env>" http://localhost:8000/api/v1/radius/health
  #   {"ok":true,"jawaban":"Access-Reject",...}  → siap
  #   {"ok":false,...}                           → baca "galat", lihat README "Galat umum"
  # API: http://localhost:8000/docs/index.html

Membuat APPLIANCE untuk mitra (bukan instance NOC)? Lihat appliance/README.md —
jangan jalankan firstboot.sh di lab, tandanya ikut ke dalam OVA.
EOF
