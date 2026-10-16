---
type: Quickstart
title: skillgraph-mcp — Quickstart
description: >
  Overview of skillgraph-mcp, a Go-based MCP gateway that eliminates tool bloat
  by building a semantic capability graph and exposing 9 gateway tools with
  progressive disclosure of downstream schemas. Covers architecture, CLI
  commands, configuration, skills, testing, and operations.
resource: /README.md
tags: [overview, gateway, mcp, progressive-disclosure, architecture]
---

# skillgraph-mcp

skillgraph-mcp is a Go-based **MCP gateway** that solves the "tool bloat"
problem: when an AI agent connects to many MCP servers, it receives dozens or
hundreds of tool schemas in its context window before the user says a word.
This degrades accuracy, increases latency, and makes adding capabilities counter-
productive.

The gateway reads a standard `mcp.json` config, connects to each downstream MCP
server, and exposes **9 lightweight gateway tools** to the agent. Specific
downstream schemas are loaded on demand via `use_skill`, collapsing thousands of
tokens into a lightweight index.

## Core Concepts

| Concept | Description |
|---|---|
| **Gateway** | The `skillgraph-mcp` binary runs as a single MCP server. Agents connect to it once. |
| **9 Gateway Tools** | `list_skills`, `use_skill`, `read_resource`, `execute_code`, `find_tools`, `register_server`, `get_skill_graph`, `plan_workflow`, `read_lattice` |
| **Semantic Skill Graph** | In-memory directed graph with 3 node types (Skill, Tool, Resource) and 5 edge types (HAS_TOOL, PREREQUISITE_FOR, PRODUCES, REQUIRES, COMMON_NEXT_STEP). |
| **SkillOpt Loop** | Background refinement loop that uses LLM traces to auto-improve skill descriptions and graph relations, persisting to `mcp.json`. |
| **Embedding Index** | Optional semantic search index (via `find_tools`) for embedding-based tool retrieval. |
| **Lattice Directory** | On-disk location for traces, fetched READMEs, and generated documentation. Defaults to a platform cache dir outside the working tree. |

## Architecture

```
Agent (Claude Code, Gemini CLI, Codex...)
  └── skillgraph-mcp gateway (9 tools)
        ├── Discovery: list_skills, use_skill, read_resource
        ├── Execution: execute_code, register_server
        ├── Search: find_tools (embedding-based, optional)
        └── Graph & Planning: get_skill_graph, plan_workflow, read_lattice
              └── Semantic Skill Graph + SkillOpt loop
                    └── N downstream MCP servers (brave-search, firecrawl, gitnexus...)
```

The gateway has two background goroutines:

1. **Bootstrap loop** — fetches GitHub READMEs into `.mcp_lattice/`, sends server
   tools + README to LLM to generate initial descriptions and graph relations.
2. **SkillOpt loop** — polls traces every 30s, batches up to 10 trajectory files,
   asks LLM to propose description/relation edits, validates, and merges
   atomically into `mcp.json`.

See [Architecture Overview](./architecture/overview.md) for full details.

## CLI Commands

| Command | Description |
|---|---|
| `skillgraph-mcp serve` | Run the MCP gateway (default) |
| `skillgraph-mcp doctor` | Diagnose local environment (version, config, LLM, runtime) |
| `skillgraph-mcp validate` | Connect to all servers in parallel, report failures |
| `skillgraph-mcp probe <skill>` | Health-probe a single downstream skill |
| `skillgraph-mcp generate-skills` | Generate Claude Code `SKILL.md` files |
| `skillgraph-mcp list-skills` | List configured skills and their tools |

See [CLI Commands Reference](./cli/commands.md) for full flag reference.

## Configuration

The gateway reads `mcp.json` with the following structure:

```json
{
  "mcpServers": { "serverName": { "command": "...", "args": [...] } },
  "skillGraph": {
    "descriptions": { "serverName": "Human-readable description." },
    "relations": [{ "source": "...", "target": "...", "type": "COMMON_NEXT_STEP" }]
  }
}
```

- `mcpServers`: Standard MCP config with Stdio, HTTP, or SSE server types.
- `skillGraph`: Auto-populated by the SkillOpt loop with descriptions and
  relations between skills.
- Lattice directory defaults to platform cache (e.g. `~/.cache/skillgraph-mcp/lattice`).

See [Configuration Reference](./configuration.md) for full schema details.

## Skills Ecosystem

The repository ships with 14+ built-in skills covering:

- **Code intelligence**: gitnexus (6 sub-skills for analysis, debugging, refactoring)
- **Search**: brave-search, firecrawl, context7
- **Browser automation**: chrome-devtools, playwright
- **Diagramming**: architecture (12 styles), uml (14 diagram types), mindmap
- **Development**: golang-patterns, golang-testing, sequential-thinking
- **Infrastructure**: magic (UI generation), memory (cross-session), wsl-terminal

See [Skills Ecosystem](./skills/README.md) for the full catalog.

## Testing & Benchmarks

Unit tests cover graph, config, tools, and embedding index. E2E tests spin up
fake MCP servers and mock LLM endpoints. A structured benchmark compares
embedding-based vs. typed-graph retrieval across 30 tasks.

- **Unit tests**: `go test ./...` covers all internal packages.
- **E2E tests**: `go test ./internal/e2e/...` spins up fake MCP servers and
  mock LLM endpoints.
- **Embedding vs Graph benchmark**: Controlled experiment comparing typed-edge
  graph retrieval vs. embedding-based retrieval across 30 tasks (95 tools, 9
  servers).

See [Testing & Benchmarks](./testing/README.md) for details.

See [Skills](./skills/README.md) for the full skills catalog.

## Operations

- **`doctor`**: Checks binary version, latest release (GitHub API), config
  validity, lattice directory, LLM provider, and runtime. Supports `--json` and
  `--skip-network`.
- **`validate`**: Connects to all servers in parallel, reports per-server status.
- **`probe <skill>`**: Health-probes a single downstream skill with timeout.

### Automated Wiki Updates

The repository can be configured to automatically refresh OpenWiki documentation.
The update workflow uses `openwiki code --update` to regenerate documentation
from source.

### CodeAlmanac

The repository uses [CodeAlmanac](https://github.com/codealmanac/codealmanac) for durable documentation that captures non-obvious knowledge the code cannot express.

**Location:** `almanac/`

- Pages are Markdown files under `almanac/` (supports nested folders)
- Topics are registered in `almanac/topics.yaml` (currently: `concepts`)
- Pages should capture: decisions, cross-file flows, invariants, gotchas, external dependencies, operational constraints
- Links between pages use standard Markdown links; source evidence goes in `sources:` fields

**Notability bar:** Write a page when it preserves non-obvious knowledge that will help a future agent work safely in this codebase.

See `almanac/README.md` for full guidance on writing almanac pages.

### Code Quality

The repository is configured for SonarQube code quality analysis.

**Config:** `sonar-project.properties`

- **Project key:** `skillgraph-mcp`
- **Project name:** `jrodeiro5/skillgraph-mcp`
- **Source:** `.` (entire repository)
- **Exclusions:** `benchmarks/**`, `docs/**`, `vendor/**`

This config is used by SonarQube scanners to analyze code quality, coverage, and maintainability.

## Backlog

The following areas were identified but deferred on this initial run:

| Area | Source Anchor | Reason Deferred |
|---|---|---|
| Detailed tool implementation (each of 9 gateway tools) | `internal/tools/*.go` | Covered in architecture overview; individual tool deep-dives can be added later |
| ADR documentation (0001–0003) | `docs/adr/` | Summarized in configuration page; full ADRs are accessible from source |
| Embedding migration plan | `docs/EMBEDDING_MIGRATION_PLAN.md` | Proposal document; implementation status unclear |
| Bootstrap metadata script | `scripts/bootstrap_metadata.py` | Utility script; not core to gateway operation |
