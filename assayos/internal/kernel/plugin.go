package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"
	"time"

	"go.uber.org/zap"
)

type PluginType string

const (
	PluginTypeTool        PluginType = "tool"
	PluginTypeAgent       PluginType = "agent"
	PluginTypeVisualizer  PluginType = "visualizer"
	PluginTypeExecutor    PluginType = "executor"
	PluginTypeAuthz       PluginType = "authz"
)

type PluginManifest struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Type        PluginType             `json:"type"`
	Description string                 `json:"description"`
	Author      string                 `json:"author"`
	EntryPoint  string                 `json:"entry_point"` // exported symbol name
	Config      map[string]interface{} `json:"config"`
	Dependencies []string              `json:"dependencies"`
}

type PluginInstance struct {
	Manifest  *PluginManifest
	Symbol    interface{}
	LoadedAt  time.Time
	Enabled   bool
}

type PluginManager struct {
	logger    *zap.Logger
	pluginDir string
	mu        sync.RWMutex
	plugins   map[string]*PluginInstance
	registry  *PluginRegistry
}

type PluginRegistry struct {
	mu       sync.RWMutex
	tools    map[string]ToolPlugin
	agents   map[string]AgentPlugin
	visualizers map[string]VisualizerPlugin
	executors map[string]ExecutorPlugin
}

type ToolPlugin interface {
	Name() string
	Category() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

type AgentPlugin interface {
	ID() string
	Name() string
	Description() string
	Capabilities() []string
	Execute(ctx context.Context, task Task) (Result, error)
	HandleMessage(ctx context.Context, msg Message) (Message, error)
}

type VisualizerPlugin interface {
	Name() string
	Description() string
	Render(ctx context.Context, data map[string]interface{}) (map[string]interface{}, error)
}

type ExecutorPlugin interface {
	Name() string
	Description() string
	Execute(ctx context.Context, spec map[string]interface{}) (map[string]interface{}, error)
}

type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Priority    int                    `json:"priority"`
	Dependencies []string              `json:"dependencies"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type Result struct {
	TaskID    string                 `json:"task_id"`
	AgentID   string                 `json:"agent_id"`
	Status    string                 `json:"status"`
	Output    map[string]interface{} `json:"output"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Artifacts []Artifact             `json:"artifacts"`
}

type Artifact struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Content  interface{}            `json:"content"`
	Path     string                 `json:"path,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
}

type Message struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	From      string          `json:"from"`
	To        string          `json:"to"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	TraceID   string          `json:"trace_id"`
}

func NewPluginManager(pluginDir string, logger *zap.Logger) *PluginManager {
	return &PluginManager{
		logger:    logger.Named("plugins"),
		pluginDir: pluginDir,
		plugins:   make(map[string]*PluginInstance),
		registry: &PluginRegistry{
			tools:       make(map[string]ToolPlugin),
			agents:      make(map[string]AgentPlugin),
			visualizers: make(map[string]VisualizerPlugin),
			executors:   make(map[string]ExecutorPlugin),
		},
	}
}

func (pm *PluginManager) LoadAll(ctx context.Context) error {
	entries, err := os.ReadDir(pm.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			pm.logger.Info("Plugin directory does not exist, skipping", zap.String("dir", pm.pluginDir))
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			pluginPath := filepath.Join(pm.pluginDir, entry.Name())
			if err := pm.LoadPlugin(ctx, pluginPath); err != nil {
				pm.logger.Warn("Failed to load plugin", zap.String("plugin", entry.Name()), zap.Error(err))
			}
		}
	}

	pm.logger.Info("Plugin loading complete", zap.Int("loaded", len(pm.plugins)))
	return nil
}

func (pm *PluginManager) LoadPlugin(ctx context.Context, path string) error {
	manifestPath := filepath.Join(path, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("manifest: %w", err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}

	// Load .so file
	soPath := filepath.Join(path, manifest.EntryPoint+".so")
	p, err := plugin.Open(soPath)
	if err != nil {
		return fmt.Errorf("open plugin: %w", err)
	}

	// Look up exported symbol
	sym, err := p.Lookup(manifest.EntryPoint)
	if err != nil {
		return fmt.Errorf("lookup symbol: %w", err)
	}

	instance := &PluginInstance{
		Manifest: &manifest,
		Symbol:   sym,
		LoadedAt: time.Now(),
		Enabled:  true,
	}

	// Register based on type
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.plugins[manifest.Name] = instance

	switch manifest.Type {
	case PluginTypeTool:
		if tp, ok := sym.(ToolPlugin); ok {
			pm.registry.mu.Lock()
			pm.registry.tools[manifest.Name] = tp
			pm.registry.mu.Unlock()
		}
	case PluginTypeAgent:
		if ap, ok := sym.(AgentPlugin); ok {
			pm.registry.mu.Lock()
			pm.registry.agents[manifest.Name] = ap
			pm.registry.mu.Unlock()
		}
	case PluginTypeVisualizer:
		if vp, ok := sym.(VisualizerPlugin); ok {
			pm.registry.mu.Lock()
			pm.registry.visualizers[manifest.Name] = vp
			pm.registry.mu.Unlock()
		}
	case PluginTypeExecutor:
		if ep, ok := sym.(ExecutorPlugin); ok {
			pm.registry.mu.Lock()
			pm.registry.executors[manifest.Name] = ep
			pm.registry.mu.Unlock()
		}
	}

	pm.logger.Info("Plugin loaded", zap.String("name", manifest.Name), zap.String("type", string(manifest.Type)))
	return nil
}

func (pm *PluginManager) UnloadAll(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for name := range pm.plugins {
		delete(pm.plugins, name)
	}
	pm.registry.tools = make(map[string]ToolPlugin)
	pm.registry.agents = make(map[string]AgentPlugin)
	pm.registry.visualizers = make(map[string]VisualizerPlugin)
	pm.registry.executors = make(map[string]ExecutorPlugin)

	pm.logger.Info("All plugins unloaded")
	return nil
}

func (pm *PluginManager) GetTool(name string) (ToolPlugin, bool) {
	pm.registry.mu.RLock()
	defer pm.registry.mu.RUnlock()
	t, ok := pm.registry.tools[name]
	return t, ok
}

func (pm *PluginManager) GetAgent(name string) (AgentPlugin, bool) {
	pm.registry.mu.RLock()
	defer pm.registry.mu.RUnlock()
	a, ok := pm.registry.agents[name]
	return a, ok
}

func (pm *PluginManager) GetVisualizer(name string) (VisualizerPlugin, bool) {
	pm.registry.mu.RLock()
	defer pm.registry.mu.RUnlock()
	v, ok := pm.registry.visualizers[name]
	return v, ok
}

func (pm *PluginManager) GetExecutor(name string) (ExecutorPlugin, bool) {
	pm.registry.mu.RLock()
	defer pm.registry.mu.RUnlock()
	e, ok := pm.registry.executors[name]
	return e, ok
}

func (pm *PluginManager) ListPlugins() []*PluginManifest {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	manifests := make([]*PluginManifest, 0, len(pm.plugins))
	for _, p := range pm.plugins {
		if p.Enabled {
			manifests = append(manifests, p.Manifest)
		}
	}
	return manifests
}