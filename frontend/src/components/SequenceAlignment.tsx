import { useMemo } from "react";
import "./SequenceAlignment.css";

interface SequenceAlignmentProps {
  sequence1: string;
  sequence2: string;
  label1?: string;
  label2?: string;
}

export function SequenceAlignment({ sequence1, sequence2, label1 = "Sequence 1", label2 = "Sequence 2" }: SequenceAlignmentProps) {
  const { seq1, seq2, matches, identity, conservedRuns } = useMemo(() => {
    return computeAlignment(sequence1, sequence2);
  }, [sequence1, sequence2]);

  return (
    <div className="sequence-alignment" role="img" aria-label={`Sequence alignment: ${label1} vs ${label2}`}>
      <div className="sequence-alignment__header">
        <h3>{label1} vs {label2}</h3>
        <div className="sequence-alignment__stats">
          <span>Length: {sequence1.length} / {sequence2.length}</span>
          <span>Identity: {identity}%</span>
          <span>Conserved runs (≥3): {conservedRuns.length}</span>
        </div>
      </div>

      <div className="sequence-alignment__sequences">
        <SequenceRow label={label1} sequence={seq1} matches={matches} />
        <MatchRow matches={matches} />
        <SequenceRow label={label2} sequence={seq2} matches={matches} />
      </div>

      {conservedRuns.length > 0 && (
        <div className="sequence-alignment__conserved">
          <h4>Conserved Regions (≥3 residues)</h4>
          <div className="sequence-alignment__conserved-list">
            {conservedRuns.map((run, i) => (
              <div key={i} className="sequence-alignment__conserved-run">
                <span className="sequence-alignment__run-pos">{run.start1 + 1}-{run.end1 + 1}</span>
                <span className="sequence-alignment__run-seq">{run.sequence}</span>
                <span className="sequence-alignment__run-len">{run.length} residues</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function SequenceRow({ label, sequence, matches }: { label: string; sequence: string; matches: boolean[] }) {
  return (
    <div className="sequence-alignment__row">
      <span className="sequence-alignment__label">{label}</span>
      <div className="sequence-alignment__sequence">
        {sequence.split("").map((char, i) => (
          <span
            key={i}
            className={`sequence-alignment__residue ${matches[i] ? "sequence-alignment__residue--match" : ""}`}
            title={`Position ${i + 1}: ${char}`}
          >
            {char}
          </span>
        ))}
      </div>
    </div>
  );
}

function MatchRow({ matches }: { matches: boolean[] }) {
  return (
    <div className="sequence-alignment__row">
      <span className="sequence-alignment__label" aria-hidden="true">Match</span>
      <div className="sequence-alignment__match-line">
        {matches.map((match, i) => (
          <span key={i} className={match ? "sequence-alignment__match" : "sequence-alignment__mismatch"}>
            {match ? "|" : "·"}
          </span>
        ))}
      </div>
    </div>
  );
}

function computeAlignment(seq1: string, seq2: string) {
  // Simple pairwise alignment using Needleman-Wunsch (global alignment)
  const n = seq1.length;
  const m = seq2.length;
  
  // Scoring
  const MATCH = 2;
  const MISMATCH = -1;
  const GAP = -2;

  // DP matrix
  const dp: number[][] = Array(n + 1).fill(0).map(() => Array(m + 1).fill(0));
  const trace: string[][] = Array(n + 1).fill(0).map(() => Array(m + 1).fill(""));

  // Initialize
  for (let i = 1; i <= n; i++) {
    dp[i][0] = i * GAP;
    trace[i][0] = "up";
  }
  for (let j = 1; j <= m; j++) {
    dp[0][j] = j * GAP;
    trace[0][j] = "left";
  }

  // Fill
  for (let i = 1; i <= n; i++) {
    for (let j = 1; j <= m; j++) {
      const matchScore = seq1[i - 1] === seq2[j - 1] ? MATCH : MISMATCH;
      const diag = dp[i - 1][j - 1] + matchScore;
      const up = dp[i - 1][j] + GAP;
      const left = dp[i][j - 1] + GAP;

      let max = diag;
      let dir = "diag";
      if (up > max) { max = up; dir = "up"; }
      if (left > max) { max = left; dir = "left"; }

      dp[i][j] = max;
      trace[i][j] = dir;
    }
  }

  // Traceback
  let aligned1 = "";
  let aligned2 = "";
  let i = n, j = m;

  while (i > 0 || j > 0) {
    if (i > 0 && j > 0 && trace[i][j] === "diag") {
      aligned1 = seq1[i - 1] + aligned1;
      aligned2 = seq2[j - 1] + aligned2;
      i--; j--;
    } else if (i > 0 && trace[i][j] === "up") {
      aligned1 = seq1[i - 1] + aligned1;
      aligned2 = "-" + aligned2;
      i--;
    } else {
      aligned1 = "-" + aligned1;
      aligned2 = seq2[j - 1] + aligned2;
      j--;
    }
  }

  // Calculate matches on aligned sequences
  const matches = aligned1.split("").map((char, idx) => char === aligned2[idx] && char !== "-");
  const matchCount = matches.filter(Boolean).length;
  const identity = Math.round((matchCount / Math.max(aligned1.length, aligned2.length)) * 100);

  // Find conserved runs (≥3 consecutive matches)
  const conservedRuns: { start1: number; end1: number; start2: number; end2: number; sequence: string; length: number }[] = [];
  let runStart = -1;
  let runSeq = "";
  let pos1 = 0, pos2 = 0;

  for (let idx = 0; idx < aligned1.length; idx++) {
    const c1 = aligned1[idx];
    const c2 = aligned2[idx];
    const isMatch = c1 === c2 && c1 !== "-";

    if (isMatch) {
      if (runStart === -1) {
        runStart = idx;
        runSeq = "";
      }
      runSeq += c1;
      if (c1 !== "-") pos1++;
      if (c2 !== "-") pos2++;
    } else {
      if (runStart !== -1 && runSeq.length >= 3) {
        conservedRuns.push({
          start1: pos1 - runSeq.length,
          end1: pos1 - 1,
          start2: pos2 - runSeq.length,
          end2: pos2 - 1,
          sequence: runSeq,
          length: runSeq.length,
        });
      }
      runStart = -1;
      runSeq = "";
      if (c1 !== "-") pos1++;
      if (c2 !== "-") pos2++;
    }
  }

  // Check final run
  if (runStart !== -1 && runSeq.length >= 3) {
    conservedRuns.push({
      start1: pos1 - runSeq.length,
      end1: pos1 - 1,
      start2: pos2 - runSeq.length,
      end2: pos2 - 1,
      sequence: runSeq,
      length: runSeq.length,
    });
  }

  return {
    seq1: aligned1,
    seq2: aligned2,
    matches,
    identity,
    conservedRuns,
  };
}