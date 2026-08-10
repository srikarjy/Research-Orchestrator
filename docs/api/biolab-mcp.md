# Biolab MCP Server API

Base URL: `http://localhost:8081`

## Agents

### List Agents
```http
GET /api/v1/agents
```

### Get Agent Status
```http
GET /api/v1/agents/{id}/status
```

## Workflows (MCP-style)

### Create Workflow
```http
POST /api/v1/workflows
Content-Type: application/json

{
  "name": "Full Pipeline: BRAF V600E",
  "description": "explain why BRAF V600E reduces binding affinity",
  "tasks": [
    {"id": "t1", "type": "plan", "description": "Decompose query", "input": {"query": "..."}, "priority": 10, "dependencies": []},
    {"id": "t2", "type": "research", "description": "Retrieve evidence", "input": {"query": "..."}, "priority": 8, "dependencies": ["t1"]},
    {"id": "t3", "type": "analyze", "description": "Analyze evidence", "input": {}, "priority": 6, "dependencies": ["t2"]},
    {"id": "t4", "type": "synthesize", "description": "Synthesize findings", "input": {}, "priority": 4, "dependencies": ["t3"]}
  ],
  "metadata": {"pipeline": true}
}
```

### List Workflows
```http
GET /api/v1/workflows
```

### Get Workflow
```http
GET /api/v1/workflows/{id}
```

### Execute Workflow
```http
POST /api/v1/workflows/{id}/execute
```

### Delete Workflow
```http
DELETE /api/v1/workflows/{id}
```

## Tools

### List Tools
```http
GET /api/v1/tools
GET /api/v1/tools/{category}
```

Categories: `retriever`, `analyzer`, `visualizer`, `executor`

### Get Tool Schema
```http
GET /api/v1/tools/{category}/{name}/schema
```

### Execute Tool
```http
POST /api/v1/tools/{category}/{name}/execute
Content-Type: application/json

{"input": {...}}
```

## Retrievers (Convenience)

### PubMed
```http
POST /api/v1/tools/retriever/PubMed/execute
{"input": {"query": "BRAF V600E", "max_results": 20}}
```

### UniProt
```http
POST /api/v1/tools/retriever/UniProt/execute
{"input": {"query": "BRAF", "include_isoforms": true}}
```

### ChEMBL
```http
POST /api/v1/tools/retriever/ChEMBL/execute
{"input": {"query": "vemurafenib", "search_type": "compound"}}
```

### PDB
```http
POST /api/v1/tools/retriever/PDB/execute
{"input": {"pdb_id": "4RZW"}}
```

### KEGG
```http
POST /api/v1/tools/retriever/KEGG/execute
{"input": {"query": "mapk", "entry_type": "pathway"}}
```

### BindingDB
```http
POST /api/v1/tools/retriever/BindingDB/execute
{"input": {"target_name": "BRAF", "ligand_name": "vemurafenib"}}
```

## Analyzers

### Protein Stability Predictor
```http
POST /api/v1/tools/analyzer/ProteinStabilityPredictor/execute
{"input": {"pdb_id": "4RZW", "chain": "A", "mutation": "V600E", "method": "FoldX"}}
```

### Molecular Docking
```http
POST /api/v1/tools/analyzer/Docking/execute
{"input": {"receptor_pdb": "4RZW", "ligand_smiles": "CC1=CC=C...", "center_x": 10, "center_y": 20, "center_z": 30}}
```

### Evidence Merge
```http
POST /api/v1/tools/analyzer/EvidenceMerge/execute
{"input": {"sources": [...], "dedup_threshold": 0.85}}
```

### Critic
```http
POST /api/v1/tools/analyzer/Critic/execute
{"input": {"claim": "BRAF V600E reduces binding", "evidence": [...], "require_stance": true}}
```

## Visualizers

### Structure Viewer
```http
POST /api/v1/tools/visualizer/StructureViewer/execute
{"input": {"pdb_id": "4RZW", "mutation_residue": {"chain": "A", "position": 600}, "binding_pocket": {"chain": "A", "positions": [400, 500, 600]}}}
```

### Molecule Viewer
```http
POST /api/v1/tools/visualizer/MoleculeViewer/execute
{"input": {"smiles": "CC1=CC=...", "width": 400, "height": 300}}
```

## Sandbox

### Create Session
```http
POST /api/v1/sandbox/sessions
{"experiment_id": "exp-001", "metadata": {}}
```

### Execute Experiment
```http
POST /api/v1/sandbox/sessions/{id}/execute
{"spec": {...}}
```

## Notifications

### Send Notification
```http
POST /api/v1/notifications/send
{"notification_type": "review_required", "recipients": ["pi@lab.com"], "channels": ["email"], "data": {...}}
```

## Health
```http
GET /health
```