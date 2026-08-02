import type { EvidenceSource } from "../types/evidence";
import "./ContradictionGraph.css";

interface ContradictionGraphProps {
  sources: EvidenceSource[];
  claim: string;
}

const STANCE_COLORS = {
  supports: "var(--signal)",
  contradicts: "var(--alert)",
  neutral: "var(--muted)",
};

interface SourcePayload {
  excerpt?: string;
  [key: string]: any;
}

export function ContradictionGraph({ sources, claim }: ContradictionGraphProps) {
  const supporting = sources.filter((s) => s.stance === "supports");
  const contradicting = sources.filter((s) => s.stance === "contradicts");
  const neutral = sources.filter((s) => s.stance === "neutral" || !s.stance);

  return (
    <div className="contradiction-graph" role="img" aria-label={`Contradiction graph for claim: ${claim}`}>
      <div className="contradiction-graph__header">
        <h3 className="contradiction-graph__claim">{claim}</h3>
        <div className="contradiction-graph__legend">
          <span className="contradiction-graph__legend-item">
            <span className="contradiction-graph__legend-dot" style={{ background: STANCE_COLORS.supports }} />
            Supports ({supporting.length})
          </span>
          <span className="contradiction-graph__legend-item">
            <span className="contradiction-graph__legend-dot" style={{ background: STANCE_COLORS.contradicts }} />
            Contradicts ({contradicting.length})
          </span>
          {neutral.length > 0 && (
            <span className="contradiction-graph__legend-item">
              <span className="contradiction-graph__legend-dot" style={{ background: STANCE_COLORS.neutral }} />
              Neutral ({neutral.length})
            </span>
          )}
        </div>
      </div>

      <div className="contradiction-graph__columns">
        <Column
          title="Supports"
          color={STANCE_COLORS.supports}
          sources={supporting}
          position="left"
        />
        <div className="contradiction-graph__center">
          <div className="contradiction-graph__claim-node">
            <span className="contradiction-graph__claim-text">CLAIM</span>
            <div className="contradiction-graph__claim-id">{sources[0]?.id || "EV-XXX"}</div>
          </div>
          <div className="contradiction-graph__contradiction-score">
            Contradiction Score: <strong>{calculateContradictionScore(supporting.length, contradicting.length)}</strong>
          </div>
        </div>
        <Column
          title="Contradicts"
          color={STANCE_COLORS.contradicts}
          sources={contradicting}
          position="right"
        />
      </div>

      {neutral.length > 0 && (
        <div className="contradiction-graph__neutral">
          <h4>Neutral Sources</h4>
          <div className="contradiction-graph__neutral-list">
            {neutral.map((src) => (
              <NeutralSourceRow key={src.id} source={src} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function Column({ title, color, sources, position }: {
  title: string;
  color: string;
  sources: EvidenceSource[];
  position: "left" | "right";
}) {
  return (
    <div className={`contradiction-graph__column contradiction-graph__column--${position}`}>
      <div className="contradiction-graph__column-header" style={{ borderBottomColor: color }}>
        <h4>{title}</h4>
        <span className="contradiction-graph__count">{sources.length}</span>
      </div>
      <div className="contradiction-graph__sources">
{sources.length === 0 ? (
        <div className="contradiction-graph__empty">No sources</div>
      ) : (
        sources.map((src, idx) => (
          <SourceRow
            key={src.id}
            source={src}
            color={color}
            index={idx}
            total={sources.length}
            position={position}
          />
        ))
      )}
      </div>
    </div>
  );
}

function SourceRow({ source, color, index, total, position }: {
  source: EvidenceSource;
  color: string;
  index: number;
  total: number;
  position: "left" | "right";
}) {
  const payload = source.payload as SourcePayload | undefined;
  const yOffset = (index / Math.max(total, 1)) * 100;
  const excerpt = payload?.excerpt || source.title;

  return (
    <div
      className="contradiction-graph__source-row"
      style={{
        "--row-color": color,
        "--y-offset": `${yOffset}%`,
      } as React.CSSProperties}
    >
      <div className="contradiction-graph__source-main">
        <div className="contradiction-graph__source-dot" style={{ background: color }} />
        <div className="contradiction-graph__source-info">
          <div className="contradiction-graph__source-title">{source.title}</div>
          <div className="contradiction-graph__source-type">{source.type}</div>
        </div>
      </div>
      <div className="contradiction-graph__source-excerpt" title={excerpt}>
        {excerpt}
      </div>
      <div
        className={`contradiction-graph__edge contradiction-graph__edge--${position}`}
        style={{ borderColor: color }}
      />
    </div>
  );
}

function NeutralSourceRow({ source }: { source: EvidenceSource }) {
  const payload = source.payload as SourcePayload | undefined;
  const excerpt = payload?.excerpt || source.title;
  return (
    <div className="contradiction-graph__neutral-row">
      <span className="contradiction-graph__neutral-dot" style={{ background: STANCE_COLORS.neutral }} />
      <span className="contradiction-graph__neutral-title">{source.title}</span>
      <span className="contradiction-graph__neutral-excerpt" title={excerpt}>{excerpt}</span>
    </div>
  );
}

function calculateContradictionScore(supports: number, contradicts: number): string {
  const total = supports + contradicts;
  if (total === 0) return "N/A";
  const score = contradicts / total;
  return `${Math.round(score * 100)}% (${contradicts}/${total})`;
}