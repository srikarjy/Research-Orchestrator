import { useState } from "react";
import { sendNotification } from "../api/agents";
import type { NotificationRequest } from "../types/agents";
import "./NotificationPanel.css";

export function NotificationPanel() {
  const [notificationType, setNotificationType] = useState<NotificationRequest["notification_type"]>("completion");
  const [recipients, setRecipients] = useState("user@example.com");
  const [channels, setChannels] = useState<string[]>(["email"]);
  const [workflowName, setWorkflowName] = useState("Research Workflow");
  const [summary, setSummary] = useState("");
  const [message, setMessage] = useState("");
  const [sending, setSending] = useState(false);
  const [result, setResult] = useState<Record<string, unknown> | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSending(true);
    setError(null);
    setResult(null);

    try {
      const data: Record<string, unknown> = {
        workflow_name: workflowName,
        summary,
        message,
      };

      if (notificationType === "alert") {
        data.alert_type = "warning";
        data.details = summary;
      } else if (notificationType === "progress") {
        data.current_step = workflowName;
        data.progress = 0.5;
        data.total_steps = 5;
        data.completed = "Task 1, Task 2";
        data.next_step = "Task 3";
      } else if (notificationType === "error") {
        data.error = message;
        data.failed_task = workflowName;
        data.stack_trace = summary;
      }

      const request: NotificationRequest = {
        notification_type: notificationType,
        recipients: recipients.split(",").map(r => r.trim()),
        channels,
        data,
      };

      const response = await sendNotification(request);
      setResult(response as Record<string, unknown>);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send notification");
    } finally {
      setSending(false);
    }
  };

  const channelOptions = [
    { value: "email", label: "📧 Email" },
    { value: "webhook", label: "🔗 Webhook" },
    { value: "slack", label: "💬 Slack" },
  ];

  const typeOptions = [
    { value: "completion", label: "✅ Completion" },
    { value: "alert", label: "🚨 Alert" },
    { value: "progress", label: "📊 Progress" },
    { value: "error", label: "❌ Error" },
  ];

  return (
    <div className="notification-panel">
      <h2>Notifications</h2>

      <form onSubmit={handleSubmit}>
        <div className="form-section">
          <h3>Notification Type</h3>
          <div className="radio-group">
            {typeOptions.map((opt) => (
              <label key={opt.value} className="radio-label">
                <input
                  type="radio"
                  name="notificationType"
                  value={opt.value}
                  checked={notificationType === opt.value}
                  onChange={() => setNotificationType(opt.value as NotificationRequest["notification_type"])}
                />
                <span>{opt.label}</span>
              </label>
            ))}
          </div>
        </div>

        <div className="form-section">
          <h3>Channels</h3>
          <div className="checkbox-group">
            {channelOptions.map((opt) => (
              <label key={opt.value} className="checkbox-label">
                <input
                  type="checkbox"
                  value={opt.value}
                  checked={channels.includes(opt.value)}
                  onChange={(e) => {
                    if (e.target.checked) {
                      setChannels([...channels, opt.value]);
                    } else {
                      setChannels(channels.filter(c => c !== opt.value));
                    }
                  }}
                />
                <span>{opt.label}</span>
              </label>
            ))}
          </div>
        </div>

        <div className="form-section">
          <h3>Recipients (comma-separated)</h3>
          <input
            type="text"
            value={recipients}
            onChange={(e) => setRecipients(e.target.value)}
            placeholder="user@example.com, team@example.com"
          />
        </div>

        <div className="form-section">
          <h3>Workflow/Task Name</h3>
          <input
            type="text"
            value={workflowName}
            onChange={(e) => setWorkflowName(e.target.value)}
            placeholder="e.g., BRAF Inhibitor Discovery"
          />
        </div>

        {notificationType !== "progress" && (
          <div className="form-section">
            <h3>Summary / Details</h3>
            <textarea
              value={summary}
              onChange={(e) => setSummary(e.target.value)}
              placeholder="Enter summary or details..."
              rows={3}
            />
          </div>
        )}

        {["alert", "error"].includes(notificationType) && (
          <div className="form-section">
            <h3>Message / Error</h3>
            <textarea
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder="Enter message or error details..."
              rows={3}
            />
          </div>
        )}

        <div className="form-actions">
          <button type="submit" className="btn primary" disabled={sending || channels.length === 0 || !recipients.trim()}>
            {sending ? "Sending..." : "Send Notification"}
          </button>
        </div>
      </form>

      {error && (
        <div className="result error">
          <strong>Error:</strong> {error}
        </div>
      )}

      {result && (
        <div className="result success">
          <strong>Success:</strong>
          <pre>{JSON.stringify(result, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}