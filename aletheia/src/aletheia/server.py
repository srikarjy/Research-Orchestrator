"""
Aletheia - Reasoning Plane for Research Orchestrator
LangGraph multi-agent system: Planner → Researcher → Critic → Synthesizer
"""
import os
import uuid
from contextlib import asynccontextmanager
from typing import Any

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

WORKFLOW_ENGINE_URL = os.getenv("WORKFLOW_ENGINE_URL", "http://workflow-engine:8080")
BIOLAB_MCP_URL = os.getenv("BIOLAB_MCP_URL", "http://biolab-mcp:8081")


class InvestigateRequest(BaseModel):
    query: str = Field(..., min_length=3, max_length=500)
    options: dict[str, Any] = Field(default_factory=dict)


class InvestigateResponse(BaseModel):
    workflow_id: str
    status: str
    plan: dict[str, Any]
    estimated_duration_ms: int


class HealthResponse(BaseModel):
    status: str = "ok"
    service: str = "aletheia"
    version: str = "0.1.0"


async def call_workflow_engine(method: str, path: str, **kwargs) -> dict:
    async with httpx.AsyncClient(base_url=WORKFLOW_ENGINE_URL, timeout=30.0) as client:
        resp = await client.request(method, f"/api/v1{path}", **kwargs)
        resp.raise_for_status()
        return resp.json()


async def call_biolab_mcp(method: str, path: str, **kwargs) -> dict:
    async with httpx.AsyncClient(base_url=BIOLAB_MCP_URL, timeout=30.0) as client:
        resp = await client.request(method, f"/api/v1{path}", **kwargs)
        resp.raise_for_status()
        return resp.json()


def create_investigation_plan(query: str) -> dict:
    """Create a structured investigation plan from the query."""
    # This would be the Planner agent in full implementation
    return {
        "id": str(uuid.uuid4()),
        "goal": query,
        "hypothesis": f"Systematic investigation of '{query}' will yield mechanistic insights with >80% confidence",
        "methodology": "Multi-phase: Literature review → Target identification → Computational modeling → Evidence synthesis → Experimental design",
        "tasks": [
            {"id": "pubmed", "name": "Literature Review", "type": "research", "agent": "researcher", "estimated_duration": "2 days"},
            {"id": "uniprot", "name": "Target Identification", "type": "analysis", "agent": "researcher", "estimated_duration": "3 days"},
            {"id": "stability", "name": "Stability Prediction", "type": "compute", "agent": "executor", "estimated_duration": "5 days"},
            {"id": "docking", "name": "Molecular Docking", "type": "compute", "agent": "executor", "estimated_duration": "5 days"},
            {"id": "critic", "name": "Evidence Critique", "type": "critique", "agent": "critic", "estimated_duration": "2 days"},
            {"id": "synthesize", "name": "Evidence Synthesis", "type": "synthesis", "agent": "synthesizer", "estimated_duration": "2 days"},
        ],
        "budget": 10000,
        "timeline": "4 weeks",
        "success_criteria": [
            "Statistical significance p < 0.05",
            "Reproducible across 3+ replicates",
            "Validated by orthogonal method",
        ],
    }


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup
    print(f"Aletheia starting - Workflow Engine: {WORKFLOW_ENGINE_URL}, Biolab MCP: {BIOLAB_MCP_URL}")
    yield
    # Shutdown
    print("Aletheia shutting down")


app = FastAPI(
    title="Aletheia - Reasoning Plane",
    description="Multi-agent research orchestration: Planner → Researcher → Critic → Synthesizer",
    version="0.1.0",
    lifespan=lifespan,
)


@app.get("/health", response_model=HealthResponse)
async def health():
    return HealthResponse()


@app.post("/investigate", response_model=InvestigateResponse)
async def investigate(request: InvestigateRequest):
    """
    Submit a research query for full investigation.
    Creates a workflow in the Workflow Engine and starts execution.
    """
    # 1. Create investigation plan (Planner agent)
    plan = create_investigation_plan(request.query)
    
    # 2. Create workflow in Workflow Engine
    workflow_tasks = []
    for i, task in enumerate(plan["tasks"]):
        workflow_tasks.append({
            "id": task["id"],
            "type": task["type"],
            "description": task["name"],
            "input": {"query": request.query} if task["type"] == "research" else {},
            "priority": 10 - i,
            "dependencies": [],
            "metadata": {"agent": task["agent"]},
        })
    
    wf_resp = await call_workflow_engine("POST", "/workflows", json={
        "name": f"Investigation: {request.query[:50]}",
        "description": request.query,
        "tasks": workflow_tasks,
        "metadata": {"original_query": request.query, "plan": plan},
    })
    
    workflow_id = wf_resp["id"]
    
    # 3. Execute workflow
    await call_workflow_engine("POST", f"/workflows/{workflow_id}/execute")
    
    return InvestigateResponse(
        workflow_id=workflow_id,
        status="started",
        plan=plan,
        estimated_duration_ms=30000,
    )


@app.get("/investigate/{workflow_id}/status")
async def investigate_status(workflow_id: str):
    """Get real-time status of an investigation."""
    wf = await call_workflow_engine("GET", f"/workflows/{workflow_id}")
    events = await call_workflow_engine("GET", f"/workflows/{workflow_id}/events")
    
    completed = sum(1 for e in events if e["type"] == "step_completed")
    total = len([s for s in wf["steps"] if s["status"] != ""])
    
    return {
        "workflow_id": workflow_id,
        "status": wf["status"],
        "progress": f"{completed}/{total}",
        "current_step": next((s["name"] for s in wf["steps"] if s["status"] == "running"), None),
        "events": events[-10:],  # Last 10 events
    }


@app.get("/workflows")
async def list_workflows():
    return await call_workflow_engine("GET", "/workflows")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)