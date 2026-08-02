package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EvidenceType string

const (
	EvidenceTypePaper           EvidenceType = "paper"
	EvidenceTypeProteinStructure EvidenceType = "protein_structure"
	EvidenceTypeSequence        EvidenceType = "sequence"
	EvidenceTypeMolecule        EvidenceType = "molecule"
	EvidenceTypePathway         EvidenceType = "pathway"
)

type Stance string

const (
	StanceSupports    Stance = "supports"
	StanceContradicts Stance = "contradicts"
	StanceNeutral     Stance = "neutral"
)

type EvidenceSource struct {
	ID        string                 `json:"id"`
	Type      EvidenceType           `json:"type"`
	Title     string                 `json:"title"`
	Retracted bool                   `json:"retracted,omitempty"`
	Stance    Stance                 `json:"stance,omitempty"`
	RefURL    string                 `json:"ref_url,omitempty"`
	Payload   map[string]interface{} `json:"payload"`
	ContentHash string               `json:"content_hash"`
	CreatedAt time.Time              `json:"created_at"`
}

func NewEvidenceSource(typ EvidenceType, title string, payload map[string]interface{}) *EvidenceSource {
	src := &EvidenceSource{
		ID:        uuid.New().String(),
		Type:      typ,
		Title:     title,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}
	src.ContentHash = src.ComputeContentHash()
	return src
}

func (s *EvidenceSource) ComputeContentHash() string {
	data := map[string]interface{}{
		"type":    s.Type,
		"title":   s.Title,
		"payload": s.Payload,
	}
	bytes, _ := json.Marshal(data)
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:])
}

type EvidenceCard struct {
	ID        string             `json:"id"`
	Claim     string             `json:"claim"`
	Confidence ConfidenceScore   `json:"confidence"`
	Sources   []*EvidenceSource  `json:"sources"`
	ToolCalls []ToolCallTrace    `json:"tool_calls"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type ConfidenceScore struct {
	Overall  float64            `json:"overall"`
	Signals  ConfidenceSignals  `json:"signals"`
}

type ConfidenceSignals struct {
	Literature       float64 `json:"literature"`
	ProteinEvidence  float64 `json:"protein_evidence"`
	ClinicalEvidence float64 `json:"clinical_evidence"`
	LLMRating        float64 `json:"llm_rating"`
}

type ToolCallTrace struct {
	Tool       string  `json:"tool"`
	Category   string  `json:"category"`
	LatencyMs  int     `json:"latency_ms"`
	CacheHit   bool    `json:"cache_hit"`
	Retries    int     `json:"retries"`
	Tokens     int     `json:"tokens,omitempty"`
}