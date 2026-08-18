# Research-Orchestrator

**Product layer that composes four independent systems into one thing a scientist can ask a question to.**

## The problem

A computational/translational biologist deciding whether a drug target is worth pursuing before committing bench time or budget wants to ask something like:

> *"Does mutation X actually reduce binding affinity for this drug, and does the literature agree?"*

Asking a bare chat model gets you an answer, but not one you can trust for a real decision — no traceable citations, no signal for when the literature disagrees with itself, one flat confidence number with no way to interrogate it.

## What this is

Research-Orchestrator is the Go gateway that ties together four separately-built, independently real systems:

| System | Repo | Role |
|---|---|---|
| [**Aletheia**](https://github.com/srikarjy/Aletheia) | separate repo | Grounded reasoning — retrieves literature, resolves the claim with an LLM call that surfaces contradictions instead of averaging them away, returns a rubric-anchored confidence score |
| [**biolab-mcp-server**](https://github.com/srikarjy/biolab-mcp-server) | separate repo, published | Audit/logging interception layer — every retrieval gets a traceable `retrieval_id` |
| [**workflow-Engine**](https://github.com/srikarjy/workflow-Engine) | git submodule (`./workflow-engine`) | Durable execution — Saga-pattern compensation, idempotent step execution, faultinject-proven crash recovery |
| `orchestrator` | this repo | Go API gateway that wires the above together and exposes them to a frontend |

Plus a React frontend (`frontend/`) rendering the reasoning transcript, confidence breakdown, and supporting visualizations (contradiction graph, structure/molecule viewers, sequence alignment, pathway viewer).

## Why not just multi-agent debate?

The original design ran a three-agent Advocate → Skeptic → Synthesizer debate pipeline before answering. It got measured against a single well-prompted call on a real n=10 eval (real Claude calls, real PubMed retrieval, externally-cited ground truth) and **lost on every metric at 7.4x the cost** (citation accuracy −0.45, +10pp unsupported-claim rate, worse verdict match, $0.106 vs $0.014 per claim).

The debate pipeline was killed as the default and is kept only as an explicit, reachable alternative (`POST /debate/multi-agent`) for the eval harness that measured it. `single_call` — one grounded retrieval + one rubric-anchored Claude call — is what ships. Measuring the fancier architecture and dropping it when the data said no, rather than keeping it because it sounds more impressive, is the point of the project as much as the retrieval pipeline is.

## Architecture

```
                    ┌──────────────┐
                    │   frontend   │  React 19 + Vite + TS
                    │  (port 5173) │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ orchestrator │  Go API gateway
                    │  (port 8080) │
                    └──┬───┬───┬───┘
           ┌───────────┘   │   └────────────┐
    ┌──────▼──────┐ ┌──────▼──────┐ ┌───────▼────────┐
    │  aletheia   │ │ biolab-mcp  │ │ workflow-engine │
    │ (port 8000) │ │ (port 8081) │ │  (worker pool)  │
    └──────┬──────┘ └──────┬──────┘ └────────┬────────┘
           │               │                  │
    ┌──────▼──────┐  ┌─────▼─────┐    ┌───────▼───────┐
    │  pgvector   │  │  postgres │    │ postgres+redis │
    │  (aletheia) │  │  + redis  │    │  (shared with  │
    └─────────────┘  └───────────┘    │  orchestrator) │
                                       └────────────────┘
```

`orchestrator` talks to `workflow-Engine` over its real durable boundaries (Postgres event log + Redis Streams), not a Go import — its execution internals are intentionally not exposed as a shared package, so the faultinject-proven crash-recovery guarantee can't be silently bypassed by a caller that skips the durable path.

## What's real vs. what's still fixture-fed

This project is built and documented against actual running code, not aspirational docs — worth stating plainly since that's not always true of portfolio repos:

- **Durable execution**: real, tested, faultinject-proven crash recovery on the queue-consumer execution path.
- **Reasoning**: real, tested — the debate-vs-single-call decision above is a real measured result, not a guess.
- **Retrieval audit trail**: real, published to PyPI + Docker, every retrieval gets a traceable ID.
- **Frontend**: builds clean, real implementations of most visualization components — currently rendering fixture data, not yet wired to a live end-to-end query.
- **The gateway route that connects `orchestrator` → `Aletheia`** for the killer query above is the current integration gap — the individual systems are proven, composing them into one live request/response path is the active work.

See [`HANDOFF.md`](./HANDOFF.md) for the full, continuously-updated state of what's verified, what's not, and what's next — written to survive a change in who (or what) is working on this next.

## Running it locally

```bash
cp .env.example .env   # add ANTHROPIC_API_KEY, OPENAI_API_KEY
docker compose up
```

This brings up `orchestrator` (:8080), `aletheia` (:8000), `biolab-mcp` (:8081), the `workflow-engine` worker pool, shared Postgres/Redis, and the frontend (:5173).

## Tech stack

**Backend:** Go (gateway, durable execution), Python/FastAPI (reasoning service), Postgres + pgvector, Redis Streams
**Frontend:** React 19, TypeScript, Vite, Vitest
**Infra:** Docker Compose, git submodules for the durable-execution engine

## Status

Portfolio / demonstration project — explicitly not a validated or regulatory-compliant system. No FDA/EMA or 21 CFR Part 11 claims apply.
