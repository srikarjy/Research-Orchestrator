package retrievers

import (
	"context"
	"math/rand"
	"time"

)

type BindingDBRetriever struct{}

func NewBindingDBRetriever() *BindingDBRetriever { return &BindingDBRetriever{} }

func (b *BindingDBRetriever) Name() string        { return "BindingDB" }
func (b *BindingDBRetriever) Category() string    { return "retriever" }
func (b *BindingDBRetriever) Description() string { return "Query BindingDB for protein-ligand binding affinities" }
func (b *BindingDBRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"target_name":  map[string]interface{}{"type": "string", "description": "Protein target name"},
			"ligand_name":  map[string]interface{}{"type": "string", "description": "Ligand name or SMILES"},
			"affinity_type": map[string]interface{}{"type": "string", "enum": []string{"Ki", "IC50", "Kd", "EC50"}, "default": "Ki"},
		},
	}
}

func (b *BindingDBRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	targetName, _ := input["target_name"].(string)
	ligandName, _ := input["ligand_name"].(string)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(100+rand.Intn(250)) * time.Millisecond):
	}

	results := []map[string]interface{}{
		{
			"bindingdb_id":    "BDBM50012345",
			"target_name":     "BRAF",
			"target_source":   "UniProt:P15056",
			"ligand_name":     "Vemurafenib",
			"ligand_smiles":   "CCCS(=O)(=O)Nc1ccc(F)c(C(=O)c2c[nH]c3ncc(-c4ccc(Cl)cc4)cc23)c1F",
			"affinity_type":   "Ki",
			"affinity_value":  0.6,
			"affinity_unit":   "nM",
			"assay_type":      "Biochemical",
			"conditions":      "pH 7.5, 10mM MgCl2",
			"pubmed_id":       24567890,
		},
		{
			"bindingdb_id":    "BDBM50012346",
			"target_name":     "BRAF V600E",
			"target_source":   "UniProt:P15056 (V600E)",
			"ligand_name":     "Vemurafenib",
			"ligand_smiles":   "CCCS(=O)(=O)Nc1ccc(F)c(C(=O)c2c[nH]c3ncc(-c4ccc(Cl)cc4)cc23)c1F",
			"affinity_type":   "Ki",
			"affinity_value":  31,
			"affinity_unit":   "nM",
			"assay_type":      "Biochemical",
			"pubmed_id":       24567890,
		},
		{
			"bindingdb_id":    "BDBM50012347",
			"target_name":     "BRAF V600E",
			"target_source":   "UniProt:P15056 (V600E)",
			"ligand_name":     "Dabrafenib",
			"ligand_smiles":   "CC(C)N1CCN(CC1)C2=NC(=NC3=C2C=CC(=C3)Cl)NC4=NC=CC(=N4)C5=CC=CC=C5F",
			"affinity_type":   "Ki",
			"affinity_value":  0.5,
			"affinity_unit":   "nM",
			"assay_type":      "Biochemical",
			"pubmed_id":       23515885,
		},
	}

	return map[string]interface{}{
		"target_name":  targetName,
		"ligand_name":  ligandName,
		"results":      results,
		"cache_hit":    rand.Float32() < 0.35,
	}, nil
}