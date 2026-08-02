import { useCallback, useRef, useState } from "react";
import "./PathwayViewer.css";

interface PathwayNode {
  id: string;
  name: string;
  type: "gene" | "protein" | "compound" | "reaction" | "pathway" | "complex";
  x: number;
  y: number;
  width?: number;
  height?: number;
  color?: string;
  fgColor?: string;
  bgColor?: string;
  data?: Record<string, any>;
}

interface PathwayEdge {
  source: string;
  target: string;
  type: "activation" | "inhibition" | "expression" | "binding" | "phosphorylation" | "dephosphorylation" | "translocation" | "catalysis" | "complex_formation" | "dissociation";
  label?: string;
}

interface PathwayData {
  nodes: PathwayNode[];
  edges: PathwayEdge[];
  name: string;
  description?: string;
  source: "KEGG" | "Reactome";
  pathwayId: string;
  imageUrl?: string;
}

interface PathwayViewerProps {
  pathwayData: PathwayData | null;
  width?: number;
  height?: number;
  onNodeClick?: (node: PathwayNode) => void;
  showLegend?: boolean;
}

const NODE_COLORS: Record<string, string> = {
  gene: "var(--signal)",
  protein: "var(--structural)",
  compound: "var(--alert)",
  reaction: "#F59E0B",
  pathway: "var(--muted)",
  complex: "#8B5CF6",
};

const EDGE_COLORS: Record<string, string> = {
  activation: "var(--signal)",
  inhibition: "var(--alert)",
  expression: "var(--structural)",
  binding: "#8B5CF6",
  phosphorylation: "#F59E0B",
  dephosphorylation: "#F59E0B",
  translocation: "#6366F1",
  catalysis: "#EC4899",
  complex_formation: "#14B8A6",
  dissociation: "#EF4444",
};

const EDGE_ARROWS: Record<string, "arrow" | "circle" | "none"> = {
  activation: "arrow",
  inhibition: "circle",
  expression: "arrow",
  binding: "none",
  phosphorylation: "arrow",
  dephosphorylation: "arrow",
  translocation: "arrow",
  catalysis: "arrow",
  complex_formation: "none",
  dissociation: "none",
};

export function PathwayViewer({
  pathwayData,
  width = 800,
  height = 600,
  onNodeClick,
  showLegend = true,
}: PathwayViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const [transform, setTransform] = useState({ x: 0, y: 0, scale: 1 });
  const [selectedNode, setSelectedNode] = useState<PathwayNode | null>(null);
  const isPanning = useRef(false);
  const panStart = useRef({ x: 0, y: 0 });
  const bounds = useRef({ minX: 0, maxX: width, minY: 0, maxY: height });

  const fitToView = useCallback(() => {
    const { minX, maxX, minY, maxY } = bounds.current;
    const contentWidth = maxX - minX;
    const contentHeight = maxY - minY;
    const scaleX = (width - 100) / contentWidth;
    const scaleY = (height - 100) / contentHeight;
    const scale = Math.min(scaleX, scaleY, 2);
    const centerX = (minX + maxX) / 2;
    const centerY = (minY + maxY) / 2;

    setTransform({
      x: width / 2 - centerX * scale,
      y: height / 2 - centerY * scale,
      scale,
    });
  }, [width, height]);


  if (!pathwayData) {
    return (
      <div className="pathway-viewer" style={{ width, height }}>
        <div className="pathway-viewer__empty">
          <p>No pathway data available</p>
        </div>
      </div>
    );
  }

  const handleWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const rect = containerRef.current?.getBoundingClientRect();
    if (!rect) return;

    const mouseX = e.clientX - rect.left - transform.x;
    const mouseY = e.clientY - rect.top - transform.y;

    const zoomFactor = e.deltaY > 0 ? 0.9 : 1.1;
    const newScale = Math.max(0.1, Math.min(5, transform.scale * zoomFactor));

    setTransform(prev => ({
      x: mouseX - (mouseX - prev.x) * (newScale / prev.scale),
      y: mouseY - (mouseY - prev.y) * (newScale / prev.scale),
      scale: newScale,
    }));
  };

  const handleMouseDown = (e: React.MouseEvent) => {
    if (e.button === 1 || (e.button === 0 && e.altKey)) {
      isPanning.current = true;
      panStart.current = { x: e.clientX - transform.x, y: e.clientY - transform.y };
      e.preventDefault();
    }
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (isPanning.current) {
      setTransform(prev => ({
        ...prev,
        x: e.clientX - panStart.current.x,
        y: e.clientY - panStart.current.y,
      }));
    }
  };

  const handleMouseUp = () => {
    isPanning.current = false;
  };

  const handleNodeClick = (node: PathwayNode, e: React.MouseEvent) => {
    e.stopPropagation();
    setSelectedNode(node);
    onNodeClick?.(node);
  };

  const resetView = () => {
    setTransform({ x: 0, y: 0, scale: 1 });
    setSelectedNode(null);
  };



  return (
    <div
      className="pathway-viewer"
      style={{ width, height }}
      ref={containerRef}
      onWheel={handleWheel}
      onMouseDown={handleMouseDown}
      onMouseMove={handleMouseMove}
      onMouseUp={handleMouseUp}
      onMouseLeave={handleMouseUp}
      onClick={() => setSelectedNode(null)}
    >
      <svg
        ref={svgRef}
        className="pathway-viewer__svg"
        style={{
          transform: `translate(${transform.x}px, ${transform.y}px) scale(${transform.scale})`,
          transformOrigin: "0 0",
        }}
      >
        <defs>
          <marker
            id="arrowhead"
            markerWidth="10"
            markerHeight="7"
            refX="9"
            refY="3.5"
            orient="auto"
          >
            <polygon points="0 0, 10 3.5, 0 7" fill="var(--ink)" />
          </marker>
          <marker
            id="circlehead"
            markerWidth="10"
            markerHeight="10"
            refX="5"
            refY="5"
            orient="auto"
          >
            <circle cx="5" cy="5" r="4" fill="none" stroke="var(--alert)" strokeWidth="2" />
          </marker>
        </defs>

        {pathwayData.imageUrl && (
          <image
            href={pathwayData.imageUrl}
            x={bounds.current.minX}
            y={bounds.current.minY}
            width={bounds.current.maxX - bounds.current.minX}
            height={bounds.current.maxY - bounds.current.minY}
            opacity={0.3}
            preserveAspectRatio="none"
          />
        )}

        {pathwayData.edges.map((edge, idx) => (
          <PathwayEdgeComponent
            key={`edge-${idx}`}
            edge={edge}
            nodes={pathwayData.nodes}
            color={EDGE_COLORS[edge.type] || "var(--ink)"}
            marker={EDGE_ARROWS[edge.type] === "arrow" ? "url(#arrowhead)" : EDGE_ARROWS[edge.type] === "circle" ? "url(#circlehead)" : "none"}
          />
        ))}

        {pathwayData.nodes.map((node) => (
          <PathwayNodeComponent
            key={node.id}
            node={node}
            color={NODE_COLORS[node.type] || "var(--ink)"}
            selected={selectedNode?.id === node.id}
            onClick={handleNodeClick}
          />
        ))}
      </svg>

      <div className="pathway-viewer__controls">
        <button className="pathway-viewer__btn" onClick={resetView} title="Reset view">
          ⌂
        </button>
        <button className="pathway-viewer__btn" onClick={fitToView} title="Fit to view">
          ◻
        </button>
        <button className="pathway-viewer__btn" onClick={() => setTransform(p => ({ ...p, scale: p.scale * 1.2 }))} title="Zoom in">
          +
        </button>
        <button className="pathway-viewer__btn" onClick={() => setTransform(p => ({ ...p, scale: Math.max(0.1, p.scale * 0.8) }))} title="Zoom out">
          −
        </button>
      </div>

      {showLegend && (
        <div className="pathway-viewer__legend">
          <h4>Node Types</h4>
          <div className="pathway-viewer__legend-items">
            {Object.entries(NODE_COLORS).map(([type, color]) => (
              <div key={type} className="pathway-viewer__legend-item">
                <span className="pathway-viewer__legend-color" style={{ background: color }} />
                <span className="pathway-viewer__legend-label">{type}</span>
              </div>
            ))}
          </div>
          <h4>Edge Types</h4>
          <div className="pathway-viewer__legend-items">
            {Object.entries(EDGE_COLORS).map(([type, color]) => (
              <div key={type} className="pathway-viewer__legend-item">
                <span className="pathway-viewer__legend-color" style={{ background: color }} />
                <span className="pathway-viewer__legend-label">{type}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {selectedNode && (
        <div className="pathway-viewer__node-detail">
          <h4>{selectedNode.name}</h4>
          <p className="pathway-viewer__node-type">Type: {selectedNode.type}</p>
          {selectedNode.data && (
            <details>
              <summary>Details</summary>
              <pre>{JSON.stringify(selectedNode.data, null, 2)}</pre>
            </details>
          )}
          <button onClick={() => setSelectedNode(null)}>Close</button>
        </div>
      )}
    </div>
  );
}

interface PathwayNodeComponentProps {
  node: PathwayNode;
  color: string;
  selected: boolean;
  onClick: (node: PathwayNode, e: React.MouseEvent) => void;
}

function PathwayNodeComponent({ node, color, selected, onClick }: PathwayNodeComponentProps) {
  const { x, y, width = 60, height = 30, type } = node;

  const shape = type === "compound" ? "circle" : type === "reaction" ? "diamond" : "rect";

  return (
    <g onClick={(e) => onClick(node, e)} style={{ cursor: "pointer" }}>
      {shape === "rect" && (
        <rect
          x={x - width / 2}
          y={y - height / 2}
          width={width}
          height={height}
          rx={4}
          ry={4}
          fill={color}
          opacity={selected ? 1 : 0.8}
          stroke={selected ? "var(--alert)" : "none"}
          strokeWidth={selected ? 2 : 0}
          filter={selected ? "drop-shadow(0 0 4px var(--alert))" : "none"}
        />
      )}
      {shape === "circle" && (
        <circle
          cx={x}
          cy={y}
          r={Math.max(width, height) / 2}
          fill={color}
          opacity={selected ? 1 : 0.8}
          stroke={selected ? "var(--alert)" : "none"}
          strokeWidth={selected ? 2 : 0}
          filter={selected ? "drop-shadow(0 0 4px var(--alert))" : "none"}
        />
      )}
      {shape === "diamond" && (
        <polygon
          points={`${x},${y - height / 2} ${x + width / 2},${y} ${x},${y + height / 2} ${x - width / 2},${y}`}
          fill={color}
          opacity={selected ? 1 : 0.8}
          stroke={selected ? "var(--alert)" : "none"}
          strokeWidth={selected ? 2 : 0}
          filter={selected ? "drop-shadow(0 0 4px var(--alert))" : "none"}
        />
      )}
      <text
        x={x}
        y={y + height / 2 + 14}
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="10"
        fill="var(--ink)"
        style={{ pointerEvents: "none" }}
      >
        {node.name.length > 12 ? node.name.slice(0, 12) + "…" : node.name}
      </text>
    </g>
  );
}

interface PathwayEdgeComponentProps {
  edge: PathwayEdge;
  nodes: PathwayNode[];
  color: string;
  marker: string;
}

function PathwayEdgeComponent({ edge, nodes, color, marker }: PathwayEdgeComponentProps) {
  const sourceNode = nodes.find(n => n.id === edge.source);
  const targetNode = nodes.find(n => n.id === edge.target);

  if (!sourceNode || !targetNode) return null;

  const x1 = sourceNode.x;
  const y1 = sourceNode.y;
  const x2 = targetNode.x;
  const y2 = targetNode.y;

  const dx = x2 - x1;
  const dy = y2 - y1;
  const dist = Math.sqrt(dx * dx + dy * dy);

  const offset = 15;
  const sx = x1 + (dx / dist) * offset;
  const sy = y1 + (dy / dist) * offset;
  const tx = x2 - (dx / dist) * offset;
  const ty = y2 - (dy / dist) * offset;

  const midX = (sx + tx) / 2;
  const midY = (sy + ty) / 2;

  return (
    <g>
      <path
        d={`M ${sx} ${sy} L ${tx} ${ty}`}
        stroke={color}
        strokeWidth={2}
        fill="none"
        markerEnd={marker}
        style={{ opacity: 0.7 }}
      />
      {edge.label && (
        <text
          x={midX}
          y={midY - 5}
          textAnchor="middle"
          fontFamily="var(--font-mono)"
          fontSize="9"
          fill="var(--muted)"
          style={{ pointerEvents: "none" }}
        >
          {edge.label}
        </text>
      )}
    </g>
  );
}

export default PathwayViewer;