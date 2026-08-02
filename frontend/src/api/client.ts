const API_BASE = import.meta.env.VITE_API_BASE || "http://localhost:8080";
const MCP_BASE = import.meta.env.VITE_MCP_BASE || "http://localhost:8081";

export interface ApiConfig {
  workflowEngine: string;
  biolabMcp: string;
}

export const config: ApiConfig = {
  workflowEngine: API_BASE,
  biolabMcp: MCP_BASE,
};

export async function fetchJson<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { "Content-Type": "application/json", ...options?.headers },
    ...options,
  });
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  return res.json();
}

// Workflow Engine API
export const workflowApi = {
  createWorkflow: (name: string, query: string) =>
    fetchJson<{ id: string }>(`${config.workflowEngine}/api/v1/workflows`, {
      method: "POST",
      body: JSON.stringify({ name, query }),
    }),

  executeWorkflow: (id: string) =>
    fetchJson<{ status: string }>(`${config.workflowEngine}/api/v1/workflows/${id}/execute`, {
      method: "POST",
    }),

  getWorkflow: (id: string) =>
    fetchJson<any>(`${config.workflowEngine}/api/v1/workflows/${id}`),

  getWorkflowEvents: (id: string) =>
    fetchJson<any[]>(`${config.workflowEngine}/api/v1/workflows/${id}/events`),

  // Executor endpoints
  getCalendarEvents: () =>
    fetchJson<any[]>(`${config.workflowEngine}/api/v1/executor/calendar`),

  getNotifications: () =>
    fetchJson<any[]>(`${config.workflowEngine}/api/v1/executor/notifications`),

  getRunningTasks: () =>
    fetchJson<any[]>(`${config.workflowEngine}/api/v1/executor/tasks`),

  markNotificationRead: (id: string) =>
    fetchJson<any>(`${config.workflowEngine}/api/v1/executor/notifications/${id}/read`, {
      method: "POST",
    }),
};

// Biolab MCP Server API
export const mcpApi = {
  listTools: (category?: string) => {
    const url = category
      ? `${config.biolabMcp}/api/v1/tools/${category}`
      : `${config.biolabMcp}/api/v1/tools`;
    return fetchJson<any[]>(url);
  },

  getToolSchema: (category: string, name: string) =>
    fetchJson<any>(`${config.biolabMcp}/api/v1/tools/${category}/${name}/schema`),

  executeTool: (category: string, name: string, input: Record<string, any>) =>
    fetchJson<any>(`${config.biolabMcp}/api/v1/tools/${category}/${name}/execute`, {
      method: "POST",
      body: JSON.stringify(input),
    }),

  // Convenience methods for common tools
  retrievers: {
    pubmed: (query: string, maxResults = 20) =>
      mcpApi.executeTool("retriever", "PubMed", { query, max_results: maxResults }),
    uniprot: (query: string, includeIsoforms = true) =>
      mcpApi.executeTool("retriever", "UniProt", { query, include_isoforms: includeIsoforms }),
    chembl: (query: string, searchType = "target") =>
      mcpApi.executeTool("retriever", "ChEMBL", { query, search_type: searchType }),
    pdb: (pdbId?: string, query?: string) =>
      mcpApi.executeTool("retriever", "PDB", { pdb_id: pdbId, query }),
    kegg: (query: string, entryType = "pathway") =>
      mcpApi.executeTool("retriever", "KEGG", { query, entry_type: entryType }),
    bindingdb: (targetName?: string, ligandName?: string) =>
      mcpApi.executeTool("retriever", "BindingDB", { target_name: targetName, ligand_name: ligandName }),
  },

  analyzers: {
    stability: (pdbId: string, chain: string, mutation: string, method = "FoldX") =>
      mcpApi.executeTool("analyzer", "ProteinStabilityPredictor", { pdb_id: pdbId, chain, mutation, method }),
    docking: (receptorPdb: string, ligandSmiles: string, center: { x: number; y: number; z: number }) =>
      mcpApi.executeTool("analyzer", "Docking", { receptor_pdb: receptorPdb, ligand_smiles: ligandSmiles, ...center }),
    merge: (sources: any[], dedupThreshold = 0.85) =>
      mcpApi.executeTool("analyzer", "EvidenceMerge", { sources, dedup_threshold: dedupThreshold }),
    critic: (claim: string, evidence: any[]) =>
      mcpApi.executeTool("analyzer", "Critic", { claim, evidence }),
  },

  visualizers: {
    structure: (pdbId: string, mutationResidue?: { chain: string; position: number }, bindingPocket?: { chain: string; positions: number[] }) =>
      mcpApi.executeTool("visualizer", "StructureViewer", { pdb_id: pdbId, mutation_residue: mutationResidue, binding_pocket: bindingPocket }),
    molecule: (smiles: string, width = 400, height = 300) =>
      mcpApi.executeTool("visualizer", "MoleculeViewer", { smiles, width, height }),
  },
};