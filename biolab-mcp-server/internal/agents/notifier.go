package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type NotifierAgent struct {
	*BaseAgent
	emailConfig EmailConfig
	logger      *zap.Logger
	webhooks    []WebhookConfig
}

type EmailConfig struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	FromAddress  string `json:"from_address"`
	FromName     string `json:"from_name"`
	UseTLS       bool   `json:"use_tls"`
}

type WebhookConfig struct {
	URL        string            `json:"url"`
	Secret     string            `json:"secret"`
	Headers    map[string]string `json:"headers"`
	Events     []string          `json:"events"`
}

type Notification struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Recipient string                 `json:"recipient"`
	Subject   string                 `json:"subject"`
	Body      string                 `json:"body"`
	Priority  string                 `json:"priority"`
	Channels  []string               `json:"channels"`
	Metadata  map[string]interface{} `json:"metadata"`
	SentAt    *time.Time             `json:"sent_at,omitempty"`
	Status    string                 `json:"status"`
}

func NewNotifierAgent(config AgentConfig, msgBus MessageBus, emailConfig EmailConfig, logger *zap.Logger) *NotifierAgent {
	base := NewBaseAgent(config, msgBus)
	return &NotifierAgent{
		BaseAgent:   base,
		emailConfig: emailConfig,
		logger:      logger,
		webhooks:    []WebhookConfig{},
	}
}

func (n *NotifierAgent) AddWebhook(webhook WebhookConfig) {
	n.webhooks = append(n.webhooks, webhook)
}

func (n *NotifierAgent) Execute(ctx context.Context, task Task) (Result, error) {
	n.SetStatus(AgentStatusRunning)
	defer n.SetStatus(AgentStatusIdle)

	start := time.Now()

	notificationType := getString(task.Input, "notification_type", "completion")
	recipients := getStringSlice(task.Input, "recipients")
	channels := getStringSlice(task.Input, "channels")
	
	if len(channels) == 0 {
		channels = []string{"email"}
	}

	data := getMap(task.Input, "data")
	
	var notifications []Notification
	var err error

	switch notificationType {
	case "completion":
		notifications, err = n.sendCompletionNotifications(ctx, recipients, channels, data)
	case "alert":
		notifications, err = n.sendAlertNotifications(ctx, recipients, channels, data)
	case "progress":
		notifications, err = n.sendProgressNotifications(ctx, recipients, channels, data)
	case "error":
		notifications, err = n.sendErrorNotifications(ctx, recipients, channels, data)
	default:
		notifications, err = n.sendCompletionNotifications(ctx, recipients, channels, data)
	}

	output := map[string]interface{}{
		"notifications_sent": len(notifications),
		"notifications":      notifications,
		"channels_used":      channels,
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  n.ID(),
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
	}, err
}

func (n *NotifierAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	switch msg.Type {
	case MessageTypeTask:
		var task Task
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return Message{}, err
		}
		result, err := n.Execute(ctx, task)
		return Message{
			ID:        uuid.New().String(),
			Type:      MessageTypeResult,
			From:      n.ID(),
			To:        msg.From,
			Payload:   mustMarshal(result),
			Timestamp: time.Now(),
			TraceID:   msg.TraceID,
		}, err
	case MessageTypeNotification:
		var notif Notification
		if err := json.Unmarshal(msg.Payload, &notif); err != nil {
			return Message{}, err
		}
		_ = n.sendNotification(ctx, notif)
		return Message{}, nil
	}
	return Message{}, nil
}

func (n *NotifierAgent) sendCompletionNotifications(ctx context.Context, recipients, channels []string, data map[string]interface{}) ([]Notification, error) {
	workflowName := getString(data, "workflow_name", "Research Workflow")
	status := getString(data, "status", "completed")
	duration := getString(data, "duration", "N/A")
	
	subject := fmt.Sprintf("✅ %s %s", workflowName, strings.Title(status))
	body := fmt.Sprintf(`Workflow: %s
Status: %s
Duration: %s

Summary:
%s

Details available in the Research Orchestrator dashboard.`, workflowName, status, duration, getString(data, "summary", "Workflow completed successfully"))

	return n.sendBatch(ctx, recipients, channels, subject, body, "normal", data)
}

func (n *NotifierAgent) sendAlertNotifications(ctx context.Context, recipients, channels []string, data map[string]interface{}) ([]Notification, error) {
	alertType := getString(data, "alert_type", "warning")
	message := getString(data, "message", "Alert triggered")
	
	subject := fmt.Sprintf("🚨 Research Alert: %s", strings.Title(alertType))
	body := fmt.Sprintf(`Alert Type: %s
Message: %s
Time: %s

Details:
%s`, alertType, message, time.Now().Format(time.RFC3339), getString(data, "details", ""))

	return n.sendBatch(ctx, recipients, channels, subject, body, "high", data)
}

func (n *NotifierAgent) sendProgressNotifications(ctx context.Context, recipients, channels []string, data map[string]interface{}) ([]Notification, error) {
	step := getString(data, "current_step", "Unknown")
	progress := getFloat(data, "progress", 0)
	totalSteps := getInt(data, "total_steps", 0)
	
	subject := fmt.Sprintf("📊 Progress Update: %s (%.0f%%)", step, progress*100)
	body := fmt.Sprintf(`Current Step: %s
Progress: %.0f%% (%d/%d steps)

Completed: %s
Next: %s`, step, progress*100, int(progress*float64(totalSteps)), totalSteps,
		getString(data, "completed", "N/A"), getString(data, "next_step", "N/A"))

	return n.sendBatch(ctx, recipients, channels, subject, body, "low", data)
}

func (n *NotifierAgent) sendErrorNotifications(ctx context.Context, recipients, channels []string, data map[string]interface{}) ([]Notification, error) {
	errorMsg := getString(data, "error", "Unknown error")
	task := getString(data, "failed_task", "Unknown")
	
	subject := fmt.Sprintf("❌ Task Failed: %s", task)
	body := fmt.Sprintf(`Failed Task: %s
Error: %s
Time: %s

Stack Trace:
%s

Please check the dashboard for details.`, task, errorMsg, time.Now().Format(time.RFC3339), getString(data, "stack_trace", ""))

	return n.sendBatch(ctx, recipients, channels, subject, body, "critical", data)
}

func (n *NotifierAgent) sendBatch(ctx context.Context, recipients, channels []string, subject, body, priority string, metadata map[string]interface{}) ([]Notification, error) {
	notifications := make([]Notification, 0)
	
	for _, recipient := range recipients {
		for _, channel := range channels {
			notif := Notification{
				ID:        uuid.New().String(),
				Type:      "notification",
				Recipient: recipient,
				Subject:   subject,
				Body:      body,
				Priority:  priority,
				Channels:  []string{channel},
				Metadata:  metadata,
				Status:    "pending",
			}
			
			err := n.sendNotification(ctx, notif)
			now := time.Now()
			notif.SentAt = &now
			if err != nil {
				notif.Status = "failed"
				n.logger.Error("Notification failed", zap.String("recipient", recipient), zap.String("channel", channel), zap.Error(err))
			} else {
				notif.Status = "sent"
			}
			notifications = append(notifications, notif)
		}
	}
	
	return notifications, nil
}

func (n *NotifierAgent) sendNotification(ctx context.Context, notif Notification) error {
	for _, channel := range notif.Channels {
		switch channel {
		case "email":
			if err := n.sendEmail(notif); err != nil {
				return err
			}
		case "webhook":
			if err := n.sendWebhook(notif); err != nil {
				return err
			}
		case "slack":
			if err := n.sendSlack(notif); err != nil {
				return err
			}
		}
	}
	return nil
}

func (n *NotifierAgent) sendEmail(notif Notification) error {
	if n.emailConfig.SMTPHost == "" {
		return nil // Skip if not configured
	}

	auth := smtp.PlainAuth("", n.emailConfig.Username, n.emailConfig.Password, n.emailConfig.SMTPHost)
	
	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		n.emailConfig.FromName, n.emailConfig.FromAddress, notif.Recipient, notif.Subject, notif.Body)

	addr := fmt.Sprintf("%s:%d", n.emailConfig.SMTPHost, n.emailConfig.SMTPPort)
	return smtp.SendMail(addr, auth, n.emailConfig.FromAddress, []string{notif.Recipient}, []byte(msg))
}

func (n *NotifierAgent) sendWebhook(notif Notification) error {
	// Implementation would use http.Post
	return nil
}

func (n *NotifierAgent) sendSlack(notif Notification) error {
	// Implementation would use Slack webhook
	return nil
}

func (n *NotifierAgent) SendWorkflowNotification(ctx context.Context, workflowID, workflowName, status string, recipients []string) error {
	notif := Notification{
		ID:        uuid.New().String(),
		Type:      "workflow_" + status,
		Recipient: strings.Join(recipients, ","),
		Subject:   fmt.Sprintf("Workflow %s: %s", status, workflowName),
		Body:      fmt.Sprintf("Workflow %s has %s", workflowName, status),
		Priority:  "normal",
		Channels:  []string{"email"},
		Metadata:  map[string]interface{}{"workflow_id": workflowID, "workflow_name": workflowName},
	}
	return n.sendNotification(ctx, notif)
}