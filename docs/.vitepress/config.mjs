import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Research Orchestrator',
  description: 'Three-plane architecture for agentic biotech research',
  themeConfig: {
    nav: [
      { text: 'Architecture', link: '/architecture/overview' },
      { text: 'API Reference', link: '/api/workflow-engine' },
      { text: 'Evaluation', link: '/evaluation/results' },
      { text: 'GitHub', link: 'https://github.com/srikarjy/Research-Orchestrator' },
    ],
    sidebar: {
      '/architecture/': [
        { text: 'Overview', link: '/architecture/overview' },
        { text: 'Plane 1: Aletheia', link: '/architecture/aletheia' },
        { text: 'Plane 2: Workflow Engine', link: '/architecture/workflow-engine' },
        { text: 'Plane 3: Biolab MCP', link: '/architecture/biolab-mcp' },
        { text: 'Data Flow', link: '/architecture/data-flow' },
      ],
      '/api/': [
        { text: 'Workflow Engine', link: '/api/workflow-engine' },
        { text: 'Biolab MCP', link: '/api/biolab-mcp' },
        { text: 'Aletheia', link: '/api/aletheia' },
        { text: 'Shared Types', link: '/api/shared-types' },
      ],
      '/evaluation/': [
        { text: 'Results', link: '/evaluation/results' },
        { text: 'Contradiction Detection', link: '/evaluation/contradiction' },
        { text: 'Confidence Calibration', link: '/evaluation/calibration' },
        { text: 'Agent Benchmarks', link: '/evaluation/agents' },
      ],
    },
    socialLinks: [
      { icon: 'github', link: 'https://github.com/srikarjy/Research-Orchestrator' },
    ],
    footer: {
      message: 'Built for biotech R&D — Aletheia + Workflow Engine + Biolab-MCP',
      copyright: 'Copyright © 2026',
    },
  },
})