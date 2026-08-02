package docking

import (
	"context"
	"fmt"
	"sync"

	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/tools/analyzers"
)

type DockingTool interface {
	analyzers.Analyzer
}

type DockingFactory struct {
	mu      sync.RWMutex
	tools   map[string]DockingTool
	workDir string
}

func NewDockingFactory(workDir string) *DockingFactory {
	return &DockingFactory{
		tools:   make(map[string]DockingTool),
		workDir: workDir,
	}
}

func (f *DockingFactory) Register(name string, tool DockingTool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tools[name] = tool
}

func (f *DockingFactory) Get(name string) (DockingTool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	tool, ok := f.tools[name]
	if !ok {
		return nil, fmt.Errorf("docking tool %s not found", name)
	}
	return tool, nil
}

func (f *DockingFactory) GetAll() map[string]DockingTool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make(map[string]DockingTool, len(f.tools))
	for k, v := range f.tools {
		result[k] = v
	}
	return result
}

func (f *DockingFactory) Initialize() {
	f.Register("AutoDockVina", NewVinaDockingTool(f.workDir))
	f.Register("GNINA", NewGNINADockingTool(f.workDir))
}

type DockingRegistry struct {
	factory *DockingFactory
}

func NewDockingRegistry(workDir string) *DockingRegistry {
	factory := NewDockingFactory(workDir)
	factory.Initialize()
	return &DockingRegistry{factory: factory}
}

func (r *DockingRegistry) GetTool(name string) (DockingTool, error) {
	return r.factory.Get(name)
}

func (r *DockingRegistry) GetAllTools() map[string]DockingTool {
	return r.factory.GetAll()
}

func (r *DockingRegistry) ExecuteAll(ctx context.Context, input map[string]interface{}) (map[string]map[string]interface{}, error) {
	results := make(map[string]map[string]interface{})
	tools := r.factory.GetAll()

	for name, tool := range tools {
		res, err := tool.Execute(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		results[name] = res
	}

	return results, nil
}

func (r *DockingRegistry) ExecuteBest(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	tools := r.factory.GetAll()
	var bestResult map[string]interface{}
	var bestScore float64 = 999.0

	for name, tool := range tools {
		res, err := tool.Execute(ctx, input)
		if err != nil {
			continue
		}
		if affinity, ok := res["best_affinity_kcal_mol"].(float64); ok {
			if affinity < bestScore {
				bestScore = affinity
				bestResult = res
				bestResult["tool_used"] = name
			}
		} else if cnnScore, ok := res["best_cnn_score"].(float64); ok {
			if cnnScore < bestScore {
				bestScore = cnnScore
				bestResult = res
				bestResult["tool_used"] = name
			}
		}
	}

	if bestResult == nil {
		return nil, fmt.Errorf("no docking tool produced valid results")
	}
	return bestResult, nil
}