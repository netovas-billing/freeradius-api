#!/usr/bin/env bash
# Uji radius-docker-net tanpa Docker sungguhan: `docker` dan `systemctl`
# diganti fungsi tiruan. Jalankan: bash appliance/test-radius-docker-net.sh
#
# Yang dikunci (staging JKT-1, 21 Okt 2026 — docker0 172.17/16 bentrok pool
# NAS 172.17/17): subnet 172.16–172.31 terdeteksi; daemon.json orang lain
# TIDAK ditimpa; daemon.json kosong ditulis; yang sudah aman tak disentuh.
set -uo pipefail
cd "$(dirname "$0")"

gagal=0
cek() { if eval "$2"; then echo "ok   $1"; else echo "GAGAL $1"; gagal=1; fi; }

# shellcheck source=radius-docker-net
source ./radius-docker-net
set +e # skrip yang di-source menyalakan -e; uji butuh kode keluar main

cek "172.17.0.0/16 = ruang manajemen"   'dalam_ruang_manajemen 172.17.0.0/16'
cek "172.31.255.0/24 = ruang manajemen" 'dalam_ruang_manajemen 172.31.255.0/24'
cek "172.16.0.1 = ruang manajemen"      'dalam_ruang_manajemen 172.16.0.1'
cek "172.32.0.0/16 BUKAN"               '! dalam_ruang_manajemen 172.32.0.0/16'
cek "172.15.0.0/16 BUKAN"               '! dalam_ruang_manajemen 172.15.0.0/16'
cek "198.19.0.0/24 BUKAN"               '! dalam_ruang_manajemen 198.19.0.0/24'
cek "10.17.0.0/16 BUKAN"                '! dalam_ruang_manajemen 10.17.0.0/16'

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
DAEMON_JSON="$tmp/daemon.json"
STACK_DIR="$tmp/stack"

# docker tiruan: daftar jaringan dari variabel JARINGAN ("nama subnet;…").
docker() {
  case "$1 ${2:-}" in
    "network ls") [ -n "${JARINGAN:-}" ] && echo "id1" ;;
    "network inspect") echo "${JARINGAN:-}" | tr ';' '\n' ;;
    "compose down"|"compose up") echo "docker $*" >>"$tmp/jejak" ;;
    *) : ;;
  esac
}
systemctl() { echo "systemctl $*" >>"$tmp/jejak"; }

JARINGAN="bridge 172.17.0.0/16;freeradius-api_default 172.18.0.0/16 fd00::/64"
cek "bentrok terdeteksi (docker0 + compose)" '[ "$(subnet_bentrok | wc -l)" = 2 ]'
cek "IPv6 diabaikan" '! subnet_bentrok | grep -q fd00'

JARINGAN="bridge 198.18.0.0/24;freeradius-api_default 198.19.0.0/24"
cek "jaringan 198.18/15 aman" '[ -z "$(subnet_bentrok)" ]'

# daemon.json milik orang lain → TIDAK ditimpa, keluar galat.
echo '{ "log-driver": "journald" }' >"$DAEMON_JSON"
JARINGAN="bridge 172.17.0.0/16"
main >/dev/null 2>&1; rc=$?
cek "daemon.json asing → galat" '[ "$rc" = 1 ]'
cek "daemon.json asing TIDAK ditimpa" 'grep -q journald "$DAEMON_JSON"'

# daemon.json belum ada → ditulis, Docker di-restart.
rm -f "$DAEMON_JSON" "$tmp/jejak"
JARINGAN="bridge 172.17.0.0/16"
# sesudah restart, docker tiruan "melaporkan" rentang baru.
systemctl() { echo "systemctl $*" >>"$tmp/jejak"; JARINGAN="bridge 198.18.0.0/24"; }
main >/dev/null 2>&1; rc=$?
cek "daemon.json kosong → ditulis" 'grep -q "198.18.0.1/24" "$DAEMON_JSON" && grep -q "198.19.0.0/16" "$DAEMON_JSON"'
cek "Docker di-restart" 'grep -q "systemctl restart docker" "$tmp/jejak"'
cek "selesai sukses" '[ "$rc" = 0 ]'

# Sudah aman → tak ada restart.
rm -f "$tmp/jejak"
JARINGAN="bridge 198.18.0.0/24"
main >/dev/null 2>&1; rc=$?
cek "sudah aman → tidak restart" '[ ! -f "$tmp/jejak" ] && [ "$rc" = 0 ]'

# Stack ada → down (TANPA -v) lalu up.
mkdir -p "$STACK_DIR"; touch "$STACK_DIR/docker-compose.yml"
rm -f "$DAEMON_JSON" "$tmp/jejak"
JARINGAN="bridge 172.17.0.0/16"
main >/dev/null 2>&1
cek "stack dibuat ulang" 'grep -q "docker compose down" "$tmp/jejak" && grep -q "docker compose up -d" "$tmp/jejak"'
cek "down TANPA -v (volume DB aman)" '! grep -q "down.*-v" "$tmp/jejak"'

# Docker belum terpasang → daemon.json ditulis, TANPA restart.
rm -f "$DAEMON_JSON" "$tmp/jejak"
command() { if [ "$1" = "-v" ] && [ "$2" = "docker" ]; then return 1; fi; builtin command "$@"; }
JARINGAN=""
main >/dev/null 2>&1; rc=$?
cek "tanpa Docker → daemon.json ditulis, tanpa restart" '[ "$rc" = 0 ] && [ -f "$DAEMON_JSON" ] && [ ! -f "$tmp/jejak" ]'
unset -f command

# --periksa: bentrok → keluar 1, tanpa menulis apa pun.
rm -f "$DAEMON_JSON"
JARINGAN="bridge 172.17.0.0/16"
main --periksa >/dev/null 2>&1; rc=$?
cek "--periksa bentrok → 1" '[ "$rc" = 1 ] && [ ! -f "$DAEMON_JSON" ]'

# LAN mitra di 172.16/12 → terdeteksi; ppp/docker/br/veth diabaikan.
alamat_lan() { printf '%s\n' "ens18 192.168.10.56/24" "ppp1 172.16.0.1/32" "docker0 172.17.0.1/16" "br-x 172.18.0.1/16" "veth9 172.19.0.1/16"; }
cek "LAN 192.168 + jalur VPN/Docker → tak bentrok" '[ -z "$(lan_bentrok)" ]'
alamat_lan() { printf '%s\n' "ens18 172.16.5.20/24" "ppp1 172.16.0.1/32"; }
cek "LAN 172.16.5.0/24 → bentrok" '[ "$(lan_bentrok)" = "ens18 172.16.5.20/24" ]'
alamat_lan() { printf '%s\n' "eth0 172.31.0.10/16"; }
cek "LAN 172.31.x → bentrok" '[ -n "$(lan_bentrok)" ]'

exit $gagal
