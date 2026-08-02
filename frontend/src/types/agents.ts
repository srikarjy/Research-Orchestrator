export interface Agent {
  id: string;
  name: string;
  description: string;
  capabilities: string[];
  status: "idle" | "running" | "waiting" | "completed" | "failed";
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

export interface Workflow {
  id: string;
  name: string;
  description: string;
  tasks: Task[];
  status: "pending" | "running" | "completed" | "failed" | "cancelled";
  results: Record<string, WorkflowResult>;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  metadata: Record<string, unknown>;
}

export interface WorkflowResult {
  task_id: string;
  agent_id: string;
  status: string;
  output: Record<string, unknown>;
  error?: string;
  duration: string;
  artifacts: Artifact[];
}

export interface Artifact {
  name: string;
  type: string;
  content: unknown;
  path?: string;
  metadata: Record<string, unknown>;
}

export interface SandboxSession {
  id: string;
  experiment_id: string;
  work_dir: string;
  status: "pending" | "running" | "completed" | "failed" | "cancelled";
  process: unknown;
  stdout: unknown;
  stderr: unknown;
  start_time: string;
  end_time?: string;
  exit_code: number;
  resources: ResourceUsage;
  artifacts: SessionArtifact[];
  logs: LogEntry[];
  metadata: Record<string, unknown>;
}

export interface ResourceUsage {
  cpu_time: string;
  max_memory_bytes: number;
  disk_used_bytes: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
}

export interface SessionArtifact {
  name: string;
  path: string;
  size: number;
  mime_type: string;
}

export interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
  source: string;
}

export interface ExperimentSpec {
  type: string;
  command: string[];
  env: Record<string, string>;
  timeout: string;
  resources: ResourceLimits;
  input_files: Record<string, string>;
  output_files: string[];
}

export interface ResourceLimits {
  max_cpu_percent: number;
  max_memory_mb: number;
  max_disk_mb: number;
  max_processes: number;
  network_enabled: boolean;
}

export interface NotificationRequest {
  notification_type: "completion" | "alert" | "progress" | "error";
  recipients: string[];
  channels: string[];
  data: Record<string, unknown>;
}