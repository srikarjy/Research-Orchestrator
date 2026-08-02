import type { Agent, Workflow, Task, SandboxSession, ExperimentSpec, NotificationRequest } from "../types/agents";

const API_BASE = "/api/v1";

export async function fetchAgents(): Promise<Agent[]> {
  const res = await fetch(`${API_BASE}/agents`);
  if (!res.ok) throw new Error("Failed to fetch agents");
  return res.json();
}

export async function fetchAgentStatus(id: string): Promise<Agent> {
  const res = await fetch(`${API_BASE}/agents/${id}/status`);
  if (!res.ok) throw new Error("Failed to fetch agent status");
  return res.json();
}

export async function fetchWorkflows(): Promise<Workflow[]> {
  const res = await fetch(`${API_BASE}/workflows`);
  if (!res.ok) throw new Error("Failed to fetch workflows");
  return res.json();
}

export async function createWorkflow(workflow: {
  name: string;
  description: string;
  tasks: Task[];
  metadata?: Record<string, unknown>;
}): Promise<Workflow> {
  const res = await fetch(`${API_BASE}/workflows`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(workflow),
  });
  if (!res.ok) throw new Error("Failed to create workflow");
  return res.json();
}

export async function fetchWorkflow(id: string): Promise<Workflow> {
  const res = await fetch(`${API_BASE}/workflows/${id}`);
  if (!res.ok) throw new Error("Failed to fetch workflow");
  return res.json();
}

export async function executeWorkflow(id: string): Promise<{ status: string; workflow_id: string }> {
  const res = await fetch(`${API_BASE}/workflows/${id}/execute`, {
    method: "POST",
  });
  if (!res.ok) throw new Error("Failed to execute workflow");
  return res.json();
}

export async function cancelWorkflow(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/workflows/${id}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error("Failed to cancel workflow");
}

export async function fetchSandboxSessions(): Promise<SandboxSession[]> {
  const res = await fetch(`${API_BASE}/sandbox/sessions`);
  if (!res.ok) throw new Error("Failed to fetch sandbox sessions");
  return res.json();
}

export async function createSandboxSession(experimentId: string, metadata?: Record<string, unknown>): Promise<SandboxSession> {
  const res = await fetch(`${API_BASE}/sandbox/sessions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ experiment_id: experimentId, metadata }),
  });
  if (!res.ok) throw new Error("Failed to create sandbox session");
  return res.json();
}

export async function fetchSandboxSession(id: string): Promise<SandboxSession> {
  const res = await fetch(`${API_BASE}/sandbox/sessions/${id}`);
  if (!res.ok) throw new Error("Failed to fetch sandbox session");
  return res.json();
}

export async function executeSandboxSession(id: string, spec: ExperimentSpec): Promise<SandboxSession> {
  const res = await fetch(`${API_BASE}/sandbox/sessions/${id}/execute`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(spec),
  });
  if (!res.ok) throw new Error("Failed to execute sandbox session");
  return res.json();
}

export async function cancelSandboxSession(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/sandbox/sessions/${id}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error("Failed to cancel sandbox session");
}

export async function sendNotification(request: NotificationRequest): Promise<unknown> {
  const res = await fetch(`${API_BASE}/notifications/send`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(request),
  });
  if (!res.ok) throw new Error("Failed to send notification");
  return res.json();
}