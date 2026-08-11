package mcp

import (
	"context"
)

type Tool interface {
	Name() string
	Category() string // retriever, analyzer, visualizer, executor
	Description() string
	InputSchema() map[string]interface{}
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

func (r *ToolRegistry) Register(tool Tool) {
	key := tool.Category() + ":" + tool.Name()
	r.tools[key] = tool
}

func (r *ToolRegistry) Get(category, name string) (Tool, bool) {
	tool, ok := r.tools[category+":"+name]
	return tool, ok
}

func (r *ToolRegistry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

func (r *ToolRegistry) ListByCategory(category string) []Tool {
	var tools []Tool
	for _, t := range r.tools {
		if t.Category() == category {
			tools = append(tools, t)
		}
	}
	return tools
}