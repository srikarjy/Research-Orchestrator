import { useEffect, useState } from "react";
import type { Agent } from "../types/agents";
import { fetchAgents } from "../api/agents";
import "./AgentPanel.css";

interface AgentPanelProps {
  onAgentSelect?: (agent: Agent) => void;
}

export function AgentPanel({ onAgentSelect }: AgentPanelProps) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);

  useEffect(() => {
    loadAgents();
    const interval = setInterval(loadAgents, 5000);
    return () => clearInterval(interval);
  }, []);

  const loadAgents = async () => {
    try {
      setError(null);
      const data = await fetchAgents();
      setAgents(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load agents");
    } finally {
      setLoading(false);
    }
  };

  const handleSelect = (agent: Agent) => {
    setSelectedAgent(agent);
    onAgentSelect?.(agent);
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "running": return "var(--signal)";
      case "waiting": return "var(--alert)";
      case "completed": return "var(--structural)";
      case "failed": return "var(--alert)";
      default: return "var(--muted)";
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case "running": return "⚡";
      case "waiting": return "⏳";
      case "completed": return "✅";
      case "failed": return "❌";
      default: return "⭘";
    }
  };

  if (loading) {
    return (
      <div className="agent-panel">
        <div className="agent-panel-header">
          <h2>Agents</h2>
          <span className="loading">Loading...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="agent-panel">
      <div className="agent-panel-header">
        <h2>Agents ({agents.length})</h2>
        <button className="refresh-btn" onClick={loadAgents} disabled={loading}>
          ↻ Refresh
        </button>
      </div>

      {error && <div className="agent-error">{error}</div>}

      <div className="agent-list">
        {agents.map((agent) => (
          <div
            key={agent.id}
            className={`agent-card ${selectedAgent?.id === agent.id ? "selected" : ""}`}
            onClick={() => handleSelect(agent)}
          >
            <div className="agent-card-header">
              <div className="agent-info">
                <h3>{agent.name}</h3>
                <span className="agent-id">{agent.id}</span>
              </div>
              <div className="agent-status">
                <span className="status-dot" style={{ backgroundColor: getStatusColor(agent.status) }} />
                <span style={{ color: getStatusColor(agent.status) }}>
                  {getStatusIcon(agent.status)} {agent.status}
                </span>
              </div>
            </div>
            <p className="agent-description">{agent.description}</p>
            <div className="agent-capabilities">
              {agent.capabilities.map((cap) => (
                <span key={cap} className="capability-tag">{cap}</span>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}