# Research-Orchestrator


An agentic research assistant that doesn't just retrieve scientific evidence — it debates it, gates it through confidence and human review, and shows its work end to end.

## What it does

Ask a research question (e.g. *"explain why mutation X reduces binding affinity"*) and Assay:

1. Decomposes it into an investigation plan
2. Pulls evidence across modalities — papers, protein structures, sequences, molecules, pathways — not just text
3. Runs it through a Proposer/Critic/Synthesizer debate, not a single model call
4. Scores confidence from multiple signals (literature, protein evidence, clinical evidence, LLM self-rating — capped at ≤15% weight, since a model rating itself isn't a calibrated probability)
5. Routes anything uncertain or irreversible to a human reviewer before any side effect fires
6. Executes idempotently — a notification or calendar event fires exactly once, even under retry or crash
7. Leaves every conclusion traceable back to its source, replayable, and diffable against the current literature

## Why

Biotech R&D has a specific trust gap: AI can accelerate literature triage and hypothesis generation, but a finding nobody can trace, re-check, or attribute to a specific reviewer isn't usable in a regulated environment. Assay is built around that constraint — reproducibility and human accountability, not just answer quality — in line with the FDA/EMA's January 2026 joint guidance on AI in drug development and ALCOA+ data-integrity principles.

## Architecture

Three planes:

- **Reasoning plane** (Aletheia / LangGraph) — the Planner and debate agents; Postgres-checkpointed, resumable, replayable
- **Durable execution plane** (Workflow Engine, Go) — owns exactly-once step execution, append-only event log, SHA-256 dedup keys
- **Evidence + action plane** (Biolab-MCP-Server, Go) — fail-closed ACID tool calls, content-addressed evidence storage, idempotent side effects

Four tool categories:

- **Retrievers** — PubMed, ChEMBL, UniProt, PDB, KEGG, BindingDB
- **Analyzers** — protein stability, docking, pipeline resource estimation, statistics
- **Visualizers** — structure viewer, molecule viewer, timeline, confidence heatmap, contradiction graph, tool execution graph
- **Executors** — Workflow Engine, calendar, notifications (confidence-gated, idempotency-keyed, never called directly by an agent)

## Signature design

A persistent **confidence ladder** runs down the left margin — styled like a gel electrophoresis lane. Each finding is a row; each row is four horizontal bands (literature / protein / clinical / LLM) whose brightness encodes signal strength. It's the confidence display, the primary navigation, and the loading state, all in one motif.

## Status

This is a portfolio/demonstration project. It is **designed to align with** 21 CFR Part 11 and ALCOA+ principles — it is not a validated GxP system, and nothing here should be read as a compliance claim. Fault-injection and reliability numbers reported anywhere in this repo are measured on this project's own test harness, not industry benchmarks.

## Roadmap

See [`ROADMAP.md`](./ROADMAP.md) for the step-by-step build order, design tokens, and data contracts.

## Stack

Go (Workflow Engine, Biolab-MCP-Server) · Python/LangGraph (Aletheia) · React + TypeScript (dashboard) · 3Dmol.js + RDKit.js (structure/molecule rendering)
