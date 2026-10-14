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
install -m 755 radius-vpn-watchdog   /usr/local/sbin/radius-vpn-watchdog
install -m 644 radius-appliance-firstboot.service \
  /etc/systemd/system/radius-appliance-firstboot.service
install -m 644 radius-vpn-watchdog.service /etc/systemd/system/radius-vpn-watchdog.service
install -m 644 radius-vpn-watchdog.timer   /etc/systemd/system/radius-vpn-watchdog.timer

# STACK_DIR disuntik ke unit kalau bukan nilai bawaan, supaya skripnya tidak
# perlu menebak lokasi stack.
if [ "$STACK_DIR" != "/opt/freeradius-api" ]; then
  mkdir -p /etc/systemd/system/radius-appliance-firstboot.service.d
  cat > /etc/systemd/system/radius-appliance-firstboot.service.d/stack-dir.conf <<EOF
[Service]
Environment=STACK_DIR=${STACK_DIR}
EOF
fi

# ---- Konsol serial: supaya mitra bisa MENEMPEL, bukan mengetik ------------
#
# Baris enroll panjangnya ~180 karakter, 112 di antaranya dua nilai hex. Di
# konsol noVNC Proxmox, "tempel" tidak ada artinya: VNC mengirim SCANCODE —
# seolah mitra menekan tombol satu per satu — jadi panel clipboard-nya men-set
# clipboard remote yang tak dibaca siapa pun di guest headless. Mitra menempel,
# TIDAK TERJADI APA-APA, tak ada galat, dan ia menyimpulkan appliance hang.
#
# xterm.js Proxmox bicara lewat `serial0 socket` dan mengirim BYTE. Seluruh
# kelas galat hilang sekaligus: layout keyboard mitra jadi tidak relevan,
# capslock/numlock tak bisa desinkron, dan karakter tak hilang karena ketik
# cepat. Ctrl+Shift+V bekerja.
#
# Sisi guest-nya disiapkan DI SINI, sekali, di golden image. Sisi hypervisor
# (perangkat Serial Port) tidak bisa: impor OVA Proxmox tidak memetakan
# ResourceType 21, jadi mitra tetap menambahnya lewat UI — 5 klik, VM mati.
#
# URUTAN `console=` PENTING. Yang TERAKHIR menentukan /dev/console, jadi tty0
# ditulis lebih dulu supaya noVNC tetap menampilkan pesan boot sebagai jalan
# mundur. Appliance yang kehilangan SEMUA konsol tidak bisa ditolong dari jauh.
#
# Tidak ada `systemctl enable serial-getty@ttyS0`: systemd-getty-generator
# men-spawn-nya sendiri saat `console=ttyS0` ada DAN portnya memang ada. Enable
# eksplisit akan meninggalkan unit `failed` di VM yang tak punya serial.
GRUB_DEF=/etc/default/grub
if [ -f "$GRUB_DEF" ] && command -v update-grub >/dev/null 2>&1; then
  if grep -q 'console=ttyS0' "$GRUB_DEF"; then
    echo "konsol serial: sudah ada di $GRUB_DEF, dilewati"
  else
    cp -a "$GRUB_DEF" "$GRUB_DEF.sebelum-radius"
    sed -i -E 's|^GRUB_CMDLINE_LINUX="(.*)"$|GRUB_CMDLINE_LINUX="\1 console=tty0 console=ttyS0,115200n8"|' "$GRUB_DEF"
    # DIVERIFIKASI, bukan diasumsikan: kalau barisnya tak berbentuk seperti
    # dugaan sed, ia gagal diam dan appliance lahir tanpa konsol serial —
    # ketahuannya di tempat mitra, saat xterm.js memberi layar hitam.
    grep -q 'console=ttyS0' "$GRUB_DEF" || {
      echo "GAGAL menambahkan console=ttyS0 ke $GRUB_DEF — periksa baris" >&2
      echo "GRUB_CMDLINE_LINUX di sana, lalu tambahkan tangan:" >&2
      echo '  GRUB_CMDLINE_LINUX="console=tty0 console=ttyS0,115200n8"' >&2
      exit 1
    }
    update-grub
    echo "konsol serial dipasang — mitra bisa memakai Console > xterm.js"
  fi
else
  echo "PERINGATAN: $GRUB_DEF/update-grub tidak ada — konsol serial dilewati." >&2
  echo "Mitra hanya punya noVNC, dan di sana baris enroll TIDAK bisa ditempel." >&2
fi

# ---- Paket yang harus ikut di image ---------------------------------------
#
# Diperiksa di sini, bukan diandalkan dari ingatan: ketiadaannya tidak terasa
# di NOC sama sekali — ia muncul di tempat mitra, berbulan-bulan kemudian.
#
# qemu-guest-agent WAJIB ikut kalau mitra diminta mencentang "QEMU Guest Agent"
# di Options VM. Centang tanpa paketnya bukan sekadar tak berguna: Proxmox
# menunggu agent yang tak pernah menjawab, sehingga Shutdown dari UI menggantung
# sampai timeout dan snapshot/backup ber-fsfreeze gagal. Dengan paketnya, mitra
# dapat alamat IP VM tampil di ringkasan Proxmox — justru nilai yang ia butuhkan
# saat mendaftarkan appliance.
kurang=""
for paket in xl2tpd curl openssl qemu-guest-agent; do
  command -v "$paket" >/dev/null 2>&1 || dpkg -s "$paket" >/dev/null 2>&1 || kurang="$kurang $paket"
done
if [ -n "$kurang" ]; then
  echo "PERINGATAN: paket belum terpasang:$kurang" >&2
  echo "  apt install -y$kurang" >&2
  echo "Tanpa qemu-guest-agent, JANGAN suruh mitra mencentang QEMU Guest Agent." >&2
fi

systemctl daemon-reload
systemctl enable radius-appliance-firstboot.service
# TIMER-nya yang di-enable, bukan servicenya: meng-enable unit oneshot membuat
# systemd menjalankannya sekali saat boot lalu tak pernah lagi — persis
# kebalikan dari gunanya. Aman menyala di golden image: skripnya keluar diam
# selama xl2tpd.conf belum memuat LAC hasil radius-vpn-connect.
systemctl enable radius-vpn-watchdog.timer

cat <<EOF

Terpasang:
  /usr/local/sbin/firstboot.sh
  /usr/local/sbin/radius-vpn-connect
  /usr/local/sbin/radius-erp-enroll
  /usr/local/sbin/radius-vpn-watchdog
  radius-appliance-firstboot.service (enabled, belum dijalankan)
  radius-vpn-watchdog.timer (enabled — diam sampai appliance di-enroll)

Sisa langkah A1 sebelum sanitasi:
  1. cd ${STACK_DIR} && ./setup.sh        # bootstrap raddb dari image FreeRADIUS
  2. docker compose build                 # image api DIBANGUN SEKARANG — mitra
                                          # bisa saja tanpa egress ke registry
  3. docker compose pull db freeradius    # image dasar ikut ter-cache
  4. apt install -y xl2tpd curl openssl qemu-guest-agent
                                          # xl2tpd/curl/openssl: radius-vpn-connect
                                          # qemu-guest-agent: Proxmox mitra butuhnya

JANGAN jalankan firstboot.sh di lab: ia menandai dirinya selesai, dan tanda itu
ikut ke dalam OVA sehingga appliance mitra lahir TANPA kredensial unik.
Sanitasi (A3) harus membuang /var/lib/radius-appliance/firstboot-selesai dan
.env — keduanya ada di daftar A3.
EOF
