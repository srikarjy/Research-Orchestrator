package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type ClinicalTrialAgent struct {
	*BaseAgent
}

func NewClinicalTrialAgent(config AgentConfig, msgBus MessageBus) *ClinicalTrialAgent {
	base := NewBaseAgent(config, msgBus)
	return &ClinicalTrialAgent{BaseAgent: base}
}

func (c *ClinicalTrialAgent) Execute(ctx context.Context, task Task) (Result, error) {
	c.SetStatus(AgentStatusRunning)
	defer c.SetStatus(AgentStatusIdle)

	start := time.Now()

	trialType := getString(task.Input, "trial_type", "phase1")
	indication := getString(task.Input, "indication", "")
	target := getString(task.Input, "target", "")
	biomarker := getString(task.Input, "biomarker", "")
	patientPopulation := getString(task.Input, "patient_population", "")
	primaryEndpoint := getString(task.Input, "primary_endpoint", "")
	secondaryEndpoints := getStringSlice(task.Input, "secondary_endpoints")

	design := ClinicalTrialDesign{
		ID:                  uuid.New().String(),
		ProtocolNumber:      generateProtocolNumber(),
		Title:               fmt.Sprintf("A %s Study of %s in %s", trialType, target, indication),
		Phase:               trialType,
		Indication:          indication,
		Target:              target,
		Biomarker:           biomarker,
		DesignType:          "Randomized, Double-blind, Placebo-controlled",
		PatientPopulation:   patientPopulation,
		InclusionCriteria:   generateInclusionCriteria(indication, biomarker),
		ExclusionCriteria:   generateExclusionCriteria(),
		PrimaryEndpoint:     primaryEndpoint,
		SecondaryEndpoints:  secondaryEndpoints,
		SampleSize:          calculateSampleSize(trialType, primaryEndpoint),
		Duration:            calculateDuration(trialType),
		Sites:               estimateSites(trialType),
		EstimatedCost:       estimateCost(trialType, calculateSampleSize(trialType, primaryEndpoint)),
		RegulatoryPathway:   determineRegulatoryPathway(trialType, indication),
		AdaptiveFeatures:    generateAdaptiveFeatures(trialType),
		BiomarkerStrategy:   generateBiomarkerStrategy(biomarker),
		StatisticalPlan:     generateStatisticalPlan(primaryEndpoint),
		SafetyMonitoring:    generateSafetyPlan(),
		CreatedAt:           time.Now(),
		Status:              "draft",
	}

	output := map[string]interface{}{
		"design":          design,
		"feasibility_score": calculateFeasibility(design),
		"risk_assessment":   assessRisks(design),
		"timeline":          generateTimeline(design),
		"budget_breakdown":  generateBudgetBreakdown(design),
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  c.ID(),
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    "clinical_trial_protocol.json",
			Type:    "application/json",
			Content: design,
		}},
	}, nil
}

func (c *ClinicalTrialAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
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

type ClinicalTrialDesign struct {
	ID                   string   `json:"id"`
	ProtocolNumber       string   `json:"protocol_number"`
	Title                string   `json:"title"`
	Phase                string   `json:"phase"`
	Indication           string   `json:"indication"`
	Target               string   `json:"target"`
	Biomarker            string   `json:"biomarker"`
	DesignType           string   `json:"design_type"`
	PatientPopulation    string   `json:"patient_population"`
	InclusionCriteria    []string `json:"inclusion_criteria"`
	ExclusionCriteria    []string `json:"exclusion_criteria"`
	PrimaryEndpoint      string   `json:"primary_endpoint"`
	SecondaryEndpoints   []string `json:"secondary_endpoints"`
	SampleSize           int      `json:"sample_size"`
	Duration             string   `json:"duration"`
	Sites                int      `json:"sites"`
	EstimatedCost        float64  `json:"estimated_cost_usd"`
	RegulatoryPathway    string   `json:"regulatory_pathway"`
	AdaptiveFeatures     []string `json:"adaptive_features"`
	BiomarkerStrategy    string   `json:"biomarker_strategy"`
	StatisticalPlan      string   `json:"statistical_plan"`
	SafetyMonitoring     string   `json:"safety_monitoring"`
	CreatedAt            time.Time `json:"created_at"`
	Status               string   `json:"status"`
}

func generateProtocolNumber() string {
	return fmt.Sprintf("PROT-%d-%04d", time.Now().Year(), rand.Intn(10000))
}

func generateInclusionCriteria(indication, biomarker string) []string {
	criteria := []string{
		"Age 18-75 years",
		"ECOG performance status 0-1",
		"Adequate organ function",
		fmt.Sprintf("Histologically confirmed %s", indication),
	}
	if biomarker != "" {
		criteria = append(criteria, fmt.Sprintf("Positive for %s biomarker", biomarker))
	}
	return criteria
}

func generateExclusionCriteria() []string {
	return []string{
		"Prior treatment with target therapy",
		"Active CNS metastases",
		"Significant cardiovascular disease",
		"Pregnancy or breastfeeding",
		"Concurrent investigational agents",
	}
}

func calculateSampleSize(phase, endpoint string) int {
	base := map[string]int{"phase1": 30, "phase2": 100, "phase3": 500, "phase4": 1000}
	return base[phase] + rand.Intn(50)
}

func calculateDuration(phase string) string {
	durations := map[string]string{"phase1": "12-18 months", "phase2": "18-24 months", "phase3": "3-5 years", "phase4": "2-3 years"}
	return durations[phase]
}

func estimateSites(phase string) int {
	sites := map[string]int{"phase1": 3, "phase2": 15, "phase3": 100, "phase4": 50}
	return sites[phase] + rand.Intn(10)
}

func estimateCost(phase string, sampleSize int) float64 {
	costPerPatient := map[string]float64{"phase1": 50000, "phase2": 75000, "phase3": 150000, "phase4": 100000}
	return costPerPatient[phase] * float64(sampleSize)
}

func determineRegulatoryPathway(phase, indication string) string {
	if phase == "phase1" {
		return "IND (Investigational New Drug)"
	}
	if phase == "phase2" || phase == "phase3" {
		return "IND -> NDA/BLA"
	}
	return "Post-marketing surveillance"
}

func generateAdaptiveFeatures(phase string) []string {
	features := []string{"Interim analysis for futility"}
	if phase == "phase2" || phase == "phase3" {
		features = append(features, "Sample size re-estimation", "Seamless phase 2/3 design")
	}
	if phase == "phase3" {
		features = append(features, "Adaptive randomization", "Biomarker-driven enrichment")
	}
	return features
}

func generateBiomarkerStrategy(biomarker string) string {
	if biomarker == "" {
		return "Exploratory biomarker analysis planned"
	}
	return fmt.Sprintf("Companion diagnostic for %s; stratified randomization; predictive/enrichment strategy", biomarker)
}

func generateStatisticalPlan(endpoint string) string {
	return fmt.Sprintf("Primary analysis: %s using stratified log-rank test; 95%% CI; multiplicity adjustment via hierarchical testing", endpoint)
}

func generateSafetyPlan() string {
	return "DSMB reviews every 6 months; SAE reporting per 21 CFR 312.32; stopping rules for grade 4 toxicities >20%%"
}

func calculateFeasibility(design ClinicalTrialDesign) float64 {
	score := 0.7
	if design.Biomarker != "" {
		score += 0.1
	}
	if design.SampleSize < 200 {
		score += 0.1
	}
	if design.Sites < 50 {
		score += 0.1
	}
	return score
}

func assessRisks(design ClinicalTrialDesign) []string {
	risks := []string{}
	if design.SampleSize > 500 {
		risks = append(risks, "Recruitment timeline risk")
	}
	if design.Biomarker != "" {
		risks = append(risks, "Biomarker assay validation risk")
	}
	if design.Phase == "phase3" {
		risks = append(risks, "Regulatory approval risk", "Competitive landscape risk")
	}
	return risks
}

func generateTimeline(design ClinicalTrialDesign) map[string]string {
	return map[string]string{
		"protocol_finalization": "Month 1-2",
		"regulatory_submission": "Month 2-3",
		"site_initiation":       "Month 3-5",
		"first_patient_in":      "Month 5-6",
		"enrollment_complete":   fmt.Sprintf("Month %d", 6+design.SampleSize/10),
		"database_lock":         fmt.Sprintf("Month %d", 6+design.SampleSize/10+3),
		"topline_results":       fmt.Sprintf("Month %d", 6+design.SampleSize/10+4),
	}
}

func generateBudgetBreakdown(design ClinicalTrialDesign) map[string]float64 {
	total := design.EstimatedCost
	return map[string]float64{
		"site_fees":           total * 0.35,
		"patient_costs":       total * 0.25,
		"drug_supply":         total * 0.15,
		"monitoring":          total * 0.10,
		"data_management":     total * 0.05,
		"statistics":          total * 0.03,
		"regulatory":          total * 0.02,
		"contingency":         total * 0.05,
	}
}