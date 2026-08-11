package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ValidatorAgent struct {
	*BaseAgent
}

func NewValidatorAgent(config AgentConfig, msgBus MessageBus) *ValidatorAgent {
	base := NewBaseAgent(config, msgBus)
	return &ValidatorAgent{BaseAgent: base}
}

func (v *ValidatorAgent) Execute(ctx context.Context, task Task) (Result, error) {
	v.SetStatus(AgentStatusRunning)
	defer v.SetStatus(AgentStatusIdle)

	start := time.Now()

	results := getMap(task.Input, "results")
	validationType := getString(task.Input, "validation_type", "statistical")
	thresholds := getMap(task.Input, "thresholds")

	validation := ValidationResult{
		ID:           uuid.New().String(),
		Type:         validationType,
		Timestamp:    time.Now(),
		Checks:       []ValidationCheck{},
		OverallPass:  true,
		Confidence:   0.0,
	}

	switch validationType {
	case "statistical":
		validation = v.validateStatistical(results, thresholds)
	case "reproducibility":
		validation = v.validateReproducibility(results)
	case "orthogonal":
		validation = v.validateOrthogonal(results)
	case "quality":
		validation = v.validateQuality(results)
	default:
		validation = v.validateStatistical(results, thresholds)
	}

	output := map[string]interface{}{
		"validation": validation,
		"passed":     validation.OverallPass,
		"confidence": validation.Confidence,
	}

	status := "completed"
	if !validation.OverallPass {
		status = "failed"
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  v.ID(),
		Status:   status,
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    "validation_report.json",
			Type:    "application/json",
			Content: validation,
		}},
	}, nil
}

func (v *ValidatorAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	switch msg.Type {
	case MessageTypeTask:
		var task Task
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return Message{}, err
		}
		result, err := v.Execute(ctx, task)
		return Message{
			ID:        uuid.New().String(),
			Type:      MessageTypeResult,
			From:      v.ID(),
			To:        msg.From,
			Payload:   mustMarshal(result),
			Timestamp: time.Now(),
			TraceID:   msg.TraceID,
		}, err
	}
	return Message{}, nil
}

type ValidationResult struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Timestamp    time.Time          `json:"timestamp"`
	Checks       []ValidationCheck  `json:"checks"`
	OverallPass  bool               `json:"overall_pass"`
	Confidence   float64            `json:"confidence"`
	Summary      string             `json:"summary"`
	Recommendations []string        `json:"recommendations"`
}

type ValidationCheck struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Passed      bool    `json:"passed"`
	Value       float64 `json:"value"`
	Threshold   float64 `json:"threshold"`
	Severity    string  `json:"severity"`
}

func (v *ValidatorAgent) validateStatistical(results map[string]interface{}, thresholds map[string]interface{}) ValidationResult {
	checks := []ValidationCheck{}
	
	pValue := getFloat(results, "p_value", 0.05)
	pThreshold := getFloat(thresholds, "p_value", 0.05)
	checks = append(checks, ValidationCheck{
		Name: "p_value", Description: "Statistical significance", Passed: pValue < pThreshold,
		Value: pValue, Threshold: pThreshold, Severity: "critical",
	})

	effectSize := getFloat(results, "effect_size", 0.5)
	effectThreshold := getFloat(thresholds, "effect_size", 0.3)
	checks = append(checks, ValidationCheck{
		Name: "effect_size", Description: "Minimum effect size", Passed: effectSize > effectThreshold,
		Value: effectSize, Threshold: effectThreshold, Severity: "high",
	})

	sampleSize := getFloat(results, "sample_size", 30)
	sizeThreshold := getFloat(thresholds, "sample_size", 20)
	checks = append(checks, ValidationCheck{
		Name: "sample_size", Description: "Adequate sample size", Passed: sampleSize >= sizeThreshold,
		Value: sampleSize, Threshold: sizeThreshold, Severity: "medium",
	})

	ciWidth := getFloat(results, "ci_width", 0.2)
	ciThreshold := getFloat(thresholds, "ci_width", 0.3)
	checks = append(checks, ValidationCheck{
		Name: "confidence_interval", Description: "CI precision", Passed: ciWidth < ciThreshold,
		Value: ciWidth, Threshold: ciThreshold, Severity: "medium",
	})

	passed := 0
	for _, c := range checks {
		if c.Passed {
			passed++
		}
	}

	confidence := float64(passed) / float64(len(checks))
	
	return ValidationResult{
		Type:            "statistical",
		Checks:          checks,
		OverallPass:     passed == len(checks),
		Confidence:      confidence,
		Summary:         fmt.Sprintf("%d/%d checks passed", passed, len(checks)),
		Recommendations: v.generateRecommendations(checks),
	}
}

func (v *ValidatorAgent) validateReproducibility(results map[string]interface{}) ValidationResult {
	replicates := getFloat(results, "replicates", 3)
	cv := getFloat(results, "coefficient_of_variation", 0.15)
	
	checks := []ValidationCheck{
		{Name: "replicate_count", Description: "Minimum 3 replicates", Passed: replicates >= 3, Value: replicates, Threshold: 3, Severity: "critical"},
		{Name: "cv_threshold", Description: "CV < 20%", Passed: cv < 0.2, Value: cv, Threshold: 0.2, Severity: "high"},
		{Name: "consistency", Description: "Direction consistent", Passed: getBool(results, "direction_consistent", true), Value: 1, Threshold: 1, Severity: "critical"},
	}

	passed := 0
	for _, c := range checks {
		if c.Passed {
			passed++
		}
	}

	return ValidationResult{
		Type:        "reproducibility",
		Checks:      checks,
		OverallPass: passed == len(checks),
		Confidence:  float64(passed) / float64(len(checks)),
		Summary:     fmt.Sprintf("Reproducibility: %d/%d criteria met", passed, len(checks)),
	}
}

func (v *ValidatorAgent) validateOrthogonal(results map[string]interface{}) ValidationResult {
	methods := getStringSlice(results, "methods")
	agreement := getFloat(results, "method_agreement", 0.8)
	
	checks := []ValidationCheck{
		{Name: "multiple_methods", Description: "≥2 orthogonal methods", Passed: len(methods) >= 2, Value: float64(len(methods)), Threshold: 2, Severity: "critical"},
		{Name: "method_agreement", Description: "Methods agree >80%", Passed: agreement > 0.8, Value: agreement, Threshold: 0.8, Severity: "high"},
	}

	passed := 0
	for _, c := range checks {
		if c.Passed {
			passed++
		}
	}

	return ValidationResult{
		Type:        "orthogonal",
		Checks:      checks,
		OverallPass: passed == len(checks),
		Confidence:  float64(passed) / float64(len(checks)),
		Summary:     fmt.Sprintf("Orthogonal validation: %d/%d passed", passed, len(checks)),
	}
}

func (v *ValidatorAgent) validateQuality(results map[string]interface{}) ValidationResult {
	checks := []ValidationCheck{
		{Name: "data_completeness", Description: "Data >95% complete", Passed: getFloat(results, "completeness", 0.98) > 0.95, Value: getFloat(results, "completeness", 0.98), Threshold: 0.95, Severity: "high"},
		{Name: "outlier_check", Description: "No extreme outliers", Passed: getFloat(results, "outlier_fraction", 0.02) < 0.05, Value: getFloat(results, "outlier_fraction", 0.02), Threshold: 0.05, Severity: "medium"},
		{Name: "batch_effects", Description: "No batch effects", Passed: getBool(results, "no_batch_effects", true), Value: 1, Threshold: 1, Severity: "high"},
	}

	passed := 0
	for _, c := range checks {
		if c.Passed {
			passed++
		}
	}

	return ValidationResult{
		Type:        "quality",
		Checks:      checks,
		OverallPass: passed == len(checks),
		Confidence:  float64(passed) / float64(len(checks)),
		Summary:     fmt.Sprintf("Quality checks: %d/%d passed", passed, len(checks)),
	}
}

func (v *ValidatorAgent) generateRecommendations(checks []ValidationCheck) []string {
	recs := []string{}
	for _, c := range checks {
		if !c.Passed {
			switch c.Name {
			case "p_value":
				recs = append(recs, "Increase sample size or reduce variance")
			case "effect_size":
				recs = append(recs, "Consider more sensitive assay or larger effect")
			case "sample_size":
				recs = append(recs, "Recruit more subjects/replicates")
			case "confidence_interval":
				recs = append(recs, "Increase precision with more measurements")
			case "replicate_count":
				recs = append(recs, "Run at least 3 biological replicates")
			case "cv_threshold":
				recs = append(recs, "Optimize protocol to reduce variability")
			case "multiple_methods":
				recs = append(recs, "Add orthogonal validation method")
			}
		}
	}
	if len(recs) == 0 {
		recs = append(recs, "All validation criteria met - ready for publication")
	}
	return recs
}

func getBool(input map[string]interface{}, key string, defaultVal bool) bool {
	if v, ok := input[key].(bool); ok {
		return v
	}
	return defaultVal
}