package retrievers

import (
	"context"
	"math/rand"
	"time"

)

type UniProtRetriever struct{}

func NewUniProtRetriever() *UniProtRetriever { return &UniProtRetriever{} }

func (u *UniProtRetriever) Name() string        { return "UniProt" }
func (u *UniProtRetriever) Category() string    { return "retriever" }
func (u *UniProtRetriever) Description() string { return "Query UniProt for protein sequences, annotations, and variants" }
func (u *UniProtRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":       map[string]interface{}{"type": "string", "description": "Search query (gene name, accession, etc.)"},
			"include_isoforms": map[string]interface{}{"type": "boolean", "default": true},
		},
		"required": []string{"query"},
	}
}

func (u *UniProtRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(50+rand.Intn(150)) * time.Millisecond):
	}

	entries := []map[string]interface{}{
		{
			"accession":   "P15056",
			"gene_name":   "BRAF",
			"protein_name": "Serine/threonine-protein kinase B-raf",
			"organism":    "Homo sapiens",
			"length":      766,
			"sequence":    "MNGTEGPNFYVPFSNKTGVVRSPFEAPQYYLAEPWQFSMLAAYMFLLIMLGFPINFLTLYVTVQHKKLRTPLNYILLNLAVADLFMVFGGFTTLTYLTKKAGL...",
			"domains": []map[string]interface{}{
				{"type": "Protein kinase", "start": 457, "end": 717},
				{"type": "P-loop", "start": 465, "end": 472},
			},
			"variants": []map[string]interface{}{
				{"position": 600, "change": "V>E", "type": "Missense", "clinical_significance": "Pathogenic"},
				{"position": 466, "change": "G>A", "type": "Missense", "clinical_significance": "Likely pathogenic"},
			},
			"go_terms": []string{"GO:0004672", "GO:0006468", "GO:0007165"},
			"pathways": []string{"MAPK signaling pathway", "Ras signaling pathway"},
		},
	}

	return map[string]interface{}{
		"query":     query,
		"results":   entries,
		"cache_hit": rand.Float32() < 0.4,
	}, nil
}