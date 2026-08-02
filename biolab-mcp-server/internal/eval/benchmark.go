package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type MetricType string

const (
	MetricPrecision      MetricType = "precision"
	MetricRecall         MetricType = "recall"
	MetricF1             MetricType = "f1"
	MetricAccuracy       MetricType = "accuracy"
	MetricLatency        MetricType = "latency"
	MetricTokenUsage     MetricType = "token_usage"
	MetricCacheHitRate   MetricType = "cache_hit_rate"
	MetricErrorRate      MetricType = "error_rate"
	MetricConfidenceCal  MetricType = "confidence_calibration"
	MetricCitationAcc    MetricType = "citation_accuracy"
	MetricContradiction  MetricType = "contradiction_detection"
)

type EvaluationResult struct {
	Metric     MetricType       `json:"metric"`
	Value      float64          `json:"value"`
	Details    map[string]any   `json:"details,omitempty"`
	Timestamp  time.Time        `json:"timestamp"`
	TestCaseID string           `json:"test_case_id"`
}

type TestCase struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]any         `json:"input"`
	Expected    map[string]any         `json:"expected"`
	Metadata    map[string]any         `json:"metadata"`
	Tags        []string               `json:"tags"`
}

type TestSuite struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	TestCases   []*TestCase   `json:"test_cases"`
	Setup       func() error  `json:"-"`
	Teardown    func() error  `json:"-"`
}

type Evaluator interface {
	Name() string
	Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error)
}

type BenchmarkHarness struct {
	suites     []*TestSuite
	evaluators []Evaluator
	results    []*EvaluationResult
	mu         sync.RWMutex
}

func NewBenchmarkHarness() *BenchmarkHarness {
	return &BenchmarkHarness{
		suites:     make([]*TestSuite, 0),
		evaluators: make([]Evaluator, 0),
		results:    make([]*EvaluationResult, 0),
	}
}

func (h *BenchmarkHarness) AddSuite(suite *TestSuite) {
	h.suites = append(h.suites, suite)
}

func (h *BenchmarkHarness) AddEvaluator(evaluator Evaluator) {
	h.evaluators = append(h.evaluators, evaluator)
}

func (h *BenchmarkHarness) Run(ctx context.Context, systemUnderTest SystemUnderTest) (*BenchmarkReport, error) {
	h.mu.Lock()
	h.results = make([]*EvaluationResult, 0)
	h.mu.Unlock()

	report := &BenchmarkReport{
		StartTime: time.Now(),
		Results:   make([]*EvaluationResult, 0),
		SuiteResults: make(map[string]*SuiteResult),
	}

	for _, suite := range h.suites {
		suiteResult := &SuiteResult{
			SuiteName: suite.Name,
			CaseResults: make([]*CaseResult, 0),
			Passed:    0,
			Failed:    0,
		}

		if suite.Setup != nil {
			if err := suite.Setup(); err != nil {
				suiteResult.Error = err.Error()
				report.SuiteResults[suite.Name] = suiteResult
				continue
			}
		}

		for _, testCase := range suite.TestCases {
			caseResult := h.runTestCase(ctx, systemUnderTest, testCase)
			suiteResult.CaseResults = append(suiteResult.CaseResults, caseResult)
			if caseResult.Passed {
				suiteResult.Passed++
			} else {
				suiteResult.Failed++
			}
		}

		if suite.Teardown != nil {
			suite.Teardown()
		}

		report.SuiteResults[suite.Name] = suiteResult
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)
	report.TotalTests = 0
	report.TotalPassed = 0
	report.TotalFailed = 0

	for _, sr := range report.SuiteResults {
		report.TotalTests += sr.Passed + sr.Failed
		report.TotalPassed += sr.Passed
		report.TotalFailed += sr.Failed
	}

	h.mu.RLock()
	report.Results = make([]*EvaluationResult, len(h.results))
	copy(report.Results, h.results)
	h.mu.RUnlock()

	return report, nil
}

func (h *BenchmarkHarness) runTestCase(ctx context.Context, system SystemUnderTest, testCase *TestCase) *CaseResult {
	start := time.Now()
	caseResult := &CaseResult{
		TestCaseID: testCase.ID,
		TestCaseName: testCase.Name,
		Metrics: make([]*EvaluationResult, 0),
	}

	actual, err := system.Execute(ctx, testCase.Input)
	caseResult.Duration = time.Since(start)

	if err != nil {
		caseResult.Error = err.Error()
		caseResult.Passed = false
		return caseResult
	}

	allPassed := true
	for _, evaluator := range h.evaluators {
		results, evalErr := evaluator.Evaluate(ctx, testCase, actual)
		if evalErr != nil {
			caseResult.Error = evalErr.Error()
			allPassed = false
			continue
		}

		for _, result := range results {
			result.TestCaseID = testCase.ID
			h.mu.Lock()
			h.results = append(h.results, result)
			h.mu.Unlock()
			caseResult.Metrics = append(caseResult.Metrics, result)

			if !resultPassed(result) {
				allPassed = false
			}
		}
	}

	caseResult.Passed = allPassed
	return caseResult
}

func resultPassed(result *EvaluationResult) bool {
	switch result.Metric {
	case MetricPrecision, MetricRecall, MetricF1, MetricAccuracy, MetricCitationAcc:
		return result.Value >= 0.8
	case MetricLatency:
		return result.Value <= 5000
	case MetricErrorRate:
		return result.Value <= 0.05
	case MetricCacheHitRate:
		return result.Value >= 0.3
	case MetricConfidenceCal:
		return result.Value <= 0.15
	default:
		return true
	}
}

type SystemUnderTest interface {
	Execute(ctx context.Context, input map[string]any) (any, error)
}

type BenchmarkReport struct {
	StartTime    time.Time                    `json:"start_time"`
	EndTime      time.Time                    `json:"end_time"`
	Duration     time.Duration                `json:"duration"`
	TotalTests   int                          `json:"total_tests"`
	TotalPassed  int                          `json:"total_passed"`
	TotalFailed  int                          `json:"total_failed"`
	Results      []*EvaluationResult          `json:"results"`
	SuiteResults map[string]*SuiteResult      `json:"suite_results"`
}

type SuiteResult struct {
	SuiteName   string       `json:"suite_name"`
	CaseResults []*CaseResult `json:"case_results"`
	Passed      int          `json:"passed"`
	Failed      int          `json:"failed"`
	Error       string       `json:"error,omitempty"`
}

type CaseResult struct {
	TestCaseID     string              `json:"test_case_id"`
	TestCaseName   string              `json:"test_case_name"`
	Passed         bool                `json:"passed"`
	Duration       time.Duration       `json:"duration"`
	Error          string              `json:"error,omitempty"`
	Metrics        []*EvaluationResult `json:"metrics"`
}

func (r *BenchmarkReport) Summary() string {
	return fmt.Sprintf("Benchmark: %d tests, %d passed, %d failed (%.1f%%), duration: %v",
		r.TotalTests, r.TotalPassed, r.TotalFailed,
		float64(r.TotalPassed)/float64(r.TotalTests)*100, r.Duration)
}

func (r *BenchmarkReport) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	return string(data), err
}

type PrecisionEvaluator struct{}

func (e *PrecisionEvaluator) Name() string { return "precision" }

func (e *PrecisionEvaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	expectedItems := extractExpectedItems(testCase.Expected)
	actualItems := extractActualItems(actual)

	if len(expectedItems) == 0 {
		return []*EvaluationResult{{Metric: MetricPrecision, Value: 1.0, Timestamp: time.Now()}}, nil
	}

	tp := 0
	for _, item := range actualItems {
		if contains(expectedItems, item) {
			tp++
		}
	}

	precision := float64(tp) / float64(len(actualItems))
	return []*EvaluationResult{{
		Metric:    MetricPrecision,
		Value:     precision,
		Details:   map[string]any{"true_positives": tp, "total_predicted": len(actualItems)},
		Timestamp: time.Now(),
	}}, nil
}

type RecallEvaluator struct{}

func (e *RecallEvaluator) Name() string { return "recall" }

func (e *RecallEvaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	expectedItems := extractExpectedItems(testCase.Expected)
	actualItems := extractActualItems(actual)

	if len(expectedItems) == 0 {
		return []*EvaluationResult{{Metric: MetricRecall, Value: 1.0, Timestamp: time.Now()}}, nil
	}

	tp := 0
	for _, item := range actualItems {
		if contains(expectedItems, item) {
			tp++
		}
	}

	recall := float64(tp) / float64(len(expectedItems))
	return []*EvaluationResult{{
		Metric:    MetricRecall,
		Value:     recall,
		Details:   map[string]any{"true_positives": tp, "total_expected": len(expectedItems)},
		Timestamp: time.Now(),
	}}, nil
}

type F1Evaluator struct{}

func (e *F1Evaluator) Name() string { return "f1" }

func (e *F1Evaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	expectedItems := extractExpectedItems(testCase.Expected)
	actualItems := extractActualItems(actual)

	if len(expectedItems) == 0 && len(actualItems) == 0 {
		return []*EvaluationResult{{Metric: MetricF1, Value: 1.0, Timestamp: time.Now()}}, nil
	}

	tp := 0
	for _, item := range actualItems {
		if contains(expectedItems, item) {
			tp++
		}
	}

	precision := float64(tp) / float64(max(len(actualItems), 1))
	recall := float64(tp) / float64(max(len(expectedItems), 1))
	f1 := 2 * precision * recall / math.Max(precision+recall, 0.001)

	return []*EvaluationResult{{
		Metric:    MetricF1,
		Value:     f1,
		Details:   map[string]any{"precision": precision, "recall": recall, "true_positives": tp},
		Timestamp: time.Now(),
	}}, nil
}

type LatencyEvaluator struct {
	ThresholdMs float64
}

func (e *LatencyEvaluator) Name() string { return "latency" }

func (e *LatencyEvaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	latency := extractLatency(actual)
	return []*EvaluationResult{{
		Metric:    MetricLatency,
		Value:     latency,
		Details:   map[string]any{"threshold_ms": e.ThresholdMs, "passed": latency <= e.ThresholdMs},
		Timestamp: time.Now(),
	}}, nil
}

type CacheHitRateEvaluator struct{}

func (e *CacheHitRateEvaluator) Name() string { return "cache_hit_rate" }

func (e *CacheHitRateEvaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	cacheHit := extractCacheHit(actual)
	return []*EvaluationResult{{
		Metric:    MetricCacheHitRate,
		Value:     cacheHit,
		Timestamp: time.Now(),
	}}, nil
}

type ConfidenceCalibrationEvaluator struct{}

func (e *ConfidenceCalibrationEvaluator) Name() string { return "confidence_calibration" }

func (e *ConfidenceCalibrationEvaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	expectedConf := extractExpectedConfidence(testCase.Expected)
	actualConf := extractActualConfidence(actual)

	if expectedConf < 0 || actualConf < 0 {
		return []*EvaluationResult{{Metric: MetricConfidenceCal, Value: 0, Timestamp: time.Now()}}, nil
	}

	calibrationError := math.Abs(expectedConf - actualConf)
	return []*EvaluationResult{{
		Metric:    MetricConfidenceCal,
		Value:     calibrationError,
		Details:   map[string]any{"expected": expectedConf, "actual": actualConf},
		Timestamp: time.Now(),
	}}, nil
}

type CitationAccuracyEvaluator struct{}

func (e *CitationAccuracyEvaluator) Name() string { return "citation_accuracy" }

func (e *CitationAccuracyEvaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	citations := extractCitations(actual)
	if len(citations) == 0 {
		return []*EvaluationResult{{Metric: MetricCitationAcc, Value: 1.0, Timestamp: time.Now()}}, nil
	}

	valid := 0
	for _, c := range citations {
		if isValidCitation(c) {
			valid++
		}
	}

	accuracy := float64(valid) / float64(len(citations))
	return []*EvaluationResult{{
		Metric:    MetricCitationAcc,
		Value:     accuracy,
		Details:   map[string]any{"valid": valid, "total": len(citations)},
		Timestamp: time.Now(),
	}}, nil
}

type ContradictionDetectionEvaluator struct{}

func (e *ContradictionDetectionEvaluator) Name() string { return "contradiction_detection" }

func (e *ContradictionDetectionEvaluator) Evaluate(ctx context.Context, testCase *TestCase, actual any) ([]*EvaluationResult, error) {
	expectedContradictions := extractExpectedContradictions(testCase.Expected)
	actualContradictions := extractActualContradictions(actual)

	if len(expectedContradictions) == 0 && len(actualContradictions) == 0 {
		return []*EvaluationResult{{Metric: MetricContradiction, Value: 1.0, Timestamp: time.Now()}}, nil
	}

	tp := 0
	for _, exp := range expectedContradictions {
		for _, act := range actualContradictions {
			if contradictionsMatch(exp, act) {
				tp++
				break
			}
		}
	}

	precision := float64(tp) / float64(max(len(actualContradictions), 1))
	recall := float64(tp) / float64(max(len(expectedContradictions), 1))
	f1 := 2 * precision * recall / math.Max(precision+recall, 0.001)

	return []*EvaluationResult{{
		Metric:    MetricContradiction,
		Value:     f1,
		Details:   map[string]any{"precision": precision, "recall": recall, "true_positives": tp},
		Timestamp: time.Now(),
	}}, nil
}

func extractExpectedItems(expected map[string]any) []string {
	if items, ok := expected["items"].([]any); ok {
		result := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if items, ok := expected["papers"].([]any); ok {
		result := make([]string, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				if pmid, ok := m["pmid"].(float64); ok {
					result = append(result, fmt.Sprintf("PMID:%d", int(pmid)))
				}
			}
		}
		return result
	}
	return nil
}

func extractActualItems(actual any) []string {
	if m, ok := actual.(map[string]any); ok {
		if papers, ok := m["papers"].([]any); ok {
			result := make([]string, 0, len(papers))
			for _, paper := range papers {
				if pm, ok := paper.(map[string]any); ok {
					if pmid, ok := pm["pmid"].(float64); ok {
						result = append(result, fmt.Sprintf("PMID:%d", int(pmid)))
					}
				}
			}
			return result
		}
	}
	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func extractLatency(actual any) float64 {
	if m, ok := actual.(map[string]any); ok {
		if latency, ok := m["latency_ms"].(float64); ok {
			return latency
		}
		if duration, ok := m["duration"].(string); ok {
			d, _ := time.ParseDuration(duration)
			return float64(d.Milliseconds())
		}
	}
	return 0
}

func extractCacheHit(actual any) float64 {
	if m, ok := actual.(map[string]any); ok {
		if hit, ok := m["cache_hit"].(bool); ok {
			if hit {
				return 1.0
			}
			return 0.0
		}
		if rate, ok := m["cache_hit_rate"].(float64); ok {
			return rate
		}
	}
	return 0.5
}

func extractExpectedConfidence(expected map[string]any) float64 {
	if conf, ok := expected["confidence"].(float64); ok {
		return conf
	}
	if signals, ok := expected["signals"].(map[string]any); ok {
		if overall, ok := signals["overall"].(float64); ok {
			return overall
		}
	}
	return -1
}

func extractActualConfidence(actual any) float64 {
	if m, ok := actual.(map[string]any); ok {
		if conf, ok := m["confidence"].(float64); ok {
			return conf
		}
		if signals, ok := m["signals"].(map[string]any); ok {
			if overall, ok := signals["overall"].(float64); ok {
				return overall
			}
		}
	}
	return -1
}

func extractCitations(actual any) []map[string]any {
	if m, ok := actual.(map[string]any); ok {
		if citations, ok := m["citations"].([]any); ok {
			result := make([]map[string]any, 0, len(citations))
			for _, c := range citations {
				if cm, ok := c.(map[string]any); ok {
					result = append(result, cm)
				}
			}
			return result
		}
	}
	return nil
}

func isValidCitation(citation map[string]any) bool {
	if pmid, ok := citation["pmid"]; ok {
		if id, ok := pmid.(float64); ok && id > 0 {
			return true
		}
		if id, ok := pmid.(string); ok && strings.HasPrefix(id, "PMID:") {
			return true
		}
	}
	if doi, ok := citation["doi"].(string); ok && strings.HasPrefix(doi, "10.") {
		return true
	}
	return false
}

func extractExpectedContradictions(expected map[string]any) []string {
	if contr, ok := expected["contradictions"].([]any); ok {
		result := make([]string, 0, len(contr))
		for _, c := range contr {
			if s, ok := c.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func extractActualContradictions(actual any) []string {
	if m, ok := actual.(map[string]any); ok {
		if contr, ok := m["contradictions"].([]any); ok {
			result := make([]string, 0, len(contr))
			for _, c := range contr {
				if s, ok := c.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func contradictionsMatch(exp, act string) bool {
	expLower := strings.ToLower(exp)
	actLower := strings.ToLower(act)
	return strings.Contains(expLower, actLower) || strings.Contains(actLower, expLower)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type RegressionDetector struct {
	baseline map[string]float64
	threshold float64
}

func NewRegressionDetector(threshold float64) *RegressionDetector {
	return &RegressionDetector{
		baseline: make(map[string]float64),
		threshold: threshold,
	}
}

func (d *RegressionDetector) SetBaseline(metric string, value float64) {
	d.baseline[metric] = value
}

func (d *RegressionDetector) Check(metric string, value float64) (bool, float64) {
	baseline, ok := d.baseline[metric]
	if !ok {
		return false, 0
	}
	diff := (value - baseline) / baseline
	return diff < -d.threshold, diff
}

func (d *RegressionDetector) GetBaseline(metric string) (float64, bool) {
	val, ok := d.baseline[metric]
	return val, ok
}

type ComparisonReport struct {
	Baseline  *BenchmarkReport `json:"baseline"`
	Current   *BenchmarkReport `json:"current"`
	Regressions []Regression   `json:"regressions"`
	Improvements []Improvement `json:"improvements"`
}

type Regression struct {
	Metric      string  `json:"metric"`
	Baseline    float64 `json:"baseline"`
	Current     float64 `json:"current"`
	ChangePct   float64 `json:"change_pct"`
	Severity    string  `json:"severity"`
}

type Improvement struct {
	Metric      string  `json:"metric"`
	Baseline    float64 `json:"baseline"`
	Current     float64 `json:"current"`
	ChangePct   float64 `json:"change_pct"`
}

func CompareReports(baseline, current *BenchmarkReport, threshold float64) *ComparisonReport {
	report := &ComparisonReport{
		Baseline: baseline,
		Current: current,
		Regressions: make([]Regression, 0),
		Improvements: make([]Improvement, 0),
	}

	baselineMetrics := aggregateMetrics(baseline.Results)
	currentMetrics := aggregateMetrics(current.Results)

	for metric, baselineVal := range baselineMetrics {
		currentVal, ok := currentMetrics[metric]
		if !ok {
			continue
		}

		changePct := (currentVal - baselineVal) / baselineVal * 100

		if changePct < -threshold*100 {
			report.Regressions = append(report.Regressions, Regression{
				Metric:    metric,
				Baseline:  baselineVal,
				Current:   currentVal,
				ChangePct: changePct,
				Severity:  severity(changePct),
			})
		} else if changePct > threshold*100 {
			report.Improvements = append(report.Improvements, Improvement{
				Metric:    metric,
				Baseline:  baselineVal,
				Current:   currentVal,
				ChangePct: changePct,
			})
		}
	}

	sort.Slice(report.Regressions, func(i, j int) bool {
		return report.Regressions[i].ChangePct < report.Regressions[j].ChangePct
	})

	return report
}

func aggregateMetrics(results []*EvaluationResult) map[string]float64 {
	sums := make(map[string]float64)
	counts := make(map[string]int)

	for _, r := range results {
		sums[string(r.Metric)] += r.Value
		counts[string(r.Metric)]++
	}

	avg := make(map[string]float64)
	for metric, sum := range sums {
		avg[metric] = sum / float64(counts[metric])
	}
	return avg
}

func severity(changePct float64) string {
	absChange := math.Abs(changePct)
	if absChange > 50 {
		return "critical"
	} else if absChange > 20 {
		return "major"
	} else if absChange > 10 {
		return "minor"
	}
	return "negligible"
}

func (r *ComparisonReport) Summary() string {
	return fmt.Sprintf("Comparison: %d regressions, %d improvements", len(r.Regressions), len(r.Improvements))
}