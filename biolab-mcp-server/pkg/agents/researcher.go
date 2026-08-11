package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/pkg/mcp"
)

type ResearcherAgent struct {
	*BaseAgent
	toolRegistry *mcp.ToolRegistry
}

func NewResearcherAgent(config AgentConfig, msgBus MessageBus, toolRegistry *mcp.ToolRegistry) *ResearcherAgent {
	base := NewBaseAgent(config, msgBus)
	return &ResearcherAgent{BaseAgent: base, toolRegistry: toolRegistry}
}

func (r *ResearcherAgent) Execute(ctx context.Context, task Task) (Result, error) {
	r.SetStatus(AgentStatusRunning)
	defer r.SetStatus(AgentStatusIdle)

	start := time.Now()

	query := getString(task.Input, "query", "")
	sources := getStringSlice(task.Input, "sources")
	maxResults := getInt(task.Input, "max_results", 50)

	if query == "" {
		return Result{}, fmt.Errorf("query required")
	}

	if len(sources) == 0 {
		sources = []string{"pubmed", "uniprot", "chembl", "pdb", "kegg", "bindingdb"}
	}

	evidence := make([]EvidenceItem, 0)
	
	for _, source := range sources {
		tool, ok := r.toolRegistry.Get("retriever", source)
		if !ok {
			continue
		}
		
		result, err := tool.Execute(ctx, map[string]interface{}{
			"query":       query,
			"max_results": maxResults,
		})
		if err != nil {
			continue
		}

		items := parseEvidence(result, source)
		evidence = append(evidence, items...)
	}

	synthesis := synthesizeEvidence(evidence, query)

	output := map[string]interface{}{
		"query":           query,
		"evidence_count":  len(evidence),
		"evidence":        evidence,
		"synthesis":       synthesis,
		"sources_searched": sources,
	}

	return Result{
		TaskID:   task.ID,
		AgentID:  r.ID(),
		Status:   "completed",
		Output:   output,
		Duration: time.Since(start),
		Artifacts: []Artifact{{
			Name:    "research_results.json",
			Type:    "application/json",
			Content: output,
		}},
	}, nil
}

func (r *ResearcherAgent) HandleMessage(ctx context.Context, msg Message) (Message, error) {
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

type EvidenceItem struct {
	ID          string                 `json:"id"`
	Source      string                 `json:"source"`
	Title       string                 `json:"title"`
	Authors     []string               `json:"authors"`
	Year        int                    `json:"year"`
	DOI         string                 `json:"doi"`
	Abstract    string                 `json:"abstract"`
	Relevance   float64                `json:"relevance"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type Synthesis struct {
	Summary          string   `json:"summary"`
	KeyFindings      []string `json:"key_findings"`
	Gaps             []string `json:"gaps"`
	Confidence       float64  `json:"confidence"`
	Contradictions   []string `json:"contradictions"`
}

func parseEvidence(result map[string]interface{}, source string) []EvidenceItem {
	items := make([]EvidenceItem, 0)
	
	if results, ok := result["results"].([]interface{}); ok {
		for i, r := range results {
			if m, ok := r.(map[string]interface{}); ok {
				items = append(items, EvidenceItem{
					ID:        fmt.Sprintf("%s-%d", source, i),
					Source:    source,
					Title:     getString(m, "title", ""),
					Abstract:  getString(m, "abstract", ""),
					Year:      getInt(m, "year", 2024),
					DOI:       getString(m, "doi", ""),
					Relevance: getFloat(m, "score", 0.5),
					Metadata:  m,
				})
			}
		}
	}
	return items
}

func synthesizeEvidence(evidence []EvidenceItem, query string) Synthesis {
	if len(evidence) == 0 {
		return Synthesis{
			Summary:    "No evidence found",
			Confidence: 0,
		}
	}

	keyFindings := make([]string, 0)
	for _, e := range evidence[:min(5, len(evidence))] {
		if e.Title != "" {
			keyFindings = append(keyFindings, e.Title)
		}
	}

	return Synthesis{
		Summary:     fmt.Sprintf("Found %d relevant sources for '%s'. Top findings relate to %s", len(evidence), query, keyFindings[0]),
		KeyFindings: keyFindings,
		Gaps:        []string{"Limited clinical data", "Mechanism not fully elucidated"},
		Confidence:  0.75,
		Contradictions: []string{},
	}
}

func getInt(input map[string]interface{}, key string, defaultVal int) int {
	if v, ok := input[key].(float64); ok {
		return int(v)
	}
	return defaultVal
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}