# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /simulator ./cmd/simulator

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /simulator /app/simulator

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Default environment variables
ENV SIMULATOR_NAME="WeldingRobot-01" \
    OPCUA_PORT=4840 \
    HEALTH_PORT=8081 \
    ERP_ENDPOINT="http://localhost:8080" \
    ERP_ORDER_PATH="/api/v1/production-orders" \
    ERP_SHIFT_PATH="/api/v1/shifts" \
    PUBLISH_INTERVAL="1s" \
    CYCLE_TIME="60s" \
    SETUP_TIME="60s" \
    SCRAP_RATE="0.03" \
    ERROR_RATE="0.02" \
    ORDER_MIN_QTY="50" \
    ORDER_MAX_QTY="500" \
    TIMEZONE="Europe/Berlin" \
    SHIFT_MODEL="3-shift"

# Expose ports
EXPOSE 4840 8081

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Run the application
CMD ["/app/simulator"]
