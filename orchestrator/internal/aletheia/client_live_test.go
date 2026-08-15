package aletheia

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestLive_RealAletheiaRoundTrip is an opt-in guardrail (same pattern as
// wfengine's integration test): proves this package's Client actually
// talks to a real, already-running Aletheia instance and correctly decodes
// its real DebateResponse shape, catching drift between this hand-written
// Go struct and Aletheia's actual JSON contract. Opt-in via
// ALETHEIA_LIVE_URL so it doesn't fail CI/every dev machine that doesn't
// have Aletheia running.
func TestLive_RealAletheiaRoundTrip(t *testing.T) {
	url := os.Getenv("ALETHEIA_LIVE_URL")
	if url == "" {
		t.Skip("set ALETHEIA_LIVE_URL to a running Aletheia instance to run this")
	}

	logger := zap.NewNop()
	client := NewClient(url, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := client.Health(ctx); err != nil {
		t.Fatalf("Health: %v", err)
	}

	result, err := client.Query(ctx, "BRCA1 mutations increase pancreatic cancer risk")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if result.DebateID == "" {
		t.Error("expected non-empty DebateID")
	}
	if result.Verdict == "" {
		t.Error("expected non-empty Verdict")
	}
	if len(result.Sources) == 0 {
		t.Error("expected at least one Source")
	}
	// single_call (the default path this client hits) always returns a
	// per-signal breakdown; a nil here means the wire contract drifted.
	if result.SignalBreakdown == nil {
		t.Error("expected a SignalBreakdown from the single_call path, got nil")
	} else {
		for name, v := range map[string]float64{
			"literature":        result.SignalBreakdown.Literature,
			"protein_evidence":  result.SignalBreakdown.ProteinEvidence,
			"clinical_evidence": result.SignalBreakdown.ClinicalEvidence,
			"llm_rating":        result.SignalBreakdown.LLMRating,
		} {
			if v < 0 || v > 1 {
				t.Errorf("signal %s = %v out of [0,1]", name, v)
			}
		}
	}
	t.Logf("verdict=%s confidence=%.2f breakdown=%+v sources=%d transcript_entries=%d",
		result.Verdict, result.Confidence, result.SignalBreakdown, len(result.Sources), len(result.Transcript))
}
