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
for paket in xl2tpd curl openssl qemu-guest-agent sudo; do
  command -v "$paket" >/dev/null 2>&1 || dpkg -s "$paket" >/dev/null 2>&1 || kurang="$kurang $paket"
done
if [ -n "$kurang" ]; then
  echo "PERINGATAN: paket belum terpasang:$kurang" >&2
  echo "  apt install -y$kurang" >&2
  echo "Tanpa qemu-guest-agent, JANGAN suruh mitra mencentang QEMU Guest Agent." >&2
fi

# ---- Jangan pernah ada paksaan ganti sandi root ---------------------------
#
# Berlaku untuk DUA orang, bukan satu. NOC login ke VM golden image ini untuk
# membangun dan mengujinya; mitra login ke appliance hasilnya. Keduanya harus
# langsung mendapat prompt perintah, bukan permintaan sandi baru.
#
# Kalau sandi root kedaluwarsa, PAM memaksa gantinya di login pertama — dan
# bagi mitra itu berarti sandi yang dipegang NOC mati sejak detik itu,
# sementara nilai penggantinya tidak tercatat di mana pun. Appliance jadi kotak
# hitam justru pada saat ia bermasalah dan perlu ditelusuri. Gejalanya pun
# bukan galat: login berhasil, diminta sandi baru, selesai.
#
# DUA jalur ditutup di sini, karena keduanya menghasilkan paksaan yang sama
# tapi lewat kolom /etc/shadow yang berbeda:
#
#   1. `chage`/`passwd --expire` pada akun root → kolom lastchg. Ini yang
#      tersembuhkan `chpasswd` di firstboot, jadi mitra tak terkena — tapi NOC
#      di VM ini terkena, dan itu yang terjadi selama ini.
#   2. PASS_MAX_DAYS di /etc/login.defs → kolom max. Ini TIDAK tersembuhkan
#      `chpasswd`, dan ia mewarisi ke setiap akun yang sandinya ditetapkan
#      sesudahnya. Appliance mitra akan jalan normal berbulan-bulan lalu
#      tiba-tiba memaksa ganti sandi, tanpa sebab yang kelihatan.
#
# firstboot.sh mengulang `chage` untuk akun root sesudah menanam sandi unik.
# Yang di sini mengurus imagenya sendiri — dan bawaan distronya.
chage -M -1 -I -1 -E -1 -d "$(date +%Y-%m-%d)" root || \
  echo "PERINGATAN: gagal mematikan umur sandi root di image ini" >&2

if [ -f /etc/login.defs ] && grep -qE '^[[:space:]]*PASS_MAX_DAYS[[:space:]]+[0-9]+' /etc/login.defs; then
  sed -i -E 's|^([[:space:]]*PASS_MAX_DAYS[[:space:]]+).*|\1-1|' /etc/login.defs
  echo "PASS_MAX_DAYS di /etc/login.defs disetel -1 (tanpa kedaluwarsa)."
fi

# ---- SSH: root boleh masuk dengan sandi ------------------------------------
#
# Keputusan pemilik 15 Okt 2026: "dengan ssh harusnya masih bisa dengan
# password walaupun login sebagai root". Bawaan Debian `prohibit-password`
# menolak sandi root lewat SSH, jadi ia ditimpa di sini.
#
# YANG PERLU DIKETAHUI ORANG BERIKUTNYA. Sandi root SAMA di semua appliance
# (keputusan 15 Okt juga), dan semua appliance duduk di SATU ruang alamat
# tunnel yang sama. Dengan `PermitRootLogin yes`, mitra mana pun yang membaca
# sandi di appliance-nya sendiri — yang memang hak dia — bisa SSH sebagai root
# ke appliance mitra LAIN. Dua pengaman yang dulu menahan ini sudah dilepas
# berurutan: sandi unik per appliance (dicabut) dan prohibit-password (ini).
#
# Karena itu ada NOC_SSH_FROM. Isi dengan alamat/CIDR tempat NOC masuk, dan
# sandi hanya berlaku dari sana — appliance tetangga di ruang tunnel yang sama
# tidak bisa memakainya, sementara NOC tetap bisa masuk dengan sandi seperti
# yang diminta. Dikosongkan = terbuka dari mana saja, dan itu dicetak sebagai
# peringatan, bukan didiamkan.
#
#   NOC_SSH_FROM="172.16.0.0/12" ./install-appliance.sh
#
# Ditulis ke sshd_config.d/, bukan menambal sshd_config: baris yang ditambal
# bisa kalah oleh nilai lain di berkas utama, dan urutan "yang pertama menang"
# di sshd membuat kegagalannya senyap.
SSHD_DROPIN="/etc/ssh/sshd_config.d/10-netovas-appliance.conf"
install -d -m 755 /etc/ssh/sshd_config.d
if ! grep -qE '^[[:space:]]*Include[[:space:]]+/etc/ssh/sshd_config\.d/\*\.conf' /etc/ssh/sshd_config; then
  echo "PERINGATAN: /etc/ssh/sshd_config tidak meng-Include sshd_config.d/." >&2
  echo "  Setelan SSH appliance TIDAK akan berlaku. Tambahkan barisnya di paling atas:" >&2
  echo "  Include /etc/ssh/sshd_config.d/*.conf" >&2
fi

if [ -n "${NOC_SSH_FROM:-}" ]; then
  cat > "$SSHD_DROPIN" <<EOF
# Dikelola install-appliance.sh — jangan disunting tangan.
# Sandi hanya berlaku dari alamat NOC; dari mana pun selain itu wajib kunci.
PermitRootLogin prohibit-password
PasswordAuthentication no
Match Address ${NOC_SSH_FROM}
    PermitRootLogin yes
    PasswordAuthentication yes
EOF
  echo "SSH: root boleh sandi HANYA dari ${NOC_SSH_FROM}; selain itu wajib kunci."
else
  cat > "$SSHD_DROPIN" <<'EOF'
# Dikelola install-appliance.sh — jangan disunting tangan.
PermitRootLogin yes
PasswordAuthentication yes
EOF
  echo "PERINGATAN: NOC_SSH_FROM kosong — root boleh masuk dengan sandi dari MANA SAJA," >&2
  echo "  termasuk dari appliance mitra lain di ruang alamat tunnel yang sama, dan sandi" >&2
  echo "  itu sama di semua appliance. Setel NOC_SSH_FROM untuk mempersempitnya." >&2
fi
chmod 644 "$SSHD_DROPIN"

# WAJIB diuji: sshd yang konfigurasinya tak sah TIDAK start di boot berikutnya,
# dan appliance itu lahir tanpa SSH sama sekali — di tempat mitra.
sshd -t || {
  echo "GAGAL: konfigurasi sshd tidak sah sesudah menulis $SSHD_DROPIN" >&2
  rm -f "$SSHD_DROPIN"
  exit 1
}
systemctl reload ssh 2>/dev/null || systemctl restart ssh 2>/dev/null || true
echo "Berlaku: $(sshd -T 2>/dev/null | grep -iE '^(permitrootlogin|passwordauthentication)' | tr '\n' ' ')"

# ---- Akun yang dipakai mitra untuk masuk -----------------------------------
#
# Mitra boleh masuk sebagai root, boleh juga sebagai pengguna biasa yang
# ber-sudo. Yang menentukan adalah NOC saat memasang Debian di image ini —
# bukan skrip ini, karena sandinya pun ditetapkan di sana.
#
# Yang DIPERIKSA di sini cuma satu hal: kalau NOC memilih jalur pengguna biasa,
# pengguna itu harus benar-benar bisa sudo. Baris enroll dari ERP berawalan
# `sudo`, jadi pengguna yang tidak ber-sudo akan menempel baris yang benar lalu
# ditolak — dan penolakannya terjadi di tempat mitra, bukan di lab.
# AKUN_MITRA=<user> membuat langkah `usermod -aG sudo` tidak perlu diketik
# tangan. Sengaja OPT-IN, bukan otomatis untuk setiap pengguna yang kebetulan
# ada: menaikkan hak akun tanpa diminta adalah hal yang tidak boleh terjadi
# diam-diam, sekalipun di appliance satu-guna.
#
#   AKUN_MITRA=opsnoc ./install-appliance.sh
if [ -n "${AKUN_MITRA:-}" ]; then
  id "$AKUN_MITRA" >/dev/null 2>&1 || {
    echo "GAGAL: pengguna '$AKUN_MITRA' tidak ada di image ini." >&2
    echo "  Buat dulu: adduser $AKUN_MITRA" >&2
    exit 1
  }
  usermod -aG sudo "$AKUN_MITRA"
  # DIVERIFIKASI, bukan diandaikan: usermod bisa sukses sementara grup sudo
  # tak ada sama sekali (paket sudo belum terpasang), dan appliance lahir
  # dengan akun yang tak bisa apa-apa.
  id -nG "$AKUN_MITRA" | tr ' ' '\n' | grep -qx sudo || {
    echo "GAGAL: '$AKUN_MITRA' tetap bukan anggota grup sudo sesudah usermod." >&2
    echo "  Paket sudo sudah terpasang? apt install -y sudo" >&2
    exit 1
  }
  echo "Akun login mitra: $AKUN_MITRA — ditambahkan ke grup sudo."
fi

manusia="$(awk -F: '$3>=1000 && $3<65534 && $7 !~ /(nologin|false)$/ {print $1}' /etc/passwd)"
if [ -n "$manusia" ]; then
  for u in $manusia; do
    if id -nG "$u" 2>/dev/null | tr ' ' '\n' | grep -qx sudo; then
      echo "Akun login mitra: $u (anggota grup sudo) — siap."
    else
      echo "PERINGATAN: pengguna '$u' ADA tapi tidak di grup sudo." >&2
      echo "  Kalau dia yang akan dipakai mitra: AKUN_MITRA=$u ./install-appliance.sh" >&2
      echo "  (atau usermod -aG sudo $u). Kalau mitra memakai root saja, abaikan." >&2
    fi
  done
else
  echo "Tidak ada pengguna biasa — mitra akan masuk sebagai root."
fi

# Akun root TERKUNCI = seluruh keputusan "root boleh SSH dengan sandi" batal
# tanpa gejala. Debian installer mengunci root (sandi `!` di /etc/shadow)
# kalau sandi root dikosongkan saat memasang — dan di jalur itu ia justru
# memasang sudo serta memasukkan pengguna pertama ke grup sudo sendiri. Jadi
# dua perilaku yang berlawanan, dan yang membedakan cuma satu isian di layar
# installer yang diisi berminggu-minggu sebelumnya.
case "$(passwd -S root 2>/dev/null | awk '{print $2}')" in
  P) : ;;
  L|NP|"")
    echo "PERINGATAN: akun root TERKUNCI atau tanpa sandi di image ini." >&2
    echo "  root tidak akan bisa masuk — lewat konsol maupun SSH — walau" >&2
    echo "  PermitRootLogin yes. Kalau mitra memakai root: passwd root" >&2
    echo "  Kalau mitra memakai pengguna ber-sudo, ini memang disengaja." >&2
    ;;
esac

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

Umur sandi root dimatikan di image ini: NOC maupun mitra tidak akan pernah
diminta mengganti sandi saat login pertama. JANGAN tambahkan
`passwd --expire root` di sanitasi (A3) — itu menghidupkannya lagi, dan sandi
yang dipegang NOC mati pada login pertama mitra.

SSH: root boleh masuk dengan sandi (${SSHD_DROPIN}). Kalau NOC_SSH_FROM tidak
diisi, sandi yang SAMA di semua appliance itu berlaku dari mana saja di dalam
tunnel — termasuk dari appliance mitra lain. Isi NOC_SSH_FROM untuk
mempersempitnya ke alamat NOC saja.

Jalur pengguna biasa? Grup sudo TIDAK otomatis kalau sandi root diisi saat
memasang Debian. Jalankan ulang dengan AKUN_MITRA=<user> (ia memverifikasi
hasilnya), atau usermod -aG sudo <user> sebelum sanitasi A3.

Sisa langkah A1 sebelum sanitasi:
  1. cd ${STACK_DIR} && ./setup.sh        # bootstrap raddb dari image FreeRADIUS
  2. docker compose build                 # image api DIBANGUN SEKARANG — mitra
                                          # bisa saja tanpa egress ke registry
  3. docker compose pull db freeradius    # image dasar ikut ter-cache
  4. apt install -y xl2tpd curl openssl qemu-guest-agent sudo
                                          # xl2tpd/curl/openssl: radius-vpn-connect
                                          # qemu-guest-agent: Proxmox mitra butuhnya
                                          # sudo: baris enroll dari ERP berawalan sudo

JANGAN jalankan firstboot.sh di lab: ia menandai dirinya selesai, dan tanda itu
ikut ke dalam OVA sehingga appliance mitra lahir TANPA kredensial unik.
Sanitasi (A3) harus membuang /var/lib/radius-appliance/firstboot-selesai dan
.env — keduanya ada di daftar A3.
EOF
