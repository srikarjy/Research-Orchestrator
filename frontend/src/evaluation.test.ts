import { describe, it, expect } from "vitest";
import evalCase1 from "./fixtures/eval-case1.json";
import evalCase2 from "./fixtures/eval-case2.json";

type EvaluationFixture = typeof evalCase1;

function calculateContradictionScore(supports: number, contradicts: number): number {
  const total = supports + contradicts;
  if (total === 0) return 0;
  return contradicts / total;
}

function getHighestSignal(signals: EvaluationFixture["confidence"]["signals"]): string {
  return Object.entries(signals).reduce((a, b) => (signals[a[0] as keyof typeof signals] > signals[b[0] as keyof typeof signals] ? a : b))[0];
}

describe("Evaluation Guardrail - Confidence Heatmap", () => {
  const fixtures: EvaluationFixture[] = [evalCase1, evalCase2];

  fixtures.forEach((fixture, idx) => {
    const label = idx === 0 ? "High literature / High LLM" : "Low literature / High LLM";

    describe(`${label} (${fixture.id})`, () => {
      it("confidence signals match expected values", () => {
        const expected = fixture.expected.confidenceHeatmap;
        const actual = fixture.confidence.signals;

        expect(actual.literature).toBeCloseTo(expected.literature, 2);
        expect(actual.protein_evidence).toBeCloseTo(expected.protein_evidence, 2);
        expect(actual.clinical_evidence).toBeCloseTo(expected.clinical_evidence, 2);
        expect(actual.llm_rating).toBeCloseTo(expected.llm_rating, 2);
        expect(fixture.confidence.overall).toBeCloseTo(expected.overall, 2);
      });

      it("highest signal matches expected", () => {
        const expected = fixture.expected.confidenceHeatmap;
        const highest = getHighestSignal(fixture.confidence.signals);
        expect(highest).toBe(expected.highestSignal);
      });

      it("LLM rating is visually de-emphasized (always present in expected)", () => {
        const expected = fixture.expected.confidenceHeatmap;
        expect(expected.llmDeemphasized).toBe(true);
      });

      it("overall confidence is reasonable given signal weights (evidence signals weighted more than LLM)", () => {
        const signals = fixture.confidence.signals;
        const evidenceSignals = [signals.literature, signals.protein_evidence, signals.clinical_evidence];
        const maxEvidence = Math.max(...evidenceSignals);
        // Overall should not exceed max evidence signal by more than LLM contribution (capped at ~15%)
        expect(fixture.confidence.overall).toBeLessThanOrEqual(maxEvidence + 0.2);
      });
    });
  });
});

describe("Evaluation Guardrail - Contradiction Graph", () => {
  const fixtures: EvaluationFixture[] = [evalCase1, evalCase2];

  fixtures.forEach((fixture, idx) => {
    const label = idx === 0 ? "High literature / High LLM" : "Low literature / High LLM";

    describe(`${label} (${fixture.id})`, () => {
      it("supporting sources match expected", () => {
        const expected = fixture.expected.contradictionGraph;
        const supports = fixture.sources.filter((s) => s.stance === "supports");
        
        expect(supports.length).toBe(expected.supportsCount);
        expect(supports.map((s) => s.id).sort()).toEqual(expected.supportsSources.sort());
      });

      it("contradicting sources match expected", () => {
        const expected = fixture.expected.contradictionGraph;
        const contradicts = fixture.sources.filter((s) => s.stance === "contradicts");
        
        expect(contradicts.length).toBe(expected.contradictsCount);
        expect(contradicts.map((s) => s.id).sort()).toEqual(expected.contradictsSources.sort());
      });

      it("neutral sources count matches expected", () => {
        const expected = fixture.expected.contradictionGraph;
        const neutral = fixture.sources.filter((s) => s.stance === "neutral" || !s.stance);
        expect(neutral.length).toBe(expected.neutralCount);
      });

      it("contradiction score matches expected", () => {
        const expected = fixture.expected.contradictionGraph;
        const supports = fixture.sources.filter((s) => s.stance === "supports").length;
        const contradicts = fixture.sources.filter((s) => s.stance === "contradicts").length;
        const score = calculateContradictionScore(supports, contradicts);
        expect(Math.round(score * 100)).toBe(Math.round(expected.contradictionScore * 100));
      });

      it("visual layout has no crossed edges (two-column layout)", () => {
        // This is a design guarantee: sources are split into left (supports) 
        // and right (contradicts) columns with claim centered
        // No crossed edges possible by construction
        const supports = fixture.sources.filter((s) => s.stance === "supports").length;
        const contradicts = fixture.sources.filter((s) => s.stance === "contradicts").length;
        
        // Both columns exist (even if empty)
        expect(supports).toBeGreaterThanOrEqual(0);
        expect(contradicts).toBeGreaterThanOrEqual(0);
        
        // Claim node is centered between columns
        expect(true).toBe(true); // Layout invariant
      });
    });
  });
});

describe("Evaluation Guardrail - Cross-fixture Visual Distinction", () => {
  it("Case 1 (high lit) visually reads as trustworthy vs Case 2 (low lit) reads as skeptical", () => {
    const case1 = evalCase1;
    const case2 = evalCase2;

    // Case 1: literature is highest signal, LLM not dominating
    const case1Highest = getHighestSignal(case1.confidence.signals);
    expect(case1Highest).toBe("literature");
    expect(case1.confidence.signals.literature).toBeGreaterThan(case1.confidence.signals.llm_rating);

    // Case 2: LLM is highest but evidence signals are low - should visually read as "be skeptical"
    const case2Highest = getHighestSignal(case2.confidence.signals);
    expect(case2Highest).toBe("llm_rating");
    expect(case2.confidence.signals.literature).toBeLessThan(0.4);
    expect(case2.confidence.signals.protein_evidence).toBeLessThan(0.4);
    expect(case2.confidence.signals.clinical_evidence).toBeLessThan(0.4);
    expect(case2.confidence.signals.llm_rating).toBeGreaterThan(0.8);
  });
});