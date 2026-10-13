# APK_MIRROR — cermin Alpine pengganti dl-cdn.alpinelinux.org.
#
# Jaringan tempat image ini dibangun TIDAK selalu bisa menjangkau dl-cdn:
# terbukti 28 Sep 2026 dan 13 Okt 2026 di mesin lab, `apk add` menggantung lalu
# berkata `WARNING: fetching …: Permission denied` PADAHAL Docker Hub jalan
# (`docker pull` lewat namespace daemon, `RUN` lewat namespace build — yang
# tersaring hanya yang kedua). Tanpa jalan keluar, satu-satunya pilihan adalah
# membatalkan build dan menebak.
#
# Contoh: docker compose build --build-arg APK_MIRROR=mirror.leaseweb.com
ARG APK_MIRROR=""

FROM golang:1.23-alpine AS build
ARG APK_MIRROR
WORKDIR /src

# git + ca-certificates: `go install` menarik swag lewat proxy modul (HTTPS).
RUN set -eu; \
    if [ -n "${APK_MIRROR}" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories 2>/dev/null || true; \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories.d/*.repo 2>/dev/null || true; \
    fi; \
    apk add --no-cache git ca-certificates

# Install swag CLI (di luar module supaya tidak menarik tool dep ke go.mod app)
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.4

COPY go.mod ./
COPY . .

# Generate OpenAPI docs (creates ./docs/docs.go yg di-import oleh main.go).
RUN swag init -g main.go -o ./docs --parseInternal

# Resolve semua deps termasuk yang baru muncul di docs/docs.go.
RUN go mod tidy

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/freeradius-api .

FROM alpine:3.20
ARG APK_MIRROR
# curl dipakai HEALTHCHECK di bawah — bukan hiasan: tanpa itu container
# dilaporkan sehat selama prosesnya hidup, termasuk saat ia menolak setiap
# panggilan karena gagal konek DB.
RUN set -eu; \
    if [ -n "${APK_MIRROR}" ]; then \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories 2>/dev/null || true; \
      sed -i "s|dl-cdn.alpinelinux.org|${APK_MIRROR}|g" /etc/apk/repositories.d/*.repo 2>/dev/null || true; \
    fi; \
    apk add --no-cache ca-certificates curl tzdata
WORKDIR /app
COPY --from=build /out/freeradius-api /app/freeradius-api

EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=3s --retries=5 \
  CMD curl -fsS http://127.0.0.1:8000/health || exit 1

ENTRYPOINT ["/app/freeradius-api"]
