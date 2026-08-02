import { useEffect, useRef, useState } from "react";
import "./StructureViewer.css";

interface StructureViewerProps {
  pdbId: string;
  mutationResidue?: { chain: string; position: number };
  bindingPocket?: { chain: string; positions: number[] };
  style?: "cartoon" | "stick" | "surface";
  width?: number;
  height?: number;
}

export function StructureViewer({
  pdbId,
  mutationResidue,
  bindingPocket,
  style = "cartoon",
  width = 600,
  height = 400,
}: StructureViewerProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [viewer, setViewer] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;

    const initViewer = async () => {
      try {
        // Load 3Dmol.js from CDN
        if (typeof window !== "undefined" && !(window as any).$3Dmol) {
          await loadScript("https://cdnjs.cloudflare.com/ajax/libs/3Dmol/2.0.0/3Dmol-min.js");
        }

        if (!mounted) return;

        const $3Dmol = (window as any).$3Dmol;
        if (!$3Dmol) {
          throw new Error("3Dmol failed to load");
        }

        const element = containerRef.current;
        if (!element) return;

        const viewerInstance = $3Dmol.createViewer(element, {
          backgroundColor: "white",
          antialias: true,
        });

        // Load PDB file
        const pdbUrl = `https://files.rcsb.org/download/${pdbId}.pdb`;
        const response = await fetch(pdbUrl);
        if (!response.ok) {
          throw new Error(`Failed to load PDB ${pdbId}: ${response.status}`);
        }
        const pdbData = await response.text();

        viewerInstance.addModel(pdbData, "pdb");
        viewerInstance.setStyle({}, { [style]: { color: "spectrum" } });
        viewerInstance.zoomTo();

        // Highlight mutation residue
        if (mutationResidue) {
          const { chain, position } = mutationResidue;
          viewerInstance.addStyle(
            { chain, resi: position },
            { stick: { color: "var(--alert)", radius: 0.8 }, sphere: { color: "var(--alert)", radius: 1.0 } }
          );
          viewerInstance.addLabel(
            `Mutation ${position}`,
            { position: { chain, resi: position }, backgroundColor: "var(--alert)", fontColor: "white", fontSize: 12 },
            { chain, resi: position }
          );
        }

        // Highlight binding pocket
        if (bindingPocket) {
          const { chain, positions } = bindingPocket;
          positions.forEach((pos) => {
            viewerInstance.addStyle(
              { chain, resi: pos },
              { stick: { color: "var(--structural)", radius: 0.6 }, sphere: { color: "var(--structural)", radius: 0.5 } }
            );
          });
          // Add pocket surface
          viewerInstance.addSurface($3Dmol.VDW, { opacity: 0.3, color: "var(--structural)" }, { chain, resi: positions.join("+") });
        }

        viewerInstance.render();
        viewerInstance.zoomTo();

        if (mounted) {
          setViewer(viewerInstance);
          setLoading(false);
        }
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : "Failed to load structure");
          setLoading(false);
        }
      }
    };

    initViewer();

    return () => {
      mounted = false;
      if (viewer) {
        viewer.remove();
      }
    };
  }, [pdbId, mutationResidue, bindingPocket, style, viewer]);

  useEffect(() => {
    if (viewer) {
      viewer.resize();
      viewer.render();
    }
  }, [width, height, viewer]);

  if (loading) {
    return (
      <div className="structure-viewer" style={{ width, height }}>
        <div className="structure-viewer__loading">
          <div className="structure-viewer__spinner" />
          <span>Loading {pdbId}...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="structure-viewer" style={{ width, height }}>
        <div className="structure-viewer__error">
          <span>⚠</span>
          <p>{error}</p>
          <button onClick={() => window.location.reload()}>Retry</button>
        </div>
      </div>
    );
  }

  return (
    <div className="structure-viewer" style={{ width, height }} ref={containerRef}>
      {viewer && (
        <div className="structure-viewer__legend">
          <div className="structure-viewer__legend-item">
            <span className="structure-viewer__legend-color" style={{ background: "var(--alert)" }} />
            Mutation
          </div>
          {bindingPocket && (
            <div className="structure-viewer__legend-item">
              <span className="structure-viewer__legend-color" style={{ background: "var(--structural)" }} />
              Binding Pocket
            </div>
          )}
          <div className="structure-viewer__legend-item">
            <span className="structure-viewer__legend-color" style={{ background: "linear-gradient(90deg, var(--signal), var(--structural))" }} />
            Protein
          </div>
        </div>
      )}
    </div>
  );
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if ((window as any).$3Dmol) {
      resolve();
      return;
    }
    const script = document.createElement("script");
    script.src = src;
    script.async = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error(`Failed to load ${src}`));
    document.head.appendChild(script);
  });
}