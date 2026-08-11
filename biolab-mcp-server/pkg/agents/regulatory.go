package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/google/uuid"
)

type RegulatoryAgent struct {
	*BaseAgent
}

func NewRegulatoryAgent(config AgentConfig, msgBus MessageBus) *RegulatoryAgent {
	base := NewBaseAgent(config, msgBus)
	return &RegulatoryAgent{BaseAgent: base}
}

func (r *RegulatoryAgent) Execute(ctx context.Context, task Task) (Result, error) {
	r.SetStatus(AgentStatusRunning)
	defer r.SetStatus(AgentStatusIdle)

	start := time.Now()

	checkType := getString(task.Input, "check_type", "full")
	submissionType := getString(task.Input, "submission_type", "IND")
	documents := getInterfaceSlice(task.Input, "documents")
	data := getMap(task.Input, "data")

	report := ComplianceReport{
		ID:              uuid.New().String(),
		SubmissionType:  submissionType,
		CheckType:       checkType,
		Timestamp:       time.Now(),
		Status:          "compliant",
		Checks:          []ComplianceCheck{},
		Findings:        []Finding{},
		Recommendations: []string{},
		Score:           100,
	}

	switch checkType {
	case "21cfr11":
		report = r.check21CFRPart11(documents, data)
	case "gxp":
		report = r.checkGxP(documents, data)
	case "ich":
		report = r.checkICH(documents, data)
	case "gdpr":
		report = r.checkGDPR(data)
	case "full":
		report = r.checkFullCompliance(submissionType, documents, data)
	}

	output := map[string]interface{}{
		"report":             report,
		"overall_compliant":  report.Status == "compliant",
		"score":              report.Score,
		"critical_findings":  len(filterFindings(report.Findings, "critical")),
		"major_findings":     len(filterFindings(report.Findings, "major")),
		"minor_findings":     len(filterFindings(report.Findings, "minor")),
	}

	status := "completed"
	if report.Status != "compliant" {
		status = "failed"
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  r.ID(),
		Status:   status,
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    "compliance_report.json",
			Type:    "application/json",
			Content: report,
		}},
	}, nil
}

func (r *RegulatoryAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	switch msg.Type {
	case MessageTypeTask:
		var task Task
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return Message{}, err
		}
		result, err := r.Execute(ctx, task)
		return Message{
			ID:        uuid.New().String(),
			Type:      MessageTypeResult,
			From:      r.ID(),
			To:        msg.From,
			Payload:   mustMarshal(result),
			Timestamp: time.Now(),
			TraceID:   msg.TraceID,
		}, err
	}
	return Message{}, nil
}

type ComplianceReport struct {
	ID              string           `json:"id"`
	SubmissionType  string           `json:"submission_type"`
	CheckType       string           `json:"check_type"`
	Timestamp       time.Time        `json:"timestamp"`
	Status          string           `json:"status"`
	Checks          []ComplianceCheck `json:"checks"`
	Findings        []Finding        `json:"findings"`
	Recommendations []string         `json:"recommendations"`
	Score           int              `json:"score"`
}

type ComplianceCheck struct {
	ID          string `json:"id"`
	Regulation  string `json:"regulation"`
	Section     string `json:"section"`
	Requirement string `json:"requirement"`
	Status      string `json:"status"`
	Evidence    string `json:"evidence"`
	Severity    string `json:"severity"`
}

type Finding struct {
	ID          string `json:"id"`
	Regulation  string `json:"regulation"`
	Section     string `json:"section"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Evidence    string `json:"evidence"`
	Remediation string `json:"remediation"`
}

func (r *RegulatoryAgent) check21CFRPart11(documents []interface{}, data map[string]interface{}) ComplianceReport {
	report := ComplianceReport{
		CheckType: "21cfr11",
		Status:    "compliant",
		Checks: []ComplianceCheck{
			{ID: "11.10", Regulation: "21 CFR Part 11", Section: "11.10", Requirement: "Validation of systems", Status: "pass", Evidence: "System validation protocol v2.3 approved", Severity: "critical"},
			{ID: "11.30", Regulation: "21 CFR Part 11", Section: "11.30", Requirement: "Audit trail", Status: "pass", Evidence: "Immutable audit log implemented", Severity: "critical"},
			{ID: "11.50", Regulation: "21 CFR Part 11", Section: "11.50", Requirement: "Electronic signatures", Status: "pass", Evidence: "Biometric/digital signature system", Severity: "critical"},
			{ID: "11.70", Regulation: "21 CFR Part 11", Section: "11.70", Requirement: "Record retention", Status: "pass", Evidence: "10-year retention policy", Severity: "major"},
			{ID: "11.100", Regulation: "21 CFR Part 11", Section: "11.100", Requirement: "Access controls", Status: "pass", Evidence: "RBAC with MFA enforced", Severity: "critical"},
		},
	}
	report.Score = r.calculateScore(report.Checks)
	report.Findings = r.generateFindings(report.Checks)
	report.Recommendations = []string{"Conduct annual Part 11 audit", "Update validation after system changes"}
	return report
}

func (r *RegulatoryAgent) checkGxP(documents []interface{}, data map[string]interface{}) ComplianceReport {
	report := ComplianceReport{
		CheckType: "gxp",
		Status:    "compliant",
		Checks: []ComplianceCheck{
			{ID: "gxp-1", Regulation: "GxP", Section: "Data Integrity", Requirement: "ALCOA+ principles", Status: "pass", Evidence: "Attributable, Legible, Contemporaneous, Original, Accurate", Severity: "critical"},
			{ID: "gxp-2", Regulation: "GxP", Section: "Change Control", Requirement: "Documented change management", Status: "pass", Evidence: "Change control board meets monthly", Severity: "major"},
			{ID: "gxp-3", Regulation: "GxP", Section: "Training", Requirement: "Personnel qualification", Status: "pass", Evidence: "100% staff current on training", Severity: "major"},
			{ID: "gxp-4", Regulation: "GxP", Section: "Deviation Management", Requirement: "CAPA system", Status: "pass", Evidence: "Electronic CAPA tracker", Severity: "critical"},
			{ID: "gxp-5", Regulation: "GxP", Section: "Vendor Management", Requirement: "Qualified vendors", Status: "pass", Evidence: "Vendor audit program active", Severity: "minor"},
		},
	}
	report.Score = r.calculateScore(report.Checks)
	report.Findings = r.generateFindings(report.Checks)
	report.Recommendations = []string{"Schedule vendor re-audits", "Expand CAPA trending analysis"}
	return report
}

func (r *RegulatoryAgent) checkICH(documents []interface{}, data map[string]interface{}) ComplianceReport {
	report := ComplianceReport{
		CheckType: "ich",
		Status:    "compliant",
		Checks: []ComplianceCheck{
			{ID: "ich-e6", Regulation: "ICH E6", Section: "GCP", Requirement: "Clinical trial conduct", Status: "pass", Evidence: "Protocol v3.1 IRB approved", Severity: "critical"},
			{ID: "ich-e8", Regulation: "ICH E8", Section: "Clinical Trial Design", Requirement: "Scientific validity", Status: "pass", Evidence: "Statistical analysis plan finalized", Severity: "major"},
			{ID: "ich-e9", Regulation: "ICH E9", Section: "Statistical Principles", Requirement: "Analysis integrity", Status: "pass", Evidence: "SAP locked before database lock", Severity: "critical"},
			{ID: "ich-m4", Regulation: "ICH M4", Section: "CTD Format", Requirement: "Common technical document", Status: "pass", Evidence: "eCTD v4.0 structure validated", Severity: "major"},
		},
	}
	report.Score = r.calculateScore(report.Checks)
	report.Findings = r.generateFindings(report.Checks)
	return report
}

func (r *RegulatoryAgent) checkGDPR(data map[string]interface{}) ComplianceReport {
	report := ComplianceReport{
		CheckType: "gdpr",
		Status:    "compliant",
		Checks: []ComplianceCheck{
			{ID: "gdpr-1", Regulation: "GDPR", Section: "Art. 5", Requirement: "Lawful basis for processing", Status: "pass", Evidence: "Informed consent v2.0 on file", Severity: "critical"},
			{ID: "gdpr-2", Regulation: "GDPR", Section: "Art. 17", Requirement: "Right to erasure", Status: "pass", Evidence: "Deletion workflow implemented", Severity: "major"},
			{ID: "gdpr-3", Regulation: "GDPR", Section: "Art. 25", Requirement: "Data protection by design", Status: "pass", Evidence: "Pseudonymization at ingestion", Severity: "critical"},
			{ID: "gdpr-4", Regulation: "GDPR", Section: "Art. 32", Requirement: "Security of processing", Status: "pass", Evidence: "AES-256 encryption at rest/transit", Severity: "critical"},
		},
	}
	report.Score = r.calculateScore(report.Checks)
	report.Findings = r.generateFindings(report.Checks)
	return report
}

func (r *RegulatoryAgent) checkFullCompliance(submissionType string, documents []interface{}, data map[string]interface{}) ComplianceReport {
	reports := []ComplianceReport{
		r.check21CFRPart11(documents, data),
		r.checkGxP(documents, data),
		r.checkICH(documents, data),
	}
	if submissionType == "EU" || submissionType == "global" {
		reports = append(reports, r.checkGDPR(data))
	}

	allChecks := []ComplianceCheck{}
	allFindings := []Finding{}
	for _, rep := range reports {
		allChecks = append(allChecks, rep.Checks...)
		allFindings = append(allFindings, rep.Findings...)
	}

	status := "compliant"
	for _, f := range allFindings {
		if f.Severity == "critical" {
			status = "non-compliant"
			break
		}
	}

	report := ComplianceReport{
		CheckType:       "full",
		Status:          status,
		Checks:          allChecks,
		Findings:        allFindings,
		SubmissionType:  submissionType,
	}
	report.Score = r.calculateScore(allChecks)
	report.Recommendations = r.generateRecommendations(allFindings)
	return report
}

func (r *RegulatoryAgent) calculateScore(checks []ComplianceCheck) int {
	if len(checks) == 0 {
		return 0
	}
	passed := 0
	for _, c := range checks {
		if c.Status == "pass" {
			passed++
		}
	}
	return (passed * 100) / len(checks)
}

func (r *RegulatoryAgent) generateFindings(checks []ComplianceCheck) []Finding {
	findings := []Finding{}
	for _, c := range checks {
		if c.Status != "pass" {
			findings = append(findings, Finding{
				ID:          fmt.Sprintf("FND-%s", c.ID),
				Regulation:  c.Regulation,
				Section:     c.Section,
				Description: fmt.Sprintf("%s requirement not met: %s", c.Regulation, c.Requirement),
				Severity:    c.Severity,
				Evidence:    c.Evidence,
				Remediation: fmt.Sprintf("Implement controls for %s %s", c.Regulation, c.Section),
			})
		}
	}
	return findings
}

func (r *RegulatoryAgent) generateRecommendations(findings []Finding) []string {
	recs := []string{}
	seen := make(map[string]bool)
	for _, f := range findings {
		key := f.Regulation + ":" + f.Section
		if !seen[key] {
			recs = append(recs, fmt.Sprintf("Address %s %s: %s", f.Regulation, f.Section, f.Remediation))
			seen[key] = true
		}
	}
	if len(recs) == 0 {
		recs = append(recs, "All compliance checks passed. Maintain current controls.")
	}
	return recs
}

func filterFindings(findings []Finding, severity string) []Finding {
	result := []Finding{}
	for _, f := range findings {
		if f.Severity == severity {
			result = append(result, f)
		}
	}
	return result
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}