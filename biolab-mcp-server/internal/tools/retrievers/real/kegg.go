package real

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	KEGGBaseURL = "https://rest.kegg.jp"
)

type KEGGRetriever struct {
	client *http.Client
}

func NewKEGGRetriever() *KEGGRetriever {
	return &KEGGRetriever{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (k *KEGGRetriever) Name() string        { return "KEGG" }
func (k *KEGGRetriever) Category() string    { return "retriever" }
func (k *KEGGRetriever) Description() string { return "Query KEGG for pathways, reactions, compounds, and genes" }

func (k *KEGGRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":        map[string]interface{}{"type": "string", "description": "Pathway ID, gene name, compound ID, or search term"},
			"search_type":  map[string]interface{}{"type": "string", "enum": []string{"pathway", "gene", "compound", "reaction", "module", "list"}, "default": "pathway"},
			"organism":     map[string]interface{}{"type": "string", "default": "hsa"},
			"include_map":  map[string]interface{}{"type": "boolean", "default": true},
		},
		"required": []string{"query"},
	}
}

type KEGGPathway struct {
	Entry       string `xml:"entry"`
	Name        string `xml:"name"`
	Description string `xml:"description"`
	Class       string `xml:"class"`
	Image       string `xml:"image"`
	URL         string `xml:"url"`
}

type KEGGPathwayDetail struct {
	Entry       string          `xml:"entry"`
	Name        string          `xml:"name"`
	Description string          `xml:"description"`
	Class       string          `xml:"class"`
	Image       string          `xml:"image"`
	URL         string          `xml:"url"`
	Components  []KEGGComponent `xml:"graphics>component"`
}

type KEGGComponent struct {
	Type   string `xml:"type,attr"`
	Name   string `xml:"name,attr"`
	FGColor string `xml:"fgcolor,attr"`
	BGColor string `xml:"bgcolor,attr"`
	Graphics []KEGGGraphics `xml:"graphics"`
}

type KEGGGraphics struct {
	Name   string `xml:"name,attr"`
	X      int    `xml:"x,attr"`
	Y      int    `xml:"y,attr"`
	Width  int    `xml:"width,attr"`
	Height int    `xml:"height,attr"`
	Type   string `xml:"type,attr"`
}

func (k *KEGGRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	searchType := "pathway"
	if st, ok := input["search_type"].(string); ok {
		searchType = st
	}
	organism := "hsa"
	if org, ok := input["organism"].(string); ok {
		organism = org
	}
	includeMap := true
	if im, ok := input["include_map"].(bool); ok {
		includeMap = im
	}

	var results map[string]interface{}
	var err error

	switch searchType {
	case "gene":
		results, err = k.searchGene(ctx, query, organism)
	case "compound":
		results, err = k.searchCompound(ctx, query)
	case "reaction":
		results, err = k.searchReaction(ctx, query)
	case "module":
		results, err = k.searchModule(ctx, query)
	case "list":
		results, err = k.listPathways(ctx, organism)
	default:
		results, err = k.getPathway(ctx, query, organism, includeMap)
	}

	if err != nil {
		return nil, fmt.Errorf("KEGG search failed: %w", err)
	}

	results["query"] = query
	results["search_type"] = searchType
	results["organism"] = organism
	results["cache_hit"] = false

	return results, nil
}

func (k *KEGGRetriever) getPathway(ctx context.Context, pathwayID, organism string, includeMap bool) (map[string]interface{}, error) {
	pathwayID = strings.TrimPrefix(pathwayID, "path:")
	pathwayID = strings.TrimPrefix(pathwayID, "map")

	urlStr := fmt.Sprintf("%s/get/%s%s", KEGGBaseURL, organism, pathwayID)
	resp, err := k.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var pathway KEGGPathwayDetail
	if err := xml.NewDecoder(resp.Body).Decode(&pathway); err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"pathway_id":   pathway.Entry,
		"name":         pathway.Name,
		"description":  pathway.Description,
		"class":        pathway.Class,
		"url":          pathway.URL,
		"image_url":    fmt.Sprintf("%s/%s.png", KEGGBaseURL, pathway.Entry),
	}

	if includeMap {
		kgmlURL := fmt.Sprintf("%s/kgml/%s%s", KEGGBaseURL, organism, pathwayID)
		kgmlResp, err := k.doRequest(ctx, kgmlURL)
		if err == nil {
			defer kgmlResp.Body.Close()
			var kgml KEGGPathwayDetail
			if err := xml.NewDecoder(kgmlResp.Body).Decode(&kgml); err == nil {
				result["kgml"] = k.parseKGML(kgml)
			}
		}
	}

	return map[string]interface{}{
		"results": []map[string]interface{}{result},
		"total_found": 1,
		"returned": 1,
	}, nil
}

func (k *KEGGRetriever) parseKGML(pathway KEGGPathwayDetail) map[string]interface{} {
	entries := make([]map[string]interface{}, 0)
	relations := make([]map[string]interface{}, 0)

	for _, comp := range pathway.Components {
		for _, g := range comp.Graphics {
			entry := map[string]interface{}{
				"name":   g.Name,
				"type":   comp.Type,
				"x":      g.X,
				"y":      g.Y,
				"width":  g.Width,
				"height": g.Height,
				"fgcolor": comp.FGColor,
				"bgcolor": comp.BGColor,
			}
			entries = append(entries, entry)
		}
	}

	return map[string]interface{}{
		"entries":   entries,
		"relations": relations,
	}
}

func (k *KEGGRetriever) searchGene(ctx context.Context, query, organism string) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/find/genes/%s/%s", KEGGBaseURL, organism, url.QueryEscape(query))
	resp, err := k.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return k.parseFindResponse(resp.Body, "genes"), nil
}

func (k *KEGGRetriever) searchCompound(ctx context.Context, query string) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/find/compound/%s", KEGGBaseURL, url.QueryEscape(query))
	resp, err := k.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return k.parseFindResponse(resp.Body, "compounds"), nil
}

func (k *KEGGRetriever) searchReaction(ctx context.Context, query string) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/find/reaction/%s", KEGGBaseURL, url.QueryEscape(query))
	resp, err := k.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return k.parseFindResponse(resp.Body, "reactions"), nil
}

func (k *KEGGRetriever) searchModule(ctx context.Context, query string) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/find/module/%s", KEGGBaseURL, url.QueryEscape(query))
	resp, err := k.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return k.parseFindResponse(resp.Body, "modules"), nil
}

func (k *KEGGRetriever) listPathways(ctx context.Context, organism string) (map[string]interface{}, error) {
	urlStr := fmt.Sprintf("%s/list/pathway/%s", KEGGBaseURL, organism)
	resp, err := k.doRequest(ctx, urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return k.parseFindResponse(resp.Body, "pathways"), nil
}

func (k *KEGGRetriever) parseFindResponse(body io.Reader, resultType string) map[string]interface{} {
	scanner := bufio.NewScanner(body)
	results := make([]map[string]interface{}, 0)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) >= 2 {
			id := strings.TrimSpace(parts[0])
			name := strings.TrimSpace(parts[1])
			results = append(results, map[string]interface{}{
				"id":   id,
				"name": name,
			})
		}
	}

	return map[string]interface{}{
		"results":     results,
		"total_found": len(results),
		"returned":    len(results),
	}
}

func (k *KEGGRetriever) doRequest(ctx context.Context, urlStr string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	return k.client.Do(req)
}

func (k *KEGGRetriever) MockExecute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
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
				"pathway_id":   "hsa04010",
				"name":         "MAPK signaling pathway",
				"description":  "Mitogen-activated protein kinase signaling pathway",
				"class":        "Signal transduction",
				"url":          "https://www.kegg.jp/kegg-bin/show_pathway?hsa04010",
				"image_url":    "https://www.kegg.jp/kegg/pathway/hsa/hsa04010.png",
				"genes":        []string{"BRAF", "MAP2K1", "MAPK1", "MAPK3", "RAF1", "HRAS", "KRAS", "NRAS"},
				"compounds":    []string{"ATP", "ADP", "GTP", "GDP"},
			},
			{
				"pathway_id":   "hsa04110",
				"name":         "Cell cycle",
				"description":  "Cell cycle regulation",
				"class":        "Cell growth and death",
				"url":          "https://www.kegg.jp/kegg-bin/show_pathway?hsa04110",
				"image_url":    "https://www.kegg.jp/kegg/pathway/hsa/hsa04110.png",
				"genes":        []string{"CDK1", "CDK2", "CCNA2", "CCNB1", "TP53", "RB1"},
			},
		}
	case "gene":
		results = []map[string]interface{}{
			{"id": "hsa:673", "name": "BRAF - B-Raf proto-oncogene, serine/threonine kinase"},
			{"id": "hsa:5604", "name": "MAP2K1 - Mitogen-activated protein kinase kinase 1"},
			{"id": "hsa:5594", "name": "MAPK1 - Mitogen-activated protein kinase 1"},
		}
	case "compound":
		results = []map[string]interface{}{
			{"id": "C00002", "name": "ATP - Adenosine triphosphate"},
			{"id": "C00008", "name": "ADP - Adenosine diphosphate"},
		}
	}

	return map[string]interface{}{
		"query":       query,
		"search_type": searchType,
		"results":     results,
		"cache_hit":   rand.Float32() < 0.3,
	}, nil
}