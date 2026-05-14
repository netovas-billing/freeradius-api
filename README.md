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

## Quickstart

```bash
./setup.sh                # bootstrap raddb + schema + .env
# (edit freeradius/raddb/mods-available/sql, sites-available/default,
#  clients.conf seperti instruksi di akhir setup.sh)
docker compose up -d
docker compose logs -f freeradius
```

Test RADIUS dari dalam container:

```bash
docker compose exec freeradius radtest bob hello 127.0.0.1 0 testing123
```

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

## Production notes

- Ganti semua secret di `.env`. Jangan commit `.env` ke git.
- Hapus user `bob` test (atau hapus `initdb/02-test-user.sql` sebelum DB pertama dibuat).
- Service `db` tidak punya `ports:` — hanya bisa diakses lewat network internal Docker.
- API service expose port `8000` ke host. Untuk akses dari luar VM, taruh di belakang reverse proxy + TLS (Caddy / nginx) dan rate-limit.
- Setelah FreeRADIUS stabil, hapus `command: ["-X"]` di compose supaya tidak run debug mode.
- Backup DB rutin: `docker compose exec db sh -c 'mariadb-dump -uradius -p"$MARIADB_PASSWORD" radius' > backup-$(date +%F).sql`.
