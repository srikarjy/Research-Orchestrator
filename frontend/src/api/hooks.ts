import { useState, useEffect, useCallback, useRef } from "react";
import { assayosApi, researchApi } from "./client";
import type {
  Workflow,
  Step,
  WorkflowEvent,
  CalendarEvent,
  Notification,
  RunningTask,
  AgentInfo,
  WorkflowMCP,
  Task,
  ToolInfo,
  ResearchQuery,
  ResearchPlan,
  ResearchEvidence,
  ResearchProgress,
} from "../types/api";

// ============ Workflow Engine Hooks ============

export function useWorkflows() {
  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await assayosApi.workflowEngine.listWorkflows();
      setWorkflows(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load workflows");
    } finally {
      setLoading(false);
    }
  }, []);

  const create = useCallback(async (name: string, query: string, steps?: Step[]) => {
    const wf = await assayosApi.workflowEngine.createWorkflow({ name, query, steps });
    await load();
    return wf;
  }, [load]);

  const execute = useCallback(async (id: string) => {
    await assayosApi.workflowEngine.executeWorkflow(id);
    await load();
  }, [load]);

  useEffect(() => {
    load();
  }, [load]);

  return { workflows, loading, error, load, create, execute };
}

export function useWorkflow(workflowId: string | null) {
  const [workflow, setWorkflow] = useState<Workflow | null>(null);
  const [events, setEvents] = useState<WorkflowEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!workflowId) return;
    setLoading(true);
    setError(null);
    try {
      const [wf, evts] = await Promise.all([
        assayosApi.workflowEngine.getWorkflow(workflowId),
        assayosApi.workflowEngine.getWorkflowEvents(workflowId),
      ]);
      setWorkflow(wf);
      setEvents(evts);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load workflow");
    } finally {
      setLoading(false);
    }
  }, [workflowId]);

  useEffect(() => {
    load();
  }, [load]);

  return { workflow, events, loading, error, reload: load };
}

export function useExecutorData() {
  const [calendar, setCalendar] = useState<CalendarEvent[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [tasks, setTasks] = useState<RunningTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [cal, notif, tsk] = await Promise.all([
        assayosApi.workflowEngine.getCalendarEvents(),
        assayosApi.workflowEngine.getNotifications(),
        assayosApi.workflowEngine.getRunningTasks(),
      ]);
      setCalendar(cal);
      setNotifications(notif);
      setTasks(tsk);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load executor data");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const interval = setInterval(load, 30000);
    return () => clearInterval(interval);
  }, [load]);

  const markRead = useCallback(async (id: string) => {
    await assayosApi.workflowEngine.markNotificationRead(id);
    setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, read: true } : n)));
  }, []);

  return { calendar, notifications, tasks, loading, error, reload: load, markRead };
}

// ============ Biolab MCP Hooks ============

export function useAgents() {
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await assayosApi.biolab.listAgents();
      setAgents(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load agents");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, [load]);

  return { agents, loading, error, reload: load };
}

export function useAgentStatus(agentId: string | null) {
  const [status, setStatus] = useState<{ id: string; name: string; status: string } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!agentId) return;
    setLoading(true);
    setError(null);
    try {
      const data = await assayosApi.biolab.getAgentStatus(agentId);
      setStatus(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load agent status");
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, [load]);

  return { status, loading, error, reload: load };
}

export function useMcpWorkflows() {
  const [workflows, setWorkflows] = useState<WorkflowMCP[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await assayosApi.biolab.listWorkflows();
      setWorkflows(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load workflows");
    } finally {
      setLoading(false);
    }
  }, []);

  const create = useCallback(async (request: { name: string; description: string; tasks: Task[]; metadata?: Record<string, unknown> }) => {
    const wf = await assayosApi.biolab.createWorkflow(request);
    await load();
    return wf;
  }, [load]);

  const execute = useCallback(async (id: string) => {
    await assayosApi.biolab.executeWorkflow(id);
    await load();
  }, [load]);

  const remove = useCallback(async (id: string) => {
    await assayosApi.biolab.deleteWorkflow(id);
    await load();
  }, [load]);

  useEffect(() => {
    load();
  }, [load]);

  return { workflows, loading, error, load, create, execute, remove };
}

export function useMcpWorkflow(workflowId: string | null) {
  const [workflow, setWorkflow] = useState<WorkflowMCP | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!workflowId) return;
    setLoading(true);
    setError(null);
    try {
      const data = await assayosApi.biolab.getWorkflow(workflowId);
      setWorkflow(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load workflow");
    } finally {
      setLoading(false);
    }
  }, [workflowId]);

  useEffect(() => {
    load();
    const interval = setInterval(load, 2000);
    return () => clearInterval(interval);
  }, [load]);

  return { workflow, loading, error, reload: load };
}

export function useMcpTools(category?: string) {
  const [tools, setTools] = useState<ToolInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await assayosApi.biolab.listTools(category);
      setTools(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load tools");
    } finally {
      setLoading(false);
    }
  }, [category]);

  useEffect(() => {
    load();
  }, [load]);

  return { tools, loading, error, reload: load };
}

export function useToolExecution() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const execute = useCallback(async (category: string, name: string, input: Record<string, unknown>) => {
    setLoading(true);
    setError(null);
    try {
      const result = await assayosApi.biolab.executeTool(category, name, input);
      return result;
    } catch (e) {
      setError(e instanceof Error ? e.message : "Tool execution failed");
      throw e;
    } finally {
      setLoading(false);
    }
  }, []);

  return { execute, loading, error };
}

// Convenience hooks for specific tool categories
export function useRetrievers() {
  return useMcpTools("retriever");
}

export function useAnalyzers() {
  return useMcpTools("analyzer");
}

export function useVisualizers() {
  return useMcpTools("visualizer");
}

// Specific tool hooks
export function usePubMed() {
  const { execute, loading, error } = useToolExecution();
  const search = useCallback(
    (query: string, maxResults = 20) => execute("retriever", "PubMed", { query, max_results: maxResults }),
    [execute]
  );
  return { search, loading, error };
}

export function useUniProt() {
  const { execute, loading, error } = useToolExecution();
  const search = useCallback(
    (query: string, includeIsoforms = true) => execute("retriever", "UniProt", { query, include_isoforms: includeIsoforms }),
    [execute]
  );
  return { search, loading, error };
}

export function useChEMBL() {
  const { execute, loading, error } = useToolExecution();
  const search = useCallback(
    (query: string, searchType = "target") => execute("retriever", "ChEMBL", { query, search_type: searchType }),
    [execute]
  );
  return { search, loading, error };
}

export function usePDB() {
  const { execute, loading, error } = useToolExecution();
  const search = useCallback(
    (pdbId?: string, query?: string) => execute("retriever", "PDB", { pdb_id: pdbId, query }),
    [execute]
  );
  return { search, loading, error };
}

export function useProteinStability() {
  const { execute, loading, error } = useToolExecution();
  const predict = useCallback(
    (pdbId: string, chain: string, mutation: string, method = "FoldX") =>
      execute("analyzer", "ProteinStabilityPredictor", { pdb_id: pdbId, chain, mutation, method }),
    [execute]
  );
  return { predict, loading, error };
}

export function useDocking() {
  const { execute, loading, error } = useToolExecution();
  const run = useCallback(
    (receptorPdb: string, ligandSmiles: string, center: { x: number; y: number; z: number }) =>
      execute("analyzer", "Docking", { receptor_pdb: receptorPdb, ligand_smiles: ligandSmiles, ...center }),
    [execute]
  );
  return { run, loading, error };
}

export function useCritic() {
  const { execute, loading, error } = useToolExecution();
  const analyze = useCallback(
    (claim: string, evidence: unknown[]) => execute("analyzer", "Critic", { claim, evidence }),
    [execute]
  );
  return { analyze, loading, error };
}

export function useEvidenceMerge() {
  const { execute, loading, error } = useToolExecution();
  const merge = useCallback(
    (sources: unknown[], dedupThreshold = 0.85) => execute("analyzer", "EvidenceMerge", { sources, dedup_threshold: dedupThreshold }),
    [execute]
  );
  return { merge, loading, error };
}

export function useStructureViewer() {
  const { execute, loading, error } = useToolExecution();
  const view = useCallback(
    (pdbId: string, mutationResidue?: { chain: string; position: number }, bindingPocket?: { chain: string; positions: number[] }) =>
      execute("visualizer", "StructureViewer", { pdb_id: pdbId, mutation_residue: mutationResidue, binding_pocket: bindingPocket }),
    [execute]
  );
  return { view, loading, error };
}

export function useMoleculeViewer() {
  const { execute, loading, error } = useToolExecution();
  const view = useCallback(
    (smiles: string, width = 400, height = 300) => execute("visualizer", "MoleculeViewer", { smiles, width, height }),
    [execute]
  );
  return { view, loading, error };
}

// ============ Unified Research Orchestration Hooks ============

export function useResearchOrchestrator() {
  const [activeQuery, setActiveQuery] = useState<ResearchQuery | null>(null);
  const [activePlan, setActivePlan] = useState<ResearchPlan | null>(null);
  const [activeEvidence, setActiveEvidence] = useState<ResearchEvidence | null>(null);
  const [progress, setProgress] = useState<ResearchProgress | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollIntervalRef = useRef<number | null>(null);

  const submitQuery = useCallback(async (query: ResearchQuery) => {
    setLoading(true);
    setError(null);
    setActiveQuery(query);
    setActivePlan(null);
    setActiveEvidence(null);
    setProgress(null);

    try {
      const plan = await researchApi.submitQuery(query);
      setActivePlan(plan);
      
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current);
      }
      pollIntervalRef.current = window.setInterval(async () => {
        if (plan?.workflow_id) {
          const prog = await researchApi.getProgress(plan.workflow_id);
          setProgress(prog);
          
          if (prog.phase === "completed" || prog.phase === "failed") {
            if (pollIntervalRef.current) {
              clearInterval(pollIntervalRef.current);
              pollIntervalRef.current = null;
            }
            if (prog.phase === "completed") {
              const workflow = await assayosApi.biolab.getWorkflow(plan.workflow_id);
              const evidenceCard = researchApi.workflowToEvidenceCard(workflow);
              setActiveEvidence({
                evidence_card: evidenceCard,
                workflow_events: [],
                agent_messages: [],
              });
            }
          }
        }
      }, 2000);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to submit query");
    } finally {
      setLoading(false);
    }
  }, []);

  const executeFullPipeline = useCallback(async (query: ResearchQuery) => {
    setLoading(true);
    setError(null);
    setActiveQuery(query);

    try {
      const evidence = await researchApi.executeFullPipeline(query);
      setActiveEvidence(evidence);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Pipeline execution failed");
    } finally {
      setLoading(false);
    }
  }, []);

  const getTimeline = useCallback(async (target: string) => {
    return researchApi.getTimeline(target);
  }, []);

  useEffect(() => {
    return () => {
      if (pollIntervalRef.current) {
        clearInterval(pollIntervalRef.current);
      }
    };
  }, []);

  return {
    activeQuery,
    activePlan,
    activeEvidence,
    progress,
    loading,
    error,
    submitQuery,
    executeFullPipeline,
    getTimeline,
  };
}

// Hook for real-time workflow progress (polling fallback)
export function useWorkflowProgress(workflowId: string | null) {
  const [progress, setProgress] = useState<ResearchProgress | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<number | null>(null);

  const load = useCallback(async () => {
    if (!workflowId) return;
    setLoading(true);
    setError(null);
    try {
      const prog = await researchApi.getProgress(workflowId);
      setProgress(prog);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load progress");
    } finally {
      setLoading(false);
    }
  }, [workflowId]);

  useEffect(() => {
    load();
    intervalRef.current = window.setInterval(load, 2000);
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [load]);

  return { progress, loading, error, reload: load };
}

// Hook for real-time workflow events via WebSocket
export function useWorkflowEventsWS(workflowId: string | null) {
  const [events, setEvents] = useState<Array<{ type: string; data: unknown }>>([]);
  const [workflow, setWorkflow] = useState<unknown>(null);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);

  const connect = useCallback(() => {
    if (!workflowId) return;
    
    const wsUrl = `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/api/v1/workflows/${workflowId}/ws`;
    
    try {
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onopen = () => {
        setConnected(true);
        setError(null);
      };

      ws.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          if (message.type === "workflow_state") {
            setWorkflow(message.data);
          } else if (message.type === "event") {
            setEvents((prev) => [...prev, message]);
          }
        } catch {
          // Ignore parse errors
        }
      };

      ws.onerror = () => {
        setError("WebSocket connection error");
        setConnected(false);
      };

      ws.onclose = () => {
        setConnected(false);
        wsRef.current = null;
        // Reconnect after 3 seconds
        if (reconnectTimeoutRef.current) {
          clearTimeout(reconnectTimeoutRef.current);
        }
        reconnectTimeoutRef.current = window.setTimeout(() => {
          connect();
        }, 3000);
      };
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to connect WebSocket");
    }
  }, [workflowId]);

  useEffect(() => {
    connect();
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [connect]);

  const sendMessage = useCallback((message: unknown) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(message));
    }
  }, []);

  return { events, workflow, connected, error, sendMessage };
}