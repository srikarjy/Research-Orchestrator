import { useState } from "react";
import type { ToolCallTrace } from "../types/evidence";
import "./ToolExecutionGraph.css";

const CATEGORY_COLORS: Record<string, string> = {
  retriever: "var(--signal)",
  analyzer: "var(--structural)",
  visualizer: "var(--muted)",
  executor: "var(--alert)",
};

export function ToolExecutionGraph({ toolCalls, onNodeClick }: { toolCalls: ToolCallTrace[]; onNodeClick: (trace: ToolCallTrace) => void }) {
  const [selectedNode, setSelectedNode] = useState<ToolCallTrace | null>(null);

  const nodes = toolCalls.map((trace, i) => ({
    ...trace,
    x: 80 + i * 200,
    y: 100,
  }));

  return (
    <div className="tool-execution-graph">
      <svg className="tool-execution-graph__svg" viewBox="0 0 1200 300">
        {nodes.slice(1).map((node, i) => (
          <line
            key={`edge-${i}`}
            x1={nodes[i].x + 40}
            y1={nodes[i].y}
            x2={node.x}
            y2={node.y}
            stroke="var(--muted)"
            strokeWidth="2"
            strokeDasharray="6 4"
            opacity="0.5"
          />
        ))}
        {nodes.map((node) => (
          <g
            key={node.tool}
            className="tool-execution-graph__node"
            onClick={() => {
              setSelectedNode(node);
              onNodeClick(node);
            }}
            style={{
              cursor: "pointer",
              filter: node.cacheHit ? "drop-shadow(0 0 4px var(--signal))" : "none",
            }}
          >
            <circle
              cx={node.x}
              cy={node.y}
              r={32}
              fill={CATEGORY_COLORS[node.category] || "var(--muted)"}
              opacity={node.cacheHit ? 1 : 0.4}
              stroke={node.retries > 0 ? "var(--alert)" : "transparent"}
              strokeWidth={node.retries > 0 ? 3 : 0}
            />
            <text
              x={node.x}
              y={node.y - 45}
              textAnchor="middle"
              fontFamily="var(--font-mono)"
              fontSize="10"
              fill="var(--ink)"
              className="tool-execution-graph__label"
            >
              {node.tool}
            </text>
            <text
              x={node.x}
              y={node.y + 50}
              textAnchor="middle"
              fontFamily="var(--font-mono)"
              fontSize="9"
              fill="var(--muted)"
            >
              {node.latencyMs}ms
            </text>
            {node.retries > 0 && (
              <text
                x={node.x + 28}
                y={node.y - 28}
                textAnchor="middle"
                fontFamily="var(--font-mono)"
                fontSize="10"
                fill="var(--alert)"
                fontWeight="bold"
              >
                ⟳{node.retries}
              </text>
            )}
          </g>
        ))}
      </svg>

      {selectedNode && (
        <aside className="tool-execution-graph__panel">
          <header>
            <h3>{selectedNode.tool}</h3>
            <button onClick={() => setSelectedNode(null)}>✕</button>
          </header>
          <dl>
            <dt>Category</dt>
            <dd>{selectedNode.category}</dd>
            <dt>Latency</dt>
            <dd>{selectedNode.latencyMs} ms</dd>
            <dt>Cache Hit</dt>
            <dd>{selectedNode.cacheHit ? "Yes" : "No"}</dd>
            <dt>Retries</dt>
            <dd>{selectedNode.retries}</dd>
            {selectedNode.tokens && (
              <>
                <dt>Tokens</dt>
                <dd>{selectedNode.tokens}</dd>
              </>
            )}
          </dl>
        </aside>
      )}
    </div>
  );
}