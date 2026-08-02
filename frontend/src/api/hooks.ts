import { useState, useEffect, useCallback } from "react";
import { workflowApi, mcpApi } from "./client";

export function useWorkflows() {
  const [workflows, ] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Note: listWorkflows endpoint would need to be implemented on backend
      // For now, we'll just track the active workflow
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load workflows");
    } finally {
      setLoading(false);
    }
  }, []);

  const create = useCallback(async (name: string, query: string) => {
    const wf = await workflowApi.createWorkflow(name, query);
    return wf;
  }, []);

  const execute = useCallback(async (id: string) => {
    await workflowApi.executeWorkflow(id);
  }, []);

  return { workflows, loading, error, load, create, execute };
}

export function useWorkflow(workflowId: string | null) {
  const [workflow, setWorkflow] = useState<any>(null);
  const [events, setEvents] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!workflowId) return;
    setLoading(true);
    setError(null);
    try {
      const [wf, evts] = await Promise.all([
        workflowApi.getWorkflow(workflowId),
        workflowApi.getWorkflowEvents(workflowId),
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
  const [calendar, setCalendar] = useState<any[]>([]);
  const [notifications, setNotifications] = useState<any[]>([]);
  const [tasks, setTasks] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [cal, notif, tsk] = await Promise.all([
        workflowApi.getCalendarEvents(),
        workflowApi.getNotifications(),
        workflowApi.getRunningTasks(),
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
    const interval = setInterval(load, 30000); // Poll every 30s
    return () => clearInterval(interval);
  }, [load]);

  const markRead = useCallback(async (id: string) => {
    await workflowApi.markNotificationRead(id);
    setNotifications((prev) => prev.map((n) => (n.id === id ? { ...n, read: true } : n)));
  }, []);

  return { calendar, notifications, tasks, loading, error, reload: load, markRead };
}

export function useMcpTools(category?: string) {
  const [tools, setTools] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await mcpApi.listTools(category);
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

  const execute = useCallback(async (category: string, name: string, input: Record<string, any>) => {
    setLoading(true);
    setError(null);
    try {
      const result = await mcpApi.executeTool(category, name, input);
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
    (claim: string, evidence: any[]) => execute("analyzer", "Critic", { claim, evidence }),
    [execute]
  );
  return { analyze, loading, error };
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