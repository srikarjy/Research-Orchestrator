package reasoning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlannerAgent(t *testing.T) {
	planner := NewPlannerAgent("test-model")
	ctx := context.Background()

	input := map[string]any{
		"question":       "How does BRAF V600E affect binding affinity?",
		"max_steps":      10,
		"domains":        []string{"literature", "protein", "clinical"},
		"require_human_review": true,
	}

	result, err := planner.Execute(ctx, input)
	require.NoError(t, err)

	assert.Contains(t, result, "question")
	assert.Contains(t, result, "steps")
	steps := result["steps"].([]any)
	assert.Greater(t, len(steps), 0)

	step := steps[0].(map[string]any)
	assert.Contains(t, step, "id")
	assert.Contains(t, step, "tool")
	assert.Contains(t, step, "input")
	assert.Contains(t, step, "depends_on")

	assert.Contains(t, result, "model")
	assert.Equal(t, "test-model", result["model"])
}

func TestProposerAgent(t *testing.T) {
	proposer := NewProposerAgent("test-model")
	ctx := context.Background()

	evidence := []any{
		map[string]any{"id": "PMID:123", "title": "Paper 1"},
		map[string]any{"id": "PMID:456", "title": "Paper 2"},
	}

	input := map[string]any{
		"claim":          "BRAF V600E stabilizes active conformation",
		"evidence":       evidence,
		"num_hypotheses": 3,
	}

	result, err := proposer.Execute(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, "BRAF V600E stabilizes active conformation", result["claim"])
	assert.Contains(t, result, "hypotheses")
	
	// Hypotheses are returned as []Hypothesis, need to handle type assertion
	hypotheses := result["hypotheses"]
	assert.NotNil(t, hypotheses)
	
	// Convert to []any for testing
	hypList := hypotheses.([]Hypothesis)
	assert.Equal(t, 3, len(hypList))

	hyp := hypList[0]
	assert.NotEmpty(t, hyp.ID)
	assert.NotEmpty(t, hyp.Statement)
	assert.NotEmpty(t, hyp.Mechanism)
	assert.Greater(t, hyp.Confidence, 0.0)
	assert.NotEmpty(t, hyp.Predictions)
}

func TestCriticAgent(t *testing.T) {
	critic := NewCriticAgent("test-model")
	ctx := context.Background()

	evidence := []any{
		map[string]any{"id": "PMID:123", "title": "Supports claim"},
		map[string]any{"id": "PMID:456", "title": "Contradicts claim"},
	}

	input := map[string]any{
		"claim":       "BRAF V600E stabilizes active conformation",
		"evidence":    evidence,
		"require_stance": true,
	}

	result, err := critic.Execute(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, "BRAF V600E stabilizes active conformation", result["claim"])
	assert.Contains(t, result, "supports")
	assert.Contains(t, result, "contradicts")
	assert.Contains(t, result, "contradiction_score")
	assert.Contains(t, result, "overall_confidence")
	assert.Contains(t, result, "requires_review")
}

func TestSynthesizerAgent(t *testing.T) {
	synthesizer := NewSynthesizerAgent("test-model")
	ctx := context.Background()

	evidence := []any{
		map[string]any{"id": "PMID:123", "title": "Paper 1"},
	}

	criticOutput := map[string]any{
		"contradiction_score": 0.1,
		"supports":            2.0,
		"contradicts":         0.0,
	}

	input := map[string]any{
		"claim":           "BRAF V600E stabilizes active conformation",
		"critic_output":   criticOutput,
		"evidence":        evidence,
	}

	result, err := synthesizer.Execute(ctx, input)
	require.NoError(t, err)

	assert.Contains(t, result, "verdict")
	assert.Contains(t, result, "confidence")
	assert.Contains(t, result, "confidence_rationale")
	assert.Contains(t, result, "rubric_anchor")
	assert.Contains(t, result, "driving_provenance_ids")
	assert.Contains(t, result, "reasoning")
	assert.Contains(t, result, "limitations")

	verdict := result["verdict"].(string)
	assert.Contains(t, []string{"supported", "refuted", "unresolved"}, verdict)

	anchor := result["rubric_anchor"].(string)
	assert.Contains(t, []string{"A", "B", "C", "D"}, anchor)
}

func TestInvestigationPlanStructure(t *testing.T) {
	plan := &InvestigationPlan{
		Question: "Test question",
		Steps: []PlanStep{
			{
				ID:          "step_1",
				Tool:        "PubMed",
				Category:    "retriever",
				Input:       map[string]any{"query": "test"},
				DependsOn:   []string{},
				Description: "Search literature",
				ExpectedOut: "Relevant papers",
			},
		},
	}

	assert.Equal(t, "Test question", plan.Question)
	assert.Len(t, plan.Steps, 1)
	assert.Equal(t, "PubMed", plan.Steps[0].Tool)
}