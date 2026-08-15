import { useState, useEffect } from "react";
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
import { useResearchOrchestrator, useExecutorData, useAgents, useMcpWorkflows } from "./api/hooks";

const evidence1 = fixture1 as EvidenceCard;
const evidence2 = fixture2 as EvidenceCard;
const evidence3 = fixture3 as EvidenceCard;

const EVIDENCE_OPTIONS = [
  { id: "1", label: "BRAF V600E Binding Affinity (High Lit / High LLM)", evidence: evidence1 },
  { id: "2", label: "Allosteric BRAF Inhibition (Low Lit / High LLM)", evidence: evidence2 },
  { id: "3", label: "BRAF V600E Contradiction Case (Mixed Stances)", evidence: evidence3 },
] as const;

function getStructureSource(sources: EvidenceSource[]): EvidenceSource | undefined {
  return sources.find(
    (s) => s.type === "protein_structure" && s.payload && typeof s.payload === "object" && "pdbId" in s.payload
  );
}

function getMoleculeSource(sources: EvidenceSource[]): EvidenceSource | undefined {
  return sources.find(
    (s) => s.type === "molecule" && s.payload && typeof s.payload === "object" && "smiles" in s.payload
  );
}

type MainTab = 
  | "evidence" 
  | "sequence" 
  | "timeline" 
  | "pathway" 
  | "agents" 
  | "workflows" 
  | "sandbox" 
  | "notifications"
  | "research";

const PATHWAY_OPTIONS = [
  { id: "kegg", label: "KEGG MAPK", data: pathwayKEGGFixture },
  { id: "reactome", label: "Reactome MAPK", data: pathwayReactomeFixture },
] as const;

function App() {
  const [activeEvidence, setActiveEvidence] = useState<EvidenceCard>(evidence1);
  const [activeTab, setActiveTab] = useState<MainTab>("evidence");
  const [activePathway, setActivePathway] = useState(0);
  const [backendConnected, setBackendConnected] = useState(false);
  const [researchQuery, setResearchQuery] = useState("");
  
  const handleNodeClick = (trace: ToolCallTrace) => {
    console.log("Node clicked:", trace);
  };

  // Backend integration hooks
  const { 
    activeQuery, 
    activePlan, 
    activeEvidence: researchEvidence, 
    progress, 
    loading: researchLoading, 
    error: researchError,
    executeFullPipeline,
  } = useResearchOrchestrator();

  const { error: executorError } = useExecutorData();
  const { agents, loading: agentsLoading } = useAgents();
  const { workflows: mcpWorkflows, loading: workflowsLoading } = useMcpWorkflows();

  // Check backend health on mount
  useEffect(() => {
    const checkBackends = async () => {
      try {
        const [weHealth, mcpHealth] = await Promise.all([
          fetch("/health").then(r => r.ok).catch(() => false),
          fetch("/health").then(r => r.ok).catch(() => false),
        ]);
        setBackendConnected(weHealth && mcpHealth);
      } catch {
        setBackendConnected(false);
      }
    };
    checkBackends();
    const interval = setInterval(checkBackends, 30000);
    return () => clearInterval(interval);
  }, []);

  // Use research evidence if available, otherwise fallback to fixtures
  const displayEvidence = (researchEvidence?.evidence_card || activeEvidence) as EvidenceCard;
  const displayStructureSource = getStructureSource(displayEvidence.sources);
  const displayMoleculeSource = getMoleculeSource(displayEvidence.sources);

  const mainTabs: { id: MainTab; label: string }[] = [
    { id: "evidence", label: "Evidence" },
    { id: "sequence", label: "Sequence" },
    { id: "timeline", label: "Timeline" },
    { id: "pathway", label: "Pathways" },
    { id: "agents", label: "Agents" },
    { id: "workflows", label: "Workflows" },
    { id: "sandbox", label: "Sandbox" },
    { id: "notifications", label: "Notify" },
    { id: "research", label: "Research" },
  ];

  const handleResearchSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!researchQuery.trim()) return;
    await executeFullPipeline({ query: researchQuery.trim() });
  };

  const handleQuickQuery = async (query: string) => {
    setResearchQuery(query);
    await executeFullPipeline({ query });
  };

  return (
    <>
      <ConfidenceLadder evidence={displayEvidence} />
      <CalendarPanel />
      <main style={{ marginLeft: 80, marginRight: 380, padding: 24 }}>
        <header style={{ marginBottom: 32 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
            <h1 style={{ fontFamily: "var(--font-sans)", fontSize: "28px", fontWeight: 600, margin: 0 }}>
              {activeTab === "evidence" ? displayEvidence.claim : mainTabs.find(t => t.id === activeTab)?.label}
            </h1>
            <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
              <span style={{
                width: 10, height: 10, borderRadius: "50%",
                background: backendConnected ? "var(--signal)" : "var(--alert)",
              }} />
              <span style={{ fontFamily: "var(--font-mono)", fontSize: "12px", color: "var(--muted)" }}>
                {backendConnected ? "Backend Connected" : "Using Fixtures"}
              </span>
            </div>
          </div>

          {activeTab === "research" && (
            <form onSubmit={handleResearchSubmit} style={{ marginBottom: 24 }}>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 12 }}>
                <input
                  type="text"
                  value={researchQuery}
                  onChange={(e) => setResearchQuery(e.target.value)}
                  placeholder="Ask a research question (e.g., 'explain why BRAF V600E reduces binding affinity')"
                  style={{
                    flex: 1,
                    minWidth: 300,
                    padding: "10px 16px",
                    border: "1px solid var(--muted)",
                    background: "var(--bg)",
                    color: "var(--ink)",
                    borderRadius: 6,
                    fontFamily: "var(--font-sans)",
                    fontSize: 14,
                  }}
                />
                <button
                  type="submit"
                  disabled={researchLoading || !researchQuery.trim()}
                  style={{
                    padding: "10px 24px",
                    border: "none",
                    background: researchLoading ? "var(--muted)" : "var(--signal)",
                    color: "var(--ink)",
                    borderRadius: 6,
                    fontFamily: "var(--font-sans)",
                    fontSize: 14,
                    fontWeight: 600,
                    cursor: researchLoading ? "not-allowed" : "pointer",
                  }}
                >
                  {researchLoading ? "Researching..." : "Investigate"}
                </button>
              </div>
              <div style={{ display: "flex", gap: 8, flexWrap: "wrap", fontSize: "12px", color: "var(--muted)" }}>
                <span>Quick queries:</span>
                <button type="button" onClick={() => handleQuickQuery("explain why BRAF V600E reduces binding affinity")} style={{ padding: "4px 10px", border: "1px solid var(--muted)", background: "var(--bg)", borderRadius: 4, cursor: "pointer" }}>BRAF V600E binding</button>
                <button type="button" onClick={() => handleQuickQuery("allosteric BRAF inhibition mechanisms")} style={{ padding: "4px 10px", border: "1px solid var(--muted)", background: "var(--bg)", borderRadius: 4, cursor: "pointer" }}>Allosteric BRAF</button>
                <button type="button" onClick={() => handleQuickQuery("KRAS G12C inhibitor resistance")} style={{ padding: "4px 10px", border: "1px solid var(--muted)", background: "var(--bg)", borderRadius: 4, cursor: "pointer" }}>KRAS G12C resistance</button>
                <button type="button" onClick={() => handleQuickQuery("EGFR exon 19 deletion vs T790M")} style={{ padding: "4px 10px", border: "1px solid var(--muted)", background: "var(--bg)", borderRadius: 4, cursor: "pointer" }}>EGFR mutations</button>
              </div>
            </form>
          )}

          {activeTab !== "research" && (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginBottom: 16 }}>
              {EVIDENCE_OPTIONS.map((opt) => (
                <button
                  key={opt.id}
                  onClick={() => { setActiveEvidence(opt.evidence); setActiveTab("evidence"); }}
                  style={{
                    padding: "8px 16px",
                    border: activeEvidence === opt.evidence ? "2px solid var(--signal)" : "1px solid var(--muted)",
                    background: activeEvidence === opt.evidence ? "var(--signal)" : "var(--bg)",
                    color: "var(--ink)",
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
          )}

          <div style={{ display: "flex", gap: 8, borderTop: "1px solid var(--muted)", paddingTop: 16, flexWrap: "wrap" }}>
            {mainTabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                style={{
                  padding: "8px 16px",
                  border: activeTab === tab.id ? "2px solid var(--signal)" : "1px solid var(--muted)",
                  background: activeTab === tab.id ? "var(--signal)" : "var(--bg)",
                  color: "var(--ink)",
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

          {researchError && (
            <div style={{ marginTop: 16, padding: 12, background: "rgba(232, 93, 74, 0.1)", border: "1px solid var(--alert)", borderRadius: 6, color: "var(--alert)" }}>
              {researchError}
            </div>
          )}

          {progress && (
            <div style={{ marginTop: 16, padding: 16, background: "var(--ink)", borderRadius: 6 }}>
              <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 8 }}>
                <span style={{ fontFamily: "var(--font-sans)", fontWeight: 600, color: "var(--bg)" }}>
                  Phase: {progress.phase}
                </span>
                <span style={{ fontFamily: "var(--font-mono)", color: "var(--signal)" }}>
                  {progress.progress_percent}% ({progress.completed_tasks}/{progress.total_tasks})
                </span>
              </div>
              <div style={{ height: 6, background: "var(--bg)", borderRadius: 3, overflow: "hidden" }}>
                <div style={{ 
                  width: `${progress.progress_percent}%`, 
                  height: "100%", 
                  background: "var(--signal)", 
                  transition: "width 0.3s ease" 
                }} />
              </div>
              {progress.current_task && (
                <div style={{ marginTop: 8, fontFamily: "var(--font-mono)", fontSize: "12px", color: "var(--muted)" }}>
                  Current: {progress.current_task}
                </div>
              )}
            </div>
          )}
        </header>

        {activeTab === "evidence" && (
          <>
            <section style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 24, marginBottom: 32, alignItems: "start" }}>
              <div>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "16px", fontWeight: 600, marginBottom: 12, color: "var(--ink)" }}>
                  Confidence Heatmap
                </h2>
                <ConfidenceHeatmap evidence={displayEvidence} />
              </div>
              <div>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "16px", fontWeight: 600, marginBottom: 12, color: "var(--ink)" }}>
                  Signal Breakdown
                </h2>
                <div style={{ fontFamily: "var(--font-mono)", fontSize: "12px", lineHeight: 2, color: "var(--ink)" }}>
                  {displayEvidence.confidence.signals ? (
                    <>
                      <div>Literature: <span style={{ color: "var(--signal)" }}>{Math.round(displayEvidence.confidence.signals.literature * 100)}%</span></div>
                      <div>Protein Evidence: <span style={{ color: "var(--structural)" }}>{Math.round(displayEvidence.confidence.signals.protein_evidence * 100)}%</span></div>
                      <div>Clinical Evidence: <span style={{ color: "var(--alert)" }}>{Math.round(displayEvidence.confidence.signals.clinical_evidence * 100)}%</span></div>
                      <div>LLM Rating: <span style={{ color: "var(--muted)" }}>{Math.round(displayEvidence.confidence.signals.llm_rating * 100)}%</span></div>
                    </>
                  ) : (
                    <div style={{ color: "var(--muted)" }}>Per-signal breakdown unavailable for this response</div>
                  )}
                  <div style={{ borderTop: "1px solid var(--muted)", paddingTop: 8, marginTop: 8, fontWeight: 600 }}>
                    Overall: <span style={{ color: "var(--signal)" }}>{Math.round(displayEvidence.confidence.overall * 100)}%</span>
                  </div>
                </div>
              </div>
            </section>

            {displayStructureSource && (
              <section style={{ marginBottom: 32 }}>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                  Protein Structure: {displayStructureSource.title}
                </h2>
                <StructureViewer
                  pdbId={(displayStructureSource.payload as any).pdbId}
                  mutationResidue={(displayStructureSource.payload as any).mutationResidue}
                  bindingPocket={(displayStructureSource.payload as any).bindingPocket}
                  width={700}
                  height={450}
                />
              </section>
            )}

            {displayMoleculeSource && (
              <section style={{ marginBottom: 32 }}>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                  Molecule: {displayMoleculeSource.title}
                </h2>
                <MoleculeViewer
                  smiles={(displayMoleculeSource.payload as any).smiles}
                  width={500}
                  height={350}
                />
              </section>
            )}

            {displayEvidence.sources.some((s) => s.stance) && (
              <section style={{ marginBottom: 32 }}>
                <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                  Contradiction Graph
                </h2>
                <ContradictionGraph sources={displayEvidence.sources} claim={displayEvidence.claim} />
              </section>
            )}

            <section style={{ marginBottom: 32 }}>
              <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                Tool Execution Graph
              </h2>
              <ToolExecutionGraph toolCalls={displayEvidence.toolCalls} onNodeClick={handleNodeClick} />
            </section>

            <section>
              <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
                Raw Evidence
              </h2>
              <pre style={{ fontFamily: "var(--font-mono)", fontSize: "12px", lineHeight: 1.6, overflow: "auto", background: "var(--ink)", color: "var(--bg)", padding: 16, borderRadius: 6 }}>
                {JSON.stringify(displayEvidence, null, 2)}
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
                    color: "var(--ink)",
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
            <div style={{ marginBottom: 16, fontFamily: "var(--font-mono)", fontSize: "12px", color: "var(--muted)" }}>
              {agentsLoading ? "Loading agents..." : `${agents.length} agents registered`}
            </div>
            <AgentPanel />
          </section>
        )}

        {activeTab === "workflows" && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Workflow Orchestration
            </h2>
            <div style={{ marginBottom: 16, display: "flex", gap: 16, flexWrap: "wrap", fontFamily: "var(--font-mono)", fontSize: "12px", color: "var(--muted)" }}>
              <span>Workflow Engine: {workflowsLoading ? "Loading..." : "Connected"}</span>
              <span>MCP Server: {mcpWorkflows.length} workflows</span>
            </div>
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
            {executorError && (
              <div style={{ marginBottom: 16, padding: 12, background: "rgba(232, 93, 74, 0.1)", border: "1px solid var(--alert)", borderRadius: 6, color: "var(--alert)" }}>
                {executorError}
              </div>
            )}
            <NotificationPanel />
          </section>
        )}

        {activeTab === "research" && researchEvidence && (
          <section>
            <h2 style={{ fontFamily: "var(--font-sans)", fontSize: "18px", fontWeight: 600, marginBottom: 12 }}>
              Research Results
            </h2>
            <div style={{ marginBottom: 24 }}>
              <h3 style={{ fontFamily: "var(--font-sans)", fontSize: "16px", fontWeight: 600, marginBottom: 8 }}>
                Query: {activeQuery?.query}
              </h3>
              {activePlan && (
                <div style={{ fontFamily: "var(--font-mono)", fontSize: "12px", color: "var(--muted)" }}>
                  Workflow ID: {activePlan.workflow_id} | Tasks: {activePlan.tasks.length}
                </div>
              )}
            </div>
            <ConfidenceHeatmap evidence={researchEvidence.evidence_card} />
            {researchEvidence.evidence_card.sources.some((s) => s.stance) && (
              <section style={{ marginTop: 24 }}>
                <h3 style={{ fontFamily: "var(--font-sans)", fontSize: "16px", fontWeight: 600, marginBottom: 12 }}>
                  Contradiction Graph
                </h3>
                <ContradictionGraph sources={researchEvidence.evidence_card.sources} claim={researchEvidence.evidence_card.claim} />
              </section>
            )}
            <section style={{ marginTop: 24 }}>
              <h3 style={{ fontFamily: "var(--font-sans)", fontSize: "16px", fontWeight: 600, marginBottom: 12 }}>
                Tool Execution Graph
              </h3>
              <ToolExecutionGraph toolCalls={researchEvidence.evidence_card.toolCalls} onNodeClick={handleNodeClick} />
            </section>
          </section>
        )}
      </main>
    </>
  );
}

export default App;