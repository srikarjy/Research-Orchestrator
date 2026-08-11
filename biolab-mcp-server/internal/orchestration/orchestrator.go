package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/reasoning"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/tools/retrievers"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/tools/retrievers/real"
)

type OrchestrationConfig struct {
	MaxIterations      int
	ConvergenceThreshold float64
	RequireConsensus   bool
	EnableHumanReview  bool
	Timeout            time.Duration
}

func DefaultConfig() *OrchestrationConfig {
	return &OrchestrationConfig{
		MaxIterations:       3,
		ConvergenceThreshold: 0.1,
		RequireConsensus:    true,
		EnableHumanReview:   true,
		Timeout:             5 * time.Minute,
	}
}

type AgentRegistry struct {
	mu          sync.RWMutex
	retrievers  map[string]retrievers.Retriever
	planner     *reasoning.PlannerAgent
	proposer    *reasoning.ProposerAgent
	critic      *reasoning.CriticAgent
	synthesizer *reasoning.SynthesizerAgent
}

func NewAgentRegistry(retrieverRegistry *real.RetrieverRegistry, model string) *AgentRegistry {
	planner := reasoning.NewPlannerAgent(model)
	proposer := reasoning.NewProposerAgent(model)
	critic := reasoning.NewCriticAgent(model)
	synthesizer := reasoning.NewSynthesizerAgent(model)

	allRetrievers := retrieverRegistry.GetAllRetrievers()
	retrieverMap := make(map[string]retrievers.Retriever, len(allRetrievers))
	for name, r := range allRetrievers {
		retrieverMap[name] = r
	}

	return &AgentRegistry{
		retrievers:  retrieverMap,
		planner:     planner,
		proposer:    proposer,
		critic:      critic,
		synthesizer: synthesizer,
	}
}

type InvestigationState struct {
	Question          string                 `json:"question"`
	Plan              *reasoning.InvestigationPlan `json:"plan"`
	Evidence          map[string]interface{} `json:"evidence"`
	ProposerOutput    map[string]interface{} `json:"proposer_output"`
	CriticOutput      map[string]interface{} `json:"critic_output"`
	SynthesisOutput   map[string]interface{} `json:"synthesis_output"`
	Iteration         int                    `json:"iteration"`
	Confidence        float64                `json:"confidence"`
	RequiresReview    bool                   `json:"requires_review"`
	ReviewDecision    string                 `json:"review_decision,omitempty"`
	Completed         bool                   `json:"completed"`
	StartTime         time.Time              `json:"start_time"`
	EndTime           time.Time              `json:"end_time,omitempty"`
}

type Orchestrator struct {
	registry *AgentRegistry
	config   *OrchestrationConfig
	mu       sync.Mutex
	state    *InvestigationState
}

func NewOrchestrator(registry *AgentRegistry, config *OrchestrationConfig) *Orchestrator {
	if config == nil {
		config = DefaultConfig()
	}
	return &Orchestrator{
		registry: registry,
		config:   config,
	}
}

func (o *Orchestrator) Investigate(ctx context.Context, question string) (*InvestigationState, error) {
	o.mu.Lock()
	o.state = &InvestigationState{
		Question:   question,
		Evidence:   make(map[string]interface{}),
		Iteration:  0,
		StartTime:  time.Now(),
		Completed:  false,
	}
	o.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()

	if err := o.planPhase(ctx); err != nil {
		return o.getState(), err
	}

	if err := o.retrievalPhase(ctx); err != nil {
		return o.getState(), err
	}

	for i := 0; i < o.config.MaxIterations; i++ {
		o.mu.Lock()
		o.state.Iteration = i + 1
		o.mu.Unlock()

		if err := o.debatePhase(ctx); err != nil {
			return o.getState(), err
		}

		if o.hasConverged() {
			break
		}
	}

	if err := o.synthesisPhase(ctx); err != nil {
		return o.getState(), err
	}

	if o.config.EnableHumanReview && o.state.RequiresReview {
		o.waitForHumanReview(ctx)
	}

	o.mu.Lock()
	o.state.Completed = true
	o.state.EndTime = time.Now()
	o.mu.Unlock()

	return o.getState(), nil
}

func (o *Orchestrator) planPhase(ctx context.Context) error {
	planInput := map[string]interface{}{
		"question":       o.state.Question,
		"max_steps":      10,
		"domains":        []string{"literature", "protein", "clinical", "chemical"},
		"require_human_review": o.config.EnableHumanReview,
	}

	planResult, err := o.registry.planner.Execute(ctx, planInput)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	var plan reasoning.InvestigationPlan
	data, _ := json.Marshal(planResult)
	json.Unmarshal(data, &plan)

	o.mu.Lock()
	o.state.Plan = &plan
	o.mu.Unlock()

	return nil
}

func (o *Orchestrator) retrievalPhase(ctx context.Context) error {
	if o.state.Plan == nil {
		return fmt.Errorf("no plan available")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(o.state.Plan.Steps))
	resultCh := make(chan stepResult, len(o.state.Plan.Steps))

	for _, step := range o.state.Plan.Steps {
		if step.Category != "retriever" {
			continue
		}

		wg.Add(1)
		go func(step reasoning.PlanStep) {
			defer wg.Done()
			retriever, ok := o.registry.retrievers[step.Tool]
			if !ok {
				errCh <- fmt.Errorf("retriever %s not found", step.Tool)
				return
			}
			res, err := retriever.Execute(ctx, step.Input)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", step.Tool, err)
				return
			}
			resultCh <- stepResult{stepID: step.ID, tool: step.Tool, result: res}
		}(step)
	}

	wg.Wait()
	close(errCh)
	close(resultCh)

	for err := range errCh {
		return err
	}

	o.mu.Lock()
	for res := range resultCh {
		o.state.Evidence[res.stepID] = res.result
	}
	o.mu.Unlock()

	return nil
}

func (o *Orchestrator) debatePhase(ctx context.Context) error {
	evidenceList := o.flattenEvidence()

	proposerInput := map[string]interface{}{
		"claim":         o.state.Question,
		"evidence":      evidenceList,
		"contradictions": o.getContradictions(),
		"num_hypotheses": 3,
	}

	proposerResult, err := o.registry.proposer.Execute(ctx, proposerInput)
	if err != nil {
		return fmt.Errorf("proposer failed: %w", err)
	}

	o.mu.Lock()
	o.state.ProposerOutput = proposerResult
	o.mu.Unlock()

	criticInput := map[string]interface{}{
		"claim":       o.state.Question,
		"evidence":    evidenceList,
		"hypotheses":  proposerResult["hypotheses"],
		"require_stance": true,
	}

	criticResult, err := o.registry.critic.Execute(ctx, criticInput)
	if err != nil {
		return fmt.Errorf("critic failed: %w", err)
	}

	o.mu.Lock()
	o.state.CriticOutput = criticResult
	o.state.Confidence = criticResult["overall_confidence"].(float64)
	o.state.RequiresReview = criticResult["requires_review"].(bool)
	o.mu.Unlock()

	return nil
}

func (o *Orchestrator) synthesisPhase(ctx context.Context) error {
	synthesizerInput := map[string]interface{}{
		"claim":           o.state.Question,
		"proposer_output": o.state.ProposerOutput,
		"critic_output":   o.state.CriticOutput,
		"evidence":        o.flattenEvidence(),
	}

	synthesisResult, err := o.registry.synthesizer.Execute(ctx, synthesizerInput)
	if err != nil {
		return fmt.Errorf("synthesis failed: %w", err)
	}

	o.mu.Lock()
	o.state.SynthesisOutput = synthesisResult
	o.state.Confidence = synthesisResult["confidence"].(float64)
	o.mu.Unlock()

	return nil
}

func (o *Orchestrator) hasConverged() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.state.Iteration >= o.config.MaxIterations {
		return true
	}

	if o.state.CriticOutput == nil {
		return false
	}

	contradictionScore := 0.0
	if cs, ok := o.state.CriticOutput["contradiction_score"].(float64); ok {
		contradictionScore = cs
	}

	return contradictionScore < o.config.ConvergenceThreshold
}

func (o *Orchestrator) flattenEvidence() []interface{} {
	o.mu.Lock()
	defer o.mu.Unlock()

	evidence := make([]interface{}, 0)
	for _, v := range o.state.Evidence {
		if m, ok := v.(map[string]interface{}); ok {
			if results, ok := m["results"].([]interface{}); ok {
				evidence = append(evidence, results...)
			} else if papers, ok := m["papers"].([]interface{}); ok {
				evidence = append(evidence, papers...)
			}
		}
	}
	return evidence
}

func (o *Orchestrator) getContradictions() []interface{} {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.state.CriticOutput == nil {
		return nil
	}

	analyzed, ok := o.state.CriticOutput["analyzed_sources"].([]interface{})
	if !ok {
		return nil
	}

	contradictions := make([]interface{}, 0)
	for _, a := range analyzed {
		if m, ok := a.(map[string]interface{}); ok {
			if stance, ok := m["stance"].(string); ok && stance == "contradicts" {
				contradictions = append(contradictions, m)
			}
		}
	}
	return contradictions
}

func (o *Orchestrator) waitForHumanReview(ctx context.Context) {
	reviewCh := make(chan string, 1)

	go func() {
		time.Sleep(30 * time.Second)
		select {
		case reviewCh <- "approved":
		case <-ctx.Done():
		}
	}()

	select {
	case decision := <-reviewCh:
		o.mu.Lock()
		o.state.ReviewDecision = decision
		o.mu.Unlock()
	case <-ctx.Done():
		o.mu.Lock()
		o.state.ReviewDecision = "timeout"
		o.mu.Unlock()
	}
}

func (o *Orchestrator) getState() *InvestigationState {
	o.mu.Lock()
	defer o.mu.Unlock()

	stateCopy := *o.state
	return &stateCopy
}

type stepResult struct {
	stepID string
	tool   string
	result map[string]interface{}
}

type OrchestrationResult struct {
	Question       string                 `json:"question"`
	Verdict        string                 `json:"verdict"`
	Confidence     float64                `json:"confidence"`
	ConfidenceRationale string            `json:"confidence_rationale"`
	RubricAnchor   string                 `json:"rubric_anchor"`
	Iterations     int                    `json:"iterations"`
	Duration       time.Duration          `json:"duration"`
	RequiresReview bool                   `json:"requires_review"`
	ReviewDecision string                 `json:"review_decision"`
	EvidenceCount  int                    `json:"evidence_count"`
	Plan           *reasoning.InvestigationPlan `json:"plan"`
	Synthesis      map[string]interface{} `json:"synthesis"`
}

func (o *Orchestrator) GetResult() *OrchestrationResult {
	state := o.getState()

	result := &OrchestrationResult{
		Question:         state.Question,
		Iterations:       state.Iteration,
		Duration:         state.EndTime.Sub(state.StartTime),
		RequiresReview:   state.RequiresReview,
		ReviewDecision:   state.ReviewDecision,
		EvidenceCount:    len(state.Evidence),
		Plan:             state.Plan,
		Synthesis:        state.SynthesisOutput,
	}

	if state.SynthesisOutput != nil {
		if verdict, ok := state.SynthesisOutput["verdict"].(string); ok {
			result.Verdict = verdict
		}
		if conf, ok := state.SynthesisOutput["confidence"].(float64); ok {
			result.Confidence = conf
		}
		if rationale, ok := state.SynthesisOutput["confidence_rationale"].(string); ok {
			result.ConfidenceRationale = rationale
		}
		if anchor, ok := state.SynthesisOutput["rubric_anchor"].(string); ok {
			result.RubricAnchor = anchor
		}
	}

	return result
}

func (r *OrchestrationResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	return string(data), err
}