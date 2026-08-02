import { useMemo } from "react";
import type { EvidenceCard } from "../types/evidence";
import "./ConfidenceHeatmap.css";

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

type SignalKey = (typeof SIGNAL_ORDER)[number];

interface ConfidenceHeatmapProps {
  evidence: EvidenceCard;
  compact?: boolean;
}

export function ConfidenceHeatmap({ evidence, compact = false }: ConfidenceHeatmapProps) {
  const signals = useMemo(() => {
    const s = evidence.confidence.signals;
    return SIGNAL_ORDER.map((key) => ({
      key,
      label: SIGNAL_LABELS[key],
      tooltip: SIGNAL_TOOLTIPS[key],
      value: s[key as SignalKey],
      color: SIGNAL_COLORS[key],
      isLLM: key === "llm_rating",
    }));
  }, [evidence.confidence.signals]);

  const maxVal = Math.max(...signals.map((s) => s.value));

  return (
    <div className={`confidence-heatmap ${compact ? "confidence-heatmap--compact" : ""}`} role="img" aria-label={`Confidence breakdown: ${signals.map(s => `${s.label} ${Math.round(s.value*100)}%`).join(", ")}`}>
      {signals.map((signal) => (
        <div key={signal.key} className="confidence-heatmap__row">
          <div className="confidence-heatmap__label" title={signal.tooltip}>
            <span className="confidence-heatmap__name">{signal.label}</span>
            <span className={`confidence-heatmap__value ${signal.isLLM ? "confidence-heatmap__value--llm" : ""}`}>
              {Math.round(signal.value * 100)}%
            </span>
          </div>
          <div className="confidence-heatmap__bar-track" role="progressbar" aria-valuenow={Math.round(signal.value * 100)} aria-valuemin={0} aria-valuemax={100} aria-label={signal.label}>
            <div
              className={`confidence-heatmap__bar ${signal.isLLM ? "confidence-heatmap__bar--llm" : ""}`}
              style={{
                width: `${(signal.value / maxVal) * 100}%`,
                backgroundColor: signal.color,
              } as React.CSSProperties}
            />
          </div>
        </div>
      ))}
      {!compact && (
        <div className="confidence-heatmap__overall">
          <span className="confidence-heatmap__overall-label">Overall</span>
          <span className="confidence-heatmap__overall-value">{Math.round(evidence.confidence.overall * 100)}%</span>
        </div>
      )}
    </div>
  );
}