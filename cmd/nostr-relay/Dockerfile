# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the relay
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=1.0.0 -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ) -X main.gitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" \
    -o nostr-relay \
    ./cmd/nostr-relay

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 nostr && \
    adduser -D -u 1000 -G nostr nostr

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/nostr-relay .
COPY --from=builder /build/cmd/nostr-relay/config.yaml.example ./config.yaml.example

# Set ownership
RUN chown -R nostr:nostr /app

# Switch to non-root user
USER nostr

# Expose ports
EXPOSE 3334 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the relay
CMD ["./nostr-relay"]
