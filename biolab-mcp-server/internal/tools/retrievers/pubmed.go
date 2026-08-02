package retrievers

import (
	"context"
	"math/rand"
	"time"

)

type PubMedRetriever struct{}

func NewPubMedRetriever() *PubMedRetriever { return &PubMedRetriever{} }

func (p *PubMedRetriever) Name() string        { return "PubMed" }
func (p *PubMedRetriever) Category() string    { return "retriever" }
func (p *PubMedRetriever) Description() string { return "Search PubMed for biomedical literature" }
func (p *PubMedRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":  map[string]interface{}{"type": "string", "description": "Search query"},
			"max_results": map[string]interface{}{"type": "integer", "default": 20},
		},
		"required": []string{"query"},
	}
}

func (p *PubMedRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	maxResults := 20
	if mr, ok := input["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	// Simulate API call latency
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(100+rand.Intn(300)) * time.Millisecond):
	}

	// Mock results
	count := 5 + rand.Intn(maxResults-5)
	papers := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		papers[i] = map[string]interface{}{
			"pmid":        rand.Intn(30000000) + 10000000,
			"title":       mockPaperTitle(query),
			"authors":     mockAuthors(),
			"journal":     mockJournal(),
			"pub_date":    mockPubDate(),
			"abstract":    mockAbstract(query),
			"doi":         mockDOI(),
			"url":         "https://pubmed.ncbi.nlm.nih.gov/" + mockPMID(),
		}
	}

	return map[string]interface{}{
		"query":       query,
		"total_found": count + rand.Intn(100),
		"returned":    count,
		"papers":      papers,
		"cache_hit":   rand.Float32() < 0.3,
	}, nil
}

func mockPaperTitle(query string) string {
	templates := []string{
		"Structural basis of %s mechanism",
		"Role of %s in drug resistance",
		"%s: implications for targeted therapy",
		"Molecular characterization of %s",
		"Clinical outcomes in %s patients",
	}
	return templates[rand.Intn(len(templates))]
}

func mockAuthors() []string {
	first := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis"}
	last := []string{"J.", "A.", "M.", "K.", "L.", "R.", "S.", "T."}
	count := 3 + rand.Intn(4)
	authors := make([]string, count)
	for i := range authors {
		authors[i] = first[rand.Intn(len(first))] + " " + last[rand.Intn(len(last))]
	}
	return authors
}

func mockJournal() string {
	journals := []string{"Nature", "Science", "Cell", "Nature Medicine", "Nature Biotechnology", "J. Biol. Chem.", "PNAS", "Mol. Cell"}
	return journals[rand.Intn(len(journals))]
}

func mockPubDate() string {
	year := 2020 + rand.Intn(7)
	month := 1 + rand.Intn(12)
	day := 1 + rand.Intn(28)
	return string(rune(year)) + "-" + string(rune(month)) + "-" + string(rune(day))
}

func mockAbstract(query string) string {
	return "We investigated the role of " + query + " using structural and biochemical approaches. Our findings demonstrate..."
}

func mockDOI() string {
	return "10.1038/s41586-2024-" + string(rune(10000+rand.Intn(90000)))
}

func mockPMID() string {
	return string(rune(10000000 + rand.Intn(20000000)))
}