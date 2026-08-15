import { useMemo, useState } from "react";
import "./ConfidenceTrend.css";

// Chart-grade series colors: darker steps of the app's brand hues, validated
// (six-check palette validator, light surface #EDEEE7) — the raw UI tokens
// fail lightness/chroma/contrast checks as data colors, so charts use these.
export const TREND_COLORS = {
  overall: "#1B2420", // ink — the hero series
  literature: "#178A5B",
  protein_evidence: "#2C6E9E",
  clinical_evidence: "#C7442E",
  llm_rating: "#7A5FA0",
} as const;

const SIGNAL_ORDER = ["literature", "protein_evidence", "clinical_evidence", "llm_rating"] as const;

const SIGNAL_LABELS: Record<string, string> = {
  literature: "Literature",
  protein_evidence: "Protein",
  clinical_evidence: "Clinical",
  llm_rating: "LLM",
};

export interface TrendPoint {
  checkedAt: string; // ISO timestamp
  confidence: number;
  verdict: string;
  changed: boolean;
  changeNote?: string;
  signals?: {
    literature: number;
    protein_evidence: number;
    clinical_evidence: number;
    llm_rating: number;
  } | null;
}

interface ConfidenceTrendProps {
  points: TrendPoint[];
  showSignals?: boolean;
}

const W = 640;
const H = 220;
const PAD = { top: 16, right: 96, bottom: 28, left: 40 };

function x(i: number, n: number): number {
  if (n <= 1) return PAD.left + (W - PAD.left - PAD.right) / 2;
  return PAD.left + (i / (n - 1)) * (W - PAD.left - PAD.right);
}

function y(v: number): number {
  return PAD.top + (1 - v) * (H - PAD.top - PAD.bottom);
}

function pathFor(values: (number | undefined)[], n: number): string {
  let d = "";
  values.forEach((v, i) => {
    if (v === undefined) return;
    d += `${d ? "L" : "M"}${x(i, n).toFixed(1)},${y(v).toFixed(1)}`;
  });
  return d;
}

function fmtDate(iso: string): string {
  const d = new Date(iso);
  return `${d.toLocaleDateString(undefined, { month: "short", day: "numeric" })} ${d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })}`;
}

export function ConfidenceTrend({ points, showSignals = true }: ConfidenceTrendProps) {
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);
  const n = points.length;

  const signalSeries = useMemo(() => {
    if (!showSignals) return [];
    return SIGNAL_ORDER.map((key) => ({
      key,
      label: SIGNAL_LABELS[key],
      color: TREND_COLORS[key],
      values: points.map((p) => p.signals?.[key]),
    })).filter((s) => s.values.some((v) => v !== undefined));
  }, [points, showSignals]);

  if (n === 0) {
    return <div className="confidence-trend__empty">No checks yet — run one to start the trend.</div>;
  }

  const onMove = (e: React.MouseEvent<SVGSVGElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const px = ((e.clientX - rect.left) / rect.width) * W;
    let best = 0;
    let bestDist = Infinity;
    for (let i = 0; i < n; i++) {
      const d = Math.abs(x(i, n) - px);
      if (d < bestDist) {
        bestDist = d;
        best = i;
      }
    }
    setHoverIdx(best);
  };

  const hover = hoverIdx !== null ? points[hoverIdx] : null;

  return (
    <div className="confidence-trend">
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="confidence-trend__svg"
        role="img"
        aria-label={`Confidence over ${n} checks, latest ${Math.round(points[n - 1].confidence * 100)}%`}
        onMouseMove={onMove}
        onMouseLeave={() => setHoverIdx(null)}
      >
        {/* recessive grid: 0/0.5/1 only */}
        {[0, 0.5, 1].map((v) => (
          <g key={v}>
            <line x1={PAD.left} x2={W - PAD.right} y1={y(v)} y2={y(v)} className="confidence-trend__grid" />
            <text x={PAD.left - 6} y={y(v) + 3.5} className="confidence-trend__tick" textAnchor="end">
              {Math.round(v * 100)}%
            </text>
          </g>
        ))}

        {/* signal lines under the hero line */}
        {signalSeries.map((s) => (
          <path key={s.key} d={pathFor(s.values, n)} className="confidence-trend__signal-line" stroke={s.color} />
        ))}

        {/* hero: overall confidence */}
        <path d={pathFor(points.map((p) => p.confidence), n)} className="confidence-trend__hero-line" stroke={TREND_COLORS.overall} />

        {/* markers: every point ≥8px hit; changed points ringed in alert */}
        {points.map((p, i) => (
          <g key={i}>
            {p.changed && (
              <circle cx={x(i, n)} cy={y(p.confidence)} r={7} className="confidence-trend__changed-ring" />
            )}
            <circle
              cx={x(i, n)}
              cy={y(p.confidence)}
              r={hoverIdx === i ? 5 : 3.5}
              fill={TREND_COLORS.overall}
              className="confidence-trend__marker"
            />
          </g>
        ))}

        {/* direct labels at line ends */}
        <text x={x(n - 1, n) + 8} y={y(points[n - 1].confidence) + 3.5} className="confidence-trend__end-label">
          Overall {Math.round(points[n - 1].confidence * 100)}%
        </text>
        {signalSeries.map((s) => {
          const last = [...s.values].reverse().find((v) => v !== undefined);
          if (last === undefined) return null;
          return (
            <text key={s.key} x={x(n - 1, n) + 8} y={y(last) + 3.5} className="confidence-trend__end-label confidence-trend__end-label--signal">
              {s.label}
            </text>
          );
        })}

        {/* crosshair */}
        {hoverIdx !== null && (
          <line
            x1={x(hoverIdx, n)}
            x2={x(hoverIdx, n)}
            y1={PAD.top}
            y2={H - PAD.bottom}
            className="confidence-trend__crosshair"
          />
        )}

        {/* x labels: first + last */}
        <text x={x(0, n)} y={H - 8} className="confidence-trend__tick" textAnchor="start">
          {fmtDate(points[0].checkedAt)}
        </text>
        {n > 1 && (
          <text x={x(n - 1, n)} y={H - 8} className="confidence-trend__tick" textAnchor="end">
            {fmtDate(points[n - 1].checkedAt)}
          </text>
        )}
      </svg>

      {hover && hoverIdx !== null && (
        <div
          className="confidence-trend__tooltip"
          style={{ left: `${(x(hoverIdx, n) / W) * 100}%` }}
        >
          <div className="confidence-trend__tooltip-date">{fmtDate(hover.checkedAt)}</div>
          <div>
            <span className="confidence-trend__tooltip-verdict" data-verdict={hover.verdict}>{hover.verdict}</span>
            {" · "}
            {Math.round(hover.confidence * 100)}%
          </div>
          {hover.signals && (
            <div className="confidence-trend__tooltip-signals">
              {SIGNAL_ORDER.map((k) => (
                <span key={k}>
                  <span className="confidence-trend__swatch" style={{ background: TREND_COLORS[k] }} />
                  {SIGNAL_LABELS[k]} {Math.round((hover.signals?.[k] ?? 0) * 100)}%
                </span>
              ))}
            </div>
          )}
          {hover.changed && <div className="confidence-trend__tooltip-changed">⚑ {hover.changeNote}</div>}
        </div>
      )}

      {signalSeries.length > 0 && (
        <div className="confidence-trend__legend">
          <span>
            <span className="confidence-trend__swatch" style={{ background: TREND_COLORS.overall }} />
            Overall
          </span>
          {signalSeries.map((s) => (
            <span key={s.key}>
              <span className="confidence-trend__swatch" style={{ background: s.color }} />
              {s.label}
            </span>
          ))}
          <span className="confidence-trend__legend-changed">◦ ring = verdict/confidence changed</span>
        </div>
      )}
    </div>
  );
}
