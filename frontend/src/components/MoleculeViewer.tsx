import { useEffect, useState } from "react";
import "./MoleculeViewer.css";

interface MoleculeViewerProps {
  smiles: string;
  width?: number;
  height?: number;
  showAtomIndices?: boolean;
  highlightAtoms?: number[];
}

export function MoleculeViewer({
  smiles,
  width = 400,
  height = 300,
  highlightAtoms = [],
}: MoleculeViewerProps) {
  const [svg, setSvg] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [properties, setProperties] = useState<Record<string, any> | null>(null);

  const highlightKey = highlightAtoms.join(",");

  useEffect(() => {
    let mounted = true;

    const renderMolecule = async () => {
      try {
        // Load RDKit.js from CDN
        if (typeof window !== "undefined" && !(window as any).RDKit) {
          await loadScript("https://unpkg.com/@rdkit/rdkit@latest/dist/RDKit_min.js");
        }

        if (!mounted) return;

        const RDKit = (window as any).RDKit;
        if (!RDKit) {
          throw new Error("RDKit failed to load");
        }

        await RDKit.init();

        const mol = RDKit.get_mol(smiles);
        if (!mol) {
          throw new Error("Invalid SMILES");
        }

        // Compute properties
        const mw = mol.get_mw();
        const logp = mol.get_logp();
        const tpsa = mol.get_tpsa();
        const rotatable = mol.get_num_rotatable_bonds();
        const hbd = mol.get_num_hbd();
        const hba = mol.get_num_hba();

        if (mounted) {
          setProperties({
            molecular_weight: Number(mw.toFixed(2)),
            logp: Number(logp.toFixed(2)),
            tpsa: Number(tpsa.toFixed(2)),
            rotatable_bonds: rotatable,
            hbd,
            hba,
          });
        }

        // Generate SVG
        const drawer = new RDKit.SVGDrawer(width, height);
        drawer.draw_mol(mol);

        // Highlight atoms if specified
        if (highlightAtoms.length > 0) {
          const highlightMap = new Map<number, string>();
          highlightAtoms.forEach((idx) => {
            highlightMap.set(idx, "rgb(232,93,74)"); // var(--alert)
          });
          drawer.draw_mol_with_highlights(mol, "", highlightMap, new Map(), new Map(), new Map());
        }

        const svgString = drawer.get_svg();
        if (mounted) {
          setSvg(svgString);
          setLoading(false);
        }

        mol.delete();
        drawer.delete();
      } catch (err) {
        if (mounted) {
          setError(err instanceof Error ? err.message : "Failed to render molecule");
          setLoading(false);
        }
      }
    };

    renderMolecule();

    return () => {
      mounted = false;
    };
  }, [smiles, width, height, highlightKey, highlightAtoms]);

  if (loading) {
    return (
      <div className="molecule-viewer" style={{ width, height }}>
        <div className="molecule-viewer__loading">
          <div className="molecule-viewer__spinner" />
          <span>Rendering molecule...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="molecule-viewer" style={{ width, height }}>
        <div className="molecule-viewer__error">
          <span>⚠</span>
          <p>{error}</p>
          <code style={{ fontFamily: "var(--font-mono)", fontSize: "11px", color: "var(--muted)" }}>{smiles}</code>
        </div>
      </div>
    );
  }

  return (
    <div className="molecule-viewer" style={{ width, height }}>
      <div className="molecule-viewer__structure" dangerouslySetInnerHTML={{ __html: svg || "" }} />
      {properties && (
        <div className="molecule-viewer__properties">
          <table>
            <tbody>
              <tr><td>MW</td><td>{properties.molecular_weight} g/mol</td></tr>
              <tr><td>LogP</td><td>{properties.logp}</td></tr>
              <tr><td>TPSA</td><td>{properties.tpsa} Å²</td></tr>
              <tr><td>Rotatable</td><td>{properties.rotatable_bonds}</td></tr>
              <tr><td>HBD</td><td>{properties.hbd}</td></tr>
              <tr><td>HBA</td><td>{properties.hba}</td></tr>
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if ((window as any).RDKit) {
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