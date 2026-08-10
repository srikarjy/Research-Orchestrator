# Workflow Engine - Durable Execution Plane
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o workflow-engine ./cmd/main.go

# Final stage
FROM alpine:3.20

RUN apk add --no-cache curl ca-certificates

WORKDIR /app

COPY --from=builder /app/workflow-engine .

# Create data directory
RUN mkdir -p /data

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

EXPOSE 8080

CMD ["./workflow-engine"]