# Research Orchestrator — Build Roadmap

How to use this doc: work top to bottom, one step at a time. Each step has a goal,
a data contract, files to touch, and acceptance criteria. Do not start a step
until the previous one's acceptance criteria pass. Do not skip ahead to
"Later / Phase 2" items even if they look more fun — they depend on schemas
built in earlier steps.

## Decision: what this project is optimizing for

Fixing this now so it doesn't drift step to step: this project optimizes for
**visual distinctiveness and portfolio impact**, not maximum scientific
rigor. That means: prioritize the steps that are visible and demoable in
under a minute (tool execution graph, confidence heatmap, structure viewer)
over deeper-but-invisible correctness work. It does NOT mean skipping the
evaluation step below — a distinctive design that misrepresents evidence
is a worse demo, not a better one, the moment someone asks a follow-up
question.

## Design direction — read this before Step 0

The brief said explicitly: "I want this visually attractive, my other
projects are bland." Bland dashboards happen when every screen defaults to
white cards, a blue accent, and a sans-serif font stack picked by whatever
component library shipped it. Avoid that by grounding the design in the
actual subject — lab evidence and instrument readouts — not a generic
admin-panel template.

**Signature element:** a persistent vertical "confidence ladder" — styled
like a gel electrophoresis lane / seismograph strip — running down the left
margin of the dashboard. Each finding gets one row; each row is four
horizontal bands (literature / protein / clinical / LLM signal) whose
brightness encodes strength. This single motif does triple duty: it's the
Step 2 confidence heatmap, it's primary navigation (click a band, jump to
that finding), and it reappears as the loading/skeleton state everywhere
else in the app. One idea, reused everywhere, is what makes a design feel
authored rather than assembled.

**Palette** (name these as CSS variables in Step 0, use nowhere else as raw hex):
- `--bg`: `#EDEEE7` — cool lab-bench paper, not warm cream
- `--ink`: `#1B2420` — near-black with a green cast, primary text
- `--signal`: `#3DDC97` — bioluminescent teal-green, the ladder's "high
  confidence" glow and the only saturated color used for anything positive
- `--alert`: `#E85D4A` — coral-red, reserved ONLY for contradiction /
  disagreement signals (Step 3) so it stays meaningful, never decorative
- `--structural`: `#2C6E9E` — instrument blue, used for protein/molecule
  structural data only (Steps 4–5), never for UI chrome
- `--muted`: `#5B6B73` — slate, secondary text and hairline borders

**Typography:**
- Display / labels / data readouts: a monospace face (IBM Plex Mono or
  Space Mono) — this is the "instrument printout" register: evidence IDs,
  confidence numbers, tool latencies all render in mono, deliberately.
- Body / prose / claim text: a humanist sans (Inter or IBM Plex Sans) for
  actual readability where it matters — the claim text and citations.
- Never mix a third family in. Two faces, used consistently, is the rule.

**Layout concept:** the confidence ladder is a fixed left rail (~80px).
Main content to its right is NOT a card grid — it's a single scrolling
column of evidence entries, each styled like a specimen label: a thin
hairline rule above, a mono-font evidence ID (`EV-0042`) and timestamp,
then the claim in the body face, then the relevant visualizer (structure
viewer, molecule, timeline) inline beneath it. No drop shadows, no
rounded-corner cards floating on gray backgrounds — flat, bordered,
label-like.

**Restraint rule:** the ladder and the alert-red are the only two "loud"
elements in the whole app. Structure viewers, molecule renderings, and
timelines stay in `--structural` blue or grayscale. If a step's spec below
says to add a new accent color, don't — reuse one of the five above or ask
first.

## Tech stack for this build:
- Frontend: React + TypeScript, single dashboard app
- Mock data layer: JSON fixtures served from a local API (json-server or a
  thin Express/FastAPI mock) so the UI never talks to real APIs until Step 8
- Molecule/structure rendering: 3Dmol.js (protein structures), RDKit.js
  (SMILES → 2D), both free, no backend compute required
- Real connectors (Step 8+): PubMed, ChEMBL, UniProt — wrap each behind a
  single internal interface so swapping mock → real doesn't touch UI code

Tool category model (keep this as the organizing principle in the codebase):
- **Retrievers** — PubMed, UniProt, ChEMBL, PDB, KEGG, BindingDB
- **Analyzers** — protein stability predictor, docking, FlowCast, stats
- **Visualizers** — 3D structure viewer, molecule viewer, timeline, heatmap,
  contradiction graph, tool execution graph
- **Executors** — Workflow Engine, calendar, notifications (not in this
  roadmap — these already exist per the architecture doc; do not rebuild)

---

## Step 0 — Scaffold

**Goal:** empty but running app with the data contract in place, no real
features yet.

**Do:**
- Create the frontend app (React + TS + Vite).
- Set up the five CSS variables from the Design Direction section above
  and the two font families, before writing any component. Every later
  step styles from these variables — no ad hoc hex codes, no default
  component-library theme.
- Build the confidence-ladder rail as a static, non-functional shell first
  (fixed left column, correct width and colors, no data yet) so it's
  structurally present from the first commit, not bolted on in Step 2.
- Create a `fixtures/` directory with hand-written JSON files matching the
  schemas below — start with ONE example query end to end
  ("explain why mutation X reduces binding affinity"), not many.
- Create a single `EvidenceCard` type in TypeScript that every visualizer
  will consume. Every feature below renders into or reads from this shape:

```ts
type EvidenceCard = {
  id: string;
  claim: string;
  confidence: {
    overall: number;        // 0-1
    signals: {
      literature: number;
      protein_evidence: number;
      clinical_evidence: number;
      llm_rating: number;   // must be visually de-emphasized — see Step 2
    };
  };
  sources: EvidenceSource[];
  toolCalls: ToolCallTrace[];
};

type EvidenceSource = {
  id: string;
  type: "paper" | "protein_structure" | "sequence" | "molecule" | "pathway";
  title: string;
  retracted?: boolean;
  stance?: "supports" | "contradicts" | "neutral"; // used in Step 4
  refUrl?: string;
  payload: unknown; // shape depends on `type` — defined per-step below
};

type ToolCallTrace = {
  tool: string;            // e.g. "PubMed", "UniProt"
  category: "retriever" | "analyzer" | "visualizer" | "executor";
  latencyMs: number;
  cacheHit: boolean;
  retries: number;
  tokens?: number;
};
```

**Acceptance:** app runs locally, loads the one fixture, renders raw JSON on
screen. Nothing fancy yet — this step is about the contract, not the UI.

---

## Step 1 — Tool execution graph

Why first: cheapest to build, uses data you already have from the durable
execution plane's event log, and is the single best "this person understands
production agent systems" signal.

**Goal:** a flow diagram — Planner → [Retrievers in parallel] → Evidence
Merge → Critic → Synthesizer → Workflow → Executor — where each node shows
latency, retries, cache hit, and token usage on hover/click.

**Do:**
- Render nodes from `ToolCallTrace[]` on the `EvidenceCard`.
- Node color: green tint if cacheHit, gray if not. Retries > 0 gets a small
  warning indicator — don't invent a new color ramp, reuse existing status
  colors if this is a Claude Design/Visualizer context, or a simple
  amber-on-hover badge if plain React.
- Click a node → side panel with the raw trace (tool name, latency, retries,
  tokens).

**Acceptance:** given the one fixture, the graph renders with correct edges
and every node's hover state shows real numbers from the fixture (not
placeholders).

---

## Step 2 — Confidence heatmap

**Goal:** replace a single "83% confidence" number with a per-signal bar
breakdown: literature, protein evidence, clinical evidence, LLM rating.

**Do:**
- Render `confidence.signals` as four horizontal bars, not four
  arbitrary colors — order them the same way every time so users learn the
  layout.
- The LLM rating bar must be visually the smallest/least prominent by
  design intent (it's capped at ≤15% weight in the actual gate) — don't let
  it visually dominate just because its number happens to be high in a
  given case. Consider a fixed max-width scale so all four bars share one
  axis.
- Add a one-line tooltip per signal explaining what it means in plain
  language (e.g. "Literature: how many independent papers support this").

**Acceptance:** two fixtures — one where literature is high and LLM rating
is low (should look trustworthy), one where it's reversed (should visually
read as "be skeptical of this one"). The visual difference must be obvious
without reading numbers.

---

## Step 3 — Contradiction graph

This step requires a schema decision before any UI work — do this first.

**Goal:** don't average away disagreement between sources. Show it.

**Do:**
- Extend `EvidenceSource.stance` (already in the Step 0 schema) to be
  populated by the Critic agent, not inferred later. If you're mocking this
  for now, hand-write fixtures where two sources have opposite stances on
  the same claim.
- Render as a simple two-column layout: "Supports" sources on one side,
  "Contradicts" on the other, both pointing at the central claim node.
  Do NOT try to force this into a circular/network graph — a two-column
  layout with a shared center node is far easier to keep readable and
  avoids crossed edges.
- Hovering an edge shows the exact source excerpt (paraphrased, not
  verbatim — respect copyright even in your own fixtures/demo data) that
  drives the stance.

**Acceptance:** a fixture with 3 supporting + 2 contradicting sources on one
claim renders with zero crossed lines and the claim node visually centered
between the two columns.

---

## Step 3.5 — Evaluation guardrail (small, don't skip)

Why this exists: a visualization that looks authoritative but silently
misrepresents the underlying debate is worse than no visualization —
confidence isn't a calibrated probability, and neither is a nice-looking
graph unless it's checked.

**Do:**
- Hand-label 2-3 fixtures with known-correct contradiction stances and
  confidence signal values.
- Write a small test asserting the rendered graph/heatmap matches the
  hand-labeled ground truth — not a pixel-diff, just: does the "supports"
  column contain exactly the sources marked supports, does the highest bar
  actually correspond to the highest signal value.

**Acceptance:** test passes on all 2-3 fixtures before Step 4 starts.

---

## Step 4 — Structure viewer (protein + mutation)

**Goal:** one modality, done well, before touching any others.

**Do:**
- Add 3Dmol.js via CDN. Load a single PDB ID from a fixture (pick a real,
  well-known one, e.g. a kinase structure, for realistic testing).
- Highlight two things distinctly: the mutation residue and the binding
  pocket — different colors, with a small legend.
- `EvidenceSource.payload` for `type: "protein_structure"`:

```ts
type ProteinStructurePayload = {
  pdbId: string;
  mutationResidue?: { chain: string; position: number; };
  bindingPocket?: { chain: string; positions: number[]; };
};
```

**Acceptance:** viewer loads and rotates smoothly, mutation and pocket are
visually distinguishable at a glance, no console errors on load.

---

## Step 5 — Molecule visualization

**Goal:** ligand/compound rendering from ChEMBL/BindingDB-shaped data.

**Do:**
- Add RDKit.js (or a lightweight SMILES→2D renderer) via CDN.
- `EvidenceSource.payload` for `type: "molecule"`:

```ts
type MoleculePayload = {
  smiles: string;
  molecularWeight?: number;
  target?: string;
  assay?: string;
};
```
- Render: 2D structure image + a small metadata table (MW, target, assay)
  next to it — not raw JSON.

**Acceptance:** fixture SMILES renders correctly as a 2D structure; metadata
table shows real values from the fixture.

---

## Step 6 — Sequence alignment viewer

**Goal:** show conserved regions between two sequences, not a similarity %.

**Do:**
- Simple pairwise alignment display (can use a basic diff algorithm — this
  doesn't need a real bioinformatics aligner yet for the demo; note this
  clearly in code comments as "simplified for demo, not a production
  aligner" so it's never mistaken for validated output later).
- Render as two stacked sequence rows with a match-line between them
  (`|` for match, blank for mismatch), conserved regions visually
  highlighted (background tint).

**Acceptance:** fixture pair renders with correct match/mismatch marks and
highlighted conserved runs of 3+ residues.

---

## Step 7 — Timeline visualization

**Goal:** chronological view of a drug's history (discovery → trials →
approval → publication).

**Do:**
- Requires a `TimelineEvent` type: `{ date: string; label: string; type:
  "discovery" | "trial_phase" | "approval" | "publication"; sourceId: string }`.
- Render as a horizontal timeline, one dot per event, connected by a line,
  labeled above/below alternating to avoid overlap at dense periods.

**Acceptance:** fixture with 5+ events across several years renders in
correct chronological order with no overlapping labels.

---

## Step 8 — Wire real Retrievers (first real API integration)

Only start this once Steps 1–7 work end-to-end against fixtures.

**Goal:** replace fixtures with real calls for PubMed, ChEMBL, UniProt —
one at a time, in that order (PubMed is lowest friction to start with).

**Do:**
- Define one interface, e.g. `fetchEvidence(query, sourceType):
  Promise<EvidenceSource[]>`, and implement it per-connector. UI code should
  not change at all when you swap a mock implementation for a real one —
  if it does, the interface boundary from Step 0 wasn't clean enough; fix
  the boundary before continuing.
- Add response caching (this feeds the `cacheHit` field from Step 1 for
  real, instead of a mocked boolean).
- Rate-limit and error-handle each connector independently — one connector
  failing must not blank out the other evidence types on the card.

**Acceptance:** the Step 1 tool execution graph shows real latencies and a
real cache hit/miss for at least one live query, end to end.

---

## Phase 2 backlog — do not start until everything above is solid

Write these down now so they aren't lost, but treat them as separate
projects, not "step 9, 10, 11":

- **Pathway diagrams (KEGG/Reactome)** — check KEGG's redistribution
  licensing before building any UI around their diagrams; may need to
  redraw pathways from raw data rather than embedding KEGG's own images.
- **Sandbox simulators** (docking, protein stability prediction, RNA-seq
  resource estimation) — each is a real domain-software integration
  (e.g. AutoDock Vina, an actual stability predictor), not agent glue.
  Label every result in the UI with the specific tool/method used and its
  known limitations — never let an approximate score look authoritative.
  - **PyMOL** (open-source build, not the licensed Schrödinger one) is a
    concrete option for the stability/mutation Analyzer: run it headless,
    server-side, for alignment, RMSD, clash detection, and SASA on a
    mutated structure. Do NOT use it for the web viewer itself — keep
    3Dmol.js/RDKit.js for in-browser rendering (Steps 4-5) and let PyMOL
    only produce structured numeric output (coordinates, scores) that
    those viewers consume. Wrap it as an MCP tool with a small fixed set
    of operations (align, mutate+clash-check, compute RMSD) — never expose
    raw PyMOL scripting to the agent, for the same fail-closed reasons the
    architecture doc gives for every other tool call.
  - **scverse (scanpy/anndata/scvi-tools)** is a different modality
    (single-cell RNA-seq) and a bigger addition than it looks — real h5ad
    ingestion, meaningfully more compute. Only add it if a single-cell use
    case is actually part of your one concrete anchor question; don't pull
    it in just because the tooling exists in this environment.
- **Experiment planner / clinical trial eligibility checker** — real
  correctness bar, real domain logic. A wrong "you have enough statistical
  power" or wrong eligibility match is a credibility risk, not a bug to
  patch later. Needs domain review before shipping, not just testing.

## Explicit non-goals for this roadmap

- No image generation / decorative media — every visual must answer "does
  this help validate, analyze, or explain evidence?"
- No rebuilding the Executors (Workflow Engine, calendar, notifications) —
  those already exist per the architecture doc; this roadmap only adds
  Retriever/Analyzer/Visualizer features on top.
- No jumping to multi-tenancy, RBAC, or the Rust gateway from the earlier
  architecture doc — this roadmap is scoped to the evidence/visualization
  layer only.
