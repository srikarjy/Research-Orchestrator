# AssayOS Gateway - Unified API Gateway
# Builds only the Go gateway binary

# ==========================================
# Stage 1: Build Go binary
# ==========================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make gcc musl-dev

# Copy all module files first (for replace directives)
COPY assayos/go.mod assayos/go.sum ./assayos/
COPY biolab-mcp-server/go.mod biolab-mcp-server/go.sum ./assayos/biolab-mcp-server/
COPY workflow-engine/go.mod workflow-engine/go.sum ./assayos/workflow-engine/

# Download dependencies from assayos module directory
WORKDIR /app/assayos
RUN go mod download

# Copy source code
COPY assayos/ ./
COPY biolab-mcp-server/ ./biolab-mcp-server/
COPY workflow-engine/ ./workflow-engine/

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o assayos ./cmd/assayos

# ==========================================
# Stage 2: Final image
# ==========================================
FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    curl \
    tzdata \
    postgresql-client \
    redis

WORKDIR /app

# Copy Go binary
COPY --from=builder /app/assayos/assayos .

# Create directories
RUN mkdir -p /data /tmp/assayos-sandbox /var/log/assayos

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENV ASSAYOS_ENV=production

EXPOSE 8080

ENTRYPOINT ["./assayos"]
CMD ["serve"]