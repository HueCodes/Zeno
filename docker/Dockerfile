# Build stage
FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with platform-specific flags
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix cgo -ldflags '-w -s -X main.version=docker' -o controller cmd/controller/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates docker-cli tzdata && \
    addgroup -g 1000 zeno && \
    adduser -D -u 1000 -G zeno zeno

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/controller .

# Create config directory with correct permissions
RUN mkdir -p /etc/zeno && chown -R zeno:zeno /app /etc/zeno

USER zeno

EXPOSE 8080 9090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./controller"]
