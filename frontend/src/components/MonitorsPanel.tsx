import { useCallback, useEffect, useState } from "react";
import { ConfidenceTrend, TREND_COLORS, type TrendPoint } from "./ConfidenceTrend";
import { apiConfig } from "../api/client";
import "./MonitorsPanel.css";

interface SignalBreakdown {
  literature: number;
  protein_evidence: number;
  clinical_evidence: number;
  llm_rating: number;
}

interface Check {
  id: number;
  checked_at: string;
  verdict: string;
  confidence: number;
  signal_breakdown?: SignalBreakdown | null;
  source_count: number;
  changed: boolean;
  change_note?: string;
}

interface Monitor {
  id: string;
  claim: string;
  interval_seconds: number;
  created_at: string;
  latest?: Check | null;
}

async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const token = localStorage.getItem("ro.auth.token");
  const res = await fetch(`${apiConfig.base}/api/v1${path}`, {
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    ...options,
  });
  if (res.status === 401) window.dispatchEvent(new CustomEvent("ro:auth-required"));
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || `${res.status}`);
  return body as T;
}

function toTrendPoints(checks: Check[]): TrendPoint[] {
  return checks.map((c) => ({
    checkedAt: c.checked_at,
    confidence: c.confidence,
    verdict: c.verdict,
    changed: c.changed,
    changeNote: c.change_note,
    signals: c.signal_breakdown ?? null,
  }));
}

export function MonitorsPanel() {
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [history, setHistory] = useState<Check[]>([]);
  const [claim, setClaim] = useState("");
  const [intervalHours, setIntervalHours] = useState(24);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const out = await api<{ monitors: Monitor[] }>("/monitors");
      setMonitors(out.monitors ?? []);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "load failed");
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    if (!selected) return;
    api<{ checks: Check[] }>(`/monitors/${selected}/history`)
      .then((out) => setHistory(out.checks ?? []))
      .catch(() => setHistory([]));
  }, [selected]);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!claim.trim()) return;
    setBusy(true);
    try {
      const m = await api<Monitor>("/monitors", {
        method: "POST",
        body: JSON.stringify({ claim: claim.trim(), interval_hours: intervalHours }),
      });
      setClaim("");
      await refresh();
      setSelected(m.id);
      setError(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "create failed");
    } finally {
      setBusy(false);
    }
  };

  const checkNow = async (id: string) => {
    setBusy(true);
    setError(null);
    try {
      await api<Check>(`/monitors/${id}/check`, { method: "POST" });
      await refresh();
      if (selected === id) {
        const out = await api<{ checks: Check[] }>(`/monitors/${id}/history`);
        setHistory(out.checks ?? []);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "check failed");
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    try {
      await api(`/monitors/${id}`, { method: "DELETE" });
      if (selected === id) {
        setSelected(null);
        setHistory([]);
      }
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "delete failed");
    }
  };

  const selectedMonitor = monitors.find((m) => m.id === selected) ?? null;

  return (
    <div className="monitors-panel">
      <p className="monitors-panel__blurb">
        A monitored claim is re-evaluated on schedule through the same grounded retrieval + reasoning
        path as an ad-hoc query. A check is flagged when the verdict flips or confidence moves by ≥10
        points — the change a keyword alert can't see.
      </p>

      <form onSubmit={create} className="monitors-panel__form">
        <input
          value={claim}
          onChange={(e) => setClaim(e.target.value)}
          placeholder='Claim to watch, e.g. "Sotorasib resistance emerges via KRAS Y96D"'
          className="monitors-panel__claim-input"
        />
        <select
          value={intervalHours}
          onChange={(e) => setIntervalHours(Number(e.target.value))}
          className="monitors-panel__interval"
          aria-label="Check interval"
        >
          <option value={24}>daily</option>
          <option value={168}>weekly</option>
          <option value={1}>hourly (demo)</option>
        </select>
        <button type="submit" disabled={busy || !claim.trim()} className="monitors-panel__create">
          Watch claim
        </button>
      </form>

      {error && <div className="monitors-panel__error">{error}</div>}

      <div className="monitors-panel__list">
        {monitors.length === 0 && (
          <div className="monitors-panel__empty">No monitored claims yet.</div>
        )}
        {monitors.map((m) => (
          <div
            key={m.id}
            className={`monitors-panel__row ${selected === m.id ? "monitors-panel__row--active" : ""}`}
            onClick={() => setSelected(m.id)}
          >
            <div className="monitors-panel__row-main">
              <span className="monitors-panel__row-claim">{m.claim}</span>
              <span className="monitors-panel__row-meta">
                every {m.interval_seconds >= 86400 ? `${Math.round(m.interval_seconds / 86400)}d` : `${Math.round(m.interval_seconds / 3600)}h`}
                {m.latest && (
                  <>
                    {" · "}
                    <span data-verdict={m.latest.verdict} className="monitors-panel__verdict">{m.latest.verdict}</span>
                    {" "}{Math.round(m.latest.confidence * 100)}%
                    {m.latest.changed && <span className="monitors-panel__flag"> ⚑ changed</span>}
                  </>
                )}
              </span>
            </div>
            <div className="monitors-panel__row-actions">
              <button disabled={busy} onClick={(e) => { e.stopPropagation(); checkNow(m.id); }}>
                {busy ? "…" : "check now"}
              </button>
              <button onClick={(e) => { e.stopPropagation(); remove(m.id); }}>stop</button>
            </div>
          </div>
        ))}
      </div>

      {selectedMonitor && (
        <div className="monitors-panel__detail">
          <h3 className="monitors-panel__detail-title">{selectedMonitor.claim}</h3>
          <ConfidenceTrend points={toTrendPoints(history)} />

          {history.length > 0 && (
            <table className="monitors-panel__table">
              <thead>
                <tr>
                  <th>Checked</th>
                  <th>Verdict</th>
                  <th>Overall</th>
                  <th style={{ color: TREND_COLORS.literature }}>Lit</th>
                  <th style={{ color: TREND_COLORS.protein_evidence }}>Prot</th>
                  <th style={{ color: TREND_COLORS.clinical_evidence }}>Clin</th>
                  <th style={{ color: TREND_COLORS.llm_rating }}>LLM</th>
                  <th>Sources</th>
                  <th>Change</th>
                </tr>
              </thead>
              <tbody>
                {[...history].reverse().map((c) => (
                  <tr key={c.id} className={c.changed ? "monitors-panel__table-row--changed" : ""}>
                    <td>{new Date(c.checked_at).toLocaleString()}</td>
                    <td><span data-verdict={c.verdict} className="monitors-panel__verdict">{c.verdict}</span></td>
                    <td>{Math.round(c.confidence * 100)}%</td>
                    <td>{c.signal_breakdown ? `${Math.round(c.signal_breakdown.literature * 100)}%` : "—"}</td>
                    <td>{c.signal_breakdown ? `${Math.round(c.signal_breakdown.protein_evidence * 100)}%` : "—"}</td>
                    <td>{c.signal_breakdown ? `${Math.round(c.signal_breakdown.clinical_evidence * 100)}%` : "—"}</td>
                    <td>{c.signal_breakdown ? `${Math.round(c.signal_breakdown.llm_rating * 100)}%` : "—"}</td>
                    <td>{c.source_count}</td>
                    <td>{c.changed ? c.change_note : ""}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
}
