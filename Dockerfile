# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Download dependencies first (layer-cached)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gateway ./cmd/gateway

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates needed if gateway proxies HTTPS backends
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/gateway     ./gateway
COPY config.yaml                      ./config.yaml
COPY keys/                            ./keys/

EXPOSE 8080 8081

CMD ["./gateway"]
