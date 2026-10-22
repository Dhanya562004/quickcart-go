# syntax=docker/dockerfile:1

# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . .

# Compile optimized static binary with embedded version metadata
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X 'main.Version=1.0.0'" \
    -o /app/quickcart-server ./cmd/server

# Final Lightweight Runtime Stage
FROM alpine:3.20 AS runner

WORKDIR /app

# Security best practice: run as non-root user
RUN addgroup -S quickcart && adduser -S quickcart -G quickcart

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/quickcart-server /app/quickcart-server
COPY --from=builder /app/migrations /app/migrations

USER quickcart

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/quickcart-server"]
