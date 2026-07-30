import "./ConfidenceLadder.css";

// Static, non-functional shell (Step 0). Becomes the Step 2 confidence
// heatmap, doubles as primary nav, and reappears as the loading/skeleton
// state elsewhere in the app — see ROADMAP.md "Design direction".
export function ConfidenceLadder() {
  return (
    <aside className="confidence-ladder" aria-hidden="true">
      <div className="confidence-ladder__row">
        <span className="confidence-ladder__band confidence-ladder__band--literature" />
        <span className="confidence-ladder__band confidence-ladder__band--protein" />
        <span className="confidence-ladder__band confidence-ladder__band--clinical" />
        <span className="confidence-ladder__band confidence-ladder__band--llm" />
      </div>
    </aside>
  );
}
