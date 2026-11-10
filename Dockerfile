# syntax=docker/dockerfile:1
# One builder, three small runtime images: api, worker, migrate.

ARG GO_VERSION=1.27

FROM golang:${GO_VERSION}-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.21 AS base
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
USER app
WORKDIR /app

FROM base AS api
COPY --from=builder /out/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM base AS worker
COPY --from=builder /out/worker /app/worker
ENTRYPOINT ["/app/worker"]

FROM base AS migrate
COPY --from=builder /out/migrate /app/migrate
ENTRYPOINT ["/app/migrate"]

FROM base AS seed
COPY --from=builder /out/seed /app/seed
ENTRYPOINT ["/app/seed"]
