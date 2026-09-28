#!/bin/bash
#
# autobackups3.sh - dump basis data RADIUS lalu unggah ke object storage.
#
# ASAL: https://github.com/heirro/freeradius-api (Vava Heirro), dipindahkan ke
# repo ini 28 Sep 2026 karena runtime Go tidak meng-clone repo Python lagi
# sementara timer pemeliharaan tetap menunjuk skrip ini di dalam direktori
# instance. Isinya tidak bergantung Python sama sekali — murni bash.
#
# Nilai konfigurasi di bawah adalah PLACEHOLDER: radius-manager-api menambalnya
# per-instance dengan mencocokkan `^KEY=` di kolom 0 (applyShellVarPatches).
# Jangan indentasi, jangan ganti namanya.

# pipefail WAJIB, dan ini bukan kerapian.
#
# Versi asli menjalankan `mariadb-dump ... | gzip > berkas` lalu memeriksa `$?`.
# Pada pipeline, `$?` adalah exit status perintah TERAKHIR — yaitu gzip. Kalau
# mariadb-dump gagal (kredensial salah, DB mati, tabel terkunci), gzip tetap
# sukses menulis berkas ~20 byte yang berisi NOL byte SQL; pemeriksaan `-f`
# lolos, berkas itu terunggah, salinan lokal DIHAPUS, dan skripnya mencetak
# "Backup completed successfully".
#
# Akibatnya backup yang tampak ada tapi kosong — dan itu baru diketahui pada saat
# paling buruk, yaitu ketika backup-nya dibutuhkan. Diuji 28 Sep 2026: `false |
# gzip > f` menghasilkan berkas 20 byte, 0 byte terdekompresi, dan pola aslinya
# menyatakannya BERHASIL.
set -o pipefail

# Configuration
REMOTE="s3"
BUCKET="backup-db"
BACKUP_PATH="radiusdb/"
BACKUP_DATE=$(date +%Y%m%d)
FILENAME="${BACKUP_DATE}.sql.gz"
TEMP_DIR="/tmp/mariadb-backup"
LOCAL_FILE="${TEMP_DIR}/${FILENAME}"

# Database credentials
DB_HOST="localhost"
DB_PORT="3306"
DB_USER="raduser"
DB_PASS="radpass"
DB_NAME="radiusdb"

# Ukuran minimum SQL terdekompresi yang dianggap masuk akal. Dump radius yang
# sah selalu jauh di atas ini (skema saja sudah puluhan KB); nilai kecil hanya
# untuk menangkap dump KOSONG, bukan untuk menilai kelengkapan.
MIN_SQL_BYTES=1024

# Create temp directory
mkdir -p "${TEMP_DIR}"

echo "Starting backup at $(date)"
mariadb-dump --skip-ssl \
  -h "${DB_HOST}" \
  --port "${DB_PORT}" \
  -u "${DB_USER}" \
  -p"${DB_PASS}" \
  "${DB_NAME}" \
  --single-transaction \
  --quick \
  --lock-tables=false | gzip > "${LOCAL_FILE}"
DUMP_STATUS=$?

if [ ${DUMP_STATUS} -ne 0 ] || [ ! -s "${LOCAL_FILE}" ]; then
    echo "ERROR: Database dump failed (status=${DUMP_STATUS})!" >&2
    rm -f "${LOCAL_FILE}"
    exit 1
fi

# Dump bisa "sukses" tapi kosong (mis. hak akses hilang di tengah jalan), jadi
# isinya diperiksa, bukan cuma status keluarnya.
SQL_BYTES=$(gzip -dc "${LOCAL_FILE}" | wc -c)
if [ "${SQL_BYTES}" -lt "${MIN_SQL_BYTES}" ]; then
    echo "ERROR: dump hanya ${SQL_BYTES} byte SQL (minimum ${MIN_SQL_BYTES}) — menolak mengunggah backup kosong!" >&2
    rm -f "${LOCAL_FILE}"
    exit 1
fi

echo "Dump completed: ${LOCAL_FILE} ($(du -h "${LOCAL_FILE}" | cut -f1), ${SQL_BYTES} byte SQL)"

# Step 2: Upload
echo "Uploading to ${REMOTE}..."
if rclone copy "${LOCAL_FILE}" "${REMOTE}:${BUCKET}/${BACKUP_PATH}/" --checksum; then
    echo "Upload successful: ${BUCKET}/${BACKUP_PATH}/${FILENAME}"

    # Step 3: Remove local file after successful upload
    rm -f "${LOCAL_FILE}"
    echo "Local temporary file removed: ${LOCAL_FILE}"
    rmdir "${TEMP_DIR}" 2>/dev/null

    echo "Backup completed successfully at $(date)"
else
    echo "ERROR: Upload failed! Local file preserved at: ${LOCAL_FILE}" >&2
    exit 1
fi
