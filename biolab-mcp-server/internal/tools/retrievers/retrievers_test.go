package retrievers

import (
	"context"
	"math/rand"
	"time"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPubMedRetriever struct{}

func (m *mockPubMedRetriever) Name() string        { return "PubMed" }
func (m *mockPubMedRetriever) Category() string    { return "retriever" }
func (m *mockPubMedRetriever) Description() string { return "Mock PubMed" }
func (m *mockPubMedRetriever) InputSchema() map[string]any {
	return map[string]any{"type": "object"}
}
func (m *mockPubMedRetriever) Execute(ctx context.Context, input map[string]any) (map[string]any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(10+rand.Intn(50)) * time.Millisecond):
	}
	count := 5 + rand.Intn(5)
	papers := make([]map[string]any, count)
	for i := 0; i < count; i++ {
		papers[i] = map[string]any{
			"pmid":     rand.Intn(30000000) + 10000000,
			"title":    "Test paper " + string(rune(i)),
			"abstract": "Test abstract",
		}
	}
	return map[string]any{
		"query":       input["query"],
		"total_found": count + rand.Intn(100),
		"returned":    count,
		"papers":      papers,
		"cache_hit":   rand.Float32() < 0.3,
	}, nil
}

func TestPubMedRetrieverMock(t *testing.T) {
	retriever := &mockPubMedRetriever{}
	ctx := context.Background()

	input := map[string]any{
		"query":        "BRAF V600E",
		"max_results":  10,
	}

	result, err := retriever.Execute(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, "BRAF V600E", result["query"])
	assert.Contains(t, result, "papers")
	papers := result["papers"].([]map[string]any)
	assert.Greater(t, len(papers), 0)

	paper := papers[0]
	assert.Contains(t, paper, "pmid")
	assert.Contains(t, paper, "title")
	assert.Contains(t, paper, "abstract")
	assert.Contains(t, result, "cache_hit")
}

func TestChEMBLRetrieverMock(t *testing.T) {
	retriever := NewChEMBLRetriever()
	ctx := context.Background()

	input := map[string]any{
		"query":       "BRAF",
		"search_type": "target",
	}

	result, err := retriever.Execute(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, "BRAF", result["query"])
	assert.Equal(t, "target", result["search_type"])
	assert.Contains(t, result, "results")
}

func TestUniProtRetrieverMock(t *testing.T) {
	retriever := NewUniProtRetriever()
	ctx := context.Background()

	input := map[string]any{
		"query": "BRAF",
	}

	result, err := retriever.Execute(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, "BRAF", result["query"])
	assert.Contains(t, result, "results")
	results := result["results"].([]map[string]any)
	assert.Greater(t, len(results), 0)

	entry := results[0]
	assert.Equal(t, "P15056", entry["accession"])
	assert.Equal(t, "BRAF", entry["gene_name"])
	assert.Contains(t, entry, "domains")
	assert.Contains(t, entry, "variants")
}

func TestRetrieverInterface(t *testing.T) {
	var r Retriever = &mockPubMedRetriever{}
	assert.Equal(t, "PubMed", r.Name())
	assert.Equal(t, "retriever", r.Category())
	assert.NotNil(t, r.InputSchema())
}