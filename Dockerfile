# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /build/bin/zeno \
    ./cmd/zeno

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 1000 zeno && \
    adduser -D -u 1000 -G zeno zeno

# Set working directory
WORKDIR /app

# Copy binary from build stage
COPY --from=builder /build/bin/zeno /app/zeno

# Create data directory
RUN mkdir -p /var/lib/zeno && \
    chown -R zeno:zeno /var/lib/zeno /app

# Switch to non-root user
USER zeno

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set entrypoint
ENTRYPOINT ["/app/zeno"]
