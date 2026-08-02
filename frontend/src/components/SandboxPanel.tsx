import { useEffect, useState } from "react";
import type { SandboxSession, ExperimentSpec } from "../types/agents";
import { fetchSandboxSessions, createSandboxSession, executeSandboxSession, cancelSandboxSession } from "../api/agents";
import "./SandboxPanel.css";

interface SandboxPanelProps {
  onSessionSelect?: (session: SandboxSession) => void;
}

export function SandboxPanel({ onSessionSelect }: SandboxPanelProps) {
  const [sessions, setSessions] = useState<SandboxSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedSession, setSelectedSession] = useState<SandboxSession | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [creating, setCreating] = useState(false);
  const [executing, setExecuting] = useState<string | null>(null);

  useEffect(() => {
    loadSessions();
    const interval = setInterval(loadSessions, 5000);
    return () => clearInterval(interval);
  }, []);

  const loadSessions = async () => {
    try {
      setError(null);
      const data = await fetchSandboxSessions();
      setSessions(data);
      if (selectedSession) {
        const updated = data.find(s => s.id === selectedSession.id);
        if (updated) setSelectedSession(updated);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load sandbox sessions");
    } finally {
      setLoading(false);
    }
  };

  const handleCreateSession = async (experimentId: string, metadata?: Record<string, unknown>) => {
    try {
      setCreating(true);
      const session = await createSandboxSession(experimentId, metadata);
      setSessions(prev => [session, ...prev]);
      setShowCreateModal(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create session");
    } finally {
      setCreating(false);
    }
  };

  const handleExecute = async (id: string, spec: ExperimentSpec) => {
    try {
      setExecuting(id);
      const session = await executeSandboxSession(id, spec);
      setSessions(prev => prev.map(s => s.id === id ? session : s));
      if (selectedSession?.id === id) setSelectedSession(session);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to execute session");
    } finally {
      setExecuting(null);
    }
  };

  const handleCancel = async (id: string) => {
    try {
      await cancelSandboxSession(id);
      loadSessions();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel session");
    }
  };

  const handleSelect = (session: SandboxSession) => {
    setSelectedSession(session);
    onSessionSelect?.(session);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "running": return "var(--signal)";
      case "completed": return "var(--structural)";
      case "failed": return "var(--alert)";
      case "cancelled": return "var(--muted)";
      default: return "var(--muted)";
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "running": return "⚡";
      case "completed": return "✅";
      case "failed": return "❌";
      case "cancelled": return "⏹";
      default: return "⭘";
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "—";
    return new Date(dateStr).toLocaleString();
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  };

  const defaultSpec: ExperimentSpec = {
    type: "simulation",
    command: ["python", "run_experiment.py"],
    env: {},
    timeout: "30m",
    resources: { max_cpu_percent: 80, max_memory_mb: 4096, max_disk_mb: 10240, max_processes: 10, network_enabled: true },
    input_files: {},
    output_files: [],
  };

  if (loading) {
    return (
      <div className="sandbox-panel">
        <div className="sandbox-panel-header">
          <h2>Sandbox</h2>
          <span className="loading">Loading...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="sandbox-panel">
      <div className="sandbox-panel-header">
        <h2>Sandbox Sessions ({sessions.length})</h2>
        <button className="create-btn" onClick={() => setShowCreateModal(true)}>
          + New Session
        </button>
      </div>

      {error && <div className="sandbox-error">{error}</div>}

      <div className="sandbox-list">
        {sessions.length === 0 ? (
          <div className="sandbox-empty">No sandbox sessions. Create one to run experiments.</div>
        ) : (
          sessions.map((session) => (
            <div
              key={session.id}
              className={`session-card ${selectedSession?.id === session.id ? "selected" : ""}`}
              onClick={() => handleSelect(session)}
            >
              <div className="session-card-header">
                <div className="session-info">
                  <h3>{session.experiment_id || "Unnamed Experiment"}</h3>
                  <span className="session-id">{session.id.slice(0, 8)}...</span>
                </div>
                <div className="session-status">
                  <span className="status-badge" style={{ backgroundColor: getStatusColor(session.status), color: session.status === "running" ? "var(--ink)" : "var(--bg)" }}>
                    {getStatusIcon(session.status)} {session.status}
                  </span>
                </div>
              </div>
              <div className="session-meta">
                <span>Work Dir: {session.work_dir}</span>
                <span>Started: {formatDate(session.start_time)}</span>
                {session.end_time && <span>Ended: {formatDate(session.end_time)}</span>}
                {session.exit_code !== undefined && <span>Exit: {session.exit_code}</span>}
              </div>
              <div className="session-resources">
                <span>CPU: {session.resources.cpu_time}</span>
                <span>Mem: {formatBytes(session.resources.max_memory_bytes)}</span>
                <span>Disk: {formatBytes(session.resources.disk_used_bytes)}</span>
              </div>
              {session.artifacts.length > 0 && (
                <div className="session-artifacts">
                  <strong>Artifacts ({session.artifacts.length}):</strong>
                  <div className="artifact-list">
                    {session.artifacts.slice(0, 3).map((artifact) => (
                      <span key={artifact.name} className="artifact-tag" title={artifact.path}>
                        {artifact.name} ({formatBytes(artifact.size)})
                      </span>
                    ))}
                    {session.artifacts.length > 3 && (
                      <span className="artifact-tag more">+{session.artifacts.length - 3} more</span>
                    )}
                  </div>
                </div>
              )}
              <div className="session-actions">
                {session.status === "pending" && (
                  <>
                    <button className="action-btn execute" onClick={(e) => { e.stopPropagation(); handleExecute(session.id, defaultSpec); }} disabled={executing === session.id}>
                      {executing === session.id ? "Starting..." : "▶ Run Experiment"}
                    </button>
                    <button className="action-btn secondary" onClick={(e) => { e.stopPropagation(); /* Open spec editor */ }}>
                      ⚙ Configure
                    </button>
                  </>
                )}
                {session.status === "running" && (
                  <button className="action-btn cancel" onClick={(e) => { e.stopPropagation(); handleCancel(session.id); }}>
                    ■ Cancel
                  </button>
                )}
                {(session.status === "completed" || session.status === "failed") && (
                  <button className="action-btn secondary" onClick={(e) => { e.stopPropagation(); /* View logs */ }}>
                    📋 Logs
                  </button>
                )}
              </div>
            </div>
          ))
        )}
      </div>

      {showCreateModal && (
        <CreateSessionModal
          onClose={() => setShowCreateModal(false)}
          onCreate={handleCreateSession}
          creating={creating}
        />
      )}
    </div>
  );
}

interface CreateSessionModalProps {
  onClose: () => void;
  onCreate: (experimentId: string, metadata?: Record<string, unknown>) => void;
  creating: boolean;
}

function CreateSessionModal({ onClose, onCreate, creating }: CreateSessionModalProps) {
  const [experimentId, setExperimentId] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onCreate(experimentId);
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Create Sandbox Session</h3>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            <div className="form-group">
              <label>Experiment ID</label>
              <input
                type="text"
                value={experimentId}
                onChange={(e) => setExperimentId(e.target.value)}
                placeholder="e.g., braf-docking-001"
                required
              />
            </div>
            <div className="form-group">
              <label>Description (optional)</label>
              <textarea
                placeholder="Describe the experiment..."
                rows={3}
              />
            </div>
          </div>
          <div className="modal-footer">
            <button type="button" className="btn secondary" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn primary" disabled={creating || !experimentId}>
              {creating ? "Creating..." : "Create Session"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}