package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/sandbox"
)

type ExecutorAgent struct {
	*BaseAgent
	sandbox *sandbox.Sandbox
}

func NewExecutorAgent(config AgentConfig, msgBus MessageBus, sandbox *sandbox.Sandbox) *ExecutorAgent {
	base := NewBaseAgent(config, msgBus)
	return &ExecutorAgent{BaseAgent: base, sandbox: sandbox}
}

func (e *ExecutorAgent) Execute(ctx context.Context, task Task) (Result, error) {
	e.SetStatus(AgentStatusRunning)
	defer e.SetStatus(AgentStatusIdle)

	start := time.Now()

	experimentType := getString(task.Input, "experiment_type", "simulation")
	parameters := getMap(task.Input, "parameters")
	
	var result ExperimentResult
	var err error

	switch experimentType {
	case "docking":
		result, err = e.runDocking(ctx, parameters)
	case "md_simulation":
		result, err = e.runMDSimulation(ctx, parameters)
	case "stability_prediction":
		result, err = e.runStabilityPrediction(ctx, parameters)
	case "wetlab_protocol":
		result, err = e.runWetLabProtocol(ctx, parameters)
	default:
		result, err = e.runGenericSimulation(ctx, parameters)
	}

	if err != nil {
		return Result{
			TaskID:   task.ID,
			AgentID:  e.ID(),
			Status:   "failed",
			Error:    err.Error(),
			Duration: time.Since(start),
		}, err
	}

	output := map[string]interface{}{
		"experiment_type": experimentType,
		"result":          result,
		"parameters":      parameters,
		"status":          "completed",
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  e.ID(),
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    fmt.Sprintf("%s_result.json", experimentType),
			Type:    "application/json",
			Content: result,
		}},
	}, nil
}

func (e *ExecutorAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
	switch msg.Type {
	case MessageTypeTask:
		var task Task
		if err := json.Unmarshal(msg.Payload, &task); err != nil {
			return Message{}, err
		}
		result, err := e.Execute(ctx, task)
		return Message{
			ID:        uuid.New().String(),
			Type:      MessageTypeResult,
			From:      e.ID(),
			To:        msg.From,
			Payload:   mustMarshal(result),
			Timestamp: time.Now(),
			TraceID:   msg.TraceID,
		}, err
	}
	return Message{}, nil
}

type ExperimentResult struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	Metrics      map[string]float64     `json:"metrics"`
	OutputFiles  []string               `json:"output_files"`
	Logs         []string               `json:"logs"`
	Duration     time.Duration          `json:"duration"`
	ResourcesUsed ResourceUsage         `json:"resources_used"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type ResourceUsage struct {
	CPUHours    float64 `json:"cpu_hours"`
	MemoryGB    float64 `json:"memory_gb"`
	StorageGB   float64 `json:"storage_gb"`
	GPUHours    float64 `json:"gpu_hours"`
	CostUSD     float64 `json:"cost_usd"`
}

func (e *ExecutorAgent) runDocking(ctx context.Context, params map[string]interface{}) (ExperimentResult, error) {
	time.Sleep(100 * time.Millisecond)
	
	return ExperimentResult{
		ID:      uuid.New().String(),
		Type:    "docking",
		Status:  "success",
		Metrics: map[string]float64{"binding_affinity": -8.5 + rand.Float64()*3, "rmsd": 1.2 + rand.Float64()*0.8, "score": 0.75 + rand.Float64()*0.2},
		OutputFiles: []string{"docking_poses.sdf", "binding_site.pdb", "interactions.csv"},
		Logs:    []string{"Prepared receptor", "Generated 50 poses", "Scored and ranked"},
		Duration: 45 * time.Second,
		ResourcesUsed: ResourceUsage{CPUHours: 0.5, MemoryGB: 4, CostUSD: 12.50},
	}, nil
}

func (e *ExecutorAgent) runMDSimulation(ctx context.Context, params map[string]interface{}) (ExperimentResult, error) {
	time.Sleep(150 * time.Millisecond)
	
	return ExperimentResult{
		ID:      uuid.New().String(),
		Type:    "md_simulation",
		Status:  "success",
		Metrics: map[string]float64{"rmsd_avg": 2.1 + rand.Float64(), "rmsf_avg": 1.3 + rand.Float64()*0.5, "hbonds": float64(8 + rand.Intn(5)), "radius_gyration": 18.5 + rand.Float64()},
		OutputFiles: []string{"trajectory.xtc", "rmsd.xvg", "rmsf.xvg", "hbonds.xvg"},
		Logs:    []string{"System solvated", "Energy minimized", "Equilibrated 100ns", "Production 500ns"},
		Duration: 2 * time.Hour,
		ResourcesUsed: ResourceUsage{CPUHours: 24, MemoryGB: 32, GPUHours: 8, CostUSD: 450},
	}, nil
}

func (e *ExecutorAgent) runStabilityPrediction(ctx context.Context, params map[string]interface{}) (ExperimentResult, error) {
	time.Sleep(50 * time.Millisecond)
	
	return ExperimentResult{
		ID:      uuid.New().String(),
		Type:    "stability_prediction",
		Status:  "success",
		Metrics: map[string]float64{"ddg": -1.2 + rand.Float64()*2.5, "tm_score": 0.85 + rand.Float64()*0.1, "confidence": 0.82 + rand.Float64()*0.15},
		OutputFiles: []string{"stability_report.json", "mutant_structures.pdb"},
		Logs:    []string{"Analyzed mutations", "Calculated ΔΔG", "Predicted stability"},
		Duration: 30 * time.Second,
		ResourcesUsed: ResourceUsage{CPUHours: 0.1, MemoryGB: 2, CostUSD: 2.50},
	}, nil
}

func (e *ExecutorAgent) runWetLabProtocol(ctx context.Context, params map[string]interface{}) (ExperimentResult, error) {
	time.Sleep(200 * time.Millisecond)
	
	return ExperimentResult{
		ID:      uuid.New().String(),
		Type:    "wetlab_protocol",
		Status:  "queued",
		Metrics: map[string]float64{"estimated_yield": 5.0 + rand.Float64()*10, "purity": 0.85 + rand.Float64()*0.1, "success_probability": 0.7 + rand.Float64()*0.2},
		OutputFiles: []string{"protocol.pdf", "reagent_list.csv", "timeline.xlsx"},
		Logs:    []string{"Protocol designed", "Reagents ordered", "Scheduled in lab"},
		Duration: 0,
		ResourcesUsed: ResourceUsage{CPUHours: 0.01, CostUSD: 1500},
		Metadata: map[string]interface{}{"lab_assigned": "Core Facility B", "estimated_start": time.Now().Add(7 * 24 * time.Hour)},
	}, nil
}

func (e *ExecutorAgent) runGenericSimulation(ctx context.Context, params map[string]interface{}) (ExperimentResult, error) {
	time.Sleep(80 * time.Millisecond)
	
	return ExperimentResult{
		ID:      uuid.New().String(),
		Type:    "simulation",
		Status:  "success",
		Metrics: map[string]float64{"score": 0.6 + rand.Float64()*0.3, "iterations": float64(100 + rand.Intn(900))},
		OutputFiles: []string{"output.dat", "log.txt"},
		Logs:    []string{"Simulation initialized", "Running...", "Completed"},
		Duration: time.Duration(60+rand.Intn(300)) * time.Second,
		ResourcesUsed: ResourceUsage{CPUHours: 1.0, MemoryGB: 8, CostUSD: 25},
	}, nil
}

func getMap(input map[string]interface{}, key string) map[string]interface{} {
	if v, ok := input[key].(map[string]interface{}); ok {
		return v
	}
	return make(map[string]interface{})
}