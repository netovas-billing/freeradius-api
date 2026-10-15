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

# ---- 5. Sandi root unik per appliance --------------------------------------
#
# Sampai 14 Okt 2026 tidak ada apa pun di repo ini yang menetapkan sandi root,
# dan tidak ada satu pun dokumen yang menyebutkannya. Akibatnya dua-duanya
# buruk sekaligus: mitra membuka konsol, melihat prompt `login:`, dan BERHENTI
# di situ — sementara sandi yang sebenarnya dipakai adalah apa pun yang
# diketik orang yang memasang Debian di golden image, yaitu sandi yang SAMA
# untuk setiap mitra yang pernah menerima OVA ini.
#
# Alfabetnya sengaja tanpa karakter yang kembar di layar konsol: tidak ada
# 0/O, 1/I/L, U/V. Mitra membacanya dari layar lalu mengetiknya — di situ
# "sandi yang kuat" yang salah baca satu huruf sama saja dengan pintu terkunci.
# 10 karakter dari 30 lambang ≈ 49 bit; itu jauh lebih dari cukup untuk sesuatu
# yang hanya bisa dicoba lewat konsol hypervisor mitra sendiri.
#
# Dicetak di /etc/issue, yaitu layar yang sudah tampil SEBELUM login. Yang bisa
# melihatnya adalah orang yang sudah memegang konsol Proxmox mitra — dan orang
# itu toh bisa memasang disk VM ini dan membaca seluruh isinya tanpa sandi apa
# pun. Jadi yang dilindungi di sini bukan rahasianya dari pemilik hypervisor,
# melainkan dari SESAMA MITRA: tanpa ini semua appliance memakai sandi yang
# sama, dan itulah yang tidak punya gejala sampai ada yang menyadarinya.
SANDI_ROOT=""
for _ in 1 2 3 4 5; do
  SANDI_ROOT="$(openssl rand -base64 64 | tr -dc '23456789ABCDEFGHJKMNPQRSTWXYZ' | cut -c1-10)"
  [ "${#SANDI_ROOT}" -eq 10 ] && break
done
[ "${#SANDI_ROOT}" -eq 10 ] || {
  echo "GAGAL membangkitkan sandi root — jangan lanjut, mitra tak akan bisa masuk" >&2
  exit 1
}
HASH_LAMA="$(awk -F: '$1=="root"{print $2}' /etc/shadow)"
echo "root:${SANDI_ROOT}" | chpasswd
# DIVERIFIKASI: chpasswd bisa gagal karena aturan PAM (mis. pwquality) dan
# exit code-nya tidak selalu menular lewat pipe. Kalau hash-nya tidak berubah,
# yang berlaku adalah sandi lama dari golden image — dan layar konsol akan
# menampilkan sandi yang tidak bisa dipakai, bentuk kegagalan paling
# membingungkan yang bisa diterima mitra.
[ "$(awk -F: '$1=="root"{print $2}' /etc/shadow)" != "$HASH_LAMA" ] || {
  echo "GAGAL: sandi root TIDAK berubah — yang berlaku masih sandi golden image." >&2
  echo "Periksa PAM/pwquality di image; jangan kirim OVA ini ke mitra." >&2
  exit 1
}

# Umur sandi DIMATIKAN, dan ini bukan kelonggaran — ini syarat supaya appliance
# bisa di-troubleshoot. Kalau sandi root kedaluwarsa (warisan `chage`/
# `passwd --expire` di golden image, atau PASS_MAX_DAYS di /etc/login.defs),
# login pertama mitra dipaksa mengganti sandi — dan sejak detik itu sandi yang
# dipegang NOC tidak berlaku lagi, sementara nilai penggantinya tidak ada di
# mana pun. Appliance jadi kotak hitam justru pada saat ia bermasalah.
#
# Akses tetap NOC TIDAK bergantung pada baris ini: itu kunci SSH di
# /root/.ssh/authorized_keys (A3), yang selamat walau mitra mengganti sandinya
# sendiri. Yang dijaga di sini hanya satu hal — jangan pernah ada paksaan ganti
# sandi yang tak diminta siapa pun.
chage -M -1 -I -1 -E -1 -d "$(date +%Y-%m-%d)" root || {
  echo "PERINGATAN: gagal mematikan umur sandi root — periksa chage/login.defs" >&2
}

# ---- 6. Ringkasan di layar konsol ------------------------------------------
#
# Ditulis ke /etc/issue supaya terbaca LANGSUNG di layar konsol Proxmox,
# sebelum login. Mode 600: agetty membacanya sebagai root, jadi tak ada yang
# hilang, sementara pengguna lain di dalam VM tidak ikut membaca sandinya.
install -d -m 755 /var/lib/radius-appliance
cat > /etc/issue <<EOF

  Appliance RADIUS — Netovas
  --------------------------
  Stack sudah berjalan. Appliance ini BELUM tersambung ke ERP.

  MASUK DI SINI:   login: root     sandi: ${SANDI_ROOT}
  (sandi ini juga dipegang NOC — beri tahu NOC lebih dulu kalau diganti,
   kalau tidak kami tak bisa menolong appliance ini dari jarak jauh)

  Lalu daftarkan appliance ini di ERP (Infrastruktur > RADIUS > Tambah),
  salin satu baris perintah yang muncul di layar ERP, dan tempel di sini
  lewat Console > xterm.js. Tidak ada nilai lain yang perlu dibaca dari
  layar ini.

EOF
chmod 600 /etc/issue
date -Is > "$TANDA"
echo "firstboot selesai."
