# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
ENV CGO_ENABLED=0
RUN go build -o telegram-mcp .

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/telegram-mcp /app/telegram-mcp

# Create directory for session storage
RUN mkdir -p /app/data

# Set default session path
ENV TG_SESSION_PATH=/app/data/session.json

# Make binary executable
RUN chmod +x /app/telegram-mcp

# Run the application
ENTRYPOINT ["/app/telegram-mcp"]


