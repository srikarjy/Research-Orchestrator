package realretrievers

import (
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
	PubMedBaseURL = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"
	PubMedRateLimit = 3 * time.Second
)

type PubMedRetriever struct {
	client     *http.Client
	apiKey     string
	lastCall   time.Time
	rateLim    time.Duration
}

type PubMedSearchResult struct {
	Count    int      `xml:"Count"`
	IdList   []string `xml:"IdList>Id"`
}

type PubMedFetchResult struct {
	Articles []PubMedArticle `xml:"PubmedArticle"`
}

type PubMedArticle struct {
	MedlineCitation MedlineCitation `xml:"MedlineCitation"`
	PubmedData      PubmedData      `xml:"PubmedData"`
}

type MedlineCitation struct {
	PMID        string        `xml:"PMID"`
	Article     Article       `xml:"Article"`
	MeshHeadingList []MeshHeading `xml:"MeshHeadingList>MeshHeading"`
}

type Article struct {
	Journal   Journal   `xml:"Journal"`
	ArticleTitle string  `xml:"ArticleTitle"`
	Abstract  Abstract  `xml:"Abstract"`
	AuthorList AuthorList `xml:"AuthorList"`
	PubDate   PubDate   `xml:"PubDate"`
}

type Journal struct {
	Title string `xml:"Title"`
	ISOAbbreviation string `xml:"ISOAbbreviation"`
}

type Abstract struct {
	AbstractText []string `xml:"AbstractText"`
}

type AuthorList struct {
	Authors []Author `xml:"Author"`
}

type Author struct {
	LastName string `xml:"LastName"`
	ForeName string `xml:"ForeName"`
	Initials string `xml:"Initials"`
}

type PubDate struct {
	Year  string `xml:"Year"`
	Month string `xml:"Month"`
	Day   string `xml:"Day"`
}

type MeshHeading struct {
	DescriptorName string `xml:"DescriptorName"`
	QualifierName  string `xml:"QualifierName"`
}

type PubmedData struct {
	ArticleIdList []ArticleId `xml:"ArticleIdList>ArticleId"`
}

type ArticleId struct {
	IdType string `xml:"IdType,attr"`
	Value  string `xml:",chardata"`
}

func NewPubMedRetriever(apiKey string) *PubMedRetriever {
	return &PubMedRetriever{
		client:  &http.Client{Timeout: 30 * time.Second},
		apiKey:  apiKey,
		rateLim: PubMedRateLimit,
	}
}

func (p *PubMedRetriever) Name() string        { return "PubMed" }
func (p *PubMedRetriever) Category() string    { return "retriever" }
func (p *PubMedRetriever) Description() string { return "Search PubMed for biomedical literature via NCBI E-utilities" }

func (p *PubMedRetriever) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":        map[string]interface{}{"type": "string", "description": "Search query"},
			"max_results":  map[string]interface{}{"type": "integer", "default": 20},
			"retmax":       map[string]interface{}{"type": "integer", "default": 20},
			"retstart":     map[string]interface{}{"type": "integer", "default": 0},
			"sort":         map[string]interface{}{"type": "string", "enum": []string{"relevance", "pub_date", "most_recent"}, "default": "relevance"},
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
	if mr, ok := input["retmax"].(float64); ok {
		maxResults = int(mr)
	}
	retStart := 0
	if rs, ok := input["retstart"].(float64); ok {
		retStart = int(rs)
	}
	sortBy := "relevance"
	if sb, ok := input["sort"].(string); ok {
		sortBy = sb
	}

	if err := p.rateLimit(ctx); err != nil {
		return nil, err
	}

	ids, totalCount, err := p.search(ctx, query, maxResults, retStart, sortBy)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(ids) == 0 {
		return map[string]interface{}{
			"query":       query,
			"total_found": totalCount,
			"returned":    0,
			"papers":      []map[string]interface{}{},
			"cache_hit":   false,
		}, nil
	}

	papers, err := p.fetch(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	return map[string]interface{}{
		"query":       query,
		"total_found": totalCount,
		"returned":    len(papers),
		"papers":      papers,
		"cache_hit":   false,
	}, nil
}

func (p *PubMedRetriever) rateLimit(ctx context.Context) error {
	elapsed := time.Since(p.lastCall)
	if elapsed < p.rateLim {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.rateLim - elapsed):
		}
	}
	p.lastCall = time.Now()
	return nil
}

func (p *PubMedRetriever) search(ctx context.Context, query string, retMax, retStart int, sortBy string) ([]string, int, error) {
	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("term", query)
	params.Set("retmax", fmt.Sprintf("%d", retMax))
	params.Set("retstart", fmt.Sprintf("%d", retStart))
	params.Set("sort", sortBy)
	params.Set("retmode", "xml")
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}

	urlStr := PubMedBaseURL + "/esearch.fcgi?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var result PubMedSearchResult
	if err := xml.Unmarshal(body, &result); err != nil {
		return nil, 0, fmt.Errorf("unmarshal search result: %w", err)
	}

	count := result.Count
	if count == 0 {
		return []string{}, 0, nil
	}

	return result.IdList, count, nil
}

func (p *PubMedRetriever) fetch(ctx context.Context, ids []string) ([]map[string]interface{}, error) {
	idStr := strings.Join(ids, ",")

	params := url.Values{}
	params.Set("db", "pubmed")
	params.Set("id", idStr)
	params.Set("retmode", "xml")
	params.Set("rettype", "abstract")
	if p.apiKey != "" {
		params.Set("api_key", p.apiKey)
	}

	urlStr := PubMedBaseURL + "/efetch.fcgi?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result PubMedFetchResult
	if err := xml.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal fetch result: %w", err)
	}

	papers := make([]map[string]interface{}, 0, len(result.Articles))
	for _, article := range result.Articles {
		paper := p.parseArticle(article)
		papers = append(papers, paper)
	}

	return papers, nil
}

func (p *PubMedRetriever) parseArticle(article PubMedArticle) map[string]interface{} {
	citation := article.MedlineCitation
	articleData := citation.Article

	authors := make([]string, 0, len(articleData.AuthorList.Authors))
	for _, a := range articleData.AuthorList.Authors {
		name := a.LastName
		if a.ForeName != "" {
			name += " " + a.ForeName
		} else if a.Initials != "" {
			name += " " + a.Initials
		}
		authors = append(authors, name)
	}

	abstract := ""
	if len(articleData.Abstract.AbstractText) > 0 {
		abstract = strings.Join(articleData.Abstract.AbstractText, " ")
	}

	pubDate := articleData.PubDate.Year
	if articleData.PubDate.Month != "" {
		pubDate += "-" + articleData.PubDate.Month
	}
	if articleData.PubDate.Day != "" {
		pubDate += "-" + articleData.PubDate.Day
	}

	doi := ""
	for _, id := range article.PubmedData.ArticleIdList {
		if id.IdType == "doi" {
			doi = id.Value
			break
		}
	}

	meshTerms := make([]string, 0, len(citation.MeshHeadingList))
	for _, mh := range citation.MeshHeadingList {
		if mh.DescriptorName != "" {
			meshTerms = append(meshTerms, mh.DescriptorName)
		}
	}

	return map[string]interface{}{
		"pmid":           citation.PMID,
		"title":          articleData.ArticleTitle,
		"authors":        authors,
		"journal":        articleData.Journal.Title,
		"journal_abbrev": articleData.Journal.ISOAbbreviation,
		"pub_date":       pubDate,
		"abstract":       abstract,
		"doi":            doi,
		"mesh_terms":     meshTerms,
		"url":            "https://pubmed.ncbi.nlm.nih.gov/" + citation.PMID,
	}
}



func (p *PubMedRetriever) MockExecute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	query := input["query"].(string)
	maxResults := 20
	if mr, ok := input["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(100+rand.Intn(300)) * time.Millisecond):
	}

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
			"mesh_terms":  mockMeshTerms(),
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
	return fmt.Sprintf(templates[rand.Intn(len(templates))], query)
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
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func mockAbstract(query string) string {
	return "We investigated the role of " + query + " using structural and biochemical approaches. Our findings demonstrate..."
}

func mockDOI() string {
	return fmt.Sprintf("10.1038/s41586-2024-%05d", 10000+rand.Intn(90000))
}

func mockPMID() string {
	return fmt.Sprintf("%d", 10000000+rand.Intn(20000000))
}

func mockMeshTerms() []string {
	terms := []string{"Protein Kinases", "Signal Transduction", "Drug Resistance", "Mutation", "Neoplasms"}
	count := 2 + rand.Intn(3)
	result := make([]string, count)
	for i := range result {
		result[i] = terms[rand.Intn(len(terms))]
	}
	return result
}