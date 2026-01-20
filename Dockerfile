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

# Install only ca-certificates for HTTPS (no package manager cache)
RUN apk --no-cache --update add ca-certificates && \
    rm -rf /var/cache/apk/* /tmp/*

WORKDIR /app

# Copy binary from builder (already executable)
COPY --from=builder /build/telegram-mcp /app/telegram-mcp

# Create directory for session storage
RUN mkdir -p /app/data

ENV TG_SESSION_PATH=/app/data/session.json

ENTRYPOINT ["/app/telegram-mcp"]


