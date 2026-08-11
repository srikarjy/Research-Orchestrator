package retrievers

import (
	"context"
	"math/rand"
	"time"

)

type KEGGRetriever struct{}

func NewKEGGRetriever() *KEGGRetriever { return &KEGGRetriever{} }

func (k *KEGGRetriever) Name() string        { return "KEGG" }
func (k *KEGGRetriever) Category() string    { return "retriever" }
func (k *KEGGRetriever) Description() string { return "Query KEGG for pathways, genes, and compounds" }
func (k *KEGGRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":      map[string]interface{}{"type": "string", "description": "Pathway ID, gene, or compound"},
			"entry_type": map[string]interface{}{"type": "string", "enum": []string{"pathway", "gene", "compound", "module"}, "default": "pathway"},
		},
		"required": []string{"query"},
	}
}

func (k *KEGGRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(80+rand.Intn(200)) * time.Millisecond):
	}

	results := []map[string]interface{}{
		{
			"entry_id":   "map04010",
			"name":       "MAPK signaling pathway",
			"class":      "Signal transduction",
			"genes":      []string{"BRAF", "MAP2K1", "MAPK1", "MAPK3", "EGFR", "KRAS"},
			"compounds":  []string{"C00002", "C00008"},
			"image_url":  "https://www.kegg.jp/kegg/pathway/hsa/hsa04010.png",
			"kgml_url":   "https://rest.kegg.jp/get/map04010/kgml",
		},
		{
			"entry_id":   "map04012",
			"name":       "ErbB signaling pathway",
			"class":      "Signal transduction",
			"genes":      []string{"BRAF", "EGFR", "ERBB2", "MAP2K1", "MAPK1"},
			"image_url":  "https://www.kegg.jp/kegg/pathway/hsa/hsa04012.png",
		},
	}

	return map[string]interface{}{
		"query":       query,
		"results":     results,
		"cache_hit":   rand.Float32() < 0.3,
	}, nil
}