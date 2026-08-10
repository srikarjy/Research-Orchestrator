# Architecture Overview

## Three-Plane Design

The Research Orchestrator separates concerns into three independent planes that communicate via well-defined contracts.

```
┌─────────────────────────────────────────────────────────────────┐
│                        USER QUERY                                │
└────────────────────────────┬────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  PLANE 1: REASONING (Aletheia)                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Planner → Researcher → Critic → Synthesizer             │   │
│  │ • LangGraph state machine                               │   │
│  │ • Message bus for agent communication                   │   │
│  │ • Outputs: InvestigationPlan, EvidenceCard              │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  PLANE 2: DURABLE EXECUTION (Workflow Engine)                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ • Idempotent step execution (SHA-256 dedup keys)        │   │
│  │ • Append-only event log (PostgreSQL/SQLite)             │   │
│  │ • WebSocket broadcasting for real-time UI               │   │
│  │ • Retry with exponential backoff                        │   │
│  │ • Human review gating (awaiting_review status)          │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────────────────────┬────────────────────────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│  PLANE 3: EVIDENCE + ACTION (Biolab-MCP)                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │ Retrievers (6): PubMed, UniProt, ChEMBL, PDB, KEGG,    │   │
│  │            BindingDB                                    │   │
│  │ Analyzers (4): Stability, Docking, EvidenceMerge,      │   │
│  │              Critic                                     │   │
│  │ Visualizers (2): StructureViewer, MoleculeViewer       │   │
│  │ Agents (9): Planner, Researcher, Critic, Executor,     │   │
│  │             Validator, Notifier, ClinicalTrial,        │   │
│  │             Regulatory, Biomarker                       │   │
│  │ Sandbox: Isolated experiment execution                 │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Communication Contracts

| From → To | Protocol | Schema |
|-----------|----------|--------|
| Aletheia → Workflow Engine | HTTP POST | `InvestigationPlan` → `Workflow` |
| Workflow Engine → Biolab-MCP | HTTP POST | `ToolCallTrace` |
| Biolab-MCP → Workflow Engine | Artifacts in Result | `EvidenceSource`, `ToolCallTrace` |
| Workflow Engine → Frontend | WebSocket | `WorkflowEvent` stream |

## Data Flow Example

**Query**: `"explain why BRAF V600E reduces binding affinity"`

1. **Aletheia** creates `InvestigationPlan` with 8 tasks
2. **Workflow Engine** creates workflow, assigns idempotency keys
3. **Workflow Engine** executes steps in dependency order:
   - PubMed, UniProt, ChEMBL (parallel retrievers)
   - ProteinStabilityPredictor (depends on UniProt)
   - Docking (depends on Stability + ChEMBL)
   - EvidenceMerge (depends on PubMed + Docking)
   - Critic (depends on Merge)
   - StructureViewer (depends on Critic)
4. **Biolab-MCP** runs each tool, returns artifacts
5. **Workflow Engine** broadcasts events via WebSocket
6. **Frontend** renders real-time: progress → heatmap → contradiction graph → 3D structure

## Why This Separation?

| Concern | Plane | Reason |
|---------|-------|--------|
| LLM prompting, agent logic | 1 | Iterate fast, Python ecosystem |
| Reliability, replay, audit | 2 | Go performance, event sourcing |
| Domain tools, bio compute | 3 | Go performance, sandbox isolation |

Each plane can be scaled, replaced, or upgraded independently.