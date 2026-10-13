#!/usr/bin/env bash
# install-appliance.sh — DIJALANKAN NOC SEKALI di golden image (langkah A1).
#
# Memasang tiga skrip + satu unit systemd ke dalam image. Sesudah ini, mitra
# tidak menjalankan apa pun selain SATU baris radius-erp-enroll dari layar ERP.
#
# Idempoten: aman dijalankan ulang saat image diperbarui.
set -euo pipefail

cd "$(dirname "$0")"
STACK_DIR="${STACK_DIR:-/opt/freeradius-api}"

[ "$(id -u)" = "0" ] || { echo "jalankan sebagai root" >&2; exit 1; }

install -d -m 755 /var/lib/radius-appliance

# firstboot.sh di /usr/local/sbin, BUKAN dipanggil dari direktori stack: unit
# systemd menunjuk jalur absolut, dan jalur yang ikut berpindah bersama
# direktori stack akan membuat unit gagal start tanpa menyebut sebabnya.
install -m 700 firstboot.sh          /usr/local/sbin/firstboot.sh
install -m 755 radius-vpn-connect    /usr/local/sbin/radius-vpn-connect
install -m 755 radius-erp-enroll     /usr/local/sbin/radius-erp-enroll
install -m 644 radius-appliance-firstboot.service \
  /etc/systemd/system/radius-appliance-firstboot.service

# STACK_DIR disuntik ke unit kalau bukan nilai bawaan, supaya skripnya tidak
# perlu menebak lokasi stack.
if [ "$STACK_DIR" != "/opt/freeradius-api" ]; then
  mkdir -p /etc/systemd/system/radius-appliance-firstboot.service.d
  cat > /etc/systemd/system/radius-appliance-firstboot.service.d/stack-dir.conf <<EOF
[Service]
Environment=STACK_DIR=${STACK_DIR}
EOF
fi

systemctl daemon-reload
systemctl enable radius-appliance-firstboot.service

cat <<EOF

Terpasang:
  /usr/local/sbin/firstboot.sh
  /usr/local/sbin/radius-vpn-connect
  /usr/local/sbin/radius-erp-enroll
  radius-appliance-firstboot.service (enabled, belum dijalankan)

Sisa langkah A1 sebelum sanitasi:
  1. cd ${STACK_DIR} && ./setup.sh        # bootstrap raddb dari image FreeRADIUS
  2. docker compose build                 # image api DIBANGUN SEKARANG — mitra
                                          # bisa saja tanpa egress ke registry
  3. docker compose pull db freeradius    # image dasar ikut ter-cache
  4. apt install -y xl2tpd curl openssl   # radius-vpn-connect bergantung padanya

JANGAN jalankan firstboot.sh di lab: ia menandai dirinya selesai, dan tanda itu
ikut ke dalam OVA sehingga appliance mitra lahir TANPA kredensial unik.
Sanitasi (A3) harus membuang /var/lib/radius-appliance/firstboot-selesai dan
.env — keduanya ada di daftar A3.
EOF
