import { useState } from "react";
import { ConfidenceLadder } from "./components/ConfidenceLadder";
import { ToolExecutionGraph } from "./components/ToolExecutionGraph";
import { CalendarPanel } from "./components/CalendarPanel";
import { ConfidenceHeatmap } from "./components/ConfidenceHeatmap";
import { ContradictionGraph } from "./components/ContradictionGraph";
import { StructureViewer } from "./components/StructureViewer";
import { MoleculeViewer } from "./components/MoleculeViewer";
import { SequenceAlignment } from "./components/SequenceAlignment";
import { Timeline } from "./components/Timeline";
import { PathwayViewer } from "./components/PathwayViewer";
import { AgentPanel } from "./components/AgentPanel";
import { WorkflowPanel } from "./components/WorkflowPanel";
import { SandboxPanel } from "./components/SandboxPanel";
import { NotificationPanel } from "./components/NotificationPanel";
import fixture1 from "./fixtures/mutation-binding-affinity.json";
import fixture2 from "./fixtures/allosteric-braf.json";
import fixture3 from "./fixtures/contradiction-braf.json";
import seqAlignFixture from "./fixtures/sequence-alignment.json";
import timelineFixture from "./fixtures/timeline-braf.json";
import pathwayKEGGFixture from "./fixtures/pathway-mapk.json";
import pathwayReactomeFixture from "./fixtures/pathway-reactome-mapk.json";
import type { EvidenceCard, ToolCallTrace, EvidenceSource, TimelineEvent } from "./types/evidence";

const evidence1 = fixture1 as EvidenceCard;
const evidence2 = fixture2 as EvidenceCard;
const evidence3 = fixture3 as EvidenceCard;

const EVIDENCE_OPTIONS = [
  { id: "1", label: "BRAF V600E Binding Affinity (High Lit / High LLM)", evidence: evidence1 },
  { id: "2", label: "Allosteric BRAF Inhibition (Low Lit / High LLM)", evidence: evidence2 },
  { id: "3", label: "BRAF V600E Contradiction Case (Mixed Stances)", evidence: evidence3 },
] as const;

function getStructureSource(sources: EvidenceSource[]): EvidenceSource | undefined {
  return sources.find((s) => s.type === "protein_structure" && s.payload && typeof s.payload === "object" && "pdbId" in s.payload);
}

function getMoleculeSource(sources: EvidenceSource[]): EvidenceSource | undefined {
  return sources.find((s) => s.type === "molecule" && s.payload && typeof s.payload === "object" && "smiles" in s.payload);
}

type MainTab = "evidence" | "sequence" | "timeline" | "pathway" | "agents" | "workflows" | "sandbox" | "notifications";

const PATHWAY_OPTIONS = [
  { id: "kegg", label: "KEGG MAPK", data: pathwayKEGGFixture },
  { id: "reactome", label: "Reactome MAPK", data: pathwayReactomeFixture },
] as const;

function App() {
  const [activeEvidence, setActiveEvidence] = useState<EvidenceCard>(evidence1);
  const [activeTab, setActiveTab] = useState<MainTab>("evidence");
  const [activePathway, setActivePathway] = useState(0);
  const handleNodeClick = (trace: ToolCallTrace) => {
    console.log("Node clicked:", trace);
  };

  const structureSource = getStructureSource(activeEvidence.sources);
  const moleculeSource = getMoleculeSource(activeEvidence.sources);

  const mainTabs: { id: MainTab; label: string }[] = [
    { id: "evidence", label: "Evidence" },
    { id: "sequence", label: "Sequence" },
    { id: "timeline", label: "Timeline" },
    { id: "pathway", label: "Pathways" },
    { id: "agents", label: "Agents" },
    { id: "workflows", label: "Workflows" },
    { id: "sandbox", label: "Sandbox" },
    { id: "notifications", label: "Notify" },
  ];

  return (
    <>
      <ConfidenceLadder />
      <CalendarPanel />
      <main style={{ marginLeft: 80, marginRight: 380, padding: 24 }}>
        <header style={{ marginBottom: 32 }}>
          <h1 style={{ fontFamily: "var(--font-sans)", fontSize: "28px", fontWeight: 600, marginBottom: 16 }}>
            {activeTab === "evidence" ? activeEvidence.claim : mainTabs.find(t => t.id === activeTab)?.label}
          </h1>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 16 }}>
            {EVIDENCE_OPTIONS.map((opt) => (
              <button
                key={opt.id}
                onClick={() => { setActiveEvidence(opt.evidence); setActiveTab("evidence"); }}
                style={{
                  padding: "8px 16px",
                  border: activeEvidence === opt.evidence ? "2px solid var(--signal)" : "1px solid var(--muted)",
                  background: activeEvidence === opt.evidence ? "var(--signal)" : "var(--bg)",
                  color: activeEvidence === opt.evidence ? "var(--ink)" : "var(--ink)",
                  borderRadius: 6,
                  fontFamily: "var(--font-sans)",
                  fontSize: 13,
                  cursor: "pointer",
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>
          <div style={{ display: "flex", gap: 8, borderTop: "1px solid var(--muted)", paddingTop: 16 }}>
            {mainTabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                style={{
                  padding: "8px 16px",
                  border: activeTab === tab.id ? "2px solid var(--signal)" : "1px solid var(--muted)",
                  background: activeTab === tab.id ? "var(--signal)" : "var(--bg)",
                  color: activeTab === tab.id ? "var(--ink)" : "var(--ink)",
                  borderRadius: 6,
                  fontFamily: "var(--font-sans)",
                  fontSize: 13,
                  cursor: "pointer",
                }}
              >
                {tab.label}
              </button>
            ))}
          </div>
        </header>

        {activeTab === "evidence" && (
          <>
            <section style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, marginBottom: 32, alignItems: "start" }}>
              <div>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "16px", fontWeight: 600, marginBottom: 12, color: "var(--ink)" }}>
                  Confidence Heatmap
                </h2>
                <ConfidenceHeatmap evidence={activeEvidence} />
              </div>
              <div>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "16px", fontWeight: 600, marginBottom: 12, color: "var(--ink)" }}>
                  Signal Breakdown
                </h2>
                <div style={{ fontFamily: "var(--font-mono)", fontSize: "12px", lineHeight: 2, color: "var(--ink)" }}>
                  <div>Literature: <span style={{ color: "var(--signal)" }}>{Math.round(activeEvidence.confidence.signals.literature * 100)}%</span></div>
                  <div>Protein Evidence: <span style={{ color: "var(--structural)" }}>{Math.round(activeEvidence.confidence.signals.protein_evidence * 100)}%</span></div>
                  <div>Clinical Evidence: <span style={{ color: "var(--alert)" }}>{Math.round(activeEvidence.confidence.signals.clinical_evidence * 100)}%</span></div>
                  <div>LLM Rating: <span style={{ color: "var(--muted)" }}>{Math.round(activeEvidence.confidence.signals.llm_rating * 100)}%</span></div>
                  <div style={{ borderTop: "1px solid var(--muted)", paddingTop: 8, marginTop: 8, fontWeight: 600 }}>
                    Overall: <span style={{ color: "var(--signal)" }}>{Math.round(activeEvidence.confidence.overall * 100)}%</span>
                  </div>
                </div>
              </div>
            </section>

            {structureSource && (
              <section style={{ marginBottom: 32 }}>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                  Protein Structure: {structureSource.title}
                </h2>
                <StructureViewer
                  pdbId={(structureSource.payload as any).pdbId}
                  mutationResidue={(structureSource.payload as any).mutationResidue}
                  bindingPocket={(structureSource.payload as any).bindingPocket}
                  width={700}
                  height={450}
                />
              </section>
            )}

            {moleculeSource && (
              <section style={{ marginBottom: 32 }}>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                  Molecule: {moleculeSource.title}
                </h2>
                <MoleculeViewer
                  smiles={(moleculeSource.payload as any).smiles}
                  width={500}
                  height={350}
                />
              </section>
            )}

            {activeEvidence.sources.some((s) => s.stance) && (
              <section style={{ marginBottom: 32 }}>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                  Contradiction Graph
                </h2>
                <ContradictionGraph sources={activeEvidence.sources} claim={activeEvidence.claim} />
              </section>
            )}

            <section style={{ marginBottom: 32 }}>
              <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                Tool Execution Graph
              </h2>
              <ToolExecutionGraph toolCalls={activeEvidence.toolCalls} onNodeClick={handleNodeClick} />
            </section>

            <section>
              <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                Raw Evidence
              </h2>
              <pre style={{ fontFamily: "var(--font-mono)", fontSize: "12px", lineHeight: 1.6, overflow: "auto", background: "var(--ink)", color: "var(--bg)", padding: 16, borderRadius: 6 }}>
                {JSON.stringify(activeEvidence, null, 2)}
              </pre>
            </section>
          </>
        )}

        {activeTab === "sequence" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Sequence Alignment
            </h2>
            <SequenceAlignment
              sequence1={seqAlignFixture.sequence1}
              sequence2={seqAlignFixture.sequence2}
              label1={seqAlignFixture.label1}
              label2={seqAlignFixture.label2}
            />
          </section>
        )}

        {activeTab === "timeline" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Development Timeline
            </h2>
            <Timeline events={timelineFixture.events as TimelineEvent[]} />
          </section>
        )}

        {activeTab === "pathway" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Pathway Diagrams
            </h2>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 16 }}>
              {PATHWAY_OPTIONS.map((opt, idx) => (
                <button
                  key={opt.id}
                  onClick={() => setActivePathway(idx)}
                  style={{
                    padding: "8px 16px",
                    border: activePathway === idx ? "2px solid var(--signal)" : "1px solid var(--muted)",
                    background: activePathway === idx ? "var(--signal)" : "var(--bg)",
                    color: activePathway === idx ? "var(--ink)" : "var(--ink)",
                    borderRadius: 6,
                    fontFamily: "var(--font-sans)",
                    fontSize: 13,
                    cursor: "pointer",
                  }}
                >
                  {opt.label}
                </button>
              ))}
            </div>
            <div style={{ marginBottom: 16, fontFamily: "var(--font-mono)", fontSize: "12px", color: "var(--muted)" }}>
              Source: {PATHWAY_OPTIONS[activePathway].data.source} | Pathway: {PATHWAY_OPTIONS[activePathway].data.pathwayId}
            </div>
            <PathwayViewer
              pathwayData={PATHWAY_OPTIONS[activePathway].data as any}
              width={900}
              height={600}
            />
          </section>
        )}

        {activeTab === "agents" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Multi-Agent System
            </h2>
            <AgentPanel />
          </section>
        )}

        {activeTab === "workflows" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Workflow Orchestration
            </h2>
            <WorkflowPanel />
          </section>
        )}

        {activeTab === "sandbox" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Experiment Sandbox
            </h2>
            <SandboxPanel />
          </section>
        )}

        {activeTab === "notifications" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Notifications & Alerts
            </h2>
            <NotificationPanel />
          </section>
        )}
      </main>
    </>
  );
}

export default App;