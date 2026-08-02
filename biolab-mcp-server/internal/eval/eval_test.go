package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrecisionEvaluator(t *testing.T) {
	eval := &PrecisionEvaluator{}
	ctx := context.Background()

	tc := &TestCase{
		ID: "test-1",
		Expected: map[string]any{
			"papers": []any{
				map[string]any{"pmid": 1.0},
				map[string]any{"pmid": 2.0},
				map[string]any{"pmid": 3.0},
			},
		},
	}

	actual := map[string]any{
		"papers": []any{
			map[string]any{"pmid": 1.0},
			map[string]any{"pmid": 2.0},
			map[string]any{"pmid": 4.0},
		},
	}

	results, err := eval.Evaluate(ctx, tc, actual)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, MetricPrecision, results[0].Metric)
	assert.InDelta(t, 2.0/3.0, results[0].Value, 0.01)
}

func TestRecallEvaluator(t *testing.T) {
	eval := &RecallEvaluator{}
	ctx := context.Background()

	tc := &TestCase{
		ID: "test-1",
		Expected: map[string]any{
			"papers": []any{
				map[string]any{"pmid": 1.0},
				map[string]any{"pmid": 2.0},
				map[string]any{"pmid": 3.0},
			},
		},
	}

	actual := map[string]any{
		"papers": []any{
			map[string]any{"pmid": 1.0},
			map[string]any{"pmid": 2.0},
		},
	}

	results, err := eval.Evaluate(ctx, tc, actual)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, MetricRecall, results[0].Metric)
	assert.InDelta(t, 2.0/3.0, results[0].Value, 0.01)
}

func TestF1Evaluator(t *testing.T) {
	eval := &F1Evaluator{}
	ctx := context.Background()

	tc := &TestCase{
		ID: "test-1",
		Expected: map[string]any{
			"papers": []any{
				map[string]any{"pmid": 1.0},
				map[string]any{"pmid": 2.0},
				map[string]any{"pmid": 3.0},
			},
		},
	}

	actual := map[string]any{
		"papers": []any{
			map[string]any{"pmid": 1.0},
			map[string]any{"pmid": 2.0},
			map[string]any{"pmid": 4.0},
		},
	}

	results, err := eval.Evaluate(ctx, tc, actual)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, MetricF1, results[0].Metric)

	precision := 2.0 / 3.0
	recall := 2.0 / 3.0
	expectedF1 := 2 * precision * recall / (precision + recall)
	assert.InDelta(t, expectedF1, results[0].Value, 0.01)
}

func TestLatencyEvaluator(t *testing.T) {
	eval := &LatencyEvaluator{ThresholdMs: 1000}
	ctx := context.Background()

	tc := &TestCase{ID: "test-1"}
	actual := map[string]any{"latency_ms": 500.0}

	results, err := eval.Evaluate(ctx, tc, actual)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, MetricLatency, results[0].Metric)
	assert.Equal(t, 500.0, results[0].Value)
	assert.True(t, results[0].Details["passed"].(bool))
}

func TestConfidenceCalibrationEvaluator(t *testing.T) {
	eval := &ConfidenceCalibrationEvaluator{}
	ctx := context.Background()

	tc := &TestCase{
		ID: "test-1",
		Expected: map[string]any{"confidence": 0.8},
	}

	actual := map[string]any{"confidence": 0.75}

	results, err := eval.Evaluate(ctx, tc, actual)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, MetricConfidenceCal, results[0].Metric)
	assert.InDelta(t, 0.05, results[0].Value, 0.01)
}

func TestCitationAccuracyEvaluator(t *testing.T) {
	eval := &CitationAccuracyEvaluator{}
	ctx := context.Background()

	tc := &TestCase{ID: "test-1"}

	actual := map[string]any{
		"citations": []any{
			map[string]any{"pmid": 12345.0},
			map[string]any{"pmid": "PMID:67890"},
			map[string]any{"doi": "10.1038/test"},
			map[string]any{"invalid": "citation"},
		},
	}

	results, err := eval.Evaluate(ctx, tc, actual)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, MetricCitationAcc, results[0].Metric)
	assert.InDelta(t, 0.75, results[0].Value, 0.01)
}

func TestContradictionDetectionEvaluator(t *testing.T) {
	eval := &ContradictionDetectionEvaluator{}
	ctx := context.Background()

	tc := &TestCase{
		ID: "test-1",
		Expected: map[string]any{
			"contradictions": []any{"source A contradicts", "source B contradicts"},
		},
	}

	actual := map[string]any{
		"contradictions": []any{"source A contradicts", "source C contradicts"},
	}

	results, err := eval.Evaluate(ctx, tc, actual)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, MetricContradiction, results[0].Metric)

	precision := 0.5
	recall := 0.5
	expectedF1 := 2 * precision * recall / (precision + recall)
	assert.InDelta(t, expectedF1, results[0].Value, 0.01)
}

func TestBenchmarkHarness(t *testing.T) {
	harness := NewBenchmarkHarness()

	harness.AddEvaluator(&PrecisionEvaluator{})
	harness.AddEvaluator(&RecallEvaluator{})
	harness.AddEvaluator(&F1Evaluator{})

	suite := &TestSuite{
		Name: "test-suite",
		TestCases: []*TestCase{
			{
				ID: "case-1",
				Expected: map[string]any{
					"papers": []any{
						map[string]any{"pmid": 1.0},
						map[string]any{"pmid": 2.0},
					},
				},
			},
		},
	}
	harness.AddSuite(suite)

	mockSystem := &mockSystem{}
	report, err := harness.Run(context.Background(), mockSystem)
	require.NoError(t, err)
	assert.Equal(t, "test-suite", report.SuiteResults["test-suite"].SuiteName)
}

type mockSystem struct{}

func (m *mockSystem) Execute(ctx context.Context, input map[string]any) (any, error) {
	return map[string]any{
		"papers": []any{
			map[string]any{"pmid": 1.0},
			map[string]any{"pmid": 2.0},
		},
	}, nil
}

func TestRegressionDetector(t *testing.T) {
	detector := NewRegressionDetector(0.1)
	detector.SetBaseline("precision", 0.9)

	regressed, diff := detector.Check("precision", 0.75)
	assert.True(t, regressed)
	assert.InDelta(t, -0.166, diff, 0.01)

	regressed, diff = detector.Check("precision", 0.85)
	assert.False(t, regressed)
}