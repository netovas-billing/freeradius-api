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
./setup.sh          # bootstrap raddb + schema + .env (API_KEY acak)
docker compose up -d
docker compose ps   # db healthy, freeradius & api up
```

`setup.sh` aman dijalankan berulang. Yang perlu diketahui tentang `.env`:
ia DIBUAT di akhir `setup.sh`, jadi pada jalan pertama sandi DB yang ditanam
ke raddb adalah bawaan `.env.example`. Mau menggantinya? Ganti **sebelum** DB
pertama lahir, atau buang volumenya — MariaDB hanya memakai
`MARIADB_PASSWORD` saat volume masih kosong, jadi mengganti sandi di `.env`
pada DB yang sudah ada menghasilkan FreeRADIUS dan api yang gagal konek tanpa
sebab yang kelihatan:

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

## API

Stack: **Go 1.23 + Fiber v2 + GORM + swaggo**. Image final ~55 MB (alpine + static binary).

- Swagger UI: <http://localhost:8000/docs/index.html>
- OpenAPI JSON: <http://localhost:8000/docs/doc.json>
- Healthcheck: `GET /health`
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
