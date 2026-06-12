FROM golang:1.24-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -trimpath \
    -ldflags="-s -w \
    -X github.com/solat/lowcode-database/internal/version.Version=${VERSION} \
    -X github.com/solat/lowcode-database/internal/version.Commit=${COMMIT} \
    -X github.com/solat/lowcode-database/internal/version.BuildTime=${BUILD_TIME}" \
    -o /out/server ./cmd/server

RUN go build -trimpath \
    -ldflags="-s -w \
    -X github.com/solat/lowcode-database/internal/version.Version=${VERSION} \
    -X github.com/solat/lowcode-database/internal/version.Commit=${COMMIT} \
    -X github.com/solat/lowcode-database/internal/version.BuildTime=${BUILD_TIME}" \
    -o /out/migrate ./cmd/migrate

FROM alpine:3.21

ARG VERSION=dev
LABEL org.opencontainers.image.title="lowcode-database" \
      org.opencontainers.image.description="Postgres dynamic table engine (HTTP JSON API)" \
      org.opencontainers.image.version="${VERSION}"

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/server ./server
COPY --from=builder /out/migrate ./migrate

EXPOSE 8080

USER nobody

ENTRYPOINT ["./server"]
