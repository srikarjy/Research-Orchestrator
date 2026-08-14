# Handoff: Research-Orchestrator — state, decisions, and what's left

Written for whoever (or whatever coding tool) picks this project up next.
Everything in this document was verified against real code, real builds,
real tests, and real running containers during the session that produced
it — not assumed, not inferred from READMEs. Where a claim couldn't be
verified, it's marked as such. Keep that standard going forward (see
"Engineering standard to maintain" below) — it's the main thing that
separates this project from the "vibe-coded" state it started this
session in.

---

## 1. The pitch (what this project is for)

**Target user:** a computational/translational biologist at a Kendall
Square-type biotech or an academic lab (Broad/MIT-adjacent) deciding
whether a drug target is worth pursuing before committing bench time or
budget.

**Killer query:** *"Does mutation X actually reduce binding affinity for
this drug, and does the literature agree?"*

**What the product should do, end to end:** the user asks that question
once. The system retrieves real literature (audited, traceable), resolves
the claim with a real LLM call that surfaces contradictions instead of
averaging them away, breaks confidence down by signal type instead of one
fake number, and a frontend renders the real transcript — not fixtures.

**Why this beats "just ask Claude/ChatGPT":** grounded, audited
retrieval (every citation traces to a real `retrieval_id`) and a
structured, contradiction-aware, rubric-anchored confidence score are the
things a bare chat conversation doesn't give a scientist. That's the
actual value proposition — not multi-agent debate theater (see §3.2,
this was tested and killed for a documented reason).

---

## 2. Repo map — what's canonical, what's not

| Repo | Path/URL | Role | State |
|---|---|---|---|
| `Research-Orchestrator` | `github.com/srikarjy/Research-Orchestrator` (this repo) | Product layer: `orchestrator` (Go gateway) + `biolab-mcp-server` copy + frontend | Builds clean, partially wired |
| `workflow-Engine` | `github.com/srikarjy/workflow-Engine`, submodule at `./workflow-engine` | Durable execution: real Saga pattern, faultinject-proven crash recovery | Real, tested, canonical |
| `Aletheia` | `github.com/srikarjy/Aletheia` (separate repo, **not** a submodule here) | Reasoning: real grounded retrieval + Claude synthesis | Real, tested, canonical — **not yet called by orchestrator** |
| `biolab-mcp-server` (published) | `github.com/srikarjy/biolab-mcp-server` | Audit/logging interception layer, PyPI+Docker published | Real, canonical for retrieval audit trail — **different from the copy vendored in this repo** |

**Important naming collision, still unresolved:** `Research-Orchestrator/biolab-mcp-server/`
is a *different, bigger* Go HTTP service (agent roster, analyzers, sandbox,
tool registry) than the published `srikarjy/biolab-mcp-server` (a lean,
focused, audit-log-only interceptor that Aletheia actually depends on).
They share a name and nothing else. This was flagged early in the
consolidation work and never resolved — see §5.3.

**Commit state as of this handoff** (all pushed, nothing pending):
- `Research-Orchestrator`: `23c067c`
- `workflow-engine` submodule: `84a230a`
- `Aletheia`: `c9aed96`
- `biolab-mcp-server` (vendored copy in this repo): `1969df0`

---

## 3. What's real and verified right now

Verification method used throughout, keep using it: `go build ./...` +
`go vet ./...` + `go test ./...` (or `pytest`) for every change, plus at
least one test proving the *failure* path works too, not just the happy
path, before calling anything done.

### 3.1 Durable execution (`workflow-Engine`)
- Real Saga-pattern compensation, idempotent step execution via SHA-256
  dedup keys, faultinject-proven crash recovery — but **only on the
  queue-consumer path** (`ProcessStep`). The synchronous path
  (`ExecuteWorkflow`) has zero faultinject checkpoints — don't trust its
  crash-recovery properties without adding them first.
- `orchestrator` talks to it over its real durable boundaries (Postgres
  event log + Redis Streams), not a Go import — its execution logic is
  `internal/`-only by design, and promoting it to `pkg/` was deliberately
  rejected (would dilute the faultinject proof). See
  `orchestrator/internal/wfengine/client.go`'s package doc for the full
  reasoning.
- Shared Postgres+Redis topology between `orchestrator` and
  `workflow-Engine` is live-verified (docker compose up, real workflow
  row → real StepMessage → real worker → real event log write, watched
  end to end).
- **Drift guardrail exists**: `orchestrator/internal/wfengine/integration_test.go`
  (opt-in via `WFENGINE_INTEGRATION=1`, needs Docker) proves the
  hand-mirrored schema/wire format stays in sync with the real repo.
  Confirmed both ways: passes against the real contract, fails when the
  wire format is deliberately broken.
- New: `metalsw`'s `gpu_main` binary has a real `StepExecutor`
  (`workflow-engine/internal/steps/metalsw.go`), tested against the real
  stdout contract (`%-20s %d` per hit), treats an oracle mismatch
  (non-zero exit) as a hard failure, not a silent partial result.

### 3.2 Reasoning (`Aletheia`)
- **The debate-vs-single-model question is answered, not open.** Real
  n=10 eval (`scripts/run_phase6.py`, real Claude calls, real PubMed
  retrieval, externally-cited ground truth labels): the three-agent
  Advocate→Skeptic→Synthesizer debate pipeline **underperformed a single
  well-prompted call on every metric measured, at 7.4x the cost**
  (citation accuracy −0.45, +10pp unsupported-claim rate, worse verdict
  match, $0.106 vs $0.014 per claim). This is decisive, not inconclusive
  — an earlier n=5 partial run had looked more ambiguous; it didn't
  survive the full run.
- **`POST /debate` now defaults to `single_call`**
  (`app/agents/single_call.py`): real grounded retrieval + one
  rubric-anchored Claude call. Kept from the debate pipeline because
  these parts were actually earning their keep: provenance-per-paper
  retrieval, rubric v1 confidence (unchanged), code-enforced citation
  integrity (bad citation → one corrective retry → hard failure).
- The debate pipeline is **not deleted** — real, tested, reachable
  explicitly at `POST /debate/multi-agent` and still what
  `/batch/eval/run` exercises, since it's the eval harness's subject.
- **Silent mock fallback is fixed.** `search_pubmed` used to auto-fall-back
  to mock PubMed data with only a `UserWarning` if `BIOLAB_PROJECT_PATH`
  wasn't set — meaning every "mocks off, real data" claim about a past
  debate run was unverifiable. Now a hard `RuntimeError`. Mock mode is
  opt-in only (`MOCK_RETRIEVAL=true`).
- **Aletheia's retrieval integration is a filesystem/subprocess hack**,
  not an HTTP call: `BIOLAB_PROJECT_PATH` points at a local checkout of
  the *old, different* Biolab MCP Server (SQLite-backed, stdio MCP
  protocol), spawned as a subprocess. This has never been pointed at the
  real published `srikarjy/biolab-mcp-server` package/service. Fixing
  this is arguably higher priority than anything in §4 — see §5.1.

### 3.3 Evidence retrieval (`biolab-mcp-server`, published)
- Real, focused, PyPI+Docker published, green CI, dual Python/Go. Does
  one thing: intercept + audit-log every retrieval with a `retrieval_id`.
- **Gap, confirmed real (not fixed yet):** `retrieval_log.py`'s
  `response_hash` is `sha256(raw_response)` only — it does not chain to
  the previous row's hash. A deleted bad retrieval is currently
  invisible. See §4.3.

### 3.4 Product layer (`orchestrator`, `biolab-mcp-server` copy, frontend — this repo)
- `orchestrator` replaced `assayos`, which never compiled (found and
  fixed: illegal cross-module `internal/` imports, missing imports, a
  raw `interface{}` where a real type belonged). Now builds/vets clean.
- **`orchestrator` does not call Aletheia at all, anywhere, currently.**
  `orchestrator/internal/services/biolab.go` and the vendored
  `biolab-mcp-server/` copy's agents (Planner/Researcher/Critic/etc.) are
  a *separate, mock-heavy* system that has nothing to do with Aletheia's
  real `single_call`/`/debate` endpoint. This is the single biggest gap
  between "the plumbing works" (true) and "the product answers the
  killer query" (not true yet). See §5.1 — this should be the next thing
  worked on, ahead of the lower-priority items in §4.
- Frontend (`frontend/`) builds clean, 19/19 tests pass, has real
  implementations of most of the ROADMAP's visualization components
  (confidence heatmap, contradiction graph, structure/molecule viewers,
  sequence alignment, timeline, pathway viewer, tool execution graph).
  **All of it is fixture-fed.** `ConfidenceLadder.tsx` — the intended
  signature visual element — is still a 17-line non-functional shell.
- CI (`.github/workflows/ci.yml`) only tests `workflow-engine` and
  `biolab-mcp-server`. It never touches `orchestrator` or the frontend.
  That's exactly how `assayos` was allowed to silently never compile for
  as long as it did — fix this (§4.4) before it happens again.

---

## 4. What's left — prioritized, with concrete steps

### Priority 0 — the actual product gap (do this first, it's not on the original list)

**Wire `orchestrator` (or a new thin endpoint) to Aletheia's real `/debate`
endpoint, and Aletheia's retrieval to the real published `biolab-mcp-server`
instead of the filesystem/subprocess hack.**

Concrete steps:
1. ~~Fix Aletheia's retrieval integration~~ — **already correct, verified
   live during this handoff, no code change needed.** The local checkout
   at `/Users/srikarjy/resume_projects/Biolab MCP Server` (note: outside
   this repo, a sibling directory) IS a real clone of
   `srikarjy/biolab-mcp-server`, with a working `.venv` and a real
   `biolab.db`. `mcp_client.py`'s `stdio_client`/`-m biolab.server`
   subprocess spawn is the *correct* mechanism (that repo's `server.py`
   is a real `FastMCP` stdio server exposing `search_pubmed` with exactly
   the shape `mcp_client.py` expects). Verified end-to-end during this
   handoff: a real `search_pubmed("BRAF V600E binding affinity", ...)`
   call returned 2 real PubMed papers with real `retrieval_id`s, and both
   showed up as real rows in `biolab.db`'s `retrievals` table
   (`sqlite3 biolab.db "SELECT * FROM retrievals ..."` — confirmed).
   **The only real requirement going forward: whatever machine runs
   Aletheia needs that sibling checkout present with its `.venv` set up**
   (or `BIOLAB_PROJECT_PATH`/`BIOLAB_DB_PATH` repointed at wherever it
   lives) — that's a deployment/environment concern, not a code bug.
2. In `orchestrator`, add a thin HTTP client (new package, e.g.
   `orchestrator/internal/aletheia/client.go`) that calls Aletheia's
   `POST /debate` with a claim string and gets back `DebateResponse`
   (`debate_id, claim, conclusion, verdict, confidence,
   confidence_rationale, driving_provenance_ids, transcript, sources`).
   Wire this into a new gateway route, e.g. `POST /api/v1/query`.
3. Decide (and document the decision, same pattern as §3.2): does
   `orchestrator`'s durable-execution layer wrap this call as a
   `workflow-Engine` step (so it's crash-recoverable, idempotent), or is
   a single external HTTP call to Aletheia acceptable to run outside that
   guarantee for now? Given Aletheia's own latency (~single Claude call,
   seconds not ~74s like the old debate path) and that Aletheia has its
   own job queue/caching already, wrapping it as a durable step may be
   redundant — but decide explicitly, don't leave it ambiguous like the
   debate-vs-single-call question was.
4. Verify like everything else this session: real HTTP round trip,
   `orchestrator` → Aletheia → real (or explicitly mocked, opt-in)
   retrieval → real structured response → confirmed via `curl`/test, not
   assumed from reading the code.

This is what turns "the plumbing works" into "the product works."
Everything below this line is real but secondary to it.

### Priority 1 — `workflow-Engine` (in the submodule, push to `srikarjy/workflow-Engine` when done)

- [ ] Extend the faultinject harness to the metalsw step specifically:
      `SIGKILL` mid-`gpu_main`-execution, confirm the dedup key prevents
      a duplicate (expensive) GPU run on recovery. Add the real recovery
      number to the README's benchmark table. Follow the existing
      pattern in `cmd/faultinject/main.go` and `internal/faultinject/`.
- [ ] `ExecuteAgentic` mode: next step decided at runtime (by a
      router/model) based on the previous tool call's output, instead of
      a fixed `WorkflowDefinition`. Reuses the existing
      idempotency/event-log machinery per step — the new part is only the
      decision loop.
- [ ] Extend the dedup-key concept to routing decisions, not just step
      execution — replay a cached decision for a previously-seen task
      context instead of re-asking a model.
- [ ] Test concurrent workers on the same Redis Streams queue under real
      contention (multiple workers racing for one GPU) — proves
      idempotency holds under concurrent load, not just one crash at a
      time (which is already proven).
- [ ] Build the notification executor: confidence-gated (only fires above
      a threshold), idempotency-keyed (same dedup mechanism, so a retry
      never double-sends), never callable directly by an agent — must go
      through the same durable step/event-log path as everything else.

### Priority 2 — `Aletheia`

Both items originally listed here are done (§3.2). Nothing else queued
here except what Priority 0 surfaces.

### Priority 3 — `biolab-mcp-server` (published repo — clone fresh, don't reuse a temp dir)

- [ ] Hash-chain `retrieval_log.py`: each row's hash must incorporate the
      previous row's hash, not just its own response
      (`hashlib.sha256(raw_response)` today). Without this, a deleted bad
      retrieval is invisible — this is the actual audit-trail
      differentiator and it's currently unverified against tampering.
- [ ] Build and test a retraction-status check wired to a live source
      (Crossref or PubMed's own retraction API) — confirm it's real and
      tested, not just claimed in the README.
- [ ] Auth/tenant isolation, NCBI API key + rate limiting (already
      flagged, not new).

### Priority 4 — Frontend + `orchestrator`, once Priority 0 exists

- [ ] Build out `ConfidenceLadder.tsx` for real — it's the signature
      visual element and everything else is already built around it.
- [ ] Wire the new `/api/v1/query` (or whatever Priority 0 lands as) so
      `frontend/src/App.tsx`'s fixture fallback stops firing for at least
      one real query path.
- [ ] Verify the 4-signal confidence-weighting logic (literature/protein/
      clinical/LLM ≤15%) against Aletheia's actual `confidence` output —
      Aletheia's `single_call` currently returns one scalar `confidence`
      plus `confidence_rationale`, not four separate signals. Either
      extend `single_call`'s tool schema to return per-signal breakdowns,
      or decide explicitly that this is a v2 feature — don't let the
      frontend silently fake four bars from one number.
- [ ] Wire the notification executor (once real, from Priority 1) into
      `NotificationPanel.tsx`.
- [ ] Chain-of-custody visual redesign — only after a component is
      rendering live data, not before (don't restyle a fixture).
- [ ] **Fix CI** (`.github/workflows/ci.yml`) to actually build/vet/test
      `orchestrator` and the frontend, not just `workflow-engine` and
      `biolab-mcp-server`. This is how `assayos` silently rotted
      undetected for as long as it did.

### Priority 5 — Security guardrails (cross-cutting, do alongside Priority 0)

- [ ] Prompt-injection guardrail for retrieved literature fed to Claude:
      structurally separate retrieved text from instructions; treat
      instruction-like text found inside a retrieved abstract as a flag,
      not a command. This matters more once Priority 0 lands (real
      retrieved text reaching a real model).
- [ ] Rate limiting on the public query endpoint.
- [ ] Secrets management (NCBI key, Claude API creds) — out of source
      control, rotated.
- [ ] Access control on who can trigger a workflow replay.

### Priority 6 — Cost control (do alongside Priority 0, cheap and high-leverage)

- [ ] Model cascading — cheap model (Haiku-tier) for any routing
      decisions, expensive model reserved for final synthesis only.
- [ ] Prompt caching (`cache_control` breakpoints after tool definitions
      and after each tool result).
- [ ] Deterministic router before any model call, for predictable
      sequences — only escalate to a model when it genuinely can't
      decide.

### Priority 7 — `ProteinSearch-GPU` (deferred, needs a real NVIDIA GPU, not this machine)

- [ ] FastAPI wrapper around the existing `protein_search_gpu` package.
- [ ] Go `StepExecutor` (HTTP-client version, can be written/unit-tested
      without a GPU).
- [ ] One real end-to-end benchmark on a short-lived cloud GPU instance.

---

## 5. Known traps and things that will waste your time if rediscovered

### 5.1 Two different "biolab-mcp-server"s
`Research-Orchestrator/biolab-mcp-server/` (this repo, vendored) and
`srikarjy/biolab-mcp-server` (published, real, what Aletheia depends on)
share a name and nothing else. Don't assume code in one applies to the
other. Not resolved — see Priority 0 step 1.

### 5.2 Go `internal/` package visibility across modules
`orchestrator`, `biolab-mcp-server`, and `workflow-Engine` are three
separate Go modules. Anything under `internal/` in one is **illegal to
import** from another module — this is what made `assayos` never compile,
and it's why `orchestrator` talks to `workflow-Engine` over its real
Postgres/Redis boundaries instead of a Go import. If you hit
`use of internal package ... not allowed`, this is why — the fix is
either promote the specific thing to `pkg/` (if it's safe to expose,
e.g. data types) or go over the real transport boundary (safer for
anything execution-critical, see §3.1's faultinject-coverage point).

### 5.3 `workflow-Engine`'s faultinject coverage is narrower than it sounds
The crash-recovery guarantee is proven for `ProcessStep` (the
queue-consumer path) only. `ExecuteWorkflow` (synchronous, in-process) has
zero faultinject checkpoints. Don't cite "faultinject-proven crash
recovery" for anything that goes through `ExecuteWorkflow` without adding
checkpoints there first.

### 5.4 Docker port conflicts between this repo's compose and other local stacks
`docker-compose.yml` here maps Postgres to host `5434` and Redis to host
`6379`. A standalone `workflow-Engine` checkout's *own* `docker-compose.yml`
(if someone runs it separately) also wants `6379` by default. If
`docker compose up` fails on a Redis bind error, check `docker ps` for a
leftover `workflow--redis-1`-style container before assuming something's
broken.

### 5.5 Nested config structs in `orchestrator`'s `kernel.Config` need manual wiring
`cmd/assayos/main.go` does NOT automatically bind environment variables to
nested `Config` struct fields via `viper.AutomaticEnv()` — only top-level
fields that have an explicit `cfg.X = viper.GetString("x")` line in
`runServer()` actually get read from the environment. If you add a new
nested config field (like `WorkflowEngine.DSN` was), you must add the
explicit override line too, or docker-compose env vars for it will be
silently ignored. This bit us once already (§ this session's history).

### 5.6 Postgres init scripts only run on a fresh volume
`deploy/postgres-init/*.sh` only executes on a container's *first* init
against an empty data directory. If you change an init script and
`docker compose up` doesn't seem to pick it up, you likely need
`docker volume rm research-orchestrator_postgres-data` first (safe for
local dev data, not for anything you care about keeping).

---

## 6. Engineering standard to maintain

This is what changed the project from "vibe-coded" to real, over this
session. Keep doing it:

1. **Never trust a README's claim without checking the code.** Multiple
   claims in this project's own docs turned out stale or wrong when
   checked (an "inconclusive" eval result that was actually decisively
   negative; a `docker-compose.yml` service that was really an empty
   4-line stub; a "drafted" StepExecutor that turned out to be genuinely
   real and correctly specified — check both directions, don't assume
   pessimistically either).
2. **`go build && go vet && go test` (or `pytest`) before calling
   anything done.** Not optional, not "should build."
3. **Prove the failure path, not just the happy path**, for anything
   that claims to guard against a real failure mode (drift, bad
   citations, misconfiguration). A guardrail that's never been seen to
   fail hasn't been proven to work — this session's integration test and
   the citation-integrity tests were specifically verified by making them
   fail on purpose first.
4. **Don't silently make product/architecture decisions — document them
   with the reasoning, in the code and the README**, the way the
   debate-vs-single-call resolution was documented. The next person (or
   agent) shouldn't have to re-derive why a decision was made.
5. **Small, logically separate commits with a "why," not a "what."** Look
   at this session's commit history for the pattern.
6. **Never push without confirming** — local commits are cheap and
   reversible; a push to a public repo is not. The one exception this
   session made was protecting already-approved, already-tested work
   from being lost in an ephemeral temp directory when the tool/session
   was about to change — that's a "prevent data loss," not a "make a new
   decision," judgment call.
7. **When something looks too good (a "code already drafted" claim, an
   impressive README) or too broken (a claim of total dysfunction),
   verify before reacting either way.** Both happened this session.
8. **Never add a `Co-Authored-By: Claude` (or any AI) trailer to commits
   in this project or any repo it touches.** Cardinal rule, stated
   repeatedly across nearly all of this user's repos — these are
   portfolio projects shown to recruiters, and sole authorship matters.
   If a tool's default behavior adds one, strip it before committing.

---

## 7. Portfolio framing — how to talk about this project

**Don't lead with "three-plane architecture" or buzzword-dense compliance
language** (21 CFR Part 11, ALCOA+) unless you can back every claim on
demand — that was this project's original failure mode, and it's an easy
tell for a technical reviewer. Lead with the actual, checkable story:

> "I built five focused, independently real systems — a durable
> execution engine with proven Saga-pattern crash recovery, an
> audit-logging retrieval layer, a reasoning service, a GPU-accelerated
> protein search tool, a pharmacogenomics graph explorer — then built the
> product layer that composes them into one thing a scientist can
> actually use. Along the way I ran a real evaluation of my own core
> architectural bet (multi-agent debate vs. a single well-prompted call),
> and when the data said the fancier approach lost on every metric at 7x
> the cost, I killed it and shipped the one that measured better. That's
> the engineering story: not 'I built something that sounds impressive,'
> but 'I built something, measured it, and changed course on real
> evidence.'"

**What to actually demo, once Priority 0 is done:** the killer query, live,
end to end — real retrieval with a visible `retrieval_id`, a real
confidence/contradiction breakdown, not a canned fixture. A shorter,
fully-real demo beats a longer, partially-fake one every time a technical
reviewer starts asking "wait, is this actually calling anything real?"

**What NOT to claim** until it's true: FDA/EMA compliance alignment,
21 CFR Part 11 alignment, or any regulatory-adjacent language — none of
that has been built or verified. If asked, "portfolio/demonstration
project, explicitly not a validated system" is the honest, defensible
answer, and it's already in this repo's own README.
