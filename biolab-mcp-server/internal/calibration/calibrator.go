package calibration

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

type SignalWeight struct {
	Literature      float64 `json:"literature"`
	ProteinEvidence float64 `json:"protein_evidence"`
	ClinicalEvidence float64 `json:"clinical_evidence"`
	LLMRating       float64 `json:"llm_rating"`
}

func DefaultWeights() *SignalWeight {
	return &SignalWeight{
		Literature:       0.35,
		ProteinEvidence:  0.25,
		ClinicalEvidence: 0.25,
		LLMRating:        0.15,
	}
}

func (w *SignalWeight) Sum() float64 {
	return w.Literature + w.ProteinEvidence + w.ClinicalEvidence + w.LLMRating
}

func (w *SignalWeight) Normalize() *SignalWeight {
	sum := w.Sum()
	if sum == 0 {
		return DefaultWeights()
	}
	return &SignalWeight{
		Literature:       w.Literature / sum,
		ProteinEvidence:  w.ProteinEvidence / sum,
		ClinicalEvidence: w.ClinicalEvidence / sum,
		LLMRating:        w.LLMRating / sum,
	}
}

type ConfidenceSignal struct {
	Literature      float64 `json:"literature"`
	ProteinEvidence float64 `json:"protein_evidence"`
	ClinicalEvidence float64 `json:"clinical_evidence"`
	LLMRating       float64 `json:"llm_rating"`
}

type CalibrationConfig struct {
	Weights           *SignalWeight
	Temperature       float64
	MinConfidence     float64
	MaxConfidence     float64
	ContradictionPenalty float64
	EvidenceCountBonus  float64
	MaxEvidenceBonus    float64
}

func DefaultConfig() *CalibrationConfig {
	return &CalibrationConfig{
		Weights:            DefaultWeights().Normalize(),
		Temperature:        1.0,
		MinConfidence:      0.05,
		MaxConfidence:      0.95,
		ContradictionPenalty: 0.3,
		EvidenceCountBonus:  0.02,
		MaxEvidenceBonus:   0.15,
	}
}

type Calibrator struct {
	config *CalibrationConfig
	mu     sync.RWMutex
	history []CalibrationRecord
}

type CalibrationRecord struct {
	Timestamp       time.Time       `json:"timestamp"`
	Signals         ConfidenceSignal `json:"signals"`
	ContradictionScore float64      `json:"contradiction_score"`
	EvidenceCount   int             `json:"evidence_count"`
	RawConfidence   float64         `json:"raw_confidence"`
	CalibratedConfidence float64    `json:"calibrated_confidence"`
	Verdict         string          `json:"verdict"`
	Correct         *bool           `json:"correct,omitempty"`
}

func NewCalibrator(config *CalibrationConfig) *Calibrator {
	if config == nil {
		config = DefaultConfig()
	}
	return &Calibrator{
		config:  config,
		history: make([]CalibrationRecord, 0),
	}
}

func (c *Calibrator) Calibrate(signals ConfidenceSignal, contradictionScore float64, evidenceCount int) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	weights := c.config.Weights

	weightedSum := signals.Literature*weights.Literature +
		signals.ProteinEvidence*weights.ProteinEvidence +
		signals.ClinicalEvidence*weights.ClinicalEvidence +
		signals.LLMRating*weights.LLMRating

	penalty := contradictionScore * c.config.ContradictionPenalty
	weightedSum -= penalty

	evidenceBonus := math.Min(float64(evidenceCount)*c.config.EvidenceCountBonus, c.config.MaxEvidenceBonus)
	weightedSum += evidenceBonus

	calibrated := 1.0 / (1.0 + math.Exp(-c.config.Temperature*(weightedSum-0.5)))

	calibrated = math.Max(c.config.MinConfidence, math.Min(c.config.MaxConfidence, calibrated))

	record := CalibrationRecord{
		Timestamp:            time.Now(),
		Signals:              signals,
		ContradictionScore:   contradictionScore,
		EvidenceCount:        evidenceCount,
		RawConfidence:        weightedSum,
		CalibratedConfidence: calibrated,
		Verdict:              c.verdictFromConfidence(calibrated),
	}
	c.history = append(c.history, record)

	if len(c.history) > 10000 {
		c.history = c.history[len(c.history)-10000:]
	}

	return calibrated
}

func (c *Calibrator) verdictFromConfidence(conf float64) string {
	if conf >= 0.75 {
		return "supported"
	}
	if conf <= 0.35 {
		return "refuted"
	}
	return "unresolved"
}

func (c *Calibrator) GetHistory(limit int) []CalibrationRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.history) {
		limit = len(c.history)
	}
	start := len(c.history) - limit
	result := make([]CalibrationRecord, limit)
	copy(result, c.history[start:])
	return result
}

func (c *Calibrator) GetCalibrationStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.history) == 0 {
		return map[string]interface{}{
			"total_calibrations": 0,
		}
	}

	var sumRaw, sumCalibrated float64
	verdictCounts := make(map[string]int)
	correctCounts := make(map[string]int)
	totalWithGroundTruth := 0

	for _, r := range c.history {
		sumRaw += r.RawConfidence
		sumCalibrated += r.CalibratedConfidence
		verdictCounts[r.Verdict]++
		if r.Correct != nil {
			totalWithGroundTruth++
			if *r.Correct {
				correctCounts[r.Verdict]++
			}
		}
	}

	accuracy := make(map[string]float64)
	for verdict, count := range correctCounts {
		if verdictCounts[verdict] > 0 {
			accuracy[verdict] = float64(count) / float64(verdictCounts[verdict])
		}
	}

	return map[string]interface{}{
		"total_calibrations":     len(c.history),
		"avg_raw_confidence":     sumRaw / float64(len(c.history)),
		"avg_calibrated_confidence": sumCalibrated / float64(len(c.history)),
		"verdict_distribution":   verdictCounts,
		"accuracy_by_verdict":    accuracy,
		"total_with_ground_truth": totalWithGroundTruth,
		"config":                 c.config,
	}
}

func (c *Calibrator) RecordOutcome(index int, correct bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.history) {
		return fmt.Errorf("invalid index")
	}
	c.history[index].Correct = &correct
	return nil
}

type TemperatureOptimizer struct {
	calibrator *Calibrator
	mu         sync.Mutex
}

func NewTemperatureOptimizer(calibrator *Calibrator) *TemperatureOptimizer {
	return &TemperatureOptimizer{calibrator: calibrator}
}

func (o *TemperatureOptimizer) Optimize(groundTruth []CalibrationRecord) (float64, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if len(groundTruth) < 10 {
		return 1.0, fmt.Errorf("need at least 10 records with ground truth")
	}

	bestTemp := 1.0
	bestLoss := math.Inf(1)

	for temp := 0.1; temp <= 5.0; temp += 0.1 {
		loss := o.evaluateTemperature(temp, groundTruth)
		if loss < bestLoss {
			bestLoss = loss
			bestTemp = temp
		}
	}

	return bestTemp, nil
}

func (o *TemperatureOptimizer) evaluateTemperature(temp float64, records []CalibrationRecord) float64 {
	var loss float64
	for _, r := range records {
		weights := o.calibrator.config.Weights
		weightedSum := r.Signals.Literature*weights.Literature +
			r.Signals.ProteinEvidence*weights.ProteinEvidence +
			r.Signals.ClinicalEvidence*weights.ClinicalEvidence +
			r.Signals.LLMRating*weights.LLMRating

		penalty := r.ContradictionScore * o.calibrator.config.ContradictionPenalty
		weightedSum -= penalty

		evidenceBonus := math.Min(float64(r.EvidenceCount)*o.calibrator.config.EvidenceCountBonus, o.calibrator.config.MaxEvidenceBonus)
		weightedSum += evidenceBonus

		predicted := 1.0 / (1.0 + math.Exp(-temp*(weightedSum-0.5)))
		predicted = math.Max(o.calibrator.config.MinConfidence, math.Min(o.calibrator.config.MaxConfidence, predicted))

		actual := 1.0
		if r.Verdict == "refuted" {
			actual = 0.0
		} else if r.Verdict == "unresolved" {
			actual = 0.5
		}

		loss += math.Pow(predicted-actual, 2)
	}

	return loss / float64(len(records))
}

type ConfidenceExplainer struct {
	calibrator *Calibrator
}

func NewConfidenceExplainer(calibrator *Calibrator) *ConfidenceExplainer {
	return &ConfidenceExplainer{calibrator: calibrator}
}

func (e *ConfidenceExplainer) Explain(signals ConfidenceSignal, contradictionScore float64, evidenceCount int) map[string]interface{} {
	weights := e.calibrator.config.Weights

	contributions := map[string]float64{
		"literature":        signals.Literature * weights.Literature,
		"protein_evidence":  signals.ProteinEvidence * weights.ProteinEvidence,
		"clinical_evidence": signals.ClinicalEvidence * weights.ClinicalEvidence,
		"llm_rating":        signals.LLMRating * weights.LLMRating,
	}

	weightedSum := contributions["literature"] + contributions["protein_evidence"] + contributions["clinical_evidence"] + contributions["llm_rating"]
	penalty := contradictionScore * e.calibrator.config.ContradictionPenalty
	evidenceBonus := math.Min(float64(evidenceCount)*e.calibrator.config.EvidenceCountBonus, e.calibrator.config.MaxEvidenceBonus)

	calibrated := e.calibrator.Calibrate(signals, contradictionScore, evidenceCount)

	return map[string]interface{}{
		"signal_contributions":  contributions,
		"weighted_sum":          weightedSum,
		"contradiction_penalty": penalty,
		"evidence_bonus":        evidenceBonus,
		"adjusted_sum":          weightedSum - penalty + evidenceBonus,
		"temperature":           e.calibrator.config.Temperature,
		"sigmoid_output":        1.0 / (1.0 + math.Exp(-e.calibrator.config.Temperature*(weightedSum-penalty+evidenceBonus-0.5))),
		"final_confidence":      calibrated,
		"verdict":               e.calibrator.verdictFromConfidence(calibrated),
		"weights_used":          weights,
		"explanation":           e.generateExplanation(contributions, penalty, evidenceBonus, calibrated),
	}
}

func (e *ConfidenceExplainer) generateExplanation(contributions map[string]float64, penalty, bonus, confidence float64) string {
	maxSignal := ""
	maxValue := 0.0
	for k, v := range contributions {
		if v > maxValue {
			maxValue = v
			maxSignal = k
		}
	}

	explanation := fmt.Sprintf("Primary driver: %s (%.2f). ", maxSignal, maxValue)

	if penalty > 0.1 {
		explanation += fmt.Sprintf("Contradiction penalty applied: -%.2f. ", penalty)
	}
	if bonus > 0.01 {
		explanation += fmt.Sprintf("Evidence count bonus: +%.2f. ", bonus)
	}
	explanation += fmt.Sprintf("Final calibrated confidence: %.1f%% (%s).", confidence*100, e.calibrator.verdictFromConfidence(confidence))

	return explanation
}

func (c *Calibrator) ExportHistory() (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := json.MarshalIndent(c.history, "", "  ")
	return string(data), err
}

func (c *Calibrator) ImportHistory(jsonStr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var records []CalibrationRecord
	if err := json.Unmarshal([]byte(jsonStr), &records); err != nil {
		return err
	}
	c.history = records
	return nil
}