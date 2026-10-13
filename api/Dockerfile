FROM golang:1.23-alpine AS build
WORKDIR /src

RUN apk add --no-cache git ca-certificates

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
RUN apk add --no-cache ca-certificates curl tzdata
WORKDIR /app
COPY --from=build /out/freeradius-api /app/freeradius-api

EXPOSE 8000
HEALTHCHECK --interval=15s --timeout=3s --retries=5 \
  CMD curl -fsS http://127.0.0.1:8000/health || exit 1

ENTRYPOINT ["/app/freeradius-api"]
