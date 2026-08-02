package reasoning

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type ProposerAgent struct {
	name        string
	model       string
	temperature float64
}

func NewProposerAgent(model string) *ProposerAgent {
	return &ProposerAgent{
		name:        "Proposer",
		model:       model,
		temperature: 0.7,
	}
}

func (p *ProposerAgent) Name() string        { return "Proposer" }
func (p *ProposerAgent) Category() string    { return "reasoning" }
func (p *ProposerAgent) Description() string { return "Proposes hypotheses and mechanisms based on evidence" }

func (p *ProposerAgent) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"claim":            map[string]interface{}{"type": "string", "description": "Claim or question to address"},
			"evidence":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Evidence from retrievers"},
			"contradictions":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Known contradictions from Critic"},
			"num_hypotheses":   map[string]interface{}{"type": "integer", "default": 3, "description": "Number of hypotheses to generate"},
		},
		"required": []string{"claim", "evidence"},
	}
}

type Hypothesis struct {
	ID          string   `json:"id"`
	Statement   string   `json:"statement"`
	Mechanism   string   `json:"mechanism"`
	Supporting  []string `json:"supporting_evidence"`
	Confidence  float64  `json:"confidence"`
	Testable    bool     `json:"testable"`
	Predictions []string `json:"predictions"`
}

func (p *ProposerAgent) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	claim := input["claim"].(string)
	evidence := input["evidence"].([]interface{})
	numHypotheses := 3
	if nh, ok := input["num_hypotheses"].(float64); ok {
		numHypotheses = int(nh)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(300+rand.Intn(700)) * time.Millisecond):
	}

	hypotheses := p.generateHypotheses(claim, evidence, numHypotheses)

	result := map[string]interface{}{
		"claim":        claim,
		"hypotheses":   hypotheses,
		"model":        p.model,
		"temperature":  p.temperature,
		"cache_hit":    rand.Float32() < 0.05,
	}

	return result, nil
}

func (p *ProposerAgent) generateHypotheses(claim string, evidence []interface{}, n int) []Hypothesis {
	hypotheses := make([]Hypothesis, 0, n)

	claimLower := strings.ToLower(claim)
	hasMutation := strings.Contains(claimLower, "mutation") || strings.Contains(claimLower, "variant")
	hasBinding := strings.Contains(claimLower, "bind") || strings.Contains(claimLower, "affinity")
	hasResistance := strings.Contains(claimLower, "resist")
	_ = strings.Contains(claimLower, "activat") || strings.Contains(claimLower, "signaling")

	templates := []struct {
		statement string
		mechanism string
		predictions []string
	}{
		{
			statement: "The mutation stabilizes the active conformation of the kinase domain",
			mechanism: "V600E substitution mimics phosphorylation of the activation loop, locking BRAF in a constitutively active dimerization-competent state",
			predictions: []string{"Increased basal kinase activity", "Reduced dependence on RAS activation", "Altered inhibitor binding kinetics"},
		},
		{
			statement: "The mutation creates a novel binding pocket for allosteric inhibitors",
			mechanism: "Conformational change exposes a hydrophobic pocket adjacent to the ATP site, enabling selective allosteric inhibition",
			predictions: []string{"Allosteric inhibitors show mutant selectivity", "Combination with ATP-competitive inhibitors is synergistic", "Resistance mutations map to the allosteric pocket"},
		},
		{
			statement: "The mutation alters dimerization dynamics with RAF paralogs",
			mechanism: "V600E promotes BRAF-CRAF heterodimerization in a RAS-independent manner, rewiring MAPK signaling",
			predictions: []string{"CRAF phosphorylation increases", "Paradoxical activation in wild-type cells", "Dimer-disrupting inhibitors are effective"},
		},
		{
			statement: "Resistance emerges via upstream pathway reactivation",
			mechanism: "RTK upregulation or RAS mutation reactivates MAPK signaling downstream of BRAF inhibition",
			predictions: []string{"Increased pERK after initial suppression", "Combination with MEK inhibitors delays resistance", "RTK expression correlates with resistance"},
		},
		{
			statement: "The mutation affects protein stability and degradation",
			mechanism: "V600E alters the conformational ensemble, affecting ubiquitin-mediated degradation and protein half-life",
			predictions: []string{"Longer protein half-life than wild-type", "HSP90 inhibition preferentially degrades mutant", "Proteasome inhibition accumulates mutant protein"},
		},
	}

	for i := 0; i < n && i < len(templates); i++ {
		t := templates[i]
		confidence := 0.6 + rand.Float64()*0.3
		if hasMutation && (i == 0 || i == 2) {
			confidence += 0.1
		}
		if hasBinding && i == 1 {
			confidence += 0.1
		}
		if hasResistance && i == 3 {
			confidence += 0.1
		}

		hypotheses = append(hypotheses, Hypothesis{
			ID:          fmt.Sprintf("HYP-%03d", i+1),
			Statement:   t.statement,
			Mechanism:   t.mechanism,
			Supporting:  extractEvidenceIDs(evidence),
			Confidence:  minFloat(confidence, 0.95),
			Testable:    true,
			Predictions: t.predictions,
		})
	}

	return hypotheses
}

func extractEvidenceIDs(evidence []interface{}) []string {
	ids := make([]string, 0, len(evidence))
	for _, e := range evidence {
		if m, ok := e.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				ids = append(ids, id)
			} else if pmid, ok := m["pmid"].(float64); ok {
				ids = append(ids, fmt.Sprintf("PMID:%d", int(pmid)))
			}
		}
	}
	return ids
}

type CriticAgent struct {
	name        string
	model       string
	temperature float64
}

func NewCriticAgent(model string) *CriticAgent {
	return &CriticAgent{
		name:        "Critic",
		model:       model,
		temperature: 0.2,
	}
}

func (c *CriticAgent) Name() string        { return "Critic" }
func (c *CriticAgent) Category() string    { return "reasoning" }
func (c *CriticAgent) Description() string { return "Critically evaluates evidence, finds contradictions, assigns stances" }

func (c *CriticAgent) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"claim":            map[string]interface{}{"type": "string", "description": "Central claim to evaluate"},
			"evidence":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Evidence from retrievers and proposer"},
			"hypotheses":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "Hypotheses from Proposer"},
			"require_stance":   map[string]interface{}{"type": "boolean", "default": true},
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
				"source_id":  getSourceID(srcMap),
				"stance":     stance,
				"confidence": 0.6 + rand.Float64()*0.3,
				"excerpt":    "Key finding relevant to claim...",
				"reasoning":  "Evidence " + stance + " the claim based on...",
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

	result := map[string]interface{}{
		"claim":                claim,
		"evidence_analyzed":    len(evidence),
		"supports":             supports,
		"contradicts":          contradicts,
		"neutral":              neutral,
		"contradiction_score":  contradictionScore,
		"overall_confidence":   overallConfidence,
		"requires_review":      contradictionScore > 0.25,
		"analyzed_sources":     analyzed,
		"llm_rating":           0.7 + rand.Float64()*0.2,
		"cache_hit":            rand.Float32() < 0.1,
	}

	return result, nil
}

func getSourceID(m map[string]interface{}) string {
	if id, ok := m["id"].(string); ok {
		return id
	}
	if pmid, ok := m["pmid"].(float64); ok {
		return fmt.Sprintf("PMID:%d", int(pmid))
	}
	if chembl, ok := m["chembl_id"].(string); ok {
		return chembl
	}
	return "unknown"
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

type SynthesizerAgent struct {
	name        string
	model       string
	temperature float64
	rubric      map[string]interface{}
}

func NewSynthesizerAgent(model string) *SynthesizerAgent {
	return &SynthesizerAgent{
		name:        "Synthesizer",
		model:       model,
		temperature: 0.1,
		rubric: map[string]interface{}{
			"version": "v1",
			"anchors": map[string]interface{}{
				"A": "A valid challenge undermines the central evidence → confidence ≤ 0.3",
				"B": "Evidence genuinely conflicts and neither side is invalidated → confidence 0.4–0.6, verdict 'unresolved'",
				"C": "Valid challenges touch only peripheral points → confidence 0.5–0.7",
				"D": "No valid substantive challenges → confidence ≥ 0.8",
			},
		},
	}
}

func (s *SynthesizerAgent) Name() string        { return "Synthesizer" }
func (s *SynthesizerAgent) Category() string    { return "reasoning" }
func (s *SynthesizerAgent) Description() string { return "Synthesizes debate into final conclusion with rubric-anchored confidence" }

func (s *SynthesizerAgent) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"claim":               map[string]interface{}{"type": "string", "description": "Original claim"},
			"proposer_output":     map[string]interface{}{"type": "object", "description": "Output from Proposer agent"},
			"critic_output":       map[string]interface{}{"type": "object", "description": "Output from Critic agent"},
			"evidence":            map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "object"}, "description": "All evidence"},
		},
		"required": []string{"claim", "critic_output", "evidence"},
	}
}

type SynthesisResult struct {
	Claim              string   `json:"claim"`
	Verdict            string   `json:"verdict"`
	Confidence         float64  `json:"confidence"`
	ConfidenceRationale string  `json:"confidence_rationale"`
	RubricAnchor       string   `json:"rubric_anchor"`
	DrivingProvenance  []string `json:"driving_provenance_ids"`
	Reasoning          string   `json:"reasoning"`
	KeyEvidence        []string `json:"key_evidence"`
	Limitations        []string `json:"limitations"`
}

func (s *SynthesizerAgent) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	claim := input["claim"].(string)
	criticOutput := input["critic_output"].(map[string]interface{})
	evidence := input["evidence"].([]interface{})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(400+rand.Intn(600)) * time.Millisecond):
	}

	contradictionScore := 0.0
	if cs, ok := criticOutput["contradiction_score"].(float64); ok {
		contradictionScore = cs
	}
	supports := 0
	if su, ok := criticOutput["supports"].(float64); ok {
		supports = int(su)
	}
	contradicts := 0
	if co, ok := criticOutput["contradicts"].(float64); ok {
		contradicts = int(co)
	}

	verdict, confidence, anchor, rationale := s.determineVerdict(contradictionScore, supports, contradicts)

	result := SynthesisResult{
		Claim:               claim,
		Verdict:             verdict,
		Confidence:          confidence,
		ConfidenceRationale: rationale,
		RubricAnchor:        anchor,
		DrivingProvenance:   extractEvidenceIDs(evidence),
		Reasoning:           s.generateReasoning(claim, verdict, contradictionScore, supports, contradicts),
		KeyEvidence:         extractTopEvidence(evidence, 3),
		Limitations:         s.generateLimitations(contradictionScore, supports, contradicts),
	}

	data, _ := json.Marshal(result)
	var output map[string]interface{}
	json.Unmarshal(data, &output)
	output["model"] = s.model
	output["temperature"] = s.temperature
	output["rubric_version"] = s.rubric["version"]
	output["cache_hit"] = rand.Float32() < 0.05

	return output, nil
}

func (s *SynthesizerAgent) determineVerdict(contradictionScore float64, supports, contradicts int) (string, float64, string, string) {
	if contradictionScore > 0.4 {
		return "refuted", 0.25 + rand.Float64()*0.15, "A", "Valid challenges undermine central evidence; contradiction score > 0.4"
	}
	if contradictionScore > 0.2 {
		return "unresolved", 0.45 + rand.Float64()*0.15, "B", "Evidence genuinely conflicts; neither side clearly invalidated"
	}
	if contradictionScore > 0.05 {
		return "supported", 0.55 + rand.Float64()*0.15, "C", "Valid challenges touch peripheral points only"
	}
	return "supported", 0.8 + rand.Float64()*0.15, "D", "No valid substantive challenges to central claim"
}

func (s *SynthesizerAgent) generateReasoning(claim, verdict string, contradictionScore float64, supports, contradicts int) string {
	return fmt.Sprintf("Synthesizer evaluated claim: '%s'. Critic found %d supporting and %d contradicting sources (contradiction score: %.2f). Rubric anchor %s applied. Final verdict: %s.", claim, supports, contradicts, contradictionScore, s.getAnchor(contradictionScore), verdict)
}

func (s *SynthesizerAgent) getAnchor(score float64) string {
	if score > 0.4 {
		return "A"
	}
	if score > 0.2 {
		return "B"
	}
	if score > 0.05 {
		return "C"
	}
	return "D"
}

func (s *SynthesizerAgent) generateLimitations(contradictionScore float64, supports, contradicts int) []string {
	limitations := []string{
		"Limited to available published evidence",
		"Confidence reflects evidence quality, not ground truth probability",
	}
	if contradictionScore > 0.2 {
		limitations = append(limitations, "Significant contradictory evidence present; conclusion uncertain")
	}
	if supports+contradicts < 5 {
		limitations = append(limitations, "Small evidence base; conclusion may change with additional data")
	}
	return limitations
}

func extractTopEvidence(evidence []interface{}, n int) []string {
	top := make([]string, 0, minInt(n, len(evidence)))
	for i, e := range evidence {
		if i >= n {
			break
		}
		if m, ok := e.(map[string]interface{}); ok {
			if title, ok := m["title"].(string); ok {
				top = append(top, title)
			} else if id := getSourceID(m); id != "unknown" {
				top = append(top, id)
			}
		}
	}
	return top
}