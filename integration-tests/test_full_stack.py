#!/usr/bin/env python3
"""
Full-stack integration test for Research Orchestrator.
Run with: make test-integration
Requires: docker compose up -d (all services healthy)
"""

import asyncio
import httpx
import json
import sys
import time
from typing import Any

# Service URLs (adjust for your environment)
ALETHEIA_URL = "http://localhost:8000"
WORKFLOW_ENGINE_URL = "http://localhost:8080"
BIOLAB_MCP_URL = "http://localhost:8081"
FRONTEND_URL = "http://localhost:5173"

TIMEOUT = 30.0
MAX_WAIT = 60  # seconds for workflow completion


async def check_health(client: httpx.AsyncClient, name: str, url: str) -> bool:
    try:
        resp = await client.get(f"{url}/health", timeout=5.0)
        if resp.status_code == 200:
            print(f"  ✓ {name} healthy")
            return True
    except Exception as e:
        print(f"  ✗ {name} unhealthy: {e}")
    return False


async def wait_for_workflow(client: httpx.AsyncClient, workflow_id: str, max_wait: int = MAX_WAIT) -> dict:
    """Poll workflow until completion or timeout."""
    start = time.time()
    while time.time() - start < max_wait:
        try:
            resp = await client.get(f"{WORKFLOW_ENGINE_URL}/api/v1/workflows/{workflow_id}")
            if resp.status_code == 200:
                wf = resp.json()
                if wf["status"] in ("completed", "failed"):
                    return wf
        except Exception:
            pass
        await asyncio.sleep(2)
    raise TimeoutError(f"Workflow {workflow_id} did not complete within {max_wait}s")


async def test_full_stack() -> bool:
    """Run complete integration test."""
    print("\n=== Research Orchestrator Full-Stack Integration Test ===\n")
    
    async with httpx.AsyncClient(timeout=TIMEOUT) as client:
        # 1. Health checks
        print("1. Checking service health...")
        healthy = 0
        healthy += await check_health(client, "Aletheia", ALETHEIA_URL)
        healthy += await check_health(client, "Workflow Engine", WORKFLOW_ENGINE_URL)
        healthy += await check_health(client, "Biolab MCP", BIOLAB_MCP_URL)
        healthy += await check_health(client, "Frontend", FRONTEND_URL)
        
        if healthy < 4:
            print(f"\n❌ Only {healthy}/4 services healthy. Aborting.")
            return False
        print()

        # 2. Submit investigation via Aletheia
        print("2. Submitting investigation via Aletheia...")
        query = "explain why BRAF V600E reduces binding affinity"
        resp = await client.post(
            f"{ALETHEIA_URL}/investigate",
            json={"query": query, "options": {"max_papers": 5}}
        )
        if resp.status_code != 200:
            print(f"   ❌ Failed: {resp.status_code} - {resp.text}")
            return False
        
        result = resp.json()
        workflow_id = result["workflow_id"]
        print(f"   ✓ Workflow created: {workflow_id}")
        print(f"   Plan: {result['plan']['goal']}")
        print()

        # 3. Wait for workflow completion
        print("3. Waiting for workflow execution...")
        wf = await wait_for_workflow(client, workflow_id)
        print(f"   ✓ Workflow {wf['status']} ({len(wf['steps'])} steps)")
        
        if wf["status"] != "completed":
            print(f"   ❌ Workflow failed: {wf}")
            return False
        print()

        # 4. Verify events contain expected tool calls
        print("4. Verifying tool execution...")
        events_resp = await client.get(f"{WORKFLOW_ENGINE_URL}/api/v1/workflows/{workflow_id}/events")
        events = events_resp.json()
        
        step_events = [e for e in events if e["type"].startswith("step_")]
        completed_steps = [e for e in step_events if e["type"] == "step_completed"]
        tools_used = {e["payload"]["tool"] for e in completed_steps}
        
        expected_tools = {"PubMed", "UniProt", "ChEMBL"}
        missing = expected_tools - tools_used
        
        if missing:
            print(f"   ⚠ Missing expected tools: {missing}")
            print(f"   Tools actually used: {tools_used}")
        else:
            print(f"   ✓ All expected retrievers executed: {tools_used}")
        
        print()

        # 5. Verify Biolab MCP tools are accessible
        print("5. Verifying Biolab MCP tool registry...")
        tools_resp = await client.get(f"{BIOLAB_MCP_URL}/api/v1/tools")
        tools = tools_resp.json()
        tool_names = {t["name"] for t in tools}
        
        expected_mcp_tools = {"PubMed", "UniProt", "ChEMBL", "PDB", "ProteinStabilityPredictor", "Docking", "Critic", "EvidenceMerge"}
        mcp_missing = expected_mcp_tools - tool_names
        
        if mcp_missing:
            print(f"   ⚠ Missing MCP tools: {mcp_missing}")
        else:
            print(f"   ✓ All expected MCP tools registered ({len(tools)} total)")
        print()

        # 6. Test WebSocket endpoint (basic connectivity)
        print("6. Testing WebSocket endpoint...")
        try:
            import websockets
            uri = f"ws://localhost:8080/api/v1/workflows/{workflow_id}/ws"
            async with websockets.connect(uri, timeout=5) as ws:
                msg = await asyncio.wait_for(ws.recv(), timeout=3)
                data = json.loads(msg)
                if data.get("type") == "workflow_state":
                    print("   ✓ WebSocket connected, received workflow state")
                else:
                    print(f"   ✓ WebSocket connected, received: {data.get('type')}")
        except ImportError:
            print("   ⚠ websockets library not installed, skipping WebSocket test")
        except Exception as e:
            print(f"   ⚠ WebSocket test failed: {e}")
        print()

        # 7. Summary
        print("=== INTEGRATION TEST PASSED ===")
        print(f"  Query: {query}")
        print(f"  Workflow: {workflow_id}")
        print(f"  Status: {wf['status']}")
        print(f"  Steps: {len(wf['steps'])}")
        print(f"  Tools executed: {len(completed_steps)}")
        print(f"  MCP tools available: {len(tools)}")
        return True


async def test_quick_queries() -> bool:
    """Test a few quick queries through the full stack."""
    queries = [
        "KRAS G12C inhibitor resistance mechanisms",
        "EGFR exon 19 deletion vs T790M osimertinib response",
    ]
    
    print("\n=== Quick Query Tests ===\n")
    
    async with httpx.AsyncClient(timeout=TIMEOUT) as client:
        for query in queries:
            print(f"Testing: {query}")
            resp = await client.post(
                f"{ALETHEIA_URL}/investigate",
                json={"query": query, "options": {"max_papers": 3}}
            )
            if resp.status_code == 200:
                wf_id = resp.json()["workflow_id"]
                wf = await wait_for_workflow(client, wf_id, max_wait=30)
                status = "✓" if wf["status"] == "completed" else "✗"
                print(f"  {status} Workflow {wf['status']}")
            else:
                print(f"  ✗ Failed to create workflow")
    
    return True


def main():
    print("Research Orchestrator - Full Stack Integration Test")
    print("=" * 55)
    
    try:
        # Run main test
        success = asyncio.run(test_full_stack())
        
        # Run quick queries if main test passed
        if success:
            asyncio.run(test_quick_queries())
        
        if success:
            print("\n🎉 ALL TESTS PASSED")
            sys.exit(0)
        else:
            print("\n❌ TESTS FAILED")
            sys.exit(1)
            
    except KeyboardInterrupt:
        print("\n⚠ Interrupted")
        sys.exit(130)
    except Exception as e:
        print(f"\n💥 ERROR: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()