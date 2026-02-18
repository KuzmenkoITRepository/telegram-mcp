# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build
COPY . .
ENV CGO_ENABLED=0
RUN go build -o telegram-mcp .

# Runtime stage - use distroless or minimal alpine
FROM alpine:latest

# Install only ca-certificates for HTTPS (with retries for transient network issues)
RUN set -eux; \
    attempts=5; \
    i=1; \
    while [ "$i" -le "$attempts" ]; do \
        if apk --no-cache --update add ca-certificates; then \
            break; \
        fi; \
        if [ "$i" -eq "$attempts" ]; then \
            exit 1; \
        fi; \
        echo "apk add ca-certificates failed ($i/$attempts), retrying in 5s..."; \
        i=$((i + 1)); \
        sleep 5; \
    done; \
    rm -rf /var/cache/apk/* /tmp/*

WORKDIR /app

# Copy binary from builder (already executable)
COPY --from=builder /build/telegram-mcp /app/telegram-mcp

# Create directory for session storage
RUN mkdir -p /app/data

ENV TG_SESSION_PATH=/app/data/session.json

ENTRYPOINT ["/app/telegram-mcp"]

