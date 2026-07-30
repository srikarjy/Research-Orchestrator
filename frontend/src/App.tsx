import { ConfidenceLadder } from "./components/ConfidenceLadder";
import fixture from "./fixtures/mutation-binding-affinity.json";
import type { EvidenceCard } from "./types/evidence";

const evidence = fixture as EvidenceCard;

function App() {
  return (
    <>
      <ConfidenceLadder />
      <main style={{ marginLeft: 80, padding: 24, fontFamily: "var(--font-mono)" }}>
        <pre>{JSON.stringify(evidence, null, 2)}</pre>
      </main>
    </>
  );
}

export default App;
