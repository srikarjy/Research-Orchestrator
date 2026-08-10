# Biolab MCP Server - Evidence + Action Plane
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git make

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o biolab-mcp-server ./cmd/main.go

FROM alpine:3.20

RUN apk add --no-cache curl ca-certificates

WORKDIR /app

COPY --from=builder /app/biolab-mcp-server .

# Create sandbox directory
RUN mkdir -p /tmp/research-sandbox

HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8081/health || exit 1

EXPOSE 8081

CMD ["./biolab-mcp-server"]