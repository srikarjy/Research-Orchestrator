# AssayOS - Unified Platform for Agentic Biotech Research

**One binary. Three planes. Zero friction.**

AssayOS unifies the Research Orchestrator's three planes into a single deployable platform:
- **Plane 1: Aletheia** (Reasoning) - LangGraph multi-agent debate
- **Plane 2: Workflow Engine** (Durable Execution) - Event-sourced, idempotent
- **Plane 3: Biolab MCP** (Evidence + Action) - 10+ bio tools, sandbox, agents

## Quick Start

```bash
# One command to rule them all
cd assayos
docker compose up --build -d

# Open http://localhost:8080
# Try: "explain why BRAF V600E reduces binding affinity"
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      ASSAYOS (Port 8080)                         │
│  ┌──────────────┐ ┌──────────────┐ ┌────────────────────────┐  │
│  │  Aletheia    │ │ Workflow     │ │ Biolab MCP             │  │
│  │  (Reasoning) │ │ Engine       │ │ (Evidence + Action)    │  │
│  │              │ │              │ │                        │  │
│  │ Planner      │ │ Event Store  │ │ 10+ Retrievers         │  │
│  │ Researcher   │ │ Idempotency  │ │ 4 Analyzers            │  │
│  │ Critic       │ │ WebSocket    │ │ 2 Visualizers          │  │
│  │ Synthesizer  │ │ Scheduler    │ │ 9 Agents               │  │
│  └──────┬───────┘ └──────┬───────┘ └───────────┬────────────┘  │
│         │                │                      │               │
│         └────────────────┼──────────────────────┘               │
│                          ▼                                      │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              PLATFORM KERNEL (Shared Services)            │   │
│  │  Config │ Secrets │ Logging │ Metrics │ DB │ Cache │AuthZ │   │
│  └──────────────────────────────────────────────────────────┘   │
│                          │                                      │
│                          ▼                                      │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              UNIFIED API GATEWAY (Gin)                     │   │
│  │  /api/v1/workflows  → Workflow Engine                     │   │
│  │  /api/v1/agents     → Biolab MCP                          │   │
│  │  /api/v1/tools      → Biolab MCP                          │   │
│  │  /api/v1/aletheia   → Aletheia Gateway                    │   │
│  │  /ws/*              → Unified WebSocket Hub               │   │
│  │  /*                 → Embedded React SPA                  │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Features

| Feature | Description |
|---------|-------------|
| **Single Binary** | One Docker image, one process, one port (8080) |
| **Three Planes** | Aletheia + Workflow Engine + Biolab MCP unified |
| **Event Sourcing** | Append-only event log with SHA-256 dedup keys |
| **Idempotent Execution** | SHA-256 dedup keys prevent duplicate work |
| **Real-time WebSocket** | Unified hub for all plane events |
| **Plugin System** | Go plugins for tools, agents, visualizers |
| **AuthZ** | OPA-based authorization (optional) |
| **Audit Log** | 21 CFR Part 11 / ALCOA+ compliant |
| **Embedded UI** | React SPA served from same binary |

## API Endpoints

All on `http://localhost:8080`

### Plane 2: Workflow Engine
```
POST   /api/v1/workflows              # Create workflow
GET    /api/v1/workflows              # List workflows
GET    /api/v1/workflows/:id          # Get workflow
POST   /api/v1/workflows/:id/execute  # Execute workflow
GET    /api/v1/workflows/:id/events   # Get events (with WebSocket)
GET    /api/v1/executor/calendar      # Calendar events
GET    /api/v1/executor/notifications # Notifications
GET    /api/v1/executor/tasks         # Running tasks
```

### Plane 3: Biolab MCP
```
GET    /api/v1/agents                 # List agents
GET    /api/v1/agents/:id/status      # Agent status
POST   /api/v1/workflows              # Create biolab workflow
GET    /api/v1/tools                  # List tools
POST   /api/v1/tools/:cat/:name/exec  # Execute tool
POST   /api/v1/sandbox/sessions       # Create sandbox session
```

### Plane 1: Aletheia
```
POST   /api/v1/aletheia/investigate   # Submit investigation
GET    /api/v1/aletheia/investigate/:id/status
GET    /api/v1/aletheia/workflows     # List investigations
WS     /ws/aletheia/:id               # Real-time updates
```

### Unified WebSocket
```
WS /ws/workflows/:id    # Workflow Engine events
WS /ws/aletheia/:id     # Aletheia events
```

### UI
```
GET /*  # Embedded React SPA
```

## Demo Queries

```bash
# Via Aletheia (full pipeline)
curl -X POST http://localhost:8080/api/v1/aletheia/investigate \
  -H "Content-Type: application/json" \
  -d '{"query": "explain why BRAF V600E reduces binding affinity"}'

# Direct workflow execution
curl -X POST http://localhost:8080/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{"name": "Test", "query": "KRAS G12C resistance"}'

# Execute biolab tool directly
curl -X POST http://localhost:8080/api/v1/tools/retriever/PubMed/execute \
  -H "Content-Type: application/json" \
  -d '{"input": {"query": "EGFR exon 19", "max_results": 5}}'
```

## Configuration

Environment variables (or `.env`):

```bash
# Server
SERVER_PORT=8080
ASSAYOS_ENV=production

# Database
DATABASE_DSN=postgres://user:pass@host:5432/db?sslmode=disable

# Redis
REDIS_ADDR=redis:6379

# Plane toggles
WORKFLOWS_ENABLED=true
BIOLAB_ENABLED=true
ALETHEIA_ENABLED=true
ALETHEIA_ENDPOINT=http://localhost:8000

# AuthZ (optional)
AUTH_ENABLED=false
OPA_ENDPOINT=http://opa:8181/v1/data/assayos/authz
```

## Development

```bash
# Build
cd assayos
go build -o assayos ./cmd/assayos

# Run locally (needs postgres + redis)
export DATABASE_DSN="postgres://assayos:assayos@localhost:5432/assayos?sslmode=disable"
export REDIS_ADDR=localhost:6379
./assayos serve

# Run tests
go test ./...
```

## Deployment

```bash
# Production
docker compose -f assayos/docker-compose.yml up -d

# With AuthZ
docker compose -f assayos/docker-compose.yml --profile with-auth up -d

# Scale
docker compose up -d --scale assayos=3
```

## Project Structure

```
assayos/
├── cmd/assayos/main.go          # Entry point
├── internal/
│   ├── kernel/                  # Platform kernel (config, DB, event bus, plugins)
│   ├── gateway/                 # Unified API gateway (Gin)
│   ├── workflow/                # Plane 2 handler
│   ├── biolab/                  # Plane 3 handler
│   ├── aletheia/                # Plane 1 gateway
│   └── ui/                      # Embedded React SPA
├── deployments/
│   ├── init.sql                 # PostgreSQL schema
│   └── opa/authz.rego           # OPA policies
├── plugins/                     # Go plugin directory
├── Dockerfile                   # Multi-stage build
├── docker-compose.yml           # Full stack
└── go.mod
```

## Key Differentiators

| Aspect | Traditional | AssayOS |
|--------|-------------|---------|
| Deployment | 3+ services | 1 container |
| Networking | Service mesh | Local calls |
| Event bus | Kafka/Redis | In-process |
| Config | 3+ config files | 1 env file |
| Debugging | Distributed tracing | Single process |
| WebSocket | 3 endpoints | 1 unified hub |
| UI serving | Separate CDN | Embedded |

## License

MIT