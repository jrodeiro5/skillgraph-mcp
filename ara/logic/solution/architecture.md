# Architecture

## Gateway model

skillgraph-mcp sits between one MCP client and N downstream MCP servers.
The client registers a single server; skillgraph exposes exactly 8 gateway tools.

```
MCP Client (Claude Code / Gemini CLI / Codex)
    │  8 gateway tools
    ▼
skillgraph-mcp
    ├── list_skills        → compact index (skill name + description)
    ├── use_skill          → full tool schemas for one skill on demand
    ├── execute_code       → Python sandbox; all downstream tools callable
    ├── plan_workflow      → skill graph traversal for multi-step tasks
    ├── get_skill_graph    → full graph dump (nodes + edges)
    ├── read_lattice       → lattice docs (skills.md, relations.md)
    ├── read_resource      → downstream resources
    └── register_server    → runtime server registration (loopback only)
    │
    ├── N downstream MCP servers (stdio or http)
    └── SkillOpt loop (background goroutine)
```

## Skill graph

In-memory directed graph. 3 node types, 5 edge types.

Node types: Skill, Tool, Resource  
Edge types: HAS_TOOL, PREREQUISITE_FOR, PRODUCES, REQUIRES, COMMON_NEXT_STEP

Seeded from: mcp.json skillGraph section (manual) + SkillOpt loop (automatic).

## SkillOpt loop

Two background goroutines launched at startup:

1. **Bootstrap** — fetches GitHub READMEs for each downstream server into
   `.mcp_lattice/`, sends server tools + README to LLM, generates initial
   descriptions and graph relations, merges into mcp.json.

2. **Refinement** — polls `.mcp_lattice/traces/*.json` every 30s, batches
   up to 10 trajectory files, asks LLM to propose description/relation edits,
   validates (rejects hallucinated tool names), merges atomically into mcp.json,
   calls RebuildGraph.

Trajectories written by execute_code: code, tool calls, results, errors.
LLM provider: any OpenAI-compatible endpoint (LLM_BASE_URL env var).
Gracefully skips if no LLM configured.

## Progressive disclosure mechanics

Agent pays 287 tokens upfront (list_skills).  
To use a skill, agent calls use_skill(skill_name) → pays ~32 tokens/tool for that server only.  
Unused servers cost 0 tokens beyond their single line in list_skills.

Crossover vs raw loading: ~4 servers / 9 tools (see T02).
