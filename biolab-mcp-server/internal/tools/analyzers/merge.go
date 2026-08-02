package analyzers

import (
	"context"
	"math/rand"
	"time"

)

type EvidenceMerge struct{}

func NewEvidenceMerge() *EvidenceMerge { return &EvidenceMerge{} }

func (e *EvidenceMerge) Name() string        { return "EvidenceMerge" }
func (e *EvidenceMerge) Category() string    { return "analyzer" }
func (e *EvidenceMerge) Description() string { return "Merge and deduplicate evidence from multiple retrievers" }
func (e *EvidenceMerge) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sources": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{"type": "object"},
			},
			"dedup_threshold": map[string]interface{}{"type": "number", "default": 0.85},
		},
		"required": []string{"sources"},
	}
}

func (e *EvidenceMerge) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	sources := input["sources"].([]interface{})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(50+rand.Intn(100)) * time.Millisecond):
	}

	// Deduplicate by content hash
	seen := make(map[string]bool)
	unique := make([]interface{}, 0)
	duplicates := 0
	
	for _, src := range sources {
		if srcMap, ok := src.(map[string]interface{}); ok {
			hash := srcMap["content_hash"]
			if hash != nil {
				if seen[hash.(string)] {
					duplicates++
					continue
				}
				seen[hash.(string)] = true
			}
			unique = append(unique, src)
		}
	}

	return map[string]interface{}{
		"total_input":     len(sources),
		"unique_output":   len(unique),
		"duplicates_removed": duplicates,
		"merged_sources":  unique,
		"dedup_threshold": input["dedup_threshold"],
		"cache_hit":       rand.Float32() < 0.5,
	}, nil
}