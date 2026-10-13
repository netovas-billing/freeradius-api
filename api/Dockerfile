# Image ini dibangun TANPA satu pun panggilan jaringan selain menarik dua base
# image dari registry. Tidak ada `apk add`, tidak ada `go install`, tidak ada
# `go mod tidy`.
#
# KENAPA. Jaringan tempat image ini dibangun tidak selalu bisa menjangkau
# dl-cdn.alpinelinux.org — terbukti 28 Sep 2026 dan 13 Okt 2026 di mesin lab:
# `docker pull` berhasil, lalu `RUN apk add` menggantung 60 detik per repo dan
# berakhir `WARNING: fetching …: Permission denied`. Keduanya jalur BERBEDA
# (pull lewat namespace daemon, RUN lewat namespace build), jadi "Docker Hub
# jalan" bukan bukti build punya jaringan. Setiap paket yang ditarik saat build
# adalah satu cara baru bagi pemasangan di tempat mitra untuk gagal.
#
# Tiga ketergantungan itu dihapus, bukan disiasati:
#   - apk di stage build  → `git` tak dipakai (modul datang dari vendor/),
#                            ca-certificates sudah ada di image golang.
#   - go install swag +   → ./docs SUDAH ter-commit. Regenerasi dilakukan
#     swag init + tidy       pengembang di mesinnya, bukan setiap build:
#                            cd api && go run github.com/swaggo/swag/cmd/swag@v1.16.4 \
#                              init -g main.go -o ./docs --parseInternal
#   - apk di stage akhir  → tzdata digantikan tag build `timetzdata` (zona
#                            waktu ditanam ke dalam biner), curl digantikan
#                            wget bawaan busybox, dan bundel CA sudah ada di
#                            alpine (paket ca-certificates-bundle di base).

FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .

ARG VERSION=dev
# -mod=vendor + GOPROXY=off: kalau ada dependensi yang TIDAK ada di vendor/,
# build gagal di sini dengan jelas — bukan diam-diam menarik dari internet dan
# berhasil hanya di mesin yang punya jalan keluar.
# -tags timetzdata: basis data zona waktu ikut ke dalam biner (~450 KB),
# sehingga stage akhir tak perlu paket tzdata.
RUN CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=vendor GOPROXY=off \
    go build -trimpath -tags timetzdata \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/freeradius-api .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/freeradius-api /app/freeradius-api

# Bundel CA milik base image, BUKAN hasil apk. Diperiksa di sini supaya
# kehilangannya muncul saat build — bukan nanti sebagai webhook HTTPS yang
# gagal di appliance yang tak pernah dilihat siapa pun.
RUN test -s /etc/ssl/certs/ca-certificates.crt

EXPOSE 8000

# wget busybox, bukan curl: curl menuntut `apk add`, dan HEALTHCHECK ini justru
# yang membuat container tidak dilaporkan sehat saat ia hidup tapi menolak
# setiap panggilan karena gagal konek DB.
HEALTHCHECK --interval=15s --timeout=3s --retries=5 \
  CMD wget -q -O /dev/null http://127.0.0.1:8000/health || exit 1

ENTRYPOINT ["/app/freeradius-api"]
