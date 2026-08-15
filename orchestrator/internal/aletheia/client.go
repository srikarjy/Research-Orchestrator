package aletheia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

type DebateRequest struct {
	Claim string `json:"claim"`
}

type Source struct {
	PaperID string   `json:"paper_id"`
	Title   string   `json:"title"`
	UsedBy  []string `json:"used_by"`
	// Retraction screen results; false can mean "unknown" (screen degraded
	// or non-PubMed source) -- true is always a real PubMed marker.
	Retracted        bool    `json:"retracted"`
	Concern          bool    `json:"concern"`
	RetractionNotice *string `json:"retraction_notice,omitempty"`
}

type TranscriptEntry struct {
	Agent         string         `json:"agent"`
	Action        string         `json:"action"`
	Detail        map[string]any `json:"detail"`
	SourcePaperID *string        `json:"source_paper_id,omitempty"`
}

// SignalBreakdown carries single_call's per-evidence-type support scores,
// each in [0,1]. Nil when the response predates the field or came from the
// multi-agent path, which doesn't produce one -- consumers must render
// "breakdown unavailable" in that case, never fabricate four values from
// the scalar Confidence.
type SignalBreakdown struct {
	Literature       float64 `json:"literature"`
	ProteinEvidence  float64 `json:"protein_evidence"`
	ClinicalEvidence float64 `json:"clinical_evidence"`
	LLMRating        float64 `json:"llm_rating"`
}

type DebateResponse struct {
	DebateID             string            `json:"debate_id"`
	Claim                string            `json:"claim"`
	Conclusion           string            `json:"conclusion"`
	Verdict              string            `json:"verdict"`
	Confidence           float64           `json:"confidence"`
	ConfidenceRationale  string            `json:"confidence_rationale"`
	SignalBreakdown      *SignalBreakdown  `json:"signal_breakdown,omitempty"`
	DrivingProvenanceIDs []int             `json:"driving_provenance_ids"`
	Transcript           []TranscriptEntry `json:"transcript"`
	Sources              []Source          `json:"sources"`
}

func NewClient(baseURL string, logger *zap.Logger) *Client {
	if baseURL == "" {
		baseURL = "http://aletheia:8000"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger.Named("aletheia-client"),
	}
}

func (c *Client) Query(ctx context.Context, claim string) (*DebateResponse, error) {
	reqBody := DebateRequest{Claim: claim}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/debate"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("Calling Aletheia /debate", zap.String("url", url), zap.String("claim", claim))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Aletheia: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("Aletheia error: %s - %v", resp.Status, errResp)
	}

	var result DebateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("Aletheia query completed",
		zap.String("debate_id", result.DebateID),
		zap.String("verdict", result.Verdict),
		zap.Float64("confidence", result.Confidence),
	)

	return &result, nil
}

func (c *Client) Health(ctx context.Context) error {
	url := c.baseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Aletheia health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Aletheia health check failed: %s", resp.Status)
	}
	return nil
}
