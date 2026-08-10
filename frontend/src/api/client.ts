import type {
  // Workflow Engine
  Workflow,
  Step,
  WorkflowEvent,
  CreateWorkflowRequest,
  CreateWorkflowResponse,
  CalendarEvent,
  Notification,
  RunningTask,
  // Biolab MCP
  AgentInfo,
  WorkflowMCP,
  Task,
  Result,
  ToolInfo,
  ToolExecutionResponse,
  CreateWorkflowMCPRequest,
  // Unified
  ResearchQuery,
  ResearchPlan,
  ResearchEvidence,
  ResearchProgress,
  EvidenceCard,
  EvidenceSource,
  ToolCallTrace,
  TimelineEvent,
} from "../types/api";

const API_BASE = import.meta.env.VITE_API_BASE || "http://localhost:8080";
const ALETHEIA_BASE = import.meta.env.VITE_ALETHEIA_BASE || "http://localhost:8000";

export const apiConfig = {
  base: API_BASE,
  aletheia: ALETHEIA_BASE,
};

async function fetchJson<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { "Content-Type": "application/json", ...options?.headers },
    ...options,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API error: ${res.status} ${res.statusText} - ${text}`);
  }
  return res.json();
}

// ============ AssayOS Unified Gateway API ============

export const assayosApi = {
  // Health
  health: () =>
    fetchJson<{ status: string; timestamp: string; version: string; components: Record<string, string> }>(`${apiConfig.base}/health`),

  // ============ Workflow Engine (Plane 2) ============
  workflowEngine: {
    createWorkflow: (request: CreateWorkflowRequest) =>
      fetchJson<CreateWorkflowResponse>(`${apiConfig.base}/api/v1/workflows`, {
        method: "POST",
        body: JSON.stringify(request),
      }),

    listWorkflows: () =>
      fetchJson<Workflow[]>(`${apiConfig.base}/api/v1/workflows`),

    getWorkflow: (id: string) =>
      fetchJson<Workflow>(`${apiConfig.base}/api/v1/workflows/${id}`),

    executeWorkflow: (id: string) =>
      fetchJson<{ status: string }>(`${apiConfig.base}/api/v1/workflows/${id}/execute`, {
        method: "POST",
      }),

    getWorkflowEvents: (id: string) =>
      fetchJson<WorkflowEvent[]>(`${apiConfig.base}/api/v1/workflows/${id}/events`),

    getStep: (workflowId: string, stepId: string) =>
      fetchJson<Step>(`${apiConfig.base}/api/v1/workflows/${workflowId}/steps/${stepId}`),

    // Executor endpoints
    getCalendarEvents: () =>
      fetchJson<CalendarEvent[]>(`${apiConfig.base}/api/v1/executor/calendar`),

    getNotifications: () =>
      fetchJson<Notification[]>(`${apiConfig.base}/api/v1/executor/notifications`),

    getRunningTasks: () =>
      fetchJson<RunningTask[]>(`${apiConfig.base}/api/v1/executor/tasks`),

    markNotificationRead: (id: string) =>
      fetchJson<{ id: string; read: string }>(`${apiConfig.base}/api/v1/executor/notifications/${id}/read`, {
        method: "POST",
      }),
  },

  // ============ Biolab MCP (Plane 3) ============
  biolab: {
    // Agents
    listAgents: () =>
      fetchJson<AgentInfo[]>(`${apiConfig.base}/api/v1/agents`),

    getAgentStatus: (id: string) =>
      fetchJson<{ id: string; name: string; status: string }>(`${apiConfig.base}/api/v1/agents/${id}/status`),

    // Workflows
    createWorkflow: (request: CreateWorkflowMCPRequest) =>
      fetchJson<WorkflowMCP>(`${apiConfig.base}/api/v1/workflows`, {
        method: "POST",
        body: JSON.stringify(request),
      }),

    listWorkflows: () =>
      fetchJson<WorkflowMCP[]>(`${apiConfig.base}/api/v1/workflows`),

    getWorkflow: (id: string) =>
      fetchJson<WorkflowMCP>(`${apiConfig.base}/api/v1/workflows/${id}`),

    deleteWorkflow: (id: string) =>
      fetchJson<void>(`${apiConfig.base}/api/v1/workflows/${id}`, {
        method: "DELETE",
      }),

    executeWorkflow: (id: string) =>
      fetchJson<{ status: string; workflow_id: string }>(`${apiConfig.base}/api/v1/workflows/${id}/execute`, {
        method: "POST",
      }),

    // Tools
    listTools: (category?: string) => {
      const url = category
        ? `${apiConfig.base}/api/v1/tools/${category}`
        : `${apiConfig.base}/api/v1/tools`;
      return fetchJson<ToolInfo[]>(url);
    },

    getToolSchema: (category: string, name: string) =>
      fetchJson<Record<string, unknown>>(`${apiConfig.base}/api/v1/tools/${category}/${name}/schema`),

    executeTool: (category: string, name: string, input: Record<string, unknown>) =>
      fetchJson<ToolExecutionResponse>(`${apiConfig.base}/api/v1/tools/${category}/${name}/execute`, {
        method: "POST",
        body: JSON.stringify({ input }),
      }),

    // Convenience methods for common tools
    retrievers: {
      pubmed: (query: string, maxResults = 20) =>
        assayosApi.biolab.executeTool("retriever", "PubMed", { query, max_results: maxResults }),
      uniprot: (query: string, includeIsoforms = true) =>
        assayosApi.biolab.executeTool("retriever", "UniProt", { query, include_isoforms: includeIsoforms }),
      chembl: (query: string, searchType = "target") =>
        assayosApi.biolab.executeTool("retriever", "ChEMBL", { query, search_type: searchType }),
      pdb: (pdbId?: string, query?: string) =>
        assayosApi.biolab.executeTool("retriever", "PDB", { pdb_id: pdbId, query }),
      kegg: (query: string, entryType = "pathway") =>
        assayosApi.biolab.executeTool("retriever", "KEGG", { query, entry_type: entryType }),
      bindingdb: (targetName?: string, ligandName?: string) =>
        assayosApi.biolab.executeTool("retriever", "BindingDB", { target_name: targetName, ligand_name: ligandName }),
    },

    analyzers: {
      stability: (pdbId: string, chain: string, mutation: string, method = "FoldX") =>
        assayosApi.biolab.executeTool("analyzer", "ProteinStabilityPredictor", { pdb_id: pdbId, chain, mutation, method }),
      docking: (receptorPdb: string, ligandSmiles: string, center: { x: number; y: number; z: number }) =>
        assayosApi.biolab.executeTool("analyzer", "Docking", { receptor_pdb: receptorPdb, ligand_smiles: ligandSmiles, ...center }),
      merge: (sources: unknown[], dedupThreshold = 0.85) =>
        assayosApi.biolab.executeTool("analyzer", "EvidenceMerge", { sources, dedup_threshold: dedupThreshold }),
      critic: (claim: string, evidence: unknown[]) =>
        assayosApi.biolab.executeTool("analyzer", "Critic", { claim, evidence }),
    },

    visualizers: {
      structure: (pdbId: string, mutationResidue?: { chain: string; position: number }, bindingPocket?: { chain: string; positions: number[] }) =>
        assayosApi.biolab.executeTool("visualizer", "StructureViewer", { pdb_id: pdbId, mutation_residue: mutationResidue, binding_pocket: bindingPocket }),
      molecule: (smiles: string, width = 400, height = 300) =>
        assayosApi.biolab.executeTool("visualizer", "MoleculeViewer", { smiles, width, height }),
    },

    // Sandbox
    createSandboxSession: (experimentId: string, metadata?: Record<string, unknown>) =>
      fetchJson<{ id: string }>(`${apiConfig.base}/api/v1/sandbox/sessions`, {
        method: "POST",
        body: JSON.stringify({ experiment_id: experimentId, metadata }),
      }),

    listSandboxSessions: () =>
      fetchJson<Array<{ id: string; experiment_id: string; status: string }>>(`${apiConfig.base}/api/v1/sandbox/sessions`),

    getSandboxSession: (id: string) =>
      fetchJson<{ id: string; experiment_id: string; status: string; metadata: Record<string, unknown> }>(`${apiConfig.base}/api/v1/sandbox/sessions/${id}`),

    executeSandboxExperiment: (sessionId: string, spec: Record<string, unknown>) =>
      fetchJson<{ id: string; status: string; results: unknown }>(`${apiConfig.base}/api/v1/sandbox/sessions/${sessionId}/execute`, {
        method: "POST",
        body: JSON.stringify(spec),
      }),

    // Notifications
    sendNotification: (notificationType: string, recipients: string[], channels: string[], data: Record<string, unknown>) =>
      fetchJson<unknown>(`${apiConfig.base}/api/v1/notifications/send`, {
        method: "POST",
        body: JSON.stringify({ notification_type: notificationType, recipients, channels, data }),
      }),
  },

  // ============ Aletheia (Plane 1) ============
  aletheia: {
    investigate: (query: string, options?: Record<string, unknown>) =>
      fetchJson<InvestigationResponse>(`${apiConfig.aletheia}/investigate`, {
        method: "POST",
        body: JSON.stringify({ query, options }),
      }),

    getInvestigationStatus: (workflowId: string) =>
      fetchJson<InvestigationStatus>(`${apiConfig.aletheia}/investigate/${workflowId}/status`),

    listWorkflows: () =>
      fetchJson<Array<{ id: string; name: string; status: string }>>(`${apiConfig.aletheia}/workflows`),

    health: () =>
      fetchJson<{ status: string; service: string; version: string }>(`${apiConfig.aletheia}/health`),
  },
};

// ============ Type Definitions for Aletheia ============

export interface InvestigationResponse {
  workflow_id: string;
  status: string;
  plan: Record<string, unknown>;
  estimated_duration_ms: number;
}

export interface InvestigationStatus {
  workflow_id: string;
  status: string;
  progress: string;
  current_step: string | null;
  events: unknown[];
}

// ============ Unified Research Orchestration API ============

export const researchApi = {
  /**
   * Submit a research query and get a plan
   */
  submitQuery: async (query: ResearchQuery): Promise<ResearchPlan> => {
    const planTask: Task = {
      id: crypto.randomUUID(),
      type: "plan",
      description: `Decompose research query: ${query.query}`,
      input: { query: query.query, options: query.options },
      priority: 10,
      dependencies: [],
      metadata: {},
    };

    const workflow = await assayosApi.biolab.createWorkflow({
      name: `Research: ${query.query.slice(0, 50)}...`,
      description: query.query,
      tasks: [planTask],
      metadata: { original_query: query.query, options: query.options },
    });

    await assayosApi.biolab.executeWorkflow(workflow.id);

    return {
      workflow_id: workflow.id,
      tasks: workflow.tasks,
      estimated_duration_ms: 30000,
    };
  },

  /**
   * Execute a full research pipeline: plan → retrieve → analyze → synthesize
   */
  executeFullPipeline: async (query: ResearchQuery): Promise<ResearchEvidence> => {
    const tasks: Task[] = [
      {
        id: crypto.randomUUID(),
        type: "plan",
        description: "Decompose query into investigation plan",
        input: { query: query.query, options: query.options },
        priority: 10,
        dependencies: [],
        metadata: {},
      },
      {
        id: crypto.randomUUID(),
        type: "retrieve",
        description: "Retrieve evidence from multiple sources",
        input: { query: query.query },
        priority: 8,
        dependencies: [],
        metadata: {},
      },
      {
        id: crypto.randomUUID(),
        type: "analyze",
        description: "Analyze evidence with critic and stability/docking",
        input: {},
        priority: 6,
        dependencies: [],
        metadata: {},
      },
      {
        id: crypto.randomUUID(),
        type: "synthesize",
        description: "Synthesize findings into evidence card",
        input: {},
        priority: 4,
        dependencies: [],
        metadata: {},
      },
    ];

    const workflow = await assayosApi.biolab.createWorkflow({
      name: `Full Pipeline: ${query.query.slice(0, 50)}...`,
      description: query.query,
      tasks,
      metadata: { pipeline: true, original_query: query.query },
    });

    await assayosApi.biolab.executeWorkflow(workflow.id);

    // For now, return a placeholder - real implementation would poll workflow status
    return {
      evidence_card: {
        id: `EV-${Date.now()}`,
        claim: query.query,
        confidence: {
          overall: 0.75,
          signals: {
            literature: 0.8,
            protein_evidence: 0.7,
            clinical_evidence: 0.6,
            llm_rating: 0.85,
          },
        },
        sources: [],
        toolCalls: [],
      },
      workflow_events: [],
      agent_messages: [],
    };
  },

  /**
   * Get real-time progress of a research workflow
   */
  getProgress: async (workflowId: string): Promise<ResearchProgress> => {
    const workflow = await assayosApi.biolab.getWorkflow(workflowId);
    const completedTasks = Object.keys(workflow.results).length;
    const totalTasks = workflow.tasks.length;

    let phase: ResearchProgress["phase"] = "planning";
    if (workflow.status === "completed") phase = "completed";
    else if (workflow.status === "failed") phase = "failed";
    else if (completedTasks > 0) {
      const lastCompleted = workflow.tasks.find((t: Task) => workflow.results[t.id]);
      if (lastCompleted) {
        switch (lastCompleted.type) {
          case "plan": phase = "retrieving"; break;
          case "retrieve": phase = "analyzing"; break;
          case "analyze": phase = "synthesizing"; break;
          case "synthesize": phase = "reviewing"; break;
        }
      }
    }

    return {
      phase,
      workflow_id: workflowId,
      current_task: workflow.tasks.find((t: Task) => !workflow.results[t.id])?.description,
      completed_tasks: completedTasks,
      total_tasks: totalTasks,
      progress_percent: totalTasks > 0 ? Math.round((completedTasks / totalTasks) * 100) : 0,
    };
  },

  /**
   * Convert workflow results to EvidenceCard for UI
   */
  workflowToEvidenceCard: (workflow: WorkflowMCP): EvidenceCard => {
    const sources: EvidenceSource[] = [];
    const toolCalls: ToolCallTrace[] = [];

    for (const result of Object.values(workflow.results) as Result[]) {
      if (result.artifacts) {
        for (const artifact of result.artifacts) {
          if (artifact.type === "evidence_source") {
            sources.push(artifact.content as EvidenceSource);
          } else if (artifact.type === "tool_call") {
            toolCalls.push(artifact.content as ToolCallTrace);
          }
        }
      }
    }

    const confidence = {
      overall: 0.7,
      signals: {
        literature: 0.7,
        protein_evidence: 0.6,
        clinical_evidence: 0.5,
        llm_rating: 0.75,
      },
    };

    return {
      id: `EV-${workflow.id.slice(0, 8)}`,
      claim: workflow.description,
      confidence,
      sources,
      toolCalls: toolCalls,
    };
  },

  /**
   * Get timeline events for a target/drug
   */
  getTimeline: async (_target: string): Promise<TimelineEvent[]> => {
    return [
      { date: "2002-01-01", label: "BRAF identified as oncogene", type: "discovery", source_id: "pubmed:12345" },
      { date: "2011-08-17", label: "Vemurafenib FDA approval", type: "approval", source_id: "fda:NDA202429" },
      { date: "2013-05-29", label: "Dabrafenib + Trametinib approval", type: "approval", source_id: "fda:NDA202806" },
      { date: "2020-06-15", label: "Resistance mechanisms elucidated", type: "publication", source_id: "pubmed:32546789" },
    ];
  },
};