package monitors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// webhookTimeout bounds one delivery attempt. Delivery is best-effort: a
// failed POST is logged, never retried, and never fails the check that
// triggered it — the check row itself is the durable record.
const webhookTimeout = 10 * time.Second

// changePayload is what a changed check POSTs to the monitor's webhook.
// The `text` field makes the same payload render as a readable message in
// Slack/Discord-style incoming webhooks without any configuration.
type changePayload struct {
	Text       string  `json:"text"`
	MonitorID  string  `json:"monitor_id"`
	Claim      string  `json:"claim"`
	Verdict    string  `json:"verdict"`
	Confidence float64 `json:"confidence"`
	ChangeNote string  `json:"change_note"`
	CheckedAt  string  `json:"checked_at"`
}

// ValidateWebhookURL rejects URLs a user-supplied webhook must not target:
// non-HTTP schemes, and hosts that resolve to loopback, link-local, or
// private addresses (the SSRF surface — a webhook that can reach
// localhost:5432 or the cloud metadata endpoint is an attack, not an alert).
func ValidateWebhookURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid webhook_url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook_url must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook_url has no host")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("webhook_url host does not resolve: %w", err)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("webhook_url resolves to a private or local address")
		}
	}
	return nil
}

// fireWebhook delivers the change notification. Best-effort by design; the
// URL was validated at monitor creation, and is re-validated here so a DNS
// record that later flips to a private address (DNS rebinding) is refused
// at delivery time too.
func fireWebhook(ctx context.Context, client *http.Client, logger *zap.Logger, m Monitor, c Check) {
	if err := ValidateWebhookURL(m.WebhookURL); err != nil {
		logger.Warn("webhook refused", zap.String("monitor_id", m.ID), zap.Error(err))
		return
	}
	payload := changePayload{
		Text: fmt.Sprintf("Monitored claim changed: %q — now %s at %.0f%% confidence (%s)",
			m.Claim, c.Verdict, c.Confidence*100, c.ChangeNote),
		MonitorID:  m.ID,
		Claim:      m.Claim,
		Verdict:    c.Verdict,
		Confidence: c.Confidence,
		ChangeNote: c.ChangeNote,
		CheckedAt:  c.CheckedAt.Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		logger.Warn("webhook payload marshal failed", zap.Error(err))
		return
	}

	ctx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.WebhookURL, bytes.NewReader(body))
	if err != nil {
		logger.Warn("webhook request build failed", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("webhook delivery failed", zap.String("monitor_id", m.ID), zap.Error(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		logger.Warn("webhook rejected", zap.String("monitor_id", m.ID), zap.Int("status", resp.StatusCode))
		return
	}
	logger.Info("webhook delivered", zap.String("monitor_id", m.ID), zap.String("note", c.ChangeNote))
}
