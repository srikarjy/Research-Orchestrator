package retrievers

import (
	"context"
	"math/rand"
	"time"

)

type ChEMBLRetriever struct{}

func NewChEMBLRetriever() *ChEMBLRetriever { return &ChEMBLRetriever{} }

func (c *ChEMBLRetriever) Name() string        { return "ChEMBL" }
func (c *ChEMBLRetriever) Category() string    { return "retriever" }
func (c *ChEMBLRetriever) Description() string { return "Query ChEMBL for bioactive molecules, assays, and targets" }
func (c *ChEMBLRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":        map[string]interface{}{"type": "string", "description": "Target name, compound name, or SMILES"},
			"search_type":  map[string]interface{}{"type": "string", "enum": []string{"target", "compound", "assay", "similarity"}, "default": "target"},
			"threshold":    map[string]interface{}{"type": "number", "default": 0.7},
		},
		"required": []string{"query"},
	}
}

func (c *ChEMBLRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	searchType := "target"
	if st, ok := input["search_type"].(string); ok {
		searchType = st
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(80+rand.Intn(200)) * time.Millisecond):
	}

	var results []map[string]interface{}
	switch searchType {
	case "compound":
		results = []map[string]interface{}{
			{
				"chembl_id": "CHEMBL123456",
				"smiles":    "CCCS(=O)(=O)Nc1ccc(F)c(C(=O)c2c[nH]c3ncc(-c4ccc(Cl)cc4)cc23)c1F",
				"name":      "Vemurafenib",
				"mw":        489.9,
				"logp":      3.2,
				"targets":   []string{"BRAF V600E"},
				"assays":    []string{"Biochemical IC50: 31 nM", "Cellular IC50: 15 nM"},
			},
		}
	case "assay":
		results = []map[string]interface{}{
			{
				"assay_id":    "CHEMBL_ASSAY_123",
				"description": "BRAF V600E kinase inhibition",
				"type":        "Biochemical",
				"organism":    "Homo sapiens",
				"compounds_tested": 245,
				"parameters":  []string{"IC50", "Ki", "Kd"},
			},
		}
	default: // target
		results = []map[string]interface{}{
			{
				"target_id":   "CHEMBL_TARGET_4024",
				"target_name": "BRAF",
				"organism":    "Homo sapiens",
				"target_type": "SINGLE PROTEIN",
				"compounds":   1247,
				"assays":      89,
				"known_drugs": []string{"Vemurafenib", "Dabrafenib", "Encorafenib"},
			},
		}
	}

	return map[string]interface{}{
		"query":       query,
		"search_type": searchType,
		"results":     results,
		"cache_hit":   rand.Float32() < 0.35,
	}, nil
}