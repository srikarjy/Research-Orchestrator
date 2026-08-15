// Unified API types matching both Go backends

// ============ Workflow Engine Types ============

export type StepStatus = 
  | "pending" 
  | "running" 
  | "completed" 
  | "failed" 
  | "awaiting_review";

export type WorkflowStatus = 
  | "running" 
  | "completed" 
  | "failed" 
  | "awaiting_review";

export interface Step {
  id: string;
  name: string;
  category: "retriever" | "analyzer" | "visualizer" | "executor";
  tool: string;
  input: Record<string, unknown>;
  output?: Record<string, unknown>;
  status: StepStatus;
  retries: number;
  max_retries: number;
  started_at?: string;
  completed_at?: string;
  error?: string;
  depends_on?: string[];
  idempotency_key?: string;
}

export interface Workflow {
  id: string;
  name: string;
  query: string;
  steps: Step[];
  status: WorkflowStatus;
  created_at: string;
  updated_at: string;
  completed_at?: string;
  metadata?: Record<string, unknown>;
}

export interface WorkflowEvent {
  id: string;
  workflow_id: string;
  step_id: string;
  type: string;
  payload: Record<string, unknown>;
  idempotency_key: string;
  created_at: string;
}

export interface CreateWorkflowRequest {
  name: string;
  query: string;
  steps?: Step[];
}

export interface CreateWorkflowResponse {
  id: string;
  name: string;
  query: string;
  steps: Step[];
  status: WorkflowStatus;
  created_at: string;
  updated_at: string;
}

// ============ Biolab MCP Server Types ============

export type AgentID = 
  | "planner" 
  | "researcher" 
  | "critic" 
  | "executor" 
  | "validator" 
  | "notifier" 
  | "clinical_trial" 
  | "regulatory" 
  | "biomarker";

export type AgentStatus = 
  | "idle" 
  | "running" 
  | "waiting" 
  | "completed" 
  | "failed";

export type MessageType = 
  | "task" 
  | "result" 
  | "request" 
  | "response" 
  | "notification" 
  | "error";

export interface Message {
  id: string;
  type: MessageType;
  from: AgentID;
  to: AgentID;
  payload: unknown;
  timestamp: string;
  trace_id: string;
}

export interface Task {
  id: string;
  type: string;
  description: string;
  input: Record<string, unknown>;
  priority: number;
  dependencies: string[];
  metadata: Record<string, unknown>;
}

export interface Result {
  task_id: string;
  agent_id: AgentID;
  status: string;
  output: Record<string, unknown>;
  error?: string;
  duration: number;
  artifacts: Artifact[];
}

export interface Artifact {
  name: string;
  type: string;
  content: unknown;
  path?: string;
  metadata: Record<string, unknown>;
}

export interface AgentConfig {
  id: AgentID;
  name: string;
  description: string;
  capabilities: string[];
  max_retries: number;
  timeout: number;
  settings: Record<string, unknown>;
}

export interface AgentInfo {
  id: AgentID;
  name: string;
  description: string;
  capabilities: string[];
  status: AgentStatus;
}

export interface WorkflowMCP {
  id: string;
  name: string;
  description: string;
  tasks: Task[];
  status: "pending" | "running" | "completed" | "failed" | "cancelled";
  results: Record<string, Result>;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  metadata: Record<string, unknown>;
}

export interface CreateWorkflowMCPRequest {
  name: string;
  description: string;
  tasks: Task[];
  metadata?: Record<string, unknown>;
}

// ============ Tool Types ============

export interface ToolInfo {
  name: string;
  category: string;
  description: string;
  input_schema: Record<string, unknown>;
}

export interface ToolExecutionResponse {
  output: Record<string, unknown>;
  error?: string;
}

// ============ Executor Types ============

export interface CalendarEvent {
  id: string;
  title: string;
  start: string;
  end: string;
  source: "google" | "mac" | "workflow" | string;
  calendarId: string;
  meetingUrl?: string;
  attendees?: string[];
}

export interface Notification {
  id: string;
  type: "task_running" | "review_required" | "task_completed" | "task_failed" | "calendar_conflict";
  title: string;
  message: string;
  timestamp: string;
  read: boolean;
  relatedEvidenceId?: string;
  relatedToolCall?: string;
  actionUrl?: string;
}

export interface RunningTask {
  id: string;
  name: string;
  startedAt: string;
  status: "running" | "pending" | "awaiting_review" | "completed" | "failed";
  progress: number;
  currentStep: string;
  evidenceId?: string;
}

// ============ Evidence Types (matching frontend fixtures) ============

export interface EvidenceSource {
  id: string;
  type: "paper" | "protein_structure" | "sequence" | "molecule" | "pathway";
  title: string;
  retracted?: boolean;
  stance?: "supports" | "contradicts" | "neutral";
  ref_url?: string;
  refUrl?: string;
  payload: unknown;
}

export interface ToolCallTrace {
  tool: string;
  category: "retriever" | "analyzer" | "visualizer" | "executor";
  latencyMs: number;
  cacheHit: boolean;
  retries: number;
  tokens?: number;
}

export interface EvidenceCard {
  id: string;
  claim: string;
  confidence: {
    overall: number;
    // Optional: present only when the response carried a real per-signal
    // breakdown; when absent, components render "unavailable" rather than
    // faking four values from `overall`.
    signals?: {
      literature: number;
      protein_evidence: number;
      clinical_evidence: number;
      llm_rating: number;
    };
  };
  sources: EvidenceSource[];
  toolCalls: ToolCallTrace[];
  tool_calls?: ToolCallTrace[];
}

export interface TimelineEvent {
  date: string;
  label: string;
  type: "discovery" | "trial_phase" | "approval" | "publication";
  source_id: string;
  sourceId?: string;
}

// ============ Aletheia Types ============

export interface SignalBreakdown {
  literature: number;
  protein_evidence: number;
  clinical_evidence: number;
  llm_rating: number;
}

export interface DebateResponse {
  debate_id: string;
  claim: string;
  conclusion: string;
  verdict: "supported" | "refuted" | "unresolved";
  confidence: number;
  confidence_rationale: string;
  // Absent from multi-agent responses; when missing, render "breakdown
  // unavailable" -- never fabricate four values from the scalar confidence.
  signal_breakdown?: SignalBreakdown | null;
  driving_provenance_ids: number[];
  transcript: TranscriptEntry[];
  sources: Source[];
}

export interface TranscriptEntry {
  agent: string;
  action: string;
  detail: Record<string, unknown>;
  source_paper_id: string | null;
}

export interface Source {
  paper_id: string;
  title: string;
  used_by: string[];
}

// ============ Unified Research Orchestration Types ============

export interface ResearchQuery {
  query: string;
  options?: {
    include_protein_structure?: boolean;
    include_molecules?: boolean;
    include_pathways?: boolean;
    max_papers?: number;
    confidence_threshold?: number;
  };
}

export interface ResearchPlan {
  workflow_id: string;
  tasks: Task[];
  estimated_duration_ms: number;
}

export interface ResearchEvidence {
  evidence_card: EvidenceCard;
  workflow_events: WorkflowEvent[];
  agent_messages: Message[];
}

export type ResearchPhase = 
  | "planning" 
  | "retrieving" 
  | "analyzing" 
  | "synthesizing" 
  | "reviewing" 
  | "completed" 
  | "failed";

export interface ResearchProgress {
  phase: ResearchPhase;
  workflow_id: string;
  current_task?: string;
  completed_tasks: number;
  total_tasks: number;
  progress_percent: number;
  evidence_preview?: Partial<EvidenceCard>;
}