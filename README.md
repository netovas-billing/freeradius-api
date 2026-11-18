# freeradius-api

FreeRADIUS + MariaDB + REST API (Go / Fiber / GORM / swaggo) lewat Docker Compose.

```
NAS/AP/Router  --1812/1813 UDP-->  freeradius
                                       |
                                       v
                                       db (MariaDB, named volume)
                                       ^
                                       |
                                   freeradius-api (8000/tcp)
```

- `freeradius` — stateless, config dari `./freeradius/raddb`.
- `db` — MariaDB, data di named volume `freeradius_mariadb_data`.
- `api` — REST API, akses DB via network internal Docker.

## Uji coba di lokal (lab)

Ini jalur untuk **menyalakan dan mencoba**, bukan untuk membuat OVA mitra.
Golden image punya urutannya sendiri di `appliance/README.md`, dan
`appliance/firstboot.sh` JANGAN dijalankan di sini.

```bash
./setup.sh          # bootstrap raddb + schema + .env (API_KEY & RADIUS_HEALTH_SECRET acak)
docker compose up -d
docker compose ps   # db healthy, freeradius & api up — "Restarting" = lihat Galat umum
KEY=$(grep '^API_KEY=' .env | cut -d= -f2-)
curl -s -H "X-API-Key: $KEY" localhost:8000/api/v1/radius/health
# {"ok":true,"jawaban":"Access-Reject",...}  ← WAJIB ini sebelum router diarahkan ke sini
```

`setup.sh` aman dijalankan berulang. Yang perlu diketahui tentang `.env`:
kalau belum ada, ia DIBUAT dari `.env.example` di awal `setup.sh`, jadi pada
jalan pertama sandi DB yang ditanam ke raddb adalah bawaan `.env.example`.
Mau menggantinya? Ganti **sebelum** DB
pertama lahir, atau buang volumenya — MariaDB hanya memakai
`MARIADB_PASSWORD` saat volume masih kosong, jadi mengganti sandi di `.env`
pada DB yang sudah ada menghasilkan FreeRADIUS dan api yang gagal konek tanpa
sebab yang kelihatan. `DB_USER` juga boleh diganti (mis. `radiuspali`):
`setup.sh` menanam user, sandi, DAN nama database (`DB_NAME`) dari `.env` ke
`mods-available/sql` lalu memeriksanya. Sebelum 17 Nov 2026 user di raddb
ditulis mati `radius` dan `radius_db` tetap bawaan template, jadi `DB_USER`/
`DB_NAME` lain membuat FreeRADIUS ditolak MariaDB ("Access denied") dan restart
terus — dengan `docker compose logs` KOSONG, karena galatnya masuk berkas log di
dalam container. Untuk melihat galat semacam itu:
`docker compose run --rm --entrypoint freeradius freeradius -X`.

```bash
docker compose down -v        # HAPUS data DB — hanya di lab
vim .env                      # DB_PASSWORD / DB_ROOT_PASSWORD
./setup.sh && docker compose up -d
```

### 1. API hidup?

```bash
curl -s localhost:8000/health
KEY=$(grep '^API_KEY=' .env | cut -d= -f2-)
curl -s -H "X-API-Key: $KEY" localhost:8000/api/users
```

Swagger UI: <http://localhost:8000/docs/index.html>. Kalau semua panggilan
`/api` dijawab 401, periksa `API_KEY` di `.env` TIDAK kosong — kosong berarti
X-API-Key dimatikan dengan sengaja (gagal-tertutup; lihat `appliance/`).

### 1b. FreeRADIUS hidup? (BUKAN sama dengan API hidup)

`/health` hanya mem-ping MariaDB. API bisa menjawab sempurna sementara
FreeRADIUS di sebelahnya restart terus — itu persis yang terjadi saat pindah
RADIUS Pali (17 Nov 2026): ERP memverifikasi lewat API, menyatakan server baru
siap, router dialihkan, dan semua login "radius timeout". Periksa FreeRADIUS-nya
sendiri:

```bash
curl -s -H "X-API-Key: $KEY" localhost:8000/api/v1/radius/health
```

| Jawaban | Arti |
| --- | --- |
| `200 {"ok":true,"jawaban":"Access-Reject","rtt_ms":…}` | FreeRADIUS hidup, semua modul (termasuk sql) termuat dan DB terbaca. `Access-Reject` itu BENAR — pemeriksa sengaja memakai sandi salah. `rtt_ms` ≈ 1000 wajar (`reject_delay` FreeRADIUS). |
| `503 {"ok":false,"galat":…}` | FreeRADIUS mati / restart terus / DB tak terbaca / secret pemeriksa tak cocok — baca `galat`, lalu **Galat umum** di bawah. |
| `501 {"ok":false,…}` | `RADIUS_HEALTH_SECRET` kosong di container api — jalankan `./setup.sh` lalu `docker compose up -d`. |

Cara kerjanya: api mengirim Access-Request (User-Name `__netovas_health__`,
sandi acak) ke socket INTERNAL `freeradius:18190` — virtual server
`netovas_health` (`freeradius/overlay/sites-available/netovas-health`) dengan
client-nya sendiri, secret `RADIUS_HEALTH_SECRET` yang dibangkitkan `setup.sh`.
Port itu tidak diterbitkan ke host, socket itu selalu menolak, dan tak ada yang
ditulis ke DB (radpostauth tidak tersentuh). Client pemeriksa SENGAJA bukan
blok subnet di `clients.conf`: FreeRADIUS memilih client paling spesifik, jadi
blok seperti `172.16.0.0/12` akan membajak NAS di pool concentrator
(172.17–172.31) dan semua login mereka gagal.

Server yang sudah berjalan sebelum fitur ini: `git pull`, `./setup.sh`,
`docker compose up -d --build`, `docker compose restart freeradius`. Lupa
restart → `503` "FreeRADIUS menolak paket … socket pemeriksa belum aktif":
socket 18190 baru dibuka FreeRADIUS saat start.

Penanaman raddb oleh `setup.sh` dan `appliance/firstboot.sh` (login, sandi,
`radius_db`, secret pemeriksa) dikunci uji tanpa Docker:
`bash scripts/test-setup.sh`.

### 2. Buat user uji lewat API

Tidak ada user bawaan: `initdb/02-test-user.sql.optional` sengaja tidak
dijalankan entrypoint MariaDB, supaya tiap appliance tidak lahir dengan
`bob`/`hello` yang sama. Buat lewat API saja — lebih mirip pemakaian nyata:

```bash
curl -s -X POST localhost:8000/api/users \
  -H "X-API-Key: $KEY" -H 'Content-Type: application/json' \
  -d '{"username":"bob","password":"hello","password_attr":"Cleartext-Password"}'
```

### 3. Uji auth RADIUS dari dalam container

`client localhost` (127.0.0.1, secret `testing123`) sudah ada di `clients.conf`
bawaan, jadi dari dalam container tak perlu mendaftarkan NAS:

```bash
docker compose exec freeradius sh -c \
  'echo "User-Name=bob,User-Password=hello" | radclient -x 127.0.0.1:1812 auth testing123'
```

`Received Access-Accept` = rantai API → MariaDB → FreeRADIUS utuh.
`Access-Reject` dengan log `rlm_sql: Could not connect` = sandi DB di raddb
tidak sama dengan yang dipakai DB (lihat catatan `.env` di atas).

### 4. Uji dari NAS/router sungguhan

Port 1812/1813 dipublikasikan ke host, jadi router bisa menembaknya langsung —
tapi **IP yang dilihat FreeRADIUS bukan IP router**, melainkan IP sumber
sesudah melewati bridge Docker. Itu yang harus didaftarkan, dan cara paling
jujur membacanya adalah dari log-nya sendiri:

```bash
docker compose logs -f freeradius     # "Ignoring request ... from unknown client <IP>"
curl -s -X POST localhost:8000/api/nas \
  -H "X-API-Key: $KEY" -H 'Content-Type: application/json' \
  -d '{"nasname":"<IP itu>","shortname":"lab","secret":"testing123","type":"other"}'
```

Tanpa restart: NAS yang baru masuk tabel `nas` dipakai pada paket berikutnya
lewat virtual server `dynamic_clients`. (Instance NOC di bawah
freeradius-manager BERBEDA — di sana NAS baru memang menuntut restart, karena
kliennya terdaftar statis per-socket.)

## Galat umum

**`freeradius` "Restarting" terus dan `docker compose logs freeradius` KOSONG.**
FreeRADIUS menulis galatnya ke berkas di dalam container, bukan ke stdout —
`logs` memang kosong. Jalankan sekali dalam mode debug di depan mata:

```bash
docker compose run --rm --entrypoint freeradius freeradius -X
```

Baris terakhir sebelum keluar adalah sebabnya. `Ctrl-C` kalau ternyata ia
menyala (berarti masalahnya sudah hilang — `docker compose up -d`).

**`Access denied for user '…'@'…'` atau `Access denied … to database '…'`.**
Login/sandi/nama database di raddb tidak sama dengan yang dipakai MariaDB.
Jangan sunting `mods-available/sql` dengan tangan — jalankan ulang:

```bash
./setup.sh                        # tanam DB_NAME/DB_USER/DB_PASSWORD dari .env, lalu diperiksa
docker compose restart freeradius
curl -s -H "X-API-Key: $KEY" localhost:8000/api/v1/radius/health
```

Masih ditolak padahal `.env` benar? MariaDB hanya membuat user/sandi/database
saat volume DB pertama kali lahir — mengganti `.env` sesudahnya tidak mengubah
DB yang sudah ada. Samakan `.env` dengan isi DB lama, atau (lab saja)
`docker compose down -v`.

**Port `18120` sudah dipakai / `Error binding to port`.** Ada site lain di
`sites-enabled/` yang memakai port yang sama — `18120` milik `inner-tunnel`
bawaan, itulah sebabnya pemeriksa memakai `18190`. Jangan mengubah port
pemeriksa tanpa mengubah `RADIUS_HEALTH_ADDR` juga.

**`/api/v1/radius/health` 503 "tidak menjawab" padahal `docker compose ps`
menunjukkan freeradius Up.** Secret pemeriksa di api (`.env`) tak sama dengan
raddb — mis. `.env` diganti tanpa menjalankan ulang `setup.sh`. Jalankan
`./setup.sh`, `docker compose up -d`, `docker compose restart freeradius`.

**503 "tidak bisa membaca database (modul sql gagal)".** FreeRADIUS menyala tapi
MariaDB mati / tak terjangkau sesudahnya: `docker compose ps db`,
`docker compose logs db`.

## API

Stack: **Go 1.23 + Fiber v2 + GORM + swaggo**. Image final ~55 MB (alpine + static binary).

- Swagger UI: <http://localhost:8000/docs/index.html>
- OpenAPI JSON: <http://localhost:8000/docs/doc.json>
- Healthcheck API: `GET /health` (hanya API + ping MariaDB)
- Healthcheck FreeRADIUS: `GET /api/v1/radius/health` (butuh auth; lihat 1b)
- Auth: header `X-API-Key: <isi API_KEY di .env>`

### Endpoint utama

| Method | Path                                 | Keterangan                         |
| ------ | ------------------------------------ | ---------------------------------- |
| GET    | /api/users                           | List username                      |
| POST   | /api/users                           | Create user (+ attr, group)        |
| GET    | /api/users/{username}                | Detail user                        |
| PUT    | /api/users/{username}/password       | Ganti password                     |
| DELETE | /api/users/{username}                | Hapus user                         |
| POST   | /api/users/{username}/check          | Tambah check attribute             |
| POST   | /api/users/{username}/reply          | Tambah reply attribute             |
| DELETE | /api/users/{username}/check/{id}     | Hapus check attribute              |
| DELETE | /api/users/{username}/reply/{id}     | Hapus reply attribute              |
| GET    | /api/groups                          | List group                         |
| POST   | /api/groups                          | Create group                       |
| GET    | /api/groups/{groupname}              | Detail group                       |
| DELETE | /api/groups/{groupname}              | Hapus group                        |
| POST   | /api/groups/{groupname}/users/{u}    | Tambah user ke group               |
| DELETE | /api/groups/{groupname}/users/{u}    | Hapus user dari group              |
| GET    | /api/nas                             | List NAS (dari tabel `nas`)        |
| POST   | /api/nas                             | Create NAS                         |
| DELETE | /api/nas/{id}                        | Hapus NAS                          |
| GET    | /api/accounting/sessions/active      | Session yang masih open            |
| GET    | /api/accounting/sessions             | History (filter username/nas)      |
| GET    | /api/accounting/users/{u}/usage      | Total bytes & session time         |

### Contoh

```bash
API=http://localhost:8000
KEY=$(grep ^API_KEY .env | cut -d= -f2)

# Buat user baru dengan password Cleartext + masuk group 'wifi'
curl -sS -X POST "$API/api/users" \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{
    "username": "alice",
    "password": "s3cret",
    "password_attr": "Cleartext-Password",
    "reply_attributes": [
      {"attribute": "Session-Timeout", "op": ":=", "value": "3600"}
    ],
    "groups": ["wifi"]
  }'

# Ganti password
curl -sS -X PUT "$API/api/users/alice/password" \
  -H "X-API-Key: $KEY" -H "Content-Type: application/json" \
  -d '{"password": "newpass", "password_attr": "Cleartext-Password"}'

# Lihat session aktif
curl -sS "$API/api/accounting/sessions/active" -H "X-API-Key: $KEY"
```

## Password attribute

FreeRADIUS mengenal beberapa nama attribute password tergantung versi:

- `Cleartext-Password` (3.0 / kompatibel) — plaintext, gampang test, jelek untuk produksi.
- `Password.Cleartext` (3.2+) — sama, nama baru.
- `NT-Password` / `Password.NT` — wajib untuk PEAP-MSCHAPv2 (WPA-Enterprise).
- `SSHA2-512-Password` — hashed, aman tapi tidak bisa dipakai untuk MSCHAPv2.

API menerima nama attribute apa pun di field `password_attr`; pastikan
nama itu cocok dengan apa yang dipakai FreeRADIUS di module SQL Anda.

## Dua cara repo ini dipakai

| | instance NOC | appliance mitra |
|---|---|---|
| dikelola | `radius-manager-api` | sendiri, di Proxmox mitra |
| dipakai dari repo ini | biner `api/freeradius-api` + `autoclearzombie.sh` + `autobackups3.sh` | seluruh stack Docker Compose |
| `.env` | ditulis manager (`renderAPIEnvFile`, pakai alias `SWAGGER_*`) | `appliance/firstboot.sh` + `radius-erp-enroll` |
| `setup.sh` / `docker-compose.yml` / `initdb/` | **tidak dipakai** | dipakai |

Pemisahan itu disengaja: perubahan pada `setup.sh`, compose, `initdb/`, atau
`appliance/` **tidak bisa** mengganggu instance NOC yang sedang berjalan —
manager tak pernah membacanya. Satu-satunya berkas bersama selain biner adalah
dua skrip pemeliharaan, yang ditambal manager lewat pencocokan `^KEY=` di kolom
0; jangan indentasi atau mengganti nama kuncinya.

Prosedur appliance lengkap: **`appliance/README.md`**.

### Skrip pemeliharaan di stack Docker

`autoclearzombie.sh` dan `autobackups3.sh` mengenali stack Docker sendiri
(host tanpa klien `mariadb`, `.env` di sebelah skrip, container `radius-db`
berjalan): kredensial diambil dari `.env`, SQL lewat `docker exec radius-db`.
Sebelum 18 Nov 2026 keduanya gagal di stack Docker ("mariadb: command not
found") — sesi zombie tak pernah dibersihkan dan backup tak pernah dibuat.

| Kunci `.env` | Untuk | Bawaan |
|---|---|---|
| `ZOMBIE_THRESHOLD_MINUTES` | ambang sesi tanpa Interim-Update | 10 — **router ber-interim 10 menit butuh 25** |
| `BACKUP_REMOTE` / `BACKUP_BUCKET` | tujuan `rclone` | `s3` / `backup-db` |
| `BACKUP_PATH` | folder di bucket | `radiusdb/<DB_NAME>` (bukan `radiusdb/`, supaya server yang berbagi bucket tak saling menimpa) |

Jadwalkan dari host (`crontab -e` sebagai root):

```
*/5 * * * * /root/freeradius-api/autoclearzombie.sh >> /var/log/autoclearzombie.log 2>&1
30 2 * * *  /root/freeradius-api/autobackups3.sh   >> /var/log/autobackups3.log 2>&1
```

`autobackups3.sh` butuh `rclone` terpasang di host dan remote dibuat dengan
`rclone config`; tanpa itu ia berhenti dengan pesan jelas sebelum men-dump.

## Production notes

- Ganti semua secret di `.env`. Jangan commit `.env` ke git.
- `FR_TAG` memaku versi image FreeRADIUS. SATU sumber untuk `docker-compose.yml`
  dan `setup.sh` — `raddb/` disalin dari image itu, jadi dua tempat yang
  menyebut versi berbeda berarti server berjalan dengan config yang bukan
  miliknya.
- `initdb/02-test-user.sql.optional` TIDAK dijalankan entrypoint MariaDB
  (hanya `.sh`/`.sql`/`.sql.gz`/`.sql.xz`/`.sql.zst`). Dulu namanya `.sql` dan
  ter-commit, sehingga setiap appliance lahir dengan `bob`/`hello` yang bisa
  login. Butuh untuk uji nyala? Salin jadi `.sql` sebelum DB pertama dibuat,
  lalu hapus lagi.
- Service `db` tidak punya `ports:` — hanya bisa diakses lewat network internal Docker.
- API service expose port `8000` ke host. Untuk akses dari luar VM, taruh di belakang reverse proxy + TLS (Caddy / nginx) dan rate-limit.
- FreeRADIUS berjalan `radiusd -f` (foreground), **bukan** `-X`. Mode `-X`
  mencatat tiap paket dalam bentuk penuh; di appliance yang jalan sendiri
  berbulan-bulan itu mengisi disk mitra, dan tak ada yang akan mengubahnya di
  mesin yang tak pernah dilihat siapa pun.
- Backup DB rutin: `docker compose exec db sh -c 'mariadb-dump -uradius -p"$MARIADB_PASSWORD" radius' > backup-$(date +%F).sql`.
