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
	ReactomeBaseURL = "https://reactome.org/ContentService"
)

type ReactomeRetriever struct {
	client *http.Client
}

func NewReactomeRetriever() *ReactomeRetriever {
	return &ReactomeRetriever{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *ReactomeRetriever) Name() string        { return "Reactome" }
func (r *ReactomeRetriever) Category() string    { return "retriever" }
func (r *ReactomeRetriever) Description() string { return "Query Reactome for pathways, reactions, and biological events" }

func (r *ReactomeRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":           map[string]interface{}{"type": "string", "description": "Pathway name, gene name, UniProt ID, or Reactome ID"},
			"search_type":     map[string]interface{}{"type": "string", "enum": []string{"pathway", "reaction", "protein", "complex", "event", "species"}, "default": "pathway"},
			"species":         map[string]interface{}{"type": "string", "default": "Homo sapiens"},
			"include_diagram": map[string]interface{}{"type": "boolean", "default": true},
			"include_hierarchy": map[string]interface{}{"type": "boolean", "default": false},
		},
		"required": []string{"query"},
	}
}

type ReactomePathway struct {
	DBID          int    `json:"dbId"`
	DisplayName   string `json:"displayName"`
	StID          string `json:"stId"`
	SpeciesName   string `json:"speciesName"`
	IsInDisease   bool   `json:"isInDisease"`
	Summation     []ReactomeSummation `json:"summation"`
	ComponentOf   []ReactomeComponent `json:"componentOf"`
	HasComponent  []ReactomeComponent `json:"hasComponent"`
	HasEvent      []ReactomeEvent     `json:"hasEvent"`
	LiteratureReferences []ReactomeReference `json:"literatureReferences"`
}

type ReactomeSummation struct {
	Text      string `json:"text"`
	PubMedIDs []int  `json:"literatureReferences"`
}

type ReactomeComponent struct {
	DBID        int    `json:"dbId"`
	DisplayName string `json:"displayName"`
	StID        string `json:"stId"`
	SchemaClass string `json:"schemaClass"`
}

type ReactomeEvent struct {
	DBID        int    `json:"dbId"`
	DisplayName string `json:"displayName"`
	StID        string `json:"stId"`
	SchemaClass string `json:"schemaClass"`
}

type ReactomeReference struct {
	DBID       int    `json:"dbId"`
	PubMedID   int    `json:"pubMedIdentifier"`
	Title      string `json:"title"`
	Authors    string `json:"authors"`
	Journal    string `json:"journal"`
	Year       int    `json:"year"`
}

type ReactomeSearchResult struct {
	TotalCount int                `json:"totalCount"`
	Results    []ReactomeSearchItem `json:"results"`
}

type ReactomeSearchItem struct {
	DBID        int    `json:"dbId"`
	DisplayName string `json:"displayName"`
	StID        string `json:"stId"`
	SchemaClass string `json:"schemaClass"`
	SpeciesName string `json:"speciesName"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

func (r *ReactomeRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	searchType := "pathway"
	if st, ok := input["search_type"].(string); ok {
		searchType = st
	}
	species := "Homo sapiens"
	if sp, ok := input["species"].(string); ok {
		species = sp
	}
	includeDiagram := true
	if id, ok := input["include_diagram"].(bool); ok {
		includeDiagram = id
	}
	includeHierarchy := false
	if ih, ok := input["include_hierarchy"].(bool); ok {
		includeHierarchy = ih
	}

	var results map[string]interface{}
	var err error

	switch searchType {
	case "reaction":
		results, err = r.searchReactions(ctx, query, species)
	case "protein":
		results, err = r.searchProteins(ctx, query, species)
	case "complex":
		results, err = r.searchComplexes(ctx, query, species)
	case "event":
		results, err = r.searchEvents(ctx, query, species)
	case "species":
		results, err = r.getSpecies(ctx)
	default:
		results, err = r.searchPathways(ctx, query, species, includeDiagram, includeHierarchy)
	}

	if err != nil {
		return nil, fmt.Errorf("Reactome search failed: %w", err)
	}

	results["query"] = query
	results["search_type"] = searchType
	results["species"] = species
	results["cache_hit"] = false

	return results, nil
}

func (r *ReactomeRetriever) searchPathways(ctx context.Context, query, species string, includeDiagram, includeHierarchy bool) (map[string]interface{}, error) {
	searchQuery := url.QueryEscape(query)
	urlStr := fmt.Sprintf("%s/data/query?q=%s&type=pathway&species=%s&cluster=true", ReactomeBaseURL, searchQuery, url.QueryEscape(species))
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult ReactomeSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	pathways := make([]map[string]interface{}, 0, len(searchResult.Results))
	for _, item := range searchResult.Results {
		pathway := map[string]interface{}{
			"db_id":        item.DBID,
			"st_id":        item.StID,
			"name":         item.DisplayName,
			"species":      item.SpeciesName,
			"description":  item.Description,
			"url":          item.URL,
			"schema_class": item.SchemaClass,
		}

		if includeDiagram {
			pathway["diagram_url"] = fmt.Sprintf("%s/data/diagram/%s", ReactomeBaseURL, item.StID)
			pathway["diagram_image_url"] = fmt.Sprintf("%s/data/diagram/%s.png", ReactomeBaseURL, item.StID)
		}

		if includeHierarchy {
			hierarchy, _ := r.getPathwayHierarchy(ctx, item.DBID)
			pathway["hierarchy"] = hierarchy
		}

		pathways = append(pathways, pathway)
	}

	return map[string]interface{}{
		"results":     pathways,
		"total_found": searchResult.TotalCount,
		"returned":    len(pathways),
	}, nil
}

func (r *ReactomeRetriever) getPathwayHierarchy(ctx context.Context, dbID int) ([]map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/data/pathway/%d/hierarchy", ReactomeBaseURL, dbID)
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var hierarchy []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&hierarchy); err != nil {
		return nil, err
	}
	return hierarchy, nil
}

func (r *ReactomeRetriever) getPathwayDetail(ctx context.Context, dbID int) (*ReactomePathway, error) {
	urlStr := fmt.Sprintf("%s/data/pathway/%d", ReactomeBaseURL, dbID)
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pathway ReactomePathway
	if err := json.NewDecoder(resp.Body).Decode(&pathway); err != nil {
		return nil, err
	}
	return &pathway, nil
}

func (r *ReactomeRetriever) searchReactions(ctx context.Context, query, species string) (map[string]interface{}, error) {
	searchQuery := url.QueryEscape(query)
	urlStr := fmt.Sprintf("%s/data/query?q=%s&type=reaction&species=%s", ReactomeBaseURL, searchQuery, url.QueryEscape(species))
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult ReactomeSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	reactions := make([]map[string]interface{}, 0, len(searchResult.Results))
	for _, item := range searchResult.Results {
		reactions = append(reactions, map[string]interface{}{
			"db_id":        item.DBID,
			"st_id":        item.StID,
			"name":         item.DisplayName,
			"species":      item.SpeciesName,
			"description":  item.Description,
			"url":          item.URL,
			"schema_class": item.SchemaClass,
		})
	}

	return map[string]interface{}{
		"results":     reactions,
		"total_found": searchResult.TotalCount,
		"returned":    len(reactions),
	}, nil
}

func (r *ReactomeRetriever) searchProteins(ctx context.Context, query, species string) (map[string]interface{}, error) {
	searchQuery := url.QueryEscape(query)
	urlStr := fmt.Sprintf("%s/data/query?q=%s&type=protein&species=%s", ReactomeBaseURL, searchQuery, url.QueryEscape(species))
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult ReactomeSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	proteins := make([]map[string]interface{}, 0, len(searchResult.Results))
	for _, item := range searchResult.Results {
		proteins = append(proteins, map[string]interface{}{
			"db_id":        item.DBID,
			"st_id":        item.StID,
			"name":         item.DisplayName,
			"species":      item.SpeciesName,
			"description":  item.Description,
			"url":          item.URL,
			"schema_class": item.SchemaClass,
		})
	}

	return map[string]interface{}{
		"results":     proteins,
		"total_found": searchResult.TotalCount,
		"returned":    len(proteins),
	}, nil
}

func (r *ReactomeRetriever) searchComplexes(ctx context.Context, query, species string) (map[string]interface{}, error) {
	searchQuery := url.QueryEscape(query)
	urlStr := fmt.Sprintf("%s/data/query?q=%s&type=complex&species=%s", ReactomeBaseURL, searchQuery, url.QueryEscape(species))
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult ReactomeSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	complexes := make([]map[string]interface{}, 0, len(searchResult.Results))
	for _, item := range searchResult.Results {
		complexes = append(complexes, map[string]interface{}{
			"db_id":        item.DBID,
			"st_id":        item.StID,
			"name":         item.DisplayName,
			"species":      item.SpeciesName,
			"description":  item.Description,
			"url":          item.URL,
			"schema_class": item.SchemaClass,
		})
	}

	return map[string]interface{}{
		"results":     complexes,
		"total_found": searchResult.TotalCount,
		"returned":    len(complexes),
	}, nil
}

func (r *ReactomeRetriever) searchEvents(ctx context.Context, query, species string) (map[string]interface{}, error) {
	searchQuery := url.QueryEscape(query)
	urlStr := fmt.Sprintf("%s/data/query?q=%s&type=event&species=%s", ReactomeBaseURL, searchQuery, url.QueryEscape(species))
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResult ReactomeSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, err
	}

	events := make([]map[string]interface{}, 0, len(searchResult.Results))
	for _, item := range searchResult.Results {
		events = append(events, map[string]interface{}{
			"db_id":        item.DBID,
			"st_id":        item.StID,
			"name":         item.DisplayName,
			"species":      item.SpeciesName,
			"description":  item.Description,
			"url":          item.URL,
			"schema_class": item.SchemaClass,
		})
	}

	return map[string]interface{}{
		"results":     events,
		"total_found": searchResult.TotalCount,
		"returned":    len(events),
	}, nil
}

func (r *ReactomeRetriever) getSpecies(ctx context.Context) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/data/species", ReactomeBaseURL)
	resp, err := r.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var species []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&species); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"results":     species,
		"total_found": len(species),
		"returned":    len(species),
	}, nil
}

func (r *ReactomeRetriever) doRequest(ctx context.Context, urlStr string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return r.client.Do(req)
}

func (r *ReactomeRetriever) MockExecute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	searchType := "pathway"
	if st, ok := input["search_type"].(string); ok {
		searchType = st
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(100+rand.Intn(300)) * time.Millisecond):
	}

	var results []map[string]interface{}
	switch searchType {
	case "pathway":
		results = []map[string]interface{}{
			{
				"db_id":        1640170,
				"st_id":        "R-HSA-5673001",
				"name":         "MAPK family signaling cascades",
				"species":      "Homo sapiens",
				"description":  "The MAPK signaling cascades mediate diverse cellular responses to extracellular stimuli",
				"url":          "https://reactome.org/content/detail/R-HSA-5673001",
				"schema_class": "Pathway",
				"diagram_url":  "https://reactome.org/ContentService/data/diagram/R-HSA-5673001",
				"diagram_image_url": "https://reactome.org/ContentService/data/diagram/R-HSA-5673001.png",
				"literature":   []string{"PMID:15653506", "PMID:15933720"},
			},
			{
				"db_id":        1640171,
				"st_id":        "R-HSA-5683057",
				"name":         "RAF/MAP kinase cascade",
				"species":      "Homo sapiens",
				"description":  "RAF/MAP kinase cascade activation",
				"url":          "https://reactome.org/content/detail/R-HSA-5683057",
				"schema_class": "Pathway",
				"diagram_url":  "https://reactome.org/ContentService/data/diagram/R-HSA-5683057",
				"diagram_image_url": "https://reactome.org/ContentService/data/diagram/R-HSA-5683057.png",
			},
		}
	case "reaction":
		results = []map[string]interface{}{
			{
				"db_id":        123456,
				"st_id":        "R-HSA-123456",
				"name":         "BRAF phosphorylates MAP2K1",
				"species":      "Homo sapiens",
				"description":  "BRAF kinase phosphorylates MAP2K1/MEK1",
				"schema_class": "Reaction",
			},
		}
	case "protein":
		results = []map[string]interface{}{
			{
				"db_id":        789012,
				"st_id":        "R-HSA-789012",
				"name":         "BRAF [cytosol]",
				"species":      "Homo sapiens",
				"description":  "Serine/threonine-protein kinase B-raf",
				"schema_class": "PhysicalEntity",
				"uniprot_id":   "P15056",
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