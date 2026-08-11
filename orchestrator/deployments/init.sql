-- AssayOS Database Initialization
-- Run as part of postgres container startup

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Core platform tables
CREATE TABLE IF NOT EXISTS platform_workflows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    plane TEXT NOT NULL CHECK (plane IN ('workflow', 'biolab', 'aletheia')),
    status TEXT NOT NULL,
    input JSONB,
    output JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_workflows_plane ON platform_workflows(plane);
CREATE INDEX IF NOT EXISTS idx_workflows_status ON platform_workflows(status);
CREATE INDEX IF NOT EXISTS idx_workflows_created ON platform_workflows(created_at DESC);

CREATE TABLE IF NOT EXISTS platform_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID REFERENCES platform_workflows(id) ON DELETE CASCADE,
    step_id TEXT,
    type TEXT NOT NULL,
    source TEXT NOT NULL,
    trace_id TEXT,
    payload JSONB,
    dedup_key TEXT UNIQUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_workflow ON platform_events(workflow_id);
CREATE INDEX IF NOT EXISTS idx_events_dedup ON platform_events(dedup_key);
CREATE INDEX IF NOT EXISTS idx_events_type ON platform_events(type);
CREATE INDEX IF NOT EXISTS idx_events_created ON platform_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_trace ON platform_events(trace_id);

-- Plugin registry
CREATE TABLE IF NOT EXISTS platform_plugins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    version TEXT,
    type TEXT CHECK (type IN ('tool', 'agent', 'visualizer', 'executor', 'authz')),
    description TEXT,
    author TEXT,
    entry_point TEXT,
    manifest JSONB,
    config JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    loaded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Audit log for compliance (21 CFR Part 11 / ALCOA+)
CREATE TABLE IF NOT EXISTS platform_audit (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    metadata JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_actor ON platform_audit(actor);
CREATE INDEX IF NOT EXISTS idx_audit_resource ON platform_audit(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON platform_audit(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON platform_audit(action);

-- Evidence cards (for traceability)
CREATE TABLE IF NOT EXISTS platform_evidence (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID REFERENCES platform_workflows(id) ON DELETE CASCADE,
    claim TEXT NOT NULL,
    confidence_overall REAL,
    confidence_signals JSONB,
    sources JSONB,
    tool_calls JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_evidence_workflow ON platform_evidence(workflow_id);

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply updated_at triggers
DROP TRIGGER IF EXISTS update_workflows_updated_at ON platform_workflows;
CREATE TRIGGER update_workflows_updated_at
    BEFORE UPDATE ON platform_workflows
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_evidence_updated_at ON platform_evidence;
CREATE TRIGGER update_evidence_updated_at
    BEFORE UPDATE ON platform_evidence
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Default admin user (for auth)
INSERT INTO platform_plugins (name, version, type, description, author, entry_point, enabled, loaded_at)
VALUES 
    ('pubmed', '1.0.0', 'tool', 'PubMed literature retriever', 'AssayOS', 'PubMedRetriever', true, NOW()),
    ('uniprot', '1.0.0', 'tool', 'UniProt protein database retriever', 'AssayOS', 'UniProtRetriever', true, NOW()),
    ('chembl', '1.0.0', 'tool', 'ChEMBL bioactivity database retriever', 'AssayOS', 'ChEMBLRetriever', true, NOW()),
    ('pdb', '1.0.0', 'tool', 'Protein Data Bank structure retriever', 'AssayOS', 'PDBRetriever', true, NOW()),
    ('kegg', '1.0.0', 'tool', 'KEGG pathway retriever', 'AssayOS', 'KEGGRetriever', true, NOW()),
    ('bindingdb', '1.0.0', 'tool', 'BindingDB affinity retriever', 'AssayOS', 'BindingDBRetriever', true, NOW()),
    ('stability', '1.0.0', 'analyzer', 'Protein stability predictor', 'AssayOS', 'ProteinStabilityPredictor', true, NOW()),
    ('docking', '1.0.0', 'analyzer', 'Molecular docking analyzer', 'AssayOS', 'DockingAnalyzer', true, NOW()),
    ('merge', '1.0.0', 'analyzer', 'Evidence merge analyzer', 'AssayOS', 'EvidenceMergeAnalyzer', true, NOW()),
    ('critic', '1.0.0', 'analyzer', 'Critic agent', 'AssayOS', 'CriticAgent', true, NOW()),
    ('structure_viewer', '1.0.0', 'visualizer', '3D structure visualizer', 'AssayOS', 'StructureVisualizer', true, NOW()),
    ('molecule_viewer', '1.0.0', 'visualizer', '2D molecule visualizer', 'AssayOS', 'MoleculeVisualizer', true, NOW())
ON CONFLICT (name) DO NOTHING;

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO assayos;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO assayos;