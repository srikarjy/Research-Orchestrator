package real

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

const (
	ChEMBLBaseURL = "https://www.ebi.ac.uk/chembl/api/data"
)

type ChEMBLRetriever struct {
	client *http.Client
}

func NewChEMBLRetriever() *ChEMBLRetriever {
	return &ChEMBLRetriever{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *ChEMBLRetriever) Name() string        { return "ChEMBL" }
func (c *ChEMBLRetriever) Category() string    { return "retriever" }
func (c *ChEMBLRetriever) Description() string { return "Query ChEMBL for bioactive molecules, assays, targets, and activities" }

func (c *ChEMBLRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":         map[string]interface{}{"type": "string", "description": "Target name, compound name, SMILES, or ChEMBL ID"},
			"search_type":   map[string]interface{}{"type": "string", "enum": []string{"target", "compound", "assay", "activity", "similarity", "substructure"}, "default": "target"},
			"threshold":     map[string]interface{}{"type": "number", "default": 0.7},
			"limit":         map[string]interface{}{"type": "integer", "default": 20},
			"target_id":     map[string]interface{}{"type": "string", "description": "ChEMBL target ID for activity search"},
		},
		"required": []string{"query"},
	}
}

type ChEMBLResponse struct {
	PageMeta PageMeta `json:"page_meta"`
	Compounds []ChEMBLCompound `json:"molecules,omitempty"`
	Targets []ChEMBLTarget `json:"targets,omitempty"`
	Assays []ChEMBLAssay `json:"assays,omitempty"`
	Activities []ChEMBLActivity `json:"activities,omitempty"`
}

type PageMeta struct {
	TotalCount int `json:"total_count"`
}

type ChEMBLCompound struct {
	ChEMBLID      string  `json:"molecule_chembl_id"`
	SMILES        string  `json:"molecule_structures,omitempty"`
	MolecularWeight float64 `json:"molecular_weight,omitempty"`
	ALogP         float64 `json:"alogp,omitempty"`
	PrefName      string  `json:"pref_name,omitempty"`
	MaxPhase      float64 `json:"max_phase,omitempty"`
}

type ChEMBLTarget struct {
	TargetChEMBLID string `json:"target_chembl_id"`
	TargetName     string `json:"pref_name"`
	Organism       string `json:"organism"`
	TargetType     string `json:"target_type"`
	ProteinAccession string `json:"accession,omitempty"`
}

type ChEMBLAssay struct {
	AssayChEMBLID string `json:"assay_chembl_id"`
	Description   string `json:"description"`
	AssayType     string `json:"assay_type"`
	Organism      string `json:"organism"`
}

type ChEMBLActivity struct {
	ActivityID     string  `json:"activity_id"`
	AssayChEMBLID  string  `json:"assay_chembl_id"`
	TargetChEMBLID string  `json:"target_chembl_id"`
	MoleculeChEMBLID string `json:"molecule_chembl_id"`
	StandardType   string  `json:"standard_type"`
	StandardValue  float64 `json:"standard_value"`
	StandardUnits  string  `json:"standard_units"`
	StandardRelation string `json:"standard_relation"`
	PChEMBLValue   float64 `json:"pchembl_value,omitempty"`
}

func (c *ChEMBLRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	searchType := "target"
	if st, ok := input["search_type"].(string); ok {
		searchType = st
	}
	limit := 20
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}
	threshold := 0.7
	if t, ok := input["threshold"].(float64); ok {
		threshold = t
	}
	targetID := ""
	if tid, ok := input["target_id"].(string); ok {
		targetID = tid
	}

	var results map[string]interface{}
	var err error

	switch searchType {
	case "compound":
		results, err = c.searchCompounds(ctx, query, limit)
	case "assay":
		results, err = c.searchAssays(ctx, query, limit)
	case "activity":
		results, err = c.searchActivities(ctx, targetID, query, limit)
	case "similarity":
		results, err = c.similaritySearch(ctx, query, threshold, limit)
	case "substructure":
		results, err = c.substructureSearch(ctx, query, limit)
	default:
		results, err = c.searchTargets(ctx, query, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results["query"] = query
	results["search_type"] = searchType
	if targetID != "" {
		results["target_id"] = targetID
	}
	results["cache_hit"] = false

	return results, nil
}

func (c *ChEMBLRetriever) searchTargets(ctx context.Context, query string, limit int) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/target.json?q=%s&limit=%d&format=json", ChEMBLBaseURL, url.QueryEscape(query), limit)
	resp, err := c.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChEMBLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	targets := make([]map[string]interface{}, 0, len(result.Targets))
	for _, t := range result.Targets {
		targets = append(targets, map[string]interface{}{
			"target_id":       t.TargetChEMBLID,
			"target_name":     t.TargetName,
			"organism":        t.Organism,
			"target_type":     t.TargetType,
			"protein_accession": t.ProteinAccession,
		})
	}

	return map[string]interface{}{
		"results":     targets,
		"total_found": result.PageMeta.TotalCount,
		"returned":    len(targets),
	}, nil
}

func (c *ChEMBLRetriever) searchCompounds(ctx context.Context, query string, limit int) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/molecule.json?molecule_synonyms__icontains=%s&limit=%d&format=json", ChEMBLBaseURL, url.QueryEscape(query), limit)
	resp, err := c.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChEMBLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	compounds := make([]map[string]interface{}, 0, len(result.Compounds))
	for _, cmp := range result.Compounds {
		compounds = append(compounds, map[string]interface{}{
			"chembl_id":         cmp.ChEMBLID,
			"smiles":            cmp.SMILES,
			"name":              cmp.PrefName,
			"molecular_weight":  cmp.MolecularWeight,
			"logp":              cmp.ALogP,
			"max_phase":         cmp.MaxPhase,
		})
	}

	return map[string]interface{}{
		"results":     compounds,
		"total_found": result.PageMeta.TotalCount,
		"returned":    len(compounds),
	}, nil
}

func (c *ChEMBLRetriever) searchAssays(ctx context.Context, query string, limit int) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/assay.json?description__icontains=%s&limit=%d&format=json", ChEMBLBaseURL, url.QueryEscape(query), limit)
	resp, err := c.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChEMBLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	assays := make([]map[string]interface{}, 0, len(result.Assays))
	for _, a := range result.Assays {
		assays = append(assays, map[string]interface{}{
			"assay_id":       a.AssayChEMBLID,
			"description":    a.Description,
			"type":           a.AssayType,
			"organism":       a.Organism,
		})
	}

	return map[string]interface{}{
		"results":     assays,
		"total_found": result.PageMeta.TotalCount,
		"returned":    len(assays),
	}, nil
}

func (c *ChEMBLRetriever) searchActivities(ctx context.Context, targetID, query string, limit int) (map[string]interface{}, error) {
	if targetID == "" {
		return c.searchActivitiesByCompound(ctx, query, limit)
	}

	urlStr := fmt.Sprintf("%s/activity.json?target_chembl_id=%s&limit=%d&format=json", ChEMBLBaseURL, url.QueryEscape(targetID), limit)
	resp, err := c.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChEMBLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	activities := make([]map[string]interface{}, 0, len(result.Activities))
	for _, act := range result.Activities {
		activities = append(activities, map[string]interface{}{
			"activity_id":      act.ActivityID,
			"assay_id":         act.AssayChEMBLID,
			"target_id":        act.TargetChEMBLID,
			"molecule_id":      act.MoleculeChEMBLID,
			"standard_type":    act.StandardType,
			"standard_value":   act.StandardValue,
			"standard_units":   act.StandardUnits,
			"standard_relation": act.StandardRelation,
			"pchembl_value":    act.PChEMBLValue,
		})
	}

	return map[string]interface{}{
		"results":     activities,
		"total_found": result.PageMeta.TotalCount,
		"returned":    len(activities),
	}, nil
}

func (c *ChEMBLRetriever) searchActivitiesByCompound(ctx context.Context, query string, limit int) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/activity.json?molecule_chembl_id__molecule_synonyms__icontains=%s&limit=%d&format=json", ChEMBLBaseURL, url.QueryEscape(query), limit)
	resp, err := c.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChEMBLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	activities := make([]map[string]interface{}, 0, len(result.Activities))
	for _, act := range result.Activities {
		activities = append(activities, map[string]interface{}{
			"activity_id":      act.ActivityID,
			"assay_id":         act.AssayChEMBLID,
			"target_id":        act.TargetChEMBLID,
			"molecule_id":      act.MoleculeChEMBLID,
			"standard_type":    act.StandardType,
			"standard_value":   act.StandardValue,
			"standard_units":   act.StandardUnits,
			"standard_relation": act.StandardRelation,
			"pchembl_value":    act.PChEMBLValue,
		})
	}

	return map[string]interface{}{
		"results":     activities,
		"total_found": result.PageMeta.TotalCount,
		"returned":    len(activities),
	}, nil
}

func (c *ChEMBLRetriever) similaritySearch(ctx context.Context, smiles string, threshold float64, limit int) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/similarity/%s/%d.json?limit=%d&format=json", ChEMBLBaseURL, url.QueryEscape(smiles), int(threshold*100), limit)
	resp, err := c.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChEMBLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	compounds := make([]map[string]interface{}, 0, len(result.Compounds))
	for _, cmp := range result.Compounds {
		compounds = append(compounds, map[string]interface{}{
			"chembl_id":         cmp.ChEMBLID,
			"smiles":            cmp.SMILES,
			"name":              cmp.PrefName,
			"molecular_weight":  cmp.MolecularWeight,
			"logp":              cmp.ALogP,
		})
	}

	return map[string]interface{}{
		"results":     compounds,
		"total_found": result.PageMeta.TotalCount,
		"returned":    len(compounds),
		"threshold":   threshold,
	}, nil
}

func (c *ChEMBLRetriever) substructureSearch(ctx context.Context, smiles string, limit int) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/substructure/%s.json?limit=%d&format=json", ChEMBLBaseURL, url.QueryEscape(smiles), limit)
	resp, err := c.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ChEMBLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	compounds := make([]map[string]interface{}, 0, len(result.Compounds))
	for _, cmp := range result.Compounds {
		compounds = append(compounds, map[string]interface{}{
			"chembl_id":         cmp.ChEMBLID,
			"smiles":            cmp.SMILES,
			"name":              cmp.PrefName,
			"molecular_weight":  cmp.MolecularWeight,
			"logp":              cmp.ALogP,
		})
	}

	return map[string]interface{}{
		"results":     compounds,
		"total_found": result.PageMeta.TotalCount,
		"returned":    len(compounds),
	}, nil
}

func (c *ChEMBLRetriever) doRequest(ctx context.Context, urlStr string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return c.client.Do(req)
}

func (c *ChEMBLRetriever) MockExecute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
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
				"chembl_id":        "CHEMBL123456",
				"smiles":           "CCCS(=O)(=O)Nc1ccc(F)c(C(=O)c2c[nH]c3ncc(-c4ccc(Cl)cc4)cc23)c1F",
				"name":             "Vemurafenib",
				"molecular_weight": 489.9,
				"logp":             3.2,
				"targets":          []string{"BRAF V600E"},
				"assays":           []string{"Biochemical IC50: 31 nM", "Cellular IC50: 15 nM"},
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
	case "activity":
		results = []map[string]interface{}{
			{
				"activity_id":      "ACT_123",
				"assay_id":         "CHEMBL_ASSAY_123",
				"target_id":        "CHEMBL_TARGET_4024",
				"molecule_id":      "CHEMBL123456",
				"standard_type":    "IC50",
				"standard_value":   31.0,
				"standard_units":   "nM",
				"standard_relation": "=",
				"pchembl_value":    7.5,
			},
		}
	default:
		results = []map[string]interface{}{
			{
				"target_id":       "CHEMBL_TARGET_4024",
				"target_name":     "BRAF",
				"organism":        "Homo sapiens",
				"target_type":     "SINGLE PROTEIN",
				"compounds":       1247,
				"assays":          89,
				"known_drugs":     []string{"Vemurafenib", "Dabrafenib", "Encorafenib"},
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