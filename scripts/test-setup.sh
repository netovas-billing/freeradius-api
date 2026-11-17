#!/usr/bin/env bash
# Uji penanaman raddb oleh setup.sh dan appliance/firstboot.sh — tanpa Docker
# sungguhan dan tanpa root. Jalankan: bash scripts/test-setup.sh
#
# Yang dikunci (insiden pindah RADIUS LintasHome PALI, 17 Nov 2026):
#   - login/sandi/radius_db DITANAM dari .env (DB_USER/DB_PASSWORD/DB_NAME),
#     bukan konstanta "radius": .env DB_NAME=radiuspali + raddb radius_db
#     "radius" membuat FreeRADIUS "Access denied ... to database 'radius'"
#     dan restart terus dengan `docker compose logs` kosong;
#   - contoh radius_db BERKOMENTAR (Oracle/PostgreSQL) TIDAK ikut dihidupkan;
#   - RADIUS_HEALTH_SECRET dibangkitkan ke .env bila kosong, ditanam ke
#     sites-available/netovas-health, dan TIDAK berubah di jalankan ulang
#     (api & raddb harus memegang secret yang sama);
#   - nilai berbahaya ditolak, bukan ditanam rusak diam-diam.
#
# raddb tiruan: cuplikan mods-available/sql FreeRADIUS 3.2.10 yang memuat
# semua baris yang disentuh sed (bentuk & indentasi asli, termasuk `#\t`).
set -uo pipefail
AKAR="$(cd "$(dirname "$0")/.." && pwd)"

gagal=0
jumlah=0
cek() {
  jumlah=$((jumlah + 1))
  if eval "$2"; then echo "ok   $1"; else echo "GAGAL $1"; gagal=1; fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Perintah tiruan di PATH: docker/systemctl/chage/ssh-keygen tak boleh
# menyentuh mesin penguji. docker dicatat; `image inspect` sukses.
BIN="$tmp/bin"
mkdir -p "$BIN"
cat >"$BIN/docker" <<'EOF'
#!/usr/bin/env bash
echo "docker $*" >>"${JEJAK:-/dev/null}"
exit 0
EOF
for c in systemctl chage ssh-keygen; do
  printf '#!/usr/bin/env bash\necho "%s $*" >>"${JEJAK:-/dev/null}"\nexit 0\n' "$c" >"$BIN/$c"
done
chmod +x "$BIN"/*
export PATH="$BIN:$PATH"

templatSQL() {
  cat <<'EOF'
sql {
	dialect = "sqlite"
	driver = "rlm_sql_null"
	sqlite {
		filename = "/tmp/freeradius.db"
	}
	mysql {
		tls {
			ca_file = "/etc/ssl/certs/my_ca.crt"
			certificate_password = "rahasia-tls"
		}
		warnings = auto
	}
	# Connection info:
	#
#	server = "localhost"
#	port = 3306
#	login = "radius"
#	password = "radpass"

	# Database table configuration for everything except Oracle
	radius_db = "radius"

	# If you are using Oracle then use this instead
#	radius_db = "(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=localhost)(PORT=1521))(CONNECT_DATA=(SID=your_sid)))"

	# If you're using postgresql this can also be used instead of the connection info parameters
#	radius_db = "dbname=radius host=localhost user=radius password=raddpass"

	# Postgreql doesn't take tls{} options in its module config like mysql does
#	radius_db = "host=localhost port=5432 dbname=radius user=radius password=raddpass sslmode=verify-full"
}
EOF
}

# siapkanStack DIR — salinan repo minimal + raddb tiruan (tidak kosong →
# setup.sh melewati bootstrap dari image).
siapkanStack() {
  local d="$1"
  mkdir -p "$d/freeradius/raddb/mods-available" "$d/freeradius/raddb/mods-enabled" \
    "$d/freeradius/raddb/sites-available" "$d/freeradius/raddb/sites-enabled"
  cp "$AKAR/setup.sh" "$AKAR/.env.example" "$d/"
  cp -r "$AKAR/freeradius/overlay" "$d/freeradius/overlay"
  templatSQL >"$d/freeradius/raddb/mods-available/sql"
  printf 'client localhost {\n\tipaddr = 127.0.0.1\n\tsecret = testing123\n}\n' >"$d/freeradius/raddb/clients.conf"
}

nilaiEnv() { grep "^$2=" "$1/.env" | tail -1 | cut -d= -f2-; }
SQL() { echo "$1/freeradius/raddb/mods-available/sql"; }
HEALTH() { echo "$1/freeradius/raddb/sites-available/netovas-health"; }
aktifRadiusDB() { grep -cE '^[[:space:]]*radius_db[[:space:]]*=' "$(SQL "$1")"; }

# ─── setup.sh ────────────────────────────────────────────────────────────────

echo "== setup.sh: .env belum ada (bawaan .env.example)"
S1="$tmp/s1"; siapkanStack "$S1"
( cd "$S1" && bash ./setup.sh >"$tmp/s1.log" 2>&1 ); kode=$?
cek "setup.sh sukses" '[ $kode = 0 ]'
cek ".env dibuat" '[ -f "$S1/.env" ]'
cek "API_KEY acak (bukan placeholder)" '[ "$(nilaiEnv "$S1" API_KEY | tr -d "\n" | wc -c)" = 64 ]'
cek "login bawaan radius ditanam & aktif" 'grep -qP "^\tlogin = \"radius\"$" "$(SQL "$S1")"'
cek "radius_db bawaan radius" 'grep -qP "^\tradius_db = \"radius\"$" "$(SQL "$S1")"'
cek "tepat satu radius_db aktif" '[ "$(aktifRadiusDB "$S1")" = 1 ]'
cek "contoh radius_db Oracle tetap berkomentar" 'grep -q "^#	radius_db = \"(DESCRIPTION" "$(SQL "$S1")"'
cek "certificate_password tidak tersentuh" 'grep -q "certificate_password = \"rahasia-tls\"" "$(SQL "$S1")"'
SEC1="$(nilaiEnv "$S1" RADIUS_HEALTH_SECRET)"
cek "RADIUS_HEALTH_SECRET dibangkitkan ke .env (>=32 karakter)" '[ "${#SEC1}" -ge 32 ]'
cek "secret yang sama ditanam ke netovas-health" 'grep -qF "secret = \"$SEC1\"" "$(HEALTH "$S1")"'
cek "penanda __RADIUS_HEALTH_SECRET__ tak tersisa" '! grep -q __RADIUS_HEALTH_SECRET__ "$(HEALTH "$S1")"'
cek "sites-enabled/netovas-health aktif" '[ -L "$S1/freeradius/raddb/sites-enabled/netovas-health" ]'
cek "client dynamic ditambahkan sekali" '[ "$(grep -c "^client dynamic {" "$S1/freeradius/raddb/clients.conf")" = 1 ]'

echo "== setup.sh: dijalankan ulang (idempoten)"
( cd "$S1" && bash ./setup.sh >"$tmp/s1b.log" 2>&1 ); kode=$?
cek "jalankan ulang sukses" '[ $kode = 0 ]'
cek "secret tidak berubah" '[ "$(nilaiEnv "$S1" RADIUS_HEALTH_SECRET)" = "$SEC1" ]'
cek "satu baris RADIUS_HEALTH_SECRET di .env" '[ "$(grep -c "^RADIUS_HEALTH_SECRET=" "$S1/.env")" = 1 ]'
cek "netovas-health tetap memegang secret .env" 'grep -qF "secret = \"$SEC1\"" "$(HEALTH "$S1")"'
cek "client dynamic tetap satu" '[ "$(grep -c "^client dynamic {" "$S1/freeradius/raddb/clients.conf")" = 1 ]'
cek "tetap satu radius_db aktif" '[ "$(aktifRadiusDB "$S1")" = 1 ]'

echo "== setup.sh: kasus Pali (DB_NAME/DB_USER radiuspali) di raddb yang sudah ditambal"
sed -i -e 's/^DB_NAME=.*/DB_NAME=radiuspali/' -e 's/^DB_USER=.*/DB_USER=radiuspali/' -e 's/^DB_PASSWORD=.*/DB_PASSWORD=Sandi_Pali-1/' "$S1/.env"
grep -q '^DB_NAME=' "$S1/.env" || echo 'DB_NAME=radiuspali' >>"$S1/.env"
grep -q '^DB_USER=' "$S1/.env" || echo 'DB_USER=radiuspali' >>"$S1/.env"
( cd "$S1" && bash ./setup.sh >"$tmp/s1c.log" 2>&1 ); kode=$?
cek "setup.sh Pali sukses" '[ $kode = 0 ]'
cek "radius_db = radiuspali" 'grep -qP "^\tradius_db = \"radiuspali\"$" "$(SQL "$S1")"'
cek "login = radiuspali" 'grep -qP "^\tlogin = \"radiuspali\"$" "$(SQL "$S1")"'
cek "sandi dari .env" 'grep -qP "^\tpassword = \"Sandi_Pali-1\"$" "$(SQL "$S1")"'
cek "tetap satu radius_db aktif (Pali)" '[ "$(aktifRadiusDB "$S1")" = 1 ]'
cek "tak ada lagi radius_db \"radius\" aktif" '! grep -qP "^\tradius_db = \"radius\"$" "$(SQL "$S1")"'

echo "== setup.sh: secret sudah diisi operator → dipakai apa adanya"
S2="$tmp/s2"; siapkanStack "$S2"
cp "$S2/.env.example" "$S2/.env"
sed -i '/^RADIUS_HEALTH_SECRET=/d' "$S2/.env"
echo 'RADIUS_HEALTH_SECRET=secret-operator-0123456789' >>"$S2/.env"
( cd "$S2" && bash ./setup.sh >"$tmp/s2.log" 2>&1 ); kode=$?
cek "setup.sh sukses (secret operator)" '[ $kode = 0 ]'
cek "secret operator tak diganti" '[ "$(nilaiEnv "$S2" RADIUS_HEALTH_SECRET)" = secret-operator-0123456789 ]'
cek "secret operator ditanam" 'grep -qF "secret = \"secret-operator-0123456789\"" "$(HEALTH "$S2")"'

echo "== setup.sh: nilai berbahaya ditolak (tidak ditanam rusak)"
tolak() { # $1 kunci, $2 nilai
  local d="$tmp/tolak-$1"; rm -rf "$d"; siapkanStack "$d"
  cp "$d/.env.example" "$d/.env"
  sed -i "/^$1=/d" "$d/.env"; printf '%s=%s\n' "$1" "$2" >>"$d/.env"
  ( cd "$d" && bash ./setup.sh >"$d.log" 2>&1 )
}
tolak DB_NAME 'radius;drop'; kode=$?
cek "DB_NAME berkarakter terlarang → berhenti" '[ $kode != 0 ] && grep -q "DB_NAME di .env" "$tmp/tolak-DB_NAME.log"'
cek "DB_NAME terlarang → raddb tak disentuh" 'grep -qP "^\tradius_db = \"radius\"$" "$(SQL "$tmp/tolak-DB_NAME")"'
tolak DB_USER 'radius"x'; kode=$?
cek "DB_USER berkarakter terlarang → berhenti" '[ $kode != 0 ]'
tolak RADIUS_HEALTH_SECRET 'pendek'; kode=$?
cek "secret < 16 karakter → berhenti" '[ $kode != 0 ] && grep -q "terlalu pendek" "$tmp/tolak-RADIUS_HEALTH_SECRET.log"'

echo "== setup.sh: overlay netovas-health hilang → berhenti"
S3="$tmp/s3"; siapkanStack "$S3"; rm -f "$S3/freeradius/overlay/sites-available/netovas-health"
( cd "$S3" && bash ./setup.sh >"$tmp/s3.log" 2>&1 ); kode=$?
cek "overlay hilang → kode keluar bukan 0" '[ $kode != 0 ] && grep -q "repo tidak utuh" "$tmp/s3.log"'

# ─── appliance/firstboot.sh ─────────────────────────────────────────────────

jalankanFirstboot() { # $1 dir stack
  rm -rf "$1.var"
  JEJAK="$1.jejak" TANDA="$1.var/firstboot-selesai" ISSUE="$1.issue" STACK_DIR="$1" \
    bash "$AKAR/appliance/firstboot.sh" >"$1.log" 2>&1
}

echo "== firstboot.sh: appliance baru (.env dibangkitkan)"
F1="$tmp/f1"; siapkanStack "$F1"
jalankanFirstboot "$F1"; kode=$?
cek "firstboot sukses" '[ $kode = 0 ]'
cek ".env firstboot mode 600" '[ "$(stat -c %a "$F1/.env")" = 600 ]'
FSEC="$(nilaiEnv "$F1" RADIUS_HEALTH_SECRET)"
cek "firstboot: secret dibangkitkan" '[ "${#FSEC}" -ge 32 ]'
cek "firstboot: secret ditanam" 'grep -qF "secret = \"$FSEC\"" "$(HEALTH "$F1")"'
cek "firstboot: sandi unik ditanam" 'grep -qF "password = \"$(nilaiEnv "$F1" DB_PASSWORD)\"" "$(SQL "$F1")"'
cek "firstboot: radius_db radius, satu aktif" 'grep -qP "^\tradius_db = \"radius\"$" "$(SQL "$F1")" && [ "$(aktifRadiusDB "$F1")" = 1 ]'
cek "firstboot: docker compose up dijalankan" 'grep -q "docker compose up -d" "$F1.jejak"'
cek "firstboot: tanda selesai ditulis" '[ -f "$F1.var/firstboot-selesai" ]'

echo "== firstboot.sh: .env lama ber-DB_NAME lain tanpa secret pemeriksa"
F2="$tmp/f2"; siapkanStack "$F2"
cat >"$F2/.env" <<'EOF'
DB_ROOT_PASSWORD=akar-lama-123
DB_NAME=radiuspali
DB_USER=radiuspali
DB_PASSWORD=sandi-lama-456
API_PORT=8000
EOF
jalankanFirstboot "$F2"; kode=$?
cek "firstboot (.env lama) sukses" '[ $kode = 0 ]'
cek ".env lama tidak dibuat ulang" '[ "$(nilaiEnv "$F2" DB_PASSWORD)" = sandi-lama-456 ]'
cek "radius_db = radiuspali" 'grep -qP "^\tradius_db = \"radiuspali\"$" "$(SQL "$F2")"'
cek "login = radiuspali" 'grep -qP "^\tlogin = \"radiuspali\"$" "$(SQL "$F2")"'
F2SEC="$(nilaiEnv "$F2" RADIUS_HEALTH_SECRET)"
cek "secret ditambahkan ke .env lama" '[ "${#F2SEC}" -ge 32 ] && [ "$(grep -c "^RADIUS_HEALTH_SECRET=" "$F2/.env")" = 1 ]'
cek "secret .env lama ditanam" 'grep -qF "secret = \"$F2SEC\"" "$(HEALTH "$F2")"'

echo "== firstboot.sh: DB_NAME terlarang → berhenti sebelum docker"
F3="$tmp/f3"; siapkanStack "$F3"
printf 'DB_NAME=radius db\nDB_USER=radius\nDB_PASSWORD=x-123\n' >"$F3/.env"
jalankanFirstboot "$F3"; kode=$?
cek "firstboot DB_NAME terlarang → kode bukan 0" '[ $kode != 0 ]'
cek "firstboot DB_NAME terlarang → docker tak dijalankan" '! grep -q "compose up" "$F3.jejak" 2>/dev/null'

echo
if [ $gagal = 0 ]; then
  echo "SEMUA LULUS ($jumlah kasus)"
else
  echo "ADA YANG GAGAL ($jumlah kasus)"
fi
exit $gagal
