import { useMemo } from "react";
import "./Timeline.css";

interface TimelineEvent {
  date: string;
  label: string;
  type: "discovery" | "trial_phase" | "approval" | "publication";
  sourceId: string;
}

interface TimelineProps {
  events: TimelineEvent[];
}

const TYPE_COLORS = {
  discovery: "var(--signal)",
  trial_phase: "var(--structural)",
  approval: "var(--alert)",
  publication: "var(--muted)",
};

const TYPE_LABELS = {
  discovery: "Discovery",
  trial_phase: "Trial Phase",
  approval: "Approval",
  publication: "Publication",
};

const TYPE_ICONS = {
  discovery: "🔬",
  trial_phase: "🧪",
  approval: "✅",
  publication: "📄",
};

export function Timeline({ events }: TimelineProps) {
  const sortedEvents = useMemo(() => {
    return [...events].sort((a, b) => new Date(a.date).getTime() - new Date(b.date).getTime());
  }, [events]);

  const dateRange = useMemo(() => {
    if (sortedEvents.length === 0) return { min: new Date(), max: new Date() };
    const dates = sortedEvents.map((e) => new Date(e.date).getTime());
    return { min: new Date(Math.min(...dates)), max: new Date(Math.max(...dates)) };
  }, [sortedEvents]);

  const timeSpan = dateRange.max.getTime() - dateRange.min.getTime();

  return (
    <div className="timeline" role="img" aria-label={`Timeline with ${events.length} events`}>
      <div className="timeline__header">
        <h3>Drug Development Timeline</h3>
        <div className="timeline__legend">
          {(Object.keys(TYPE_COLORS) as Array<keyof typeof TYPE_COLORS>).map((type) => (
            <span key={type} className="timeline__legend-item">
              <span className="timeline__legend-dot" style={{ background: TYPE_COLORS[type] }} />
              {TYPE_LABELS[type]}
            </span>
          ))}
        </div>
      </div>

      <div className="timeline__axis">
        {sortedEvents.map((event, index) => {
          const eventDate = new Date(event.date);
          const position = timeSpan > 0 
            ? ((eventDate.getTime() - dateRange.min.getTime()) / timeSpan) * 100 
            : 50;
          
          const isEven = index % 2 === 0;
          
          return (
            <div
              key={event.sourceId}
              className={`timeline__event ${isEven ? "timeline__event--above" : "timeline__event--below"}`}
              style={{ left: `${position}%` } as React.CSSProperties}
            >
              <div className="timeline__event-marker" style={{ background: TYPE_COLORS[event.type] }} />
              <div className="timeline__event-content">
                <div className="timeline__event-date">
                  {eventDate.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" })}
                </div>
                <div className="timeline__event-type" style={{ color: TYPE_COLORS[event.type] }}>
                  {TYPE_ICONS[event.type]} {TYPE_LABELS[event.type]}
                </div>
                <div className="timeline__event-label">{event.label}</div>
                <div className="timeline__event-source">Source: {event.sourceId}</div>
              </div>
              <div className="timeline__event-line" style={{ background: TYPE_COLORS[event.type] }} />
            </div>
          );
        })}
      </div>

      <div className="timeline__scale">
        {generateScaleLabels(dateRange.min, dateRange.max).map((label, i) => (
          <div key={i} className="timeline__scale-label" style={{ left: `${label.position}%` }}>
            {label.year}
          </div>
        ))}
      </div>
    </div>
  );
}

function generateScaleLabels(min: Date, max: Date) {
  const labels: { year: number; position: number }[] = [];
  const startYear = min.getFullYear();
  const endYear = max.getFullYear();
  const timeSpan = max.getTime() - min.getTime();

  for (let year = startYear; year <= endYear; year++) {
    const date = new Date(year, 0, 1);
    const position = timeSpan > 0 
      ? ((date.getTime() - min.getTime()) / timeSpan) * 100 
      : 50;
    if (position >= 0 && position <= 100) {
      labels.push({ year, position });
    }
  }
  return labels;
}