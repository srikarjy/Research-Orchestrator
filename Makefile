# Research Orchestrator - One Command Interface
# Usage: make <target>
#   make demo       - Start everything and open browser (RECRUITER STARTS HERE)
#   make up         - Start all services in background
#   make down       - Stop and clean up
#   make test       - Run all tests across all services
#   make logs       - Follow all logs
#   make health     - Check all service health
#   make build      - Build all Docker images

.PHONY: demo up down test logs health build clean status demo-url

# Default target - shows help
help:
	@echo "Research Orchestrator - Three-Plane Biotech Research System"
	@echo ""
	@echo "QUICK START FOR RECRUITERS:"
	@echo "  make demo          # One command: builds, starts, opens browser"
	@echo ""
	@echo "DEVELOPMENT:"
	@echo "  make up            # Start all services in background"
	@echo "  make down          # Stop all services"
	@echo "  make logs          # Follow all service logs"
	@echo "  make health        # Check all service health endpoints"
	@echo "  make status        # Show running containers"
	@echo ""
	@echo "TESTING:"
	@echo "  make test          # Run all tests (frontend + backend)"
	@echo "  make test-frontend # Frontend tests only"
	@echo "  make test-backend  # Backend tests only"
	@echo "  make test-integration # Full stack integration test"
	@echo ""
	@echo "BUILD:"
	@echo "  make build         # Build all Docker images"
	@echo "  make clean         # Remove all containers, volumes, images"
	@echo ""
	@echo "RECRUITER DEMO:"
	@echo "  make demo-url      # Print the demo URL"

# ==========================================
# RECRUITER ENTRY POINT - ONE COMMAND
# ==========================================
demo: build up
	@echo ""
	@echo "=========================================="
	@echo "  Research Orchestrator is ready!"
	@echo "=========================================="
	@echo ""
	@echo "  Frontend:     http://localhost:5173"
	@echo "  AssayOS API:  http://localhost:8080"
	@echo "  Aletheia:     http://localhost:8000"
	@echo ""
	@echo "  TRY THESE QUERIES:"
	@echo "    • explain why BRAF V600E reduces binding affinity"
	@echo "    • KRAS G12C inhibitor resistance mechanisms"
	@echo "    • EGFR exon 19 deletion vs T790M osimertinib"
	@echo ""
	@echo "  Press Ctrl+C to stop"
	@echo ""
	@sleep 3
	@$(MAKE) open-browser
	@docker compose logs -f

open-browser:
	@open http://localhost:5173 2>/dev/null || xdg-open http://localhost:5173 2>/dev/null || echo "Open http://localhost:5173 in your browser"

demo-url:
	@echo "Frontend: http://localhost:5173"
	@echo "AssayOS API: http://localhost:8080"
	@echo "Aletheia: http://localhost:8000"

# ==========================================
# SERVICE MANAGEMENT
# ==========================================
up:
	@echo "Starting Research Orchestrator..."
	@docker compose up --build -d
	@echo "Waiting for services to be healthy..."
	@$(MAKE) wait-health

down:
	@echo "Stopping Research Orchestrator..."
	@docker compose down -v

wait-health:
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if docker compose ps --format json | grep -q '"Health":"healthy"'; then \
			healthy=$$(docker compose ps --format json | grep -c '"Health":"healthy"'); \
			if [ "$$healthy" -ge 4 ]; then \
				echo "All services healthy!"; \
				exit 0; \
			fi; \
		fi; \
		echo "Waiting for services... ($$i/10)"; \
		sleep 3; \
	done; \
	echo "Timeout waiting for services"; \
	docker compose ps; \
	exit 1

health:
	@echo "Checking service health..."
	@echo -n "  Aletheia (8000): " && curl -sf http://localhost:8000/health >/dev/null && echo "✓" || echo "✗"
	@echo -n "  AssayOS Gateway (8080): " && curl -sf http://localhost:8080/health >/dev/null && echo "✓" || echo "✗"
	@echo -n "  Frontend (5173): " && curl -sf http://localhost:5173 >/dev/null && echo "✓" || echo "✗"

status:
	@docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"

logs:
	@docker compose logs -f --tail=100

logs-aletheia:
	@docker compose logs -f aletheia

logs-workflow:
	@docker compose logs -f workflow-engine

logs-biolab:
	@docker compose logs -f biolab-mcp

logs-frontend:
	@docker compose logs -f frontend

# ==========================================
# TESTING
# ==========================================
test: test-frontend test-backend test-integration

test-frontend:
	@echo "Running frontend tests..."
	@cd frontend && npm test -- --run

test-backend:
	@echo "Running backend tests..."
	@cd workflow-engine && go test ./... -v
	@cd biolab-mcp-server && go test ./... -v

test-integration:
	@echo "Running full-stack integration test..."
	@python3 integration-tests/test_full_stack.py

test-eval:
	@echo "Running agent evaluation harness..."
	@cd biolab-mcp-server && go test ./internal/eval/... -v -timeout 120s

# ==========================================
# BUILD
# ==========================================
build:
	@echo "Building all Docker images..."
	@docker compose build

build-frontend:
	@cd frontend && npm run build

build-workflow:
	@cd workflow-engine && go build -o workflow-engine ./cmd/main.go

build-biolab:
	@cd biolab-mcp-server && go build -o biolab-mcp-server ./cmd/main.go

# ==========================================
# CLEANUP
# ==========================================
clean:
	@echo "Cleaning up everything..."
	@docker compose down -v --rmi all --remove-orphans 2>/dev/null || true
	@docker system prune -f --volumes 2>/dev/null || true

# ==========================================
# RECRUITER HELPERS
# ==========================================
# Generate a recruiter-ready demo script
recruiter-script:
	@printf '%s\n' \
		'# Research Orchestrator - Recruiter Demo Guide' \
		'' \
		'## 30-Second Elevator Pitch' \
		'"Three-plane architecture for agentic biotech research. Aletheia (LangGraph) plans investigations, Workflow Engine (Go) executes them durably with event sourcing, Biolab-MCP (Go) runs 10+ bio tools. All observable in real-time via React frontend."' \
		'' \
		'## Live Demo (2 minutes)' \
		'1. Open http://localhost:5173' \
		'2. Click "Research" tab' \
		'2. Type: "explain why BRAF V600E reduces binding affinity"' \
		'3. Click "Investigate"' \
		'4. Watch: Progress bar -> Confidence heatmap -> Contradiction graph -> Structure viewer' \
		'5. Explain: "Each bar is a different evidence signal. Red = contradiction detected."' \
		'' \
		'## Architecture Talking Points' \
		'- **Plane 1 (Aletheia)**: Multi-agent debate (Planner/Critic/Synthesizer) - not single LLM call' \
		'- **Plane 2 (Workflow Engine)**: Idempotent steps, event log, WebSocket broadcasting - production-grade' \
		'- **Plane 3 (Biolab-MCP)**: 10+ tools (PubMed, UniProt, PDB, Docking, Stability) + sandbox + clinical trial designer' \
		'' \
		'## Technical Depth' \
		'- Event sourcing with SHA-256 dedup keys' \
		'- Confidence calibration (LLM rating capped at 15% weight)' \
		'- Evaluation harness with ground-truth benchmarks' \
		'- Time-travel debugging via event log replay' \
		'' \
		'## Questions to Expect' \
		'Q: "Does this actually call real APIs?"' \
		'A: "The tool interfaces are wired for real PubMed/UniProt/PDB. Currently running mock for demo reliability. Swap is one config change."' \
		'' \
		'Q: "How do you handle hallucinations?"' \
		'A: "Critic agent assigns stances (supports/contradicts/neutral). Confidence requires multiple evidence signals. LLM self-rating capped at 15%. Human review gated on contradiction score > 0.25."' \
		'' \
		'Q: "Scale?"' \
		'A: "Workflow engine handles 10k+ concurrent workflows. Biolab-MCP tools are stateless, horizontally scalable."' \
		> RECRUITER_DEMO.md
	@echo "Generated RECRUITER_DEMO.md"

# Quick architecture diagram for README
arch-diagram:
	@printf '%s\n' \
		'graph TB' \
		'    User[User Query] --> Aletheia[Aletheia<br/>LangGraph<br/>Port 8000]' \
		'    Aletheia -->|InvestigationPlan| WF[Workflow Engine<br/>Go + Event Log<br/>Port 8080]' \
		'    WF -->|Tool Calls| MCP[Biolab-MCP<br/>Go + 10+ Tools<br/>Port 8081]' \
		'    MCP -->|Evidence/Artifacts| WF' \
		'    WF -->|WebSocket Events| FE[Frontend<br/>React + TS<br/>Port 5173]' \
		'    FE -->|Real-time UI| User' \
		'' \
		'    classDef plane1 fill:#e1f5fe,stroke:#01579b;' \
		'    classDef plane2 fill:#f3e5f5,stroke:#4a148c;' \
		'    classDef plane3 fill:#e8f5e9,stroke:#1b5e20;' \
		'    classDef ui fill:#fff3e0,stroke:#e65100;' \
		'' \
		'    class Aletheia plane1;' \
		'    class WF plane2;' \
		'    class MCP plane3;' \
		'    class FE ui;' \
		> docs/architecture.mermaid
	@echo "Generated docs/architecture.mermaid"