package real

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/srikarjy/research-orchestrator/biolab-mcp-server/internal/tools/retrievers"
)

type RetrieverFactory struct {
	mu          sync.RWMutex
	retrievers  map[string]retrievers.Retriever
	useMock     bool
}

func NewRetrieverFactory(useMock bool) *RetrieverFactory {
	return &RetrieverFactory{
		retrievers: make(map[string]retrievers.Retriever),
		useMock:    useMock,
	}
}

func (f *RetrieverFactory) Register(name string, r retrievers.Retriever) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retrievers[name] = r
}

func (f *RetrieverFactory) Get(name string) (retrievers.Retriever, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	r, ok := f.retrievers[name]
	if !ok {
		return nil, fmt.Errorf("retriever %s not found", name)
	}
	return r, nil
}

func (f *RetrieverFactory) GetAll() map[string]retrievers.Retriever {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make(map[string]retrievers.Retriever, len(f.retrievers))
	for k, v := range f.retrievers {
		result[k] = v
	}
	return result
}

func (f *RetrieverFactory) Initialize() {
	if f.useMock {
		f.Register("PubMed", &MockPubMedRetriever{&PubMedRetriever{client: &http.Client{Timeout: 30 * time.Second}, rateLim: PubMedRateLimit}})
		f.Register("ChEMBL", &MockChEMBLRetriever{NewChEMBLRetriever()})
		f.Register("UniProt", &MockUniProtRetriever{NewUniProtRetriever()})
		f.Register("KEGG", &MockKEGGRetriever{NewKEGGRetriever()})
		f.Register("Reactome", &MockReactomeRetriever{NewReactomeRetriever()})
	} else {
		f.Register("PubMed", NewPubMedRetriever(""))
		f.Register("ChEMBL", NewChEMBLRetriever())
		f.Register("UniProt", NewUniProtRetriever())
		f.Register("KEGG", NewKEGGRetriever())
		f.Register("Reactome", NewReactomeRetriever())
	}
}

type MockPubMedRetriever struct {
	*PubMedRetriever
}

func (m *MockPubMedRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return m.MockExecute(ctx, input)
}

type MockChEMBLRetriever struct {
	*ChEMBLRetriever
}

func (m *MockChEMBLRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return m.MockExecute(ctx, input)
}

type MockUniProtRetriever struct {
	*UniProtRetriever
}

func (m *MockUniProtRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return m.MockExecute(ctx, input)
}

type MockKEGGRetriever struct {
	*KEGGRetriever
}

func (m *MockKEGGRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return m.MockExecute(ctx, input)
}

type MockReactomeRetriever struct {
	*ReactomeRetriever
}

func (m *MockReactomeRetriever) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	return m.MockExecute(ctx, input)
}

type RetrieverRegistry struct {
	factory *RetrieverFactory
}

func NewRetrieverRegistry(useMock bool) *RetrieverRegistry {
	factory := NewRetrieverFactory(useMock)
	factory.Initialize()
	return &RetrieverRegistry{factory: factory}
}

func (r *RetrieverRegistry) GetRetriever(name string) (retrievers.Retriever, error) {
	return r.factory.Get(name)
}

func (r *RetrieverRegistry) GetAllRetrievers() map[string]retrievers.Retriever {
	return r.factory.GetAll()
}

func (r *RetrieverRegistry) ExecuteAll(ctx context.Context, query string, maxResults int) (map[string]map[string]interface{}, error) {
	results := make(map[string]map[string]interface{})
	allRetrievers := r.factory.GetAll()

	var wg sync.WaitGroup
	errCh := make(chan error, len(allRetrievers))
	resultCh := make(chan retrieverResult, len(allRetrievers))

	for name, retriever := range allRetrievers {
		wg.Add(1)
		go func(name string, retriever retrievers.Retriever) {
			defer wg.Done()
			input := map[string]interface{}{
				"query":        query,
				"max_results":  maxResults,
			}
			res, err := retriever.Execute(ctx, input)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", name, err)
				return
			}
			resultCh <- retrieverResult{name: name, result: res}
		}(name, retriever)
	}

	wg.Wait()
	close(errCh)
	close(resultCh)

	for err := range errCh {
		return nil, err
	}

	for res := range resultCh {
		results[res.name] = res.result
	}

	return results, nil
}

type retrieverResult struct {
	name   string
	result map[string]interface{}
}