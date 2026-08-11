package agents

import (
	"time"

	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/mcp"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/sandbox"
	"go.uber.org/zap"
)

type AgentFactory struct {
	msgBus      MessageBus
	toolRegistry *mcp.ToolRegistry
	sandbox     *sandbox.Sandbox
	logger      *zap.Logger
	emailConfig EmailConfig
}

func NewAgentFactory(msgBus MessageBus, toolRegistry *mcp.ToolRegistry, sandbox *sandbox.Sandbox, logger *zap.Logger, emailConfig EmailConfig) *AgentFactory {
	return &AgentFactory{
		msgBus:       msgBus,
		toolRegistry: toolRegistry,
		sandbox:      sandbox,
		logger:       logger,
		emailConfig:  emailConfig,
	}
}

func (f *AgentFactory) CreateAllAgents() ([]Agent, error) {
	agents := make([]Agent, 0)

	planner := NewPlannerAgent(AgentConfig{
		ID:          AgentPlanner,
		Name:        "Experiment Planner",
		Description: "Designs experimental plans, generates hypotheses, creates task decomposition",
		Capabilities: []string{"plan", "design", "hypothesis_generation", "resource_allocation"},
		MaxRetries:  3,
		Timeout:     5 * time.Minute,
	}, f.msgBus)
	agents = append(agents, planner)

	researcher := NewResearcherAgent(AgentConfig{
		ID:          AgentResearcher,
		Name:        "Literature Researcher",
		Description: "Searches literature databases, retrieves evidence, synthesizes findings",
		Capabilities: []string{"research", "literature_search", "evidence_retrieval", "synthesis", "synthesize"},
		MaxRetries:  3,
		Timeout:     10 * time.Minute,
	}, f.msgBus, f.toolRegistry)
	agents = append(agents, researcher)

	executor := NewExecutorAgent(AgentConfig{
		ID:          AgentExecutor,
		Name:        "Experiment Executor",
		Description: "Executes computational experiments, runs simulations, manages wet-lab protocols",
		Capabilities: []string{"compute", "experiment", "docking", "md_simulation", "stability_prediction", "wetlab_protocol", "analyze"},
		MaxRetries:  2,
		Timeout:     60 * time.Minute,
	}, f.msgBus, f.sandbox)
	agents = append(agents, executor)

	validator := NewValidatorAgent(AgentConfig{
		ID:          AgentValidator,
		Name:        "Result Validator",
		Description: "Validates experimental results, statistical significance, reproducibility",
		Capabilities: []string{"validate", "statistical_validation", "reproducibility", "orthogonal_validation", "quality_check"},
		MaxRetries:  3,
		Timeout:     5 * time.Minute,
	}, f.msgBus)
	agents = append(agents, validator)

	critic := NewCriticAgentWrapper(AgentConfig{
		ID:          "critic",
		Name:        "Evidence Critic",
		Description: "Evaluates evidence quality, finds contradictions, assigns stances",
		Capabilities: []string{"critique", "evidence_analysis", "contradiction_detection"},
		MaxRetries:  3,
		Timeout:     5 * time.Minute,
	}, f.msgBus)
	agents = append(agents, critic)

	notifier := NewNotifierAgent(AgentConfig{
		ID:          AgentNotifier,
		Name:        "Notification Agent",
		Description: "Sends notifications via email, webhooks, Slack for workflow events",
		Capabilities: []string{"notify", "email", "webhook", "slack", "alerting"},
		MaxRetries:  3,
		Timeout:     2 * time.Minute,
	}, f.msgBus, f.emailConfig, f.logger)
	agents = append(agents, notifier)

	clinicalTrial := NewClinicalTrialAgent(AgentConfig{
		ID:          AgentClinicalTrial,
		Name:        "Clinical Trial Designer",
		Description: "Designs clinical trial protocols, calculates sample sizes, generates regulatory pathways",
		Capabilities: []string{"clinical_trial_design", "protocol_generation", "sample_size_calculation", "regulatory_pathway", "adaptive_design"},
		MaxRetries:  3,
		Timeout:     10 * time.Minute,
	}, f.msgBus)
	agents = append(agents, clinicalTrial)

	regulatory := NewRegulatoryAgent(AgentConfig{
		ID:          AgentRegulatory,
		Name:        "Regulatory Compliance",
		Description: "Checks 21 CFR Part 11, GxP, ICH, GDPR compliance for submissions",
		Capabilities: []string{"21cfr11", "gxp", "ich", "gdpr", "compliance_audit", "submission_readiness"},
		MaxRetries:  3,
		Timeout:     5 * time.Minute,
	}, f.msgBus)
	agents = append(agents, regulatory)

	biomarker := NewBiomarkerAgent(AgentConfig{
		ID:          AgentBiomarker,
		Name:        "Biomarker Discovery",
		Description: "Discovers, validates, and qualifies biomarkers for patient stratification",
		Capabilities: []string{"biomarker_discovery", "biomarker_validation", "biomarker_qualification", "companion_dx", "monitoring_panel"},
		MaxRetries:  3,
		Timeout:     10 * time.Minute,
	}, f.msgBus)
	agents = append(agents, biomarker)

	return agents, nil
}

func (f *AgentFactory) CreateOrchestrator() *Orchestrator {
	return NewOrchestrator(f.msgBus, f.logger)
}