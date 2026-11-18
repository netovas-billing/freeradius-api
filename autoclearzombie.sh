#!/bin/bash
# autoclearzombie.sh - Auto clear zombie sessions di radacct
#
# Zombie = session yang acctstoptime NULL dan tidak dapat Interim-Update selama
# lebih dari N menit (router mati / koneksi putus mendadak).
#
# ASAL: https://github.com/heirro/freeradius-api (Vava Heirro), dipindahkan ke
# repo ini 28 Sep 2026 karena runtime Go tidak meng-clone repo Python lagi
# sementara timer pemeliharaan tetap menunjuk skrip ini di dalam direktori
# instance. Isinya tidak bergantung Python sama sekali — murni bash + mariadb.
#
# Nilai DB_* di bawah adalah PLACEHOLDER: radius-manager-api menambalnya
# per-instance dengan mencocokkan `^KEY=` di kolom 0 (applyShellVarPatches).
# Jangan indentasi, jangan ganti namanya.
#
# Cron example (setiap 5 menit):
#   */5 * * * * /path/to/autoclearzombie.sh >> /var/log/autoclearzombie.log 2>&1

# --- Konfigurasi ---
DB_HOST="localhost"
DB_PORT="3306"
DB_USER="radius"
DB_PASS="radius_password"
DB_NAME="radius"

# Session dianggap zombie jika tidak dapat update lebih dari N menit.
# Harus > 2x Interim-Update router (toleransi 1 miss).
# Contoh: Interim-Update 5 menit (bawaan skrip NAS ERP) → threshold 10-15 menit.
# Router ber-interim 10 menit butuh >= 25: di stack Docker atur
# ZOMBIE_THRESHOLD_MINUTES=25 di .env (dibaca di bawah).
ZOMBIE_THRESHOLD_MINUTES=10

# --- Stack Docker Compose (freeradius-api manual / appliance) ---
#
# Instance freeradius-manager memakai MariaDB di host + nilai DB_* di atas yang
# ditambal per-instance. Stack Docker TIDAK: MariaDB ada di container
# `radius-db`, host tak punya klien `mariadb`, dan kredensialnya di `.env`
# sebelah skrip ini. Sampai 18 Nov 2026 skrip ini gagal di sana dengan
# "mariadb: command not found" (server LintasHome PALI) — sesi zombie tak
# pernah dibersihkan, padahal Simultaneous-Use=1 menolak login pelanggan yang
# sesi lamanya masih tercatat terbuka.
#
# Mode Docker dipakai bila: ada `.env` di direktori skrip DAN container
# radius-db berjalan DAN host tak punya klien mariadb. Hanya baris ^KEY=VALUE
# yang dibaca (bukan `source .env`), sama seperti setup.sh.
DIR_SKRIP="$(cd "$(dirname "$0")" && pwd)"
PAKAI_DOCKER=""
if ! command -v mariadb >/dev/null 2>&1 && [ -f "$DIR_SKRIP/.env" ] \
   && command -v docker >/dev/null 2>&1 \
   && [ "$(docker inspect -f '{{.State.Running}}' radius-db 2>/dev/null)" = "true" ]; then
    PAKAI_DOCKER=1
    while IFS= read -r baris; do
        case "$baris" in
            DB_USER=*) [ -n "${baris#*=}" ] && DB_USER="${baris#*=}" ;;
            DB_PASSWORD=*) [ -n "${baris#*=}" ] && DB_PASS="${baris#*=}" ;;
            DB_NAME=*) [ -n "${baris#*=}" ] && DB_NAME="${baris#*=}" ;;
            ZOMBIE_THRESHOLD_MINUTES=*)
                case "${baris#*=}" in ''|*[!0-9]*) ;; *) ZOMBIE_THRESHOLD_MINUTES="${baris#*=}" ;; esac ;;
        esac
    done < "$DIR_SKRIP/.env"
fi

# Sandi lewat MYSQL_PWD, bukan -p<sandi>: argumen proses terbaca `ps` semua user.
jalankan_sql() {
    if [ -n "$PAKAI_DOCKER" ]; then
        docker exec -i -e MYSQL_PWD="$DB_PASS" radius-db \
            mariadb -u "$DB_USER" "$DB_NAME" -N -e "$1"
    else
        MYSQL_PWD="$DB_PASS" mariadb -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" "$DB_NAME" --skip-ssl -N -e "$1"
    fi
}

# --- Eksekusi ---
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# Exit status DITANGKAP terpisah dari keluarannya.
#
# Versi asli menyimpan stderr ke $RESULT lalu mengambil `tail -1` sebagai jumlah
# baris terpengaruh, TANPA memeriksa exit status. Kalau mariadb gagal (kredensial
# salah, DB mati, tabel terkunci), yang tercetak adalah baris terakhir PESAN
# GALAT di tempat angka — jadi "Cleared ERROR 1045 (28000): Access denied zombie
# session(s)" terbaca seperti laporan sukses oleh siapa pun yang memindai log,
# dan sesi zombie menumpuk tanpa ada yang tahu.
if ! RESULT=$(jalankan_sql "
    UPDATE radacct
    SET
        acctstoptime       = NOW(),
        acctterminatecause = 'Admin-Reset',
        acctsessiontime    = TIMESTAMPDIFF(SECOND, acctstarttime, NOW())
    WHERE acctstoptime IS NULL
      AND COALESCE(acctupdatetime, acctstarttime) < DATE_SUB(NOW(), INTERVAL $ZOMBIE_THRESHOLD_MINUTES MINUTE);
    SELECT ROW_COUNT();
" 2>&1); then
    echo "[$TIMESTAMP] ERROR: gagal membersihkan sesi zombie: $RESULT" >&2
    exit 1
fi

AFFECTED=$(echo "$RESULT" | tail -1)

# Jaring aman: kalau yang kembali bukan angka, jangan cetak seperti laporan sukses.
if ! [[ "$AFFECTED" =~ ^[0-9]+$ ]]; then
    echo "[$TIMESTAMP] ERROR: keluaran tak terduga dari mariadb: $RESULT" >&2
    exit 1
fi

echo "[$TIMESTAMP] Cleared $AFFECTED zombie session(s) (threshold: ${ZOMBIE_THRESHOLD_MINUTES}m)"
