# Orchestrator - Workflow Engine + Planner + API Gateway
# Builds only the Go gateway binary

# ==========================================
# Stage 1: Build Go binary
# ==========================================
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make gcc musl-dev

# Copy all module files first (for replace directives)
COPY orchestrator/go.mod orchestrator/go.sum ./orchestrator/
COPY biolab-mcp-server/go.mod biolab-mcp-server/go.sum ./biolab-mcp-server/

# Download dependencies from orchestrator module directory
WORKDIR /app/orchestrator
RUN go mod download

# Copy source code (must be at /app level for correct paths)
WORKDIR /app
COPY orchestrator/ ./orchestrator/
COPY biolab-mcp-server/ ./biolab-mcp-server/

# Build
WORKDIR /app/orchestrator
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o orchestrator ./cmd/assayos

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
COPY --from=builder /app/orchestrator/orchestrator .

# Create directories
RUN mkdir -p /data /tmp/orchestrator-sandbox /var/log/orchestrator

# Health check
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

ENV ORCH_ENV=production

EXPOSE 8080

ENTRYPOINT ["./orchestrator"]
CMD ["serve"]