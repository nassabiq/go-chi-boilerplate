# syntax=docker/dockerfile:1

# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binaries
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bin/http ./cmd/http
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/bin/worker ./cmd/worker/mail

# Stage 2: HTTP API server
FROM alpine:3.20 AS http

RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/bin/http /app/http
COPY migrations ./migrations

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health/live || exit 1

ENTRYPOINT ["/app/http"]

# Stage 3: Background Worker
FROM alpine:3.20 AS worker

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/bin/worker /app/worker

USER appuser

ENTRYPOINT ["/app/worker"]
