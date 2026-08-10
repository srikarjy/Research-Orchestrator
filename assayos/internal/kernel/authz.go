package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type AuthzEngine struct {
	logger     *zap.Logger
	opaURL     string
	enabled    bool
	client     *http.Client
	cache      *redis.Client
	cacheTTL   time.Duration
}

type AuthzRequest struct {
	Input AuthzInput `json:"input"`
}

type AuthzInput struct {
	Actor    string                 `json:"actor"`
	Action   string                 `json:"action"`
	Resource string                 `json:"resource"`
	Context  map[string]interface{} `json:"context"`
}

type AuthzResponse struct {
	Result AuthzResult `json:"result"`
}

type AuthzResult struct {
	Allow    bool                   `json:"allow"`
	Reason   string                 `json:"reason,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func NewAuthzEngine(cfg Config, logger *zap.Logger) *AuthzEngine {
	return &AuthzEngine{
		logger: logger.Named("authz"),
		opaURL:  cfg.Auth.OPAEndpoint,
		enabled: cfg.Auth.Enabled,
		client:  &http.Client{Timeout: 5 * time.Second},
		cacheTTL: 30 * time.Second,
	}
}

func (ae *AuthzEngine) Authorize(ctx context.Context, actor, action, resource string, context map[string]interface{}) (bool, string, error) {
	if !ae.enabled {
		return true, "authz disabled", nil
	}

	// Check cache first
	cacheKey := fmt.Sprintf("authz:%s:%s:%s", actor, action, resource)
	if ae.cache != nil {
		if cached, err := ae.cache.Get(ctx, cacheKey).Result(); err == nil {
			var result AuthzResult
			if json.Unmarshal([]byte(cached), &result) == nil {
				return result.Allow, result.Reason, nil
			}
		}
	}

	req := AuthzRequest{
		Input: AuthzInput{
			Actor:    actor,
			Action:   action,
			Resource: resource,
			Context:  context,
		},
	}

	body, _ := json.Marshal(req)
	resp, err := ae.client.Post(ae.opaURL, "application/json", bytes.NewReader(body))
	if err != nil {
		ae.logger.Error("OPA request failed", zap.Error(err))
		return false, "authz service unavailable", err
	}
	defer resp.Body.Close()

	var authzResp AuthzResponse
	if err := json.NewDecoder(resp.Body).Decode(&authzResp); err != nil {
		return false, "invalid authz response", err
	}

	// Cache result
	if ae.cache != nil {
		if data, _ := json.Marshal(authzResp.Result); data != nil {
			ae.cache.Set(ctx, cacheKey, data, ae.cacheTTL)
		}
	}

	return authzResp.Result.Allow, authzResp.Result.Reason, nil
}

func (ae *AuthzEngine) SetCache(client *redis.Client) {
	ae.cache = client
}