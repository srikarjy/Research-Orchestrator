import { useMemo } from "react";
import type { EvidenceCard } from "../types/evidence";
import "./ConfidenceLadder.css";

const SIGNAL_ORDER = ["literature", "protein_evidence", "clinical_evidence", "llm_rating"] as const;

const SIGNAL_LABELS: Record<string, string> = {
  literature: "Literature",
  protein_evidence: "Protein Evidence",
  clinical_evidence: "Clinical Evidence",
  llm_rating: "LLM Rating",
};

const SIGNAL_TOOLTIPS: Record<string, string> = {
  literature: "Literature: how many independent papers support this claim",
  protein_evidence: "Protein Evidence: structural, mutagenesis, and biophysical data",
  clinical_evidence: "Clinical Evidence: patient outcomes, trial data, real-world evidence",
  llm_rating: "LLM Rating: model self-assessment (capped at ≤15% weight in confidence gate)",
};

const SIGNAL_COLORS: Record<string, string> = {
  literature: "var(--signal)",
  protein_evidence: "var(--structural)",
  clinical_evidence: "var(--alert)",
  llm_rating: "var(--muted)",
};

const SIGNAL_SHORT_LABELS: Record<string, string> = {
  literature: "LIT",
  protein_evidence: "PROT",
  clinical_evidence: "CLIN",
  llm_rating: "LLM",
};

type SignalKey = (typeof SIGNAL_ORDER)[number];

interface ConfidenceLadderProps {
  evidence?: EvidenceCard;
  compact?: boolean;
}

export function ConfidenceLadder({ evidence, compact = false }: ConfidenceLadderProps) {
  const signals = useMemo(() => {
    if (!evidence) {
      return SIGNAL_ORDER.map((key) => ({
        key,
        label: SIGNAL_LABELS[key],
        shortLabel: SIGNAL_SHORT_LABELS[key],
        tooltip: SIGNAL_TOOLTIPS[key],
        value: 0,
        color: SIGNAL_COLORS[key],
        isLLM: key === "llm_rating",
      }));
    }
    const s = evidence.confidence.signals;
    return SIGNAL_ORDER.map((key) => ({
      key,
      label: SIGNAL_LABELS[key],
      shortLabel: SIGNAL_SHORT_LABELS[key],
      tooltip: SIGNAL_TOOLTIPS[key],
      value: s[key as SignalKey],
      color: SIGNAL_COLORS[key],
      isLLM: key === "llm_rating",
    }));
  }, [evidence]);

  const overall = evidence?.confidence.overall ?? 0;

  return (
    <aside
      className={`confidence-ladder ${compact ? "confidence-ladder--compact" : ""}`}
      role="img"
      aria-label={`Confidence breakdown: ${signals.map((s) => `${s.label} ${Math.round(s.value * 100)}%`).join(", ")}. Overall ${Math.round(overall * 100)}%`}
    >
      <div className="confidence-ladder__signals">
        {signals.map((signal) => (
          <div
            key={signal.key}
            className="confidence-ladder__signal"
            title={signal.tooltip}
            style={{
              "--signal-height": `${signal.value * 100}%`,
              "--signal-color": signal.color,
            } as React.CSSProperties}
          >
            <div className="confidence-ladder__band-wrapper">
              <div className={`confidence-ladder__band ${signal.isLLM ? "confidence-ladder__band--llm" : ""}`} />
            </div>
            <div className="confidence-ladder__meta">
              <span className="confidence-ladder__short-label">{signal.shortLabel}</span>
              <span className="confidence-ladder__value">{Math.round(signal.value * 100)}%</span>
            </div>
          </div>
        ))}
      </div>
      <div className="confidence-ladder__overall" title={`Overall confidence: ${Math.round(overall * 100)}%`}>
        <span className="confidence-ladder__overall-label">Overall</span>
        <span className="confidence-ladder__overall-value">{Math.round(overall * 100)}%</span>
      </div>
    </aside>
  );
}