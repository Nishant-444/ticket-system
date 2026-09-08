# Multi-stage Dockerfile for lightweight, fast production image

# Stage 1: Build the Golang binary
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy application source code including vendored dependencies
COPY . .

# Build statically linked binary without CGO using vendored modules
RUN CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-s -w" -o /app/server .

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

# Create directory for SQLite database storage
RUN mkdir -p /data

# Copy compiled binary from builder stage
COPY --from=builder /app/server /app/server

# Expose default port
EXPOSE 8080

# Set environment variable defaults
ENV PORT=8080
ENV DB_PATH=/data/tickets.db

# Run the binary
CMD ["/app/server"]
