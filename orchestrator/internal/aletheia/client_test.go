package aletheia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestClient_Query_Mock(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a mock Aletheia server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/debate" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"debate_id": "12345678-1234-5678-1234-567812345678",
			"claim": "BRCA1 mutations increase pancreatic cancer risk",
			"conclusion": "Evidence is mixed",
			"verdict": "unresolved",
			"confidence": 0.5,
			"confidence_rationale": "Anchor B applies: conflicting evidence",
			"driving_provenance_ids": [1, 2, 3],
			"transcript": [
				{"agent": "advocate", "action": "retrieve", "detail": {"title": "Test paper"}, "source_paper_id": "38765432"}
			],
			"sources": [
				{"paper_id": "38765432", "title": "Test paper", "used_by": ["advocate"]}
			]
		}`))
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := client.Query(ctx, "BRCA1 mutations increase pancreatic cancer risk")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "12345678-1234-5678-1234-567812345678", result.DebateID)
	assert.Equal(t, "BRCA1 mutations increase pancreatic cancer risk", result.Claim)
	assert.Equal(t, "unresolved", result.Verdict)
	assert.Equal(t, 0.5, result.Confidence)
	assert.Len(t, result.Transcript, 1)
	assert.Len(t, result.Sources, 1)
}

func TestClient_Query_Error(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create a mock server that returns an error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := client.Query(ctx, "test claim")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Aletheia error")
}

func TestClient_Health(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "healthy"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Health(ctx)
	assert.NoError(t, err)
}

func TestClient_Health_Fail(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unhealthy", http.StatusServiceUnavailable)
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.Health(ctx)
	assert.Error(t, err)
}