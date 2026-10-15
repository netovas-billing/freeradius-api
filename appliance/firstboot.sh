#!/usr/bin/env bash
# firstboot.sh — dijalankan SEKALI saat appliance mitra pertama menyala.
#
# Dipasang NOC ke dalam golden image (lihat install-appliance.sh), dipanggil
# oleh radius-appliance-firstboot.service. Mitra TIDAK menjalankannya.
#
# Tugasnya satu: membuat appliance ini BERBEDA dari semua appliance lain yang
# lahir dari image yang sama. Tanpa berkas ini setiap mitra memegang sandi DB,
# API key, dan host key SSH yang identik — dan yang paling berbahaya dari itu
# tidak punya gejala apa pun: semuanya menyala normal, dan baru terasa saat
# satu mitra bisa membaca RADIUS mitra lain.
set -euo pipefail

STACK_DIR="${STACK_DIR:-/opt/freeradius-api}"
TANDA="/var/lib/radius-appliance/firstboot-selesai"

if [ -e "$TANDA" ]; then
  echo "firstboot sudah pernah jalan ($TANDA) — tidak diulang."
  exit 0
fi

cd "$STACK_DIR"

acak() { openssl rand -hex "${1:-16}"; }

# ---- 1. Identitas host SSH -------------------------------------------------
#
# WAJIB. Host key dibuang saat sanitasi image, dan Debian polos TIDAK
# meregenerasinya sendiri (itu perilaku image cloud). Tanpa baris ini sshd
# gagal start dan mitra tak bisa masuk sama sekali.
if ! ls /etc/ssh/ssh_host_*_key >/dev/null 2>&1; then
  echo "membuat host key SSH ..."
  ssh-keygen -A
  systemctl restart ssh 2>/dev/null || systemctl restart sshd 2>/dev/null || true
fi

# ---- 2. Kredensial unik per mitra ------------------------------------------
#
# .env DIBUAT ULANG, bukan ditambal: yang ikut di image adalah .env lab (atau
# tidak ada sama sekali sesudah sanitasi), dan menambal berkas yang isinya tak
# pasti berarti menebak.
#
# BASIC_AUTH_* SENGAJA DIKOSONGKAN di sini dan diisi radius-erp-enroll nanti.
# Itu bukan kelalaian: ERP yang membangkitkan kredensial REST (seperti ia sudah
# membangkitkan sandi PPP), supaya operator tidak perlu MEMBACA apa pun dari
# konsol Proxmox lalu mengetiknya ulang ke form. Satu perpindahan manual, satu
# arah, dan appliance yang belum di-enroll menolak SEMUA panggilan API —
# API_KEY kosong mematikan X-API-Key (config.muatAPIKey), BASIC_AUTH_* kosong
# mematikan Basic (middleware.APIKey). Gagal-tertutup, bukan jendela terbuka.
if [ ! -f .env ] || ! grep -q '^DB_PASSWORD=' .env || grep -q '^DB_PASSWORD=radiuspass_ganti$' .env; then
  echo "membangkitkan kredensial unik ..."
  DB_ROOT_PASSWORD="$(acak 24)"
  DB_PASSWORD="$(acak 24)"
  cat > .env <<EOF
# Dibangkitkan firstboot.sh — JANGAN disalin ke appliance lain.
DB_ROOT_PASSWORD=${DB_ROOT_PASSWORD}
DB_NAME=radius
DB_USER=radius
DB_PASSWORD=${DB_PASSWORD}

# Diisi radius-erp-enroll saat appliance didaftarkan di ERP.
API_KEY=
BASIC_AUTH_USER=
BASIC_AUTH_PASSWORD=

# Port terbit API. Ditulis 8000 EKSPLISIT, tidak dikosongkan: compose membaca
# `${API_PORT:-8000}` dan nilai kosong berarti port host ACAK, bukan bawaan.
# radius-erp-enroll menggantinya dengan port dst-nat milik baris ERP.
API_PORT=8000

FR_TAG=3.2.10

RATE_LIMIT_MAX=120
RATE_LIMIT_WINDOW_SECONDS=60
WEBHOOK_POLL_SECONDS=5
EOF
  chmod 600 .env
fi

# shellcheck disable=SC1091
DB_PASSWORD="$(grep '^DB_PASSWORD=' .env | cut -d= -f2-)"

# ---- 3. Tanam sandi DB ke raddb — HARUS DI SINI ----------------------------
#
# setup.sh dijalankan NOC saat membuat image (A1), jauh sebelum sandi unik
# instance ini lahir di langkah 2. Jadi raddb yang ikut di dalam OVA memegang
# sandi image, bukan sandi appliance ini — dan FreeRADIUS gagal konek DB tanpa
# satu pun pesan yang menunjuk sebabnya. Karena itu penanamannya diulang DI
# SINI, sesudah sandinya dibangkitkan.
SQL_CONF="freeradius/raddb/mods-available/sql"
if [ -f "$SQL_CONF" ]; then
  echo "menanam sandi DB ke $SQL_CONF ..."
  # Mencocokkan NILAI APA PUN, bukan hanya nilai template — itu yang membuat
  # langkah ini idempoten. `^#?` ikut WAJIB: di template FreeRADIUS barisnya
  # BERKOMENTAR dengan `#` di kolom 0 sebelum tab (`#\tpassword = "radpass"`),
  # jadi pola tanpa `#?` tidak mencocokkan apa pun di raddb yang belum pernah
  # ditambal. `certificate_password` tidak ikut tercocok karena `password`
  # harus berada langsung sesudah spasi awal.
  sed -i -E 's|^#?([[:space:]]*)password[[:space:]]*=[[:space:]]*".*"|\1password = "'"${DB_PASSWORD}"'"|' "$SQL_CONF"
  # DIVERIFIKASI, tidak diasumsikan. Kegagalan sed di sini adalah kegagalan
  # paling mahal di seluruh berkas ini — ia tidak menghasilkan galat, hanya
  # FreeRADIUS yang tak bisa konek DB berjam-jam kemudian.
  grep -q "password = \"${DB_PASSWORD}\"" "$SQL_CONF" || {
    echo "GAGAL menanam sandi DB ke $SQL_CONF — jangan lanjut, auth akan mati senyap" >&2
    exit 1
  }
else
  echo "PERINGATAN: $SQL_CONF tidak ada — raddb belum di-bootstrap NOC (setup.sh di A1)" >&2
fi

# ---- 4. Jalankan stack -----------------------------------------------------
#
# Image HARUS sudah ada di dalam image OVA (NOC menjalankan `docker compose
# build` + `pull` di A1). Appliance mitra bisa saja berada di jaringan dengan
# egress tersaring — terbukti 28 Sep 2026 saat `apk add` gagal ke
# dl-cdn.alpinelinux.org meski Docker Hub jalan. Jadi diperiksa dulu dan
# dilaporkan terang-terangan, bukan dibiarkan `up` mencoba menarik dan
# menggantung tanpa penjelasan.
for img in freeradius-api-local:1; do
  docker image inspect "$img" >/dev/null 2>&1 || {
    echo "GAGAL: image $img tidak ada di appliance ini." >&2
    echo "NOC: jalankan 'docker compose build' di golden image (langkah A1)." >&2
    exit 1
  }
done

echo "menjalankan stack ..."
docker compose up -d

# ---- 5. Umur sandi root ----------------------------------------------------
#
# Sandi root TIDAK diganti di sini, dan itu keputusan pemilik (15 Okt 2026):
# "jangan ada perubahan password otomatis di firstboot, tidak apa apa semua
# mitra nantinya sama semua passwordnya". Yang berlaku adalah sandi yang
# ditetapkan NOC saat memasang Debian di golden image, dan NOC mengirimkannya
# ke mitra bersama berkas OVA.
#
# Versi sebelumnya (b374b3f) membangkitkan sandi acak 10 karakter di sini.
# Itu dicabut karena harganya dibayar di tempat yang salah: appliance menyala,
# mitra ATAU NOC mencoba masuk dengan sandi yang mereka tahu — dan ditolak,
# tanpa petunjuk apa pun, sampai firstboot selesai dan sandi barunya muncul di
# layar. Menit-menit pertama appliance baru justru menit paling sering dipakai
# untuk memeriksanya.
#
# Yang hilang karena ini: sandi root sama di semua appliance. Yang menahan
# akibatnya: sshd Debian berjalan `PermitRootLogin prohibit-password`, jadi
# sandi itu TIDAK bisa dipakai dari jauh — termasuk oleh sesama mitra lewat
# ruang alamat tunnel yang sama. Ia hanya berlaku di konsol hypervisor mitra
# sendiri, tempat yang toh sudah bisa memasang disk VM ini dan membacanya
# tanpa sandi apa pun. Akses NOC dari jauh berjalan lewat kunci SSH, bukan
# sandi ini. Kalau kelak `PermitRootLogin` dilonggarkan jadi `yes`, keputusan
# ini HARUS ditinjau ulang — di situlah sandi bersama berubah jadi jalan masuk
# lintas-mitra.
#
# Yang tetap dikerjakan: mematikan umur sandi, supaya tidak ada yang pernah
# dipaksa mengganti sandi di login pertama. Sandi yang kedaluwarsa memaksa
# mitra membuat sandi baru, dan sejak detik itu sandi yang dipegang NOC mati
# sementara penggantinya tak tercatat di mana pun — appliance jadi kotak hitam
# justru saat ia bermasalah. install-appliance.sh sudah melakukannya di golden
# image; diulang di sini karena imagenya bisa saja dibangun sebelum baris itu
# ada.
chage -M -1 -I -1 -E -1 -d "$(date +%Y-%m-%d)" root || {
  echo "PERINGATAN: gagal mematikan umur sandi root — periksa chage/login.defs" >&2
}

# ---- 6. Ringkasan di layar konsol ------------------------------------------
#
# Ditulis ke /etc/issue supaya terbaca LANGSUNG di layar konsol Proxmox,
# sebelum login. Tidak ada rahasia di dalamnya (sandi tidak dibangkitkan —
# langkah 5), jadi modenya 644 seperti berkas ini pada umumnya.
#
# Gunanya tetap penting: tanpa ini mitra melihat prompt `login:` polos dan
# berhenti di situ, tanpa tahu appliance-nya sudah siap atau belum.
#
# AKUNNYA DIBACA, BUKAN DITEBAK. NOC boleh memilih mitra masuk sebagai root,
# boleh juga sebagai pengguna biasa yang ber-sudo, dan pilihan itu dibuat saat
# memasang Debian — jauh dari berkas ini. Layar yang menyebut akun yang salah
# lebih buruk daripada layar yang tidak menyebut apa-apa: mitra mengetik nama
# yang tidak ada, ditolak, dan menyimpulkan appliance-nya rusak.
#
# Yang dicari: pengguna ber-UID >= 1000 dengan shell sungguhan yang anggota
# grup sudo. Kalau ada lebih dari satu, yang pertama dipakai — dan kalau tidak
# ada sama sekali, root.
AKUN="root"
for u in $(awk -F: '$3>=1000 && $3<65534 && $7 !~ /(nologin|false)$/ {print $1}' /etc/passwd); do
  if id -nG "$u" 2>/dev/null | tr ' ' '\n' | grep -qx sudo; then AKUN="$u"; break; fi
done

install -d -m 755 /var/lib/radius-appliance
cat > /etc/issue <<EOF

  Appliance RADIUS — Netovas
  --------------------------
  Stack sudah berjalan. Appliance ini BELUM tersambung ke ERP.

  MASUK DI SINI:   login: ${AKUN}
  Sandinya ada di lembar kredensial yang dikirim NOC bersama berkas OVA —
  sama untuk setiap appliance, dan Anda TIDAK akan diminta menggantinya.
  Beri tahu NOC lebih dulu kalau tetap diganti, kalau tidak kami tak bisa
  menolong appliance ini dari jarak jauh.

  Lalu daftarkan appliance ini di ERP (Infrastruktur > RADIUS > Tambah),
  salin satu baris perintah yang muncul di layar ERP, dan tempel di sini
  lewat Console > xterm.js. Barisnya sudah berawalan \`sudo\` — tempel apa
  adanya, jangan ditambah apa pun. Tidak ada nilai lain yang perlu dibaca
  dari layar ini.

EOF
chmod 644 /etc/issue
date -Is > "$TANDA"
echo "firstboot selesai."
