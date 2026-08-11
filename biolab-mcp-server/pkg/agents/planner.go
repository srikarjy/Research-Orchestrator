package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PlannerAgent struct {
	*BaseAgent
}

func NewPlannerAgent(config AgentConfig, msgBus MessageBus) *PlannerAgent {
	base := NewBaseAgent(config, msgBus)
	return &PlannerAgent{BaseAgent: base}
}

func (p *PlannerAgent) Execute(ctx context.Context, task Task) (Result, error) {
	p.SetStatus(AgentStatusRunning)
	defer p.SetStatus(AgentStatusIdle)

	start := time.Now()
	
	goal := getString(task.Input, "goal", "")
	constraints := getStringSlice(task.Input, "constraints")
	budget := getFloat(task.Input, "budget", 10000)
	timeline := getString(task.Input, "timeline", "4 weeks")
	
	plan := ExperimentPlan{
		ID:          uuid.New().String(),
		Goal:        goal,
		Hypothesis:  generateHypothesis(goal),
		Methodology: generateMethodology(goal, constraints),
		Tasks:       generateTasks(goal, constraints),
		Budget:      budget,
		Timeline:    timeline,
		Resources:   generateResources(budget),
		Risks:       identifyRisks(goal),
		SuccessCriteria: []string{
			"Statistical significance p < 0.05",
			"Reproducible across 3+ replicates",
			"Validated by orthogonal method",
		},
		CreatedAt: time.Now(),
	}

	output := map[string]interface{}{
		"plan":       plan,
		"task_count": len(plan.Tasks),
		"estimated_cost": calculateCost(plan),
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  p.ID(),
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    "experiment_plan.json",
			Type:    "application/json",
			Content: plan,
		}},
	}, nil
}

func (p *PlannerAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	switch msg.Type {
	case MessageTypeTask:
		var task Task
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return Message{}, err
		}
		result, err := p.Execute(ctx, task)
		return Message{
			ID:        uuid.New().String(),
			Type:      MessageTypeResult,
			From:      p.ID(),
			To:        msg.From,
			Payload:   mustMarshal(result),
			Timestamp: time.Now(),
			TraceID:   msg.TraceID,
		}, err
	}
	return Message{}, nil
}

type ExperimentPlan struct {
	ID               string        `json:"id"`
	Goal             string        `json:"goal"`
	Hypothesis       string        `json:"hypothesis"`
	Methodology      string        `json:"methodology"`
	Tasks            []PlanTask    `json:"tasks"`
	Budget           float64       `json:"budget"`
	Timeline         string        `json:"timeline"`
	Resources        []Resource    `json:"resources"`
	Risks            []Risk        `json:"risks"`
	SuccessCriteria  []string      `json:"success_criteria"`
	CreatedAt        time.Time     `json:"created_at"`
}

type PlanTask struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Agent       AgentID  `json:"agent"`
	Dependencies []string `json:"dependencies"`
	EstimatedDuration string `json:"estimated_duration"`
	EstimatedCost float64 `json:"estimated_cost"`
}

type Resource struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Quantity    int     `json:"quantity"`
	Cost        float64 `json:"cost"`
	Provider    string  `json:"provider"`
}

type Risk struct {
	Description string  `json:"description"`
	Probability float64 `json:"probability"`
	Impact      string  `json:"impact"`
	Mitigation  string  `json:"mitigation"`
}

func generateHypothesis(goal string) string {
	return fmt.Sprintf("If we systematically investigate %s, we will identify key mechanistic insights with >80%% confidence", goal)
}

func generateMethodology(goal string, constraints []string) string {
	return fmt.Sprintf("Multi-phase approach: 1) Literature review & target identification, 2) Computational modeling, 3) Wet-lab validation, 4) Orthogonal confirmation. Constraints: %v", constraints)
}

func generateTasks(goal string, constraints []string) []PlanTask {
	return []PlanTask{
		{ID: "task-1", Name: "Literature Review", Description: "Comprehensive literature search", Type: "research", Agent: AgentResearcher, EstimatedDuration: "2 days", EstimatedCost: 500},
		{ID: "task-2", Name: "Target Identification", Description: "Identify molecular targets", Type: "analysis", Agent: AgentResearcher, EstimatedDuration: "3 days", EstimatedCost: 1000},
		{ID: "task-3", Name: "Computational Modeling", Description: "Docking & MD simulations", Type: "compute", Agent: AgentExecutor, EstimatedDuration: "5 days", EstimatedCost: 3000},
		{ID: "task-4", Name: "Evidence Synthesis", Description: "Merge & critique evidence", Type: "critique", Agent: AgentCritic, EstimatedDuration: "2 days", EstimatedCost: 500},
		{ID: "task-5", Name: "Experimental Design", Description: "Design wet-lab experiments", Type: "plan", Agent: AgentPlanner, EstimatedDuration: "3 days", EstimatedCost: 1000},
		{ID: "task-6", Name: "Validation Experiments", Description: "Run wet-lab validation", Type: "experiment", Agent: AgentExecutor, EstimatedDuration: "10 days", EstimatedCost: 5000},
		{ID: "task-7", Name: "Result Validation", Description: "Statistical validation", Type: "validate", Agent: AgentValidator, EstimatedDuration: "2 days", EstimatedCost: 500},
		{ID: "task-8", Name: "Report Generation", Description: "Generate final report", Type: "notify", Agent: AgentNotifier, EstimatedDuration: "1 day", EstimatedCost: 200},
	}
}

func generateResources(budget float64) []Resource {
	return []Resource{
		{Name: "HPC Compute", Type: "compute", Quantity: 100, Cost: budget * 0.3, Provider: "AWS/Azure"},
		{Name: "Sequencing", Type: "wetlab", Quantity: 10, Cost: budget * 0.4, Provider: "Core Facility"},
		{Name: "Reagents", Type: "wetlab", Quantity: 50, Cost: budget * 0.2, Provider: "Sigma/Thermo"},
		{Name: "Software Licenses", Type: "software", Quantity: 5, Cost: budget * 0.1, Provider: "Schrödinger/MOE"},
	}
}

func identifyRisks(goal string) []Risk {
	return []Risk{
		{Description: "Target not druggable", Probability: 0.3, Impact: "High", Mitigation: "Early computational assessment"},
		{Description: "Insufficient literature", Probability: 0.2, Impact: "Medium", Mitigation: "Broaden search criteria"},
		{Description: "Experimental failure", Probability: 0.25, Impact: "High", Mitigation: "Parallel orthogonal approaches"},
		{Description: "Budget overrun", Probability: 0.15, Impact: "Medium", Mitigation: "Phased budgeting with go/no-go gates"},
	}
}

func calculateCost(plan ExperimentPlan) float64 {
	total := 0.0
	for _, task := range plan.Tasks {
		total += task.EstimatedCost
	}
	for _, res := range plan.Resources {
		total += res.Cost * float64(res.Quantity)
	}
	return total
}

func getString(input map[string]interface{}, key, defaultVal string) string {
	if v, ok := input[key].(string); ok {
		return v
	}
	return defaultVal
}

func getStringSlice(input map[string]interface{}, key string) []string {
	if v, ok := input[key].([]interface{}); ok {
		result := make([]string, len(v))
		for i, val := range v {
			if s, ok := val.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	return nil
}

func getFloat(input map[string]interface{}, key string, defaultVal float64) float64 {
	if v, ok := input[key].(float64); ok {
		return v
	}
	return defaultVal
}

func mustMarshal(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}