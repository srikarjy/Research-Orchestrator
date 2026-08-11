# Workflow Engine worker + migrate — builds from the real
# github.com/srikarjy/workflow-Engine submodule at ./workflow-engine, not a
# vendored copy. No Dockerfile exists upstream yet; this lives here until
# one does. Context is the repo root (see docker-compose.yml's `context: .`)
# so COPY paths below are relative to the Research-Orchestrator root.
FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY workflow-engine/go.mod workflow-engine/go.sum ./
RUN go mod download

COPY workflow-engine/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/cli ./cmd/cli

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /out/worker /out/migrate /out/cli ./
COPY workflow-engine/migrations ./migrations

ENTRYPOINT ["./worker"]
