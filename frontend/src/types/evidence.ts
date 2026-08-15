export type EvidenceCard = {
  id: string;
  claim: string;
  confidence: {
    overall: number; // 0-1
    // Optional: live single_call responses carry a real per-signal breakdown;
    // when absent (e.g. multi-agent path), components must show the signals
    // as unavailable rather than faking four bars from `overall`.
    signals?: {
      literature: number;
      protein_evidence: number;
      clinical_evidence: number;
      llm_rating: number; // must be visually de-emphasized — see Step 2
    };
  };
  sources: EvidenceSource[];
  toolCalls: ToolCallTrace[];
};

export type EvidenceSource = {
  id: string;
  type: "paper" | "protein_structure" | "sequence" | "molecule" | "pathway";
  title: string;
  retracted?: boolean;
  stance?: "supports" | "contradicts" | "neutral"; // used in Step 4
  refUrl?: string;
  payload: unknown; // shape depends on `type` — defined per-step
};

export type ToolCallTrace = {
  tool: string; // e.g. "PubMed", "UniProt"
  category: "retriever" | "analyzer" | "visualizer" | "executor";
  latencyMs: number;
  cacheHit: boolean;
  retries: number;
  tokens?: number;
};

export type TimelineEvent = {
  date: string;
  label: string;
  type: "discovery" | "trial_phase" | "approval" | "publication";
  sourceId: string;
};
