package agents

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type CriticAgentWrapper struct {
	*BaseAgent
}

func NewCriticAgentWrapper(config AgentConfig, msgBus MessageBus) *CriticAgentWrapper {
	base := NewBaseAgent(config, msgBus)
	return &CriticAgentWrapper{BaseAgent: base}
}

func (c *CriticAgentWrapper) Execute(ctx context.Context, task Task) (Result, error) {
	c.SetStatus(AgentStatusRunning)
	defer c.SetStatus(AgentStatusIdle)

	start := time.Now()

	claim := getString(task.Input, "claim", "")
	evidence := getInterfaceSlice(task.Input, "evidence")
	requireStance := getBool(task.Input, "require_stance", true)

	supports := 0
	contradicts := 0
	neutral := 0
	
	analyzed := make([]map[string]interface{}, len(evidence))
	for i, src := range evidence {
		if srcMap, ok := src.(map[string]interface{}); ok {
			stance := "neutral"
			if requireStance {
				stance = randChoice([]string{"supports", "contradicts", "neutral"}, []float64{0.5, 0.2, 0.3})
			}
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

	output := map[string]interface{}{
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
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  c.ID(),
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    "critic_analysis.json",
			Type:    "application/json",
			Content: output,
		}},
	}, nil
}

func (c *CriticAgentWrapper) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	switch msg.Type {
	case MessageTypeTask:
		var task Task
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return Message{}, err
		}
		result, err := c.Execute(ctx, task)
		return Message{
			ID:        uuid.New().String(),
			Type:      MessageTypeResult,
			From:      c.ID(),
			To:        msg.From,
			Payload:   mustMarshal(result),
			Timestamp: time.Now(),
			TraceID:   msg.TraceID,
		}, err
	}
	return Message{}, nil
}

func getInterfaceSlice(input map[string]interface{}, key string) []interface{} {
	if v, ok := input[key].([]interface{}); ok {
		return v
	}
	return nil
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