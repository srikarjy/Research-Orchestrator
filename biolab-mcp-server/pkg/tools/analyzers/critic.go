package analyzers

import (
	"context"
	"math/rand"
	"time"

)

type CriticAgent struct{}

func NewCriticAgent() *CriticAgent { return &CriticAgent{} }

func (c *CriticAgent) Name() string        { return "Critic" }
func (c *CriticAgent) Category() string    { return "analyzer" }
func (c *CriticAgent) Description() string { return "Critic agent: finds contradictions, assesses evidence quality, assigns stances" }
func (c *CriticAgent) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"claim":          map[string]interface{}{"type": "string", "description": "Central claim to evaluate"},
			"evidence":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}},
			"require_stance": map[string]interface{}{"type": "boolean", "default": true},
		},
		"required": []string{"claim", "evidence"},
	}
}

func (c *CriticAgent) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	claim := input["claim"].(string)
	evidence := input["evidence"].([]interface{})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(200+rand.Intn(500)) * time.Millisecond):
	}

	// Assign stances to evidence
	supports := 0
	contradicts := 0
	neutral := 0
	
	analyzed := make([]map[string]interface{}, len(evidence))
	for i, src := range evidence {
		if srcMap, ok := src.(map[string]interface{}); ok {
			stance := randChoice([]string{"supports", "contradicts", "neutral"}, []float64{0.5, 0.2, 0.3})
			switch stance {
			case "supports":
				supports++
			case "contradicts":
				contradicts++
			default:
				neutral++
			}
			
			analyzed[i] = map[string]interface{}{
				"source_id":  srcMap["id"],
				"stance":     stance,
				"confidence": 0.6 + rand.Float64()*0.3,
				"excerpt":    "Key finding relevant to claim...",
			}
		}
	}

	contradictionScore := 0.0
	if supports+contradicts > 0 {
		contradictionScore = float64(contradicts) / float64(supports+contradicts)
	}

	overallConfidence := 0.8
	if contradictionScore > 0.3 {
		overallConfidence = 0.5
	} else if contradictionScore > 0.1 {
		overallConfidence = 0.65
	}

	return map[string]interface{}{
		"claim":               claim,
		"evidence_analyzed":   len(evidence),
		"supports":            supports,
		"contradicts":         contradicts,
		"neutral":             neutral,
		"contradiction_score": contradictionScore,
		"overall_confidence":  overallConfidence,
		"requires_review":     contradictionScore > 0.25,
		"analyzed_sources":    analyzed,
		"llm_rating":          0.7 + rand.Float64()*0.2,
		"cache_hit":           rand.Float32() < 0.1,
	}, nil
}

func randChoice(options []string, weights []float64) string {
	r := rand.Float64()
	cum := 0.0
	for i, w := range weights {
		cum += w
		if r < cum {
			return options[i]
		}
	}
	return options[len(options)-1]
}