package real

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	UniProtBaseURL = "https://rest.uniprot.org"
)

type UniProtRetriever struct {
	client *http.Client
}

func NewUniProtRetriever() *UniProtRetriever {
	return &UniProtRetriever{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (u *UniProtRetriever) Name() string        { return "UniProt" }
func (u *UniProtRetriever) Category() string    { return "retriever" }
func (u *UniProtRetriever) Description() string { return "Query UniProt for protein sequences, annotations, variants, and functions" }

func (u *UniProtRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":              map[string]interface{}{"type": "string", "description": "Search query (gene name, accession, protein name, organism)"},
			"include_isoforms":   map[string]interface{}{"type": "boolean", "default": true},
			"include_variants":   map[string]interface{}{"type": "boolean", "default": true},
			"include_go":         map[string]interface{}{"type": "boolean", "default": true},
			"include_pathways":   map[string]interface{}{"type": "boolean", "default": true},
			"format":             map[string]interface{}{"type": "string", "enum": []string{"json", "fasta", "xml"}, "default": "json"},
			"limit":              map[string]interface{}{"type": "integer", "default": 20},
		},
		"required": []string{"query"},
	}
}

type UniProtResponse struct {
	Results []UniProtEntry `json:"results"`
}

type UniProtEntry struct {
	PrimaryAccession string `json:"primaryAccession"`
	UniProtKBCrossReferences []CrossReference `json:"uniProtKBCrossReferences"`
	ProteinDescription ProteinDescription `json:"proteinDescription"`
	Genes []Gene `json:"genes"`
	Organism Organism `json:"organism"`
	Sequence Sequence `json:"sequence"`
	Features []Feature `json:"features"`
	Comments []Comment `json:"comments"`
	Keywords []Keyword `json:"keywords"`
	GOAnnotations []GOAnnotation `json:"goAnnotations"`
	Pathways []Pathway `json:"pathways"`
}

type CrossReference struct {
	Database string `json:"database"`
	ID       string `json:"id"`
	Properties []Property `json:"properties"`
}

type Property struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ProteinDescription struct {
	RecommendedName RecommendedName `json:"recommendedName"`
	AlternativeNames []AlternativeName `json:"alternativeNames"`
}

type RecommendedName struct {
	FullName FullName `json:"fullName"`
}

type FullName struct {
	Value string `json:"value"`
}

type AlternativeName struct {
	FullName FullName `json:"fullName"`
}

type Gene struct {
	GeneName GeneName `json:"geneName"`
	Synonyms []Synonym `json:"synonyms"`
}

type GeneName struct {
	Value string `json:"value"`
}

type Synonym struct {
	Value string `json:"value"`
}

type Organism struct {
	ScientificName string `json:"scientificName"`
	CommonName     string `json:"commonName"`
	TaxonID        int    `json:"taxonId"`
}

type Sequence struct {
	Value   string `json:"value"`
	Length  int    `json:"length"`
	MolWeight int  `json:"molWeight"`
	CRC64   string `json:"crc64"`
}

type Feature struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Location    Location `json:"location"`
}

type Location struct {
	Start Start `json:"start"`
	End   End   `json:"end"`
}

type Start struct {
	Value int `json:"value"`
}

type End struct {
	Value int `json:"value"`
}

type Comment struct {
	Type string `json:"commentType"`
	Texts []Text `json:"texts"`
}

type Text struct {
	Value string `json:"value"`
}

type Keyword struct {
	Value string `json:"value"`
}

type GOAnnotation struct {
	GOID string `json:"goId"`
	Term string `json:"term"`
	Type string `json:"type"`
	Evidence string `json:"evidence"`
}

type Pathway struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Database    string `json:"database"`
	Reactions   []string `json:"reactions"`
}

func (u *UniProtRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	includeIsoforms := true
	if v, ok := input["include_isoforms"].(bool); ok {
		includeIsoforms = v
	}
	includeVariants := true
	if v, ok := input["include_variants"].(bool); ok {
		includeVariants = v
	}
	includeGO := true
	if v, ok := input["include_go"].(bool); ok {
		includeGO = v
	}
	includePathways := true
	if v, ok := input["include_pathways"].(bool); ok {
		includePathways = v
	}
	format := "json"
	if f, ok := input["format"].(string); ok {
		format = f
	}
	limit := 20
	if l, ok := input["limit"].(float64); ok {
		limit = int(l)
	}

	results, err := u.search(ctx, query, limit, format)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	entries := make([]map[string]interface{}, 0, len(results.Results))
	for _, entry := range results.Results {
		entries = append(entries, u.parseEntry(entry, includeIsoforms, includeVariants, includeGO, includePathways))
	}

	return map[string]interface{}{
		"query":     query,
		"results":   entries,
		"total_found": len(results.Results),
		"returned":  len(entries),
		"cache_hit": false,
	}, nil
}

func (u *UniProtRetriever) search(ctx context.Context, query string, limit int, format string) (*UniProtResponse, error) {
	searchQuery := query
	if !strings.Contains(query, ":") {
		searchQuery = fmt.Sprintf("(gene:%s OR protein_name:%s OR accession:%s)", query, query, query)
	}

	urlStr := fmt.Sprintf("%s/uniprotkb/search?query=%s&format=%s&size=%d", UniProtBaseURL, url.QueryEscape(searchQuery), format, limit)
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result UniProtResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (u *UniProtRetriever) parseEntry(entry UniProtEntry, includeIsoforms, includeVariants, includeGO, includePathways bool) map[string]interface{} {
	proteinName := entry.ProteinDescription.RecommendedName.FullName.Value
	geneName := ""
	if len(entry.Genes) > 0 {
		geneName = entry.Genes[0].GeneName.Value
	}

	organism := entry.Organism.ScientificName

	domains := make([]map[string]interface{}, 0)
	for _, feature := range entry.Features {
		if feature.Type == "Domain" || feature.Type == "Region" {
			domains = append(domains, map[string]interface{}{
				"type":        feature.Type,
				"description": feature.Description,
				"start":       feature.Location.Start.Value,
				"end":         feature.Location.End.Value,
			})
		}
	}

	variants := make([]map[string]interface{}, 0)
	if includeVariants {
		for _, feature := range entry.Features {
			if feature.Type == "Natural variant" || feature.Type == "Mutagenesis" {
				variants = append(variants, map[string]interface{}{
					"position":     feature.Location.Start.Value,
					"description": feature.Description,
					"type":        feature.Type,
				})
			}
		}
	}

	goTerms := make([]string, 0)
	if includeGO {
		for _, goa := range entry.GOAnnotations {
			goTerms = append(goTerms, fmt.Sprintf("%s (%s)", goa.GOID, goa.Term))
		}
	}

	pathways := make([]string, 0)
	if includePathways {
		for _, p := range entry.Pathways {
			pathways = append(pathways, fmt.Sprintf("%s (%s)", p.Name, p.Database))
		}
	}

	accessions := make([]string, 0)
	accessions = append(accessions, entry.PrimaryAccession)
	for _, cr := range entry.UniProtKBCrossReferences {
		if cr.Database == "Ensembl" || cr.Database == "RefSeq" {
			accessions = append(accessions, cr.ID)
		}
	}

	sequence := ""
	if len(entry.Sequence.Value) > 200 {
		sequence = entry.Sequence.Value[:200] + "..."
	} else {
		sequence = entry.Sequence.Value
	}

	return map[string]interface{}{
		"accession":       entry.PrimaryAccession,
		"accessions":      accessions,
		"gene_name":       geneName,
		"protein_name":    proteinName,
		"organism":        organism,
		"length":          entry.Sequence.Length,
		"sequence":        sequence,
		"mol_weight":      entry.Sequence.MolWeight,
		"domains":         domains,
		"variants":        variants,
		"go_terms":        goTerms,
		"pathways":        pathways,
		"cross_references": u.parseCrossReferences(entry.UniProtKBCrossReferences),
	}
}

func (u *UniProtRetriever) parseCrossReferences(refs []CrossReference) map[string][]string {
	result := make(map[string][]string)
	for _, ref := range refs {
		result[ref.Database] = append(result[ref.Database], ref.ID)
	}
	return result
}

func (u *UniProtRetriever) MockExecute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
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