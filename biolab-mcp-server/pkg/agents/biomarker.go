package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type BiomarkerAgent struct {
	*BaseAgent
}

func NewBiomarkerAgent(config AgentConfig, msgBus MessageBus) *BiomarkerAgent {
	base := NewBaseAgent(config, msgBus)
	return &BiomarkerAgent{BaseAgent: base}
}

func (b *BiomarkerAgent) Execute(ctx context.Context, task Task) (Result, error) {
	b.SetStatus(AgentStatusRunning)
	defer b.SetStatus(AgentStatusIdle)

	start := time.Now()

	analysisType := getString(task.Input, "analysis_type", "discovery")
	disease := getString(task.Input, "disease", "")
	omicsData := getMap(task.Input, "omics_data")
	clinicalData := getMap(task.Input, "clinical_data")
	candidates := getInterfaceSlice(task.Input, "candidate_biomarkers")

	var result BiomarkerResult

	switch analysisType {
	case "discovery":
		result = b.discoverBiomarkers(disease, omicsData, clinicalData)
	case "validation":
		result = b.validateBiomarkers(disease, candidates, clinicalData)
	case "qualification":
		result = b.qualifyBiomarkers(disease, candidates)
	case "monitoring":
		result = b.designMonitoringPanel(disease, candidates)
	default:
		result = b.discoverBiomarkers(disease, omicsData, clinicalData)
	}

	output := map[string]interface{}{
		"result":            result,
		"biomarker_count":   len(result.Candidates),
		"high_confidence":   countHighConfidence(result.Candidates),
		"actionable":        countActionable(result.Candidates),
		"recommended_panel": result.RecommendedPanel,
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  b.ID(),
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    "biomarker_analysis.json",
			Type:    "application/json",
			Content: result,
		}},
	}, nil
}

func (b *BiomarkerAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	switch msg.Type {
	case MessageTypeTask:
		var task Task
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return Message{}, err
		}
		result, err := b.Execute(ctx, task)
		return Message{
			ID:        uuid.New().String(),
			Type:      MessageTypeResult,
			From:      b.ID(),
			To:        msg.From,
			Payload:   mustMarshal(result),
			Timestamp: time.Now(),
			TraceID:   msg.TraceID,
		}, err
	}
	return Message{}, nil
}

type BiomarkerResult struct {
	ID                string            `json:"id"`
	AnalysisType      string            `json:"analysis_type"`
	Disease           string            `json:"disease"`
	Candidates        []BiomarkerCandidate `json:"candidates"`
	RecommendedPanel  []string          `json:"recommended_panel"`
	PathwayAnalysis   PathwayEnrichment `json:"pathway_analysis"`
	ClinicalUtility   ClinicalUtility   `json:"clinical_utility"`
	RegulatoryPath    string            `json:"regulatory_path"`
	CompanionDxPotential string         `json:"companion_dx_potential"`
	CreatedAt         time.Time         `json:"created_at"`
}

type BiomarkerCandidate struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Type             string                 `json:"type"`
	Modality         string                 `json:"modality"`
	GeneProtein      string                 `json:"gene_protein"`
	ExpressionChange float64                `json:"expression_change"`
	PValue           float64                `json:"p_value"`
	FDR              float64                `json:"fdr"`
	AUC              float64                `json:"auc"`
	Sensitivity      float64                `json:"sensitivity"`
	Specificity      float64                `json:"specificity"`
	Confidence       float64                `json:"confidence"`
	EvidenceLevel    string                 `json:"evidence_level"`
	Mechanism        string                 `json:"mechanism"`
	Pathways         []string               `json:"pathways"`
	DrugTarget       bool                   `json:"drug_target"`
	ClinicalTrialRef string                 `json:"clinical_trial_ref"`
	Metadata         map[string]interface{} `json:"metadata"`
}

type PathwayEnrichment struct {
	EnrichedPathways []EnrichedPathway `json:"enriched_pathways"`
	KeyMechanisms    []string          `json:"key_mechanisms"`
}

type EnrichedPathway struct {
	PathwayID   string  `json:"pathway_id"`
	Name        string  `json:"name"`
	PValue      float64 `json:"p_value"`
	Genes       []string `json:"genes"`
	Description string  `json:"description"`
}

type ClinicalUtility struct {
	UseCase            string  `json:"use_case"`
	PatientStratification float64 `json:"patient_stratification"`
	ResponsePrediction float64  `json:"response_prediction"`
	PrognosticValue    float64  `json:"prognostic_value"`
	MonitoringValue    float64  `json:"monitoring_value"`
	CostEffectiveness  string   `json:"cost_effectiveness"`
}

func (b *BiomarkerAgent) discoverBiomarkers(disease string, omicsData, clinicalData map[string]interface{}) BiomarkerResult {
	candidates := []BiomarkerCandidate{
		{ID: "BM-001", Name: "PD-L1", Type: "protein", Modality: "IHC", GeneProtein: "CD274", ExpressionChange: 3.2, PValue: 1e-8, FDR: 0.001, AUC: 0.82, Sensitivity: 0.78, Specificity: 0.85, Confidence: 0.92, EvidenceLevel: "Level I", Mechanism: "Immune checkpoint", Pathways: []string{"PD-1/PD-L1", "IFN-gamma"}, DrugTarget: true, ClinicalTrialRef: "NCT02486718"},
		{ID: "BM-002", Name: "TMB", Type: "genomic", Modality: "NGS", GeneProtein: "N/A", ExpressionChange: 0, PValue: 5e-6, FDR: 0.01, AUC: 0.75, Sensitivity: 0.70, Specificity: 0.78, Confidence: 0.88, EvidenceLevel: "Level II", Mechanism: "Neoantigen load", Pathways: []string{"DNA repair", "Mismatch repair"}, DrugTarget: false, ClinicalTrialRef: "NCT03048500"},
		{ID: "BM-003", Name: "CTLA-4", Type: "protein", Modality: "Flow", GeneProtein: "CTLA4", ExpressionChange: 2.1, PValue: 0.001, FDR: 0.05, AUC: 0.68, Sensitivity: 0.65, Specificity: 0.72, Confidence: 0.75, EvidenceLevel: "Level III", Mechanism: "T-cell activation", Pathways: []string{"CTLA-4/CD28", "Co-stimulation"}, DrugTarget: true, ClinicalTrialRef: ""},
		{ID: "BM-004", Name: "CA-125", Type: "protein", Modality: "ELISA", GeneProtein: "MUC16", ExpressionChange: 5.4, PValue: 1e-12, FDR: 1e-6, AUC: 0.91, Sensitivity: 0.88, Specificity: 0.92, Confidence: 0.95, EvidenceLevel: "Level I", Mechanism: "Mucin barrier", Pathways: []string{"Cell adhesion", "Metastasis"}, DrugTarget: false, ClinicalTrialRef: "NCT01873184"},
		{ID: "BM-005", Name: "KRAS G12C", Type: "genomic", Modality: "PCR/NGS", GeneProtein: "KRAS", ExpressionChange: 0, PValue: 1e-15, FDR: 1e-9, AUC: 0.89, Sensitivity: 0.95, Specificity: 0.98, Confidence: 0.98, EvidenceLevel: "Level I", Mechanism: "Constitutive signaling", Pathways: []string{"MAPK", "PI3K-AKT"}, DrugTarget: true, ClinicalTrialRef: "NCT03785249"},
	}

	return BiomarkerResult{
		ID:               uuid.New().String(),
		AnalysisType:     "discovery",
		Disease:          disease,
		Candidates:       candidates,
		RecommendedPanel: []string{"PD-L1", "TMB", "KRAS G12C"},
		PathwayAnalysis: PathwayEnrichment{
			EnrichedPathways: []EnrichedPathway{
				{PathwayID: "hsa04668", Name: "TNF signaling", PValue: 1e-5, Genes: []string{"TNF", "TNFRSF1A", "TRAF2"}, Description: "Inflammatory response"},
				{PathwayID: "hsa04010", Name: "MAPK signaling", PValue: 1e-8, Genes: []string{"KRAS", "BRAF", "MAP2K1"}, Description: "Cell proliferation"},
			},
			KeyMechanisms: []string{"Immune evasion", "Oncogenic signaling", "DNA damage response"},
		},
		ClinicalUtility: ClinicalUtility{
			UseCase:               "Patient selection for immunotherapy",
			PatientStratification: 0.85,
			ResponsePrediction:    0.78,
			PrognosticValue:       0.72,
			MonitoringValue:       0.80,
			CostEffectiveness:     "High - $2,500/test vs $150,000/treatment",
		},
		RegulatoryPath:        "CDx pathway (PMA) or LDT (CLIA)",
		CompanionDxPotential: "High - PD-L1 and KRAS G12C have approved CDx",
		CreatedAt: time.Now(),
	}
}

func (b *BiomarkerAgent) validateBiomarkers(disease string, candidates []interface{}, clinicalData map[string]interface{}) BiomarkerResult {
	validated := []BiomarkerCandidate{}
	for i, c := range candidates {
		if cm, ok := c.(map[string]interface{}); ok {
			validated = append(validated, BiomarkerCandidate{
				ID:               fmt.Sprintf("VAL-%03d", i+1),
				Name:             getString(cm, "name", ""),
				Type:             getString(cm, "type", ""),
				Modality:         getString(cm, "modality", ""),
				GeneProtein:      getString(cm, "gene_protein", ""),
				PValue:           getFloat(cm, "p_value", 0.05),
				AUC:              getFloat(cm, "auc", 0.7),
				Confidence:       getFloat(cm, "confidence", 0.7),
				EvidenceLevel:    getString(cm, "evidence_level", "Level II"),
			})
		}
	}

	return BiomarkerResult{
		ID:               uuid.New().String(),
		AnalysisType:     "validation",
		Disease:          disease,
		Candidates:       validated,
		RecommendedPanel: extractNames(validated),
		PathwayAnalysis: PathwayEnrichment{},
		ClinicalUtility: ClinicalUtility{
			UseCase:            "Validation cohort",
			ResponsePrediction: 0.75,
		},
		RegulatoryPath:        "Analytical validation -> Clinical validation",
		CompanionDxPotential: "Medium - requires prospective trial",
		CreatedAt: time.Now(),
	}
}

func (b *BiomarkerAgent) qualifyBiomarkers(disease string, candidates []interface{}) BiomarkerResult {
	qualified := []BiomarkerCandidate{}
	for i, c := range candidates {
		if cm, ok := c.(map[string]interface{}); ok {
			qualified = append(qualified, BiomarkerCandidate{
				ID:            fmt.Sprintf("QUAL-%03d", i+1),
				Name:          getString(cm, "name", ""),
				EvidenceLevel: getString(cm, "evidence_level", "Level II"),
				Confidence:    getFloat(cm, "confidence", 0.8),
			})
		}
	}

	return BiomarkerResult{
		ID:               uuid.New().String(),
		AnalysisType:     "qualification",
		Disease:          disease,
		Candidates:       qualified,
		RecommendedPanel: extractNames(qualified),
		RegulatoryPath:   "FDA BQS (Biomarker Qualification Program) or EMA BQ",
		CompanionDxPotential: "Depends on context of use",
		CreatedAt: time.Now(),
	}
}

func (b *BiomarkerAgent) designMonitoringPanel(disease string, candidates []interface{}) BiomarkerResult {
	panel := []BiomarkerCandidate{}
	for i, c := range candidates {
		if cm, ok := c.(map[string]interface{}); ok {
			panel = append(panel, BiomarkerCandidate{
				ID:       fmt.Sprintf("MON-%03d", i+1),
				Name:     getString(cm, "name", ""),
				Type:     getString(cm, "type", ""),
				Modality: getString(cm, "modality", ""),
			})
		}
	}

	return BiomarkerResult{
		ID:               uuid.New().String(),
		AnalysisType:     "monitoring",
		Disease:          disease,
		Candidates:       panel,
		RecommendedPanel: extractNames(panel),
		ClinicalUtility: ClinicalUtility{
			UseCase:          "Longitudinal monitoring",
			MonitoringValue:  0.90,
			CostEffectiveness: "Serial liquid biopsy $500/timepoint",
		},
		RegulatoryPath:        "LDT for monitoring",
		CompanionDxPotential: "Not applicable - monitoring only",
		CreatedAt: time.Now(),
	}
}

func countHighConfidence(candidates []BiomarkerCandidate) int {
	count := 0
	for _, c := range candidates {
		if c.Confidence >= 0.85 {
			count++
		}
	}
	return count
}

func countActionable(candidates []BiomarkerCandidate) int {
	count := 0
	for _, c := range candidates {
		if c.DrugTarget || c.EvidenceLevel == "Level I" {
			count++
		}
	}
	return count
}

func extractNames(candidates []BiomarkerCandidate) []string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.Name
	}
	return names
}