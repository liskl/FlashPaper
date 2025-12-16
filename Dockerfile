# FlashPaper - Multi-stage Docker build
# Produces a minimal container (~20MB) with the compiled binary
#
# Build: docker build -t flashpaper .
# Run: docker run -p 8080:8080 flashpaper

# ============================================================================
# Stage 1: Build the Go binary
# ============================================================================
FROM golang:1.21-alpine AS builder

# Install build dependencies
# - gcc and musl-dev are required for SQLite (CGO)
# - ca-certificates for HTTPS connections
RUN apk add --no-cache gcc musl-dev ca-certificates

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better layer caching
# Dependencies are only re-downloaded if these files change
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with CGO enabled (required for SQLite)
# -ldflags -s -w strips debug info for smaller binary
# -o specifies output path
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo 'dev')" \
    -o flashpaper \
    ./cmd/flashpaper/

# ============================================================================
# Stage 2: Create minimal runtime image
# ============================================================================
FROM alpine:3.19

# Add labels for container metadata
LABEL org.opencontainers.image.title="FlashPaper"
LABEL org.opencontainers.image.description="Zero-knowledge encrypted pastebin"
LABEL org.opencontainers.image.url="https://github.com/liskl/flashpaper"
LABEL org.opencontainers.image.source="https://github.com/liskl/flashpaper"
LABEL org.opencontainers.image.vendor="liskl"
LABEL org.opencontainers.image.licenses="MIT"

# Install runtime dependencies
# - ca-certificates for HTTPS (if connecting to external services)
# - tzdata for timezone support
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user for security
# Running as non-root prevents container escape vulnerabilities
RUN addgroup -g 1000 flashpaper && \
    adduser -u 1000 -G flashpaper -s /bin/sh -D flashpaper

# Create data directory for SQLite and filesystem storage
# Must be owned by flashpaper user
RUN mkdir -p /data && chown flashpaper:flashpaper /data

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/flashpaper /app/flashpaper

# Copy sample config (can be overridden with volume mount)
COPY config.sample.ini /app/config.sample.ini

# Change ownership to non-root user
RUN chown -R flashpaper:flashpaper /app

# Switch to non-root user
USER flashpaper

# Environment variables for configuration
# These can be overridden at runtime
ENV FLASHPAPER_MAIN_BASEPATH="/"
ENV FLASHPAPER_MAIN_NAME="FlashPaper"
ENV FLASHPAPER_MODEL_CLASS="Database"
ENV FLASHPAPER_MODEL_DSN="/data/flashpaper.db"

# Expose default port
EXPOSE 8080

# Health check to verify container is running correctly
# curl is not installed in alpine, so we use wget
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Volume for persistent data (SQLite database, filesystem storage)
VOLUME ["/data"]

# Default command
CMD ["/app/flashpaper"]
