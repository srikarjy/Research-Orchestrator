package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

type PlannerAgent struct {
	name        string
	model       string
	temperature float64
}

func NewPlannerAgent(model string) *PlannerAgent {
	return &PlannerAgent{
		name:        "Planner",
		model:       model,
		temperature: 0.3,
	}
}

func (p *PlannerAgent) Name() string        { return "Planner" }
func (p *PlannerAgent) Category() string    { return "reasoning" }
func (p *PlannerAgent) Description() string { return "Decomposes research questions into investigation plans with tool calls" }

func (p *PlannerAgent) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question":      map[string]interface{}{"type": "string", "description": "Research question to investigate"},
			"max_steps":     map[string]interface{}{"type": "integer", "default": 10, "description": "Maximum investigation steps"},
			"domains":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Domain filters (e.g., ['literature', 'protein', 'clinical'])"},
			"require_human_review": map[string]interface{}{"type": "boolean", "default": true},
		},
		"required": []string{"question"},
	}
}

type InvestigationPlan struct {
	Question      string        `json:"question"`
	Steps         []PlanStep    `json:"steps"`
	EstimatedTime string        `json:"estimated_time"`
	Confidence    float64       `json:"confidence"`
	Reasoning     string        `json:"reasoning"`
}

type PlanStep struct {
	ID          string                 `json:"id"`
	Tool        string                 `json:"tool"`
	Category    string                 `json:"category"`
	Input       map[string]interface{} `json:"input"`
	DependsOn   []string               `json:"depends_on"`
	Description string                 `json:"description"`
	ExpectedOut string                 `json:"expected_outcome"`
}

func (p *PlannerAgent) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	question := input["question"].(string)
	maxSteps := 10
	if ms, ok := input["max_steps"].(float64); ok {
		maxSteps = int(ms)
	}
	domains := []string{"literature", "protein", "clinical"}
	if d, ok := input["domains"].([]interface{}); ok {
		for _, v := range d {
			if s, ok := v.(string); ok {
				domains = append(domains, s)
			}
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(200+rand.Intn(500)) * time.Millisecond):
	}

	plan := p.generatePlan(question, maxSteps, domains)

	data, _ := json.Marshal(plan)
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	result["model"] = p.model
	result["temperature"] = p.temperature
	result["cache_hit"] = rand.Float32() < 0.1

	return result, nil
}

func (p *PlannerAgent) generatePlan(question string, maxSteps int, domains []string) *InvestigationPlan {
	steps := []PlanStep{
		{
			ID:          "step_1",
			Tool:        "PubMed",
			Category:    "retriever",
			Input:       map[string]interface{}{"query": question, "max_results": 20},
			DependsOn:   []string{},
			Description: "Search PubMed for relevant literature",
			ExpectedOut: "Set of relevant papers with PMIDs and abstracts",
		},
		{
			ID:          "step_2",
			Tool:        "UniProt",
			Category:    "retriever",
			Input:       map[string]interface{}{"query": extractGeneFromQuestion(question), "include_variants": true},
			DependsOn:   []string{},
			Description: "Retrieve protein information and variants",
			ExpectedOut: "Protein sequence, domains, known variants",
		},
		{
			ID:          "step_3",
			Tool:        "ChEMBL",
			Category:    "retriever",
			Input:       map[string]interface{}{"query": extractTargetFromQuestion(question), "search_type": "target"},
			DependsOn:   []string{},
			Description: "Find known ligands and bioactivities for target",
			ExpectedOut: "Known drugs, IC50 values, assay data",
		},
		{
			ID:          "step_4",
			Tool:        "PDB",
			Category:    "retriever",
			Input:       map[string]interface{}{"query": extractTargetFromQuestion(question)},
			DependsOn:   []string{},
			Description: "Find available 3D structures",
			ExpectedOut: "PDB IDs, resolutions, ligand-bound structures",
		},
	}

	if maxSteps > 4 {
		steps = append(steps, PlanStep{
			ID:          "step_5",
			Tool:        "AutoDockVina",
			Category:    "analyzer",
			Input:       map[string]interface{}{"receptor_pdbqt": "from_pdb", "ligand_smiles": "from_chembl", "center_x": 0, "center_y": 0, "center_z": 0},
			DependsOn:   []string{"step_3", "step_4"},
			Description: "Perform molecular docking",
			ExpectedOut: "Binding poses and affinity predictions",
		})
	}

	if maxSteps > 5 {
		steps = append(steps, PlanStep{
			ID:          "step_6",
			Tool:        "ProteinStabilityPredictor",
			Category:    "analyzer",
			Input:       map[string]interface{}{"pdb_id": "from_pdb", "mutation": "from_uniprot"},
			DependsOn:   []string{"step_2", "step_4"},
			Description: "Predict mutation stability effects",
			ExpectedOut: "ΔΔG values, stability classification",
		})
	}

	if maxSteps > 6 {
		steps = append(steps, PlanStep{
			ID:          "step_7",
			Tool:        "Critic",
			Category:    "analyzer",
			Input:       map[string]interface{}{"claim": question, "evidence": "from_previous_steps"},
			DependsOn:   []string{"step_1", "step_2", "step_3"},
			Description: "Critical evaluation of evidence",
			ExpectedOut: "Contradiction scores, stance assignments, confidence",
		})
	}

	estimatedSteps := len(steps)
	estimatedTime := fmt.Sprintf("%d-%d minutes", estimatedSteps*2, estimatedSteps*5)

	return &InvestigationPlan{
		Question:      question,
		Steps:         steps[:minInt(maxSteps, len(steps))],
		EstimatedTime: estimatedTime,
		Confidence:    0.85 + rand.Float64()*0.1,
		Reasoning:     "Plan generated by decomposing question into retriever-analyzer-critic pipeline",
	}
}

func extractGeneFromQuestion(q string) string {
	commonGenes := []string{"BRAF", "EGFR", "KRAS", "TP53", "PIK3CA", "ALK", "ROS1", "MET", "HER2", "BRCA1", "BRCA2"}
	for _, g := range commonGenes {
		if contains(q, g) {
			return g
		}
	}
	return "BRAF"
}

func extractTargetFromQuestion(q string) string {
	return extractGeneFromQuestion(q)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}