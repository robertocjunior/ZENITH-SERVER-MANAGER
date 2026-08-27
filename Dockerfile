# syntax=docker/dockerfile:1

# ==============================================================================
# Stage 1: Build static Go binary
# ==============================================================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata git

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Compile static binary with zero CGO
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -ldflags="-s -w -X 'main.Version=1.0.0' -X 'main.GitCommit=docker'" \
    -o /bin/zenith-server \
    ./cmd/server

# ==============================================================================
# Stage 2: Minimal hardened runtime image
# ==============================================================================
FROM alpine:3.21.3

# Install runtime certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 10001 -S zenith && \
    adduser -u 10001 -S -G zenith -h /app zenith

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/zenith-server /app/zenith-server
COPY --from=builder --chown=zenith:zenith /src/config.yaml /app/config.yaml

# Run as non-root
USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/healthz || exit 1

ENTRYPOINT ["/app/zenith-server"]
CMD ["-config", "/app/config.yaml"]
