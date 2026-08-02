import { useEffect, useState } from "react";
import type { Workflow, Task } from "../types/agents";
import { fetchWorkflows, createWorkflow, executeWorkflow, cancelWorkflow } from "../api/agents";
import "./WorkflowPanel.css";

interface WorkflowPanelProps {
  onWorkflowSelect?: (workflow: Workflow) => void;
}

export function WorkflowPanel({ onWorkflowSelect }: WorkflowPanelProps) {
  const [workflows, setWorkflows] = useState<Workflow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedWorkflow, setSelectedWorkflow] = useState<Workflow | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    loadWorkflows();
    const interval = setInterval(loadWorkflows, 10000);
    return () => clearInterval(interval);
  }, []);

  const loadWorkflows = async () => {
    try {
      setError(null);
      const data = await fetchWorkflows();
      setWorkflows(data);
      if (selectedWorkflow) {
        const updated = data.find(w => w.id === selectedWorkflow.id);
        if (updated) setSelectedWorkflow(updated);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load workflows");
    } finally {
      setLoading(false);
    }
  };

  const handleCreateWorkflow = async (name: string, description: string, tasks: Task[]) => {
    try {
      setCreating(true);
      const workflow = await createWorkflow({ name, description, tasks });
      setWorkflows(prev => [workflow, ...prev]);
      setShowCreateModal(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create workflow");
    } finally {
      setCreating(false);
    }
  };

  const handleExecute = async (id: string) => {
    try {
      await executeWorkflow(id);
      loadWorkflows();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to execute workflow");
    }
  };

  const handleCancel = async (id: string) => {
    try {
      await cancelWorkflow(id);
      loadWorkflows();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to cancel workflow");
    }
  };

  const handleSelect = (workflow: Workflow) => {
    setSelectedWorkflow(workflow);
    onWorkflowSelect?.(workflow);
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

  const defaultTasks: Task[] = [
    { id: "task-1", type: "research", description: "Literature review", input: { query: "BRAF V600E inhibitors" }, priority: 1, dependencies: [], metadata: {} },
    { id: "task-2", type: "compute", description: "Docking simulation", input: { experiment_type: "docking" }, priority: 2, dependencies: ["task-1"], metadata: {} },
    { id: "task-3", type: "critique", description: "Evidence critique", input: { claim: "BRAF V600E binding" }, priority: 3, dependencies: ["task-2"], metadata: {} },
    { id: "task-4", type: "validate", description: "Statistical validation", input: { validation_type: "statistical" }, priority: 4, dependencies: ["task-3"], metadata: {} },
    { id: "task-5", type: "notify", description: "Send completion notification", input: { notification_type: "completion" }, priority: 5, dependencies: ["task-4"], metadata: {} },
  ];

  if (loading) {
    return (
      <div className="workflow-panel">
        <div className="workflow-panel-header">
          <h2>Workflows</h2>
          <span className="loading">Loading...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="workflow-panel">
      <div className="workflow-panel-header">
        <h2>Workflows ({workflows.length})</h2>
        <button className="create-btn" onClick={() => setShowCreateModal(true)}>
          + New Workflow
        </button>
      </div>

      {error && <div className="workflow-error">{error}</div>}

      <div className="workflow-list">
        {workflows.length === 0 ? (
          <div className="workflow-empty">No workflows yet. Create one to get started.</div>
        ) : (
          workflows.map((workflow) => (
            <div
              key={workflow.id}
              className={`workflow-card ${selectedWorkflow?.id === workflow.id ? "selected" : ""}`}
              onClick={() => handleSelect(workflow)}
            >
              <div className="workflow-card-header">
                <div className="workflow-info">
                  <h3>{workflow.name}</h3>
                  <span className="workflow-id">{workflow.id.slice(0, 8)}...</span>
                </div>
                <div className="workflow-status">
                  <span className="status-badge" style={{ backgroundColor: getStatusColor(workflow.status), color: workflow.status === "running" ? "var(--ink)" : "var(--bg)" }}>
                    {getStatusIcon(workflow.status)} {workflow.status}
                  </span>
                </div>
              </div>
              <p className="workflow-description">{workflow.description}</p>
              <div className="workflow-meta">
                <span>Tasks: {workflow.tasks.length}</span>
                <span>Created: {formatDate(workflow.created_at)}</span>
                {workflow.started_at && <span>Started: {formatDate(workflow.started_at)}</span>}
                {workflow.completed_at && <span>Completed: {formatDate(workflow.completed_at)}</span>}
              </div>
              {workflow.status === "running" && (
                <div className="workflow-progress">
                  <div className="progress-bar">
                    <div className="progress-fill" style={{ width: `${calculateProgress(workflow)}%` }} />
                  </div>
                  <span className="progress-text">{calculateProgress(workflow)}% complete</span>
                </div>
              )}
              <div className="workflow-actions">
                {workflow.status === "pending" && (
                  <button className="action-btn execute" onClick={(e) => { e.stopPropagation(); handleExecute(workflow.id); }}>
                    ▶ Execute
                  </button>
                )}
                {workflow.status === "running" && (
                  <button className="action-btn cancel" onClick={(e) => { e.stopPropagation(); handleCancel(workflow.id); }}>
                    ■ Cancel
                  </button>
                )}
              </div>
            </div>
          ))
        )}
      </div>

      {showCreateModal && (
        <CreateWorkflowModal
          onClose={() => setShowCreateModal(false)}
          onCreate={handleCreateWorkflow}
          defaultTasks={defaultTasks}
          creating={creating}
        />
      )}
    </div>
  );
}

function calculateProgress(workflow: Workflow): number {
  if (workflow.tasks.length === 0) return 0;
  const completed = Object.keys(workflow.results).length;
  return Math.round((completed / workflow.tasks.length) * 100);
}

interface CreateWorkflowModalProps {
  onClose: () => void;
  onCreate: (name: string, description: string, tasks: Task[]) => void;
  defaultTasks: Task[];
  creating: boolean;
}

function CreateWorkflowModal({ onClose, onCreate, defaultTasks, creating }: CreateWorkflowModalProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [tasks, setTasks] = useState<Task[]>(defaultTasks);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onCreate(name, description, tasks);
  };

  const addTask = () => {
    const newTask: Task = {
      id: `task-${Date.now()}`,
      type: "research",
      description: "",
      input: {},
      priority: tasks.length + 1,
      dependencies: [],
      metadata: {},
    };
    setTasks([...tasks, newTask]);
  };

  const updateTask = (index: number, field: keyof Task, value: unknown) => {
    const newTasks = [...tasks];
    newTasks[index] = { ...newTasks[index], [field]: value };
    setTasks(newTasks);
  };

  const removeTask = (index: number) => {
    setTasks(tasks.filter((_, i) => i !== index));
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h3>Create New Workflow</h3>
          <button className="modal-close" onClick={onClose}>×</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="modal-body">
            <div className="form-group">
              <label>Workflow Name</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g., BRAF Inhibitor Discovery"
                required
              />
            </div>
            <div className="form-group">
              <label>Description</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe the workflow objective..."
                rows={3}
                required
              />
            </div>
            <div className="form-group">
              <div className="form-group-header">
                <label>Tasks ({tasks.length})</label>
                <button type="button" className="add-task-btn" onClick={addTask}>+ Add Task</button>
              </div>
              {tasks.map((task, index) => (
                <div key={task.id} className="task-editor">
                  <div className="task-editor-header">
                    <span className="task-number">Task {index + 1}</span>
                    <button type="button" className="remove-task-btn" onClick={() => removeTask(index)}>×</button>
                  </div>
                  <div className="task-fields">
                    <div className="field">
                      <label>Type</label>
                      <select
                        value={task.type}
                        onChange={(e) => updateTask(index, "type", e.target.value)}
                      >
                        <option value="research">Research</option>
                        <option value="compute">Compute</option>
                        <option value="critique">Critique</option>
                        <option value="validate">Validate</option>
                        <option value="notify">Notify</option>
                        <option value="plan">Plan</option>
                        <option value="experiment">Experiment</option>
                      </select>
                    </div>
                    <div className="field">
                      <label>Description</label>
                      <input
                        type="text"
                        value={task.description}
                        onChange={(e) => updateTask(index, "description", e.target.value)}
                        placeholder="Task description"
                      />
                    </div>
                    <div className="field">
                      <label>Dependencies (comma-separated task IDs)</label>
                      <input
                        type="text"
                        value={task.dependencies.join(", ")}
                        onChange={(e) => updateTask(index, "dependencies", e.target.value.split(",").map(s => s.trim()).filter(Boolean))}
                        placeholder="task-1, task-2"
                      />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="modal-footer">
            <button type="button" className="btn secondary" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn primary" disabled={creating || !name || !description}>
              {creating ? "Creating..." : "Create Workflow"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}