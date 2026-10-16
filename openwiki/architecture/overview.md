---
type: Architecture Overview
title: skillgraph-mcp Architecture
description: >
  Complete architectural breakdown of skillgraph-mcp: package dependency order,
  the 9 gateway tools, the semantic skill graph (3 node types, 5 edge types),
  the SkillOpt refinement loop (bootstrap + SkillOpt goroutines with retry),
  embedding index, config persistence, shared trace types, and the
  execute_code re-entrant pattern.
resource: /CLAUDE.md
tags: [architecture, packages, gateway-tools, graph, skillopt, embedding]
---

# Architecture Overview

skillgraph-mcp is an **MCP gateway**: the agent connects to this server once and
sees 9 tools, while the server proxies to N downstream MCP servers. The core idea
is **progressive disclosure** — the agent starts with a small set of discovery
tools and traverses a semantic skill graph to find specific tools on demand.

## Package Dependency Order

```
graph   config
  ↑        ↑
  mcpserver
  ↑        ↑
tools    refine  docs
  ↑        ↑
     app
      ↑
     cmd → main
```

`graph` and `config` have no outward dependencies and are safe to edit
independently. Everything else flows up through `mcpserver.Manager`.

## The 9 Gateway Tools

Each gateway tool is registered as a closure capturing `*mcpserver.Manager`.
Adding a new tool means adding a file in `internal/tools/` and one
`Register*` call in `internal/app/server.go`.

| Tool | File | Description |
|---|---|---|
| `list_skills` | `internal/tools/list_skills.go` | Lists all configured skills with descriptions |
| `use_skill` | `internal/tools/use_skill.go` | Lists tools and resources of a specific skill |
| `read_resource` | `internal/tools/read_resource.go` | Reads a resource from a specific skill |
| `execute_code` | `internal/tools/execute_code.go` | Runs Python code in a gomonty sandbox |
| `find_tools` | `internal/tools/find_tools.go` | Semantic search over tools via embedding index (optional, requires embedding provider) |
| `register_server` | `internal/tools/register_server.go` | Hot-registers a new downstream MCP server at runtime |
| `get_skill_graph` | `internal/tools/get_skill_graph.go` | Returns the capability relationship graph |
| `plan_workflow` | `internal/tools/plan_workflow.go` | Plans multi-step workflows using the graph |
| `read_lattice` | `internal/tools/read_lattice.go` | Reads the semantic lattice (skills.md, relations.md) |

`find_tools` is always registered but returns a helpful error when no embedding
provider is configured. The embedding index is built synchronously at boot in
`cmd/serve.go` and rebuilt via a `Manager.SetOnRebuild` callback whenever the
graph is updated by SkillOpt.

## Semantic Skill Graph

Located in `internal/graph/graph.go`, the graph is an in-memory directed graph
with:

- **3 node types**: `Skill`, `Tool`, `Resource`
- **5 edge types**: `HAS_TOOL`, `PREREQUISITE_FOR`, `PRODUCES`, `REQUIRES`,
  `COMMON_NEXT_STEP`

`graph.New` and `config.Load` are the two highest-blast-radius symbols —
everything rebuilds from them.

## Background Refinement Loops

Both goroutines are launched from `internal/refine/engine.go`:

### Bootstrap Loop

1. Fetches GitHub READMEs into `.mcp_lattice/`
2. Sends server tools + README to LLM to generate initial descriptions and
   graph relations
3. Merges results into `mcp.json`

### SkillOpt Loop

1. Polls `.mcp_lattice/traces/*.json` every 30 seconds
2. Batches up to 10 trajectory files
3. Asks LLM to propose description/relation edits
4. Validates (rejects hallucinated tool names)
5. Merges atomically into `mcp.json`
6. Calls `mgr.RebuildGraph()`

The refine package directly calls `mcpserver.RebuildGraph`,
`mcpserver.AllTools`, and `mcpserver.ServerTools` — it is tightly coupled to
`Manager`'s concrete type. Any refactor of `Manager` will break
`refine/engine.go`.

### Retry Logic

Located in `internal/refine/retry.go`, all LLM API calls go through `callLLM`
which wraps provider-specific HTTP calls with `withRetry`. Retry policy: up to
3 attempts with exponential backoff (1s, 2s, 4s) plus ±50% jitter. Retries on
HTTP 429 (rate limit) and 5xx (server errors); non-retryable 4xx errors and
context cancellation abort immediately. The `HTTPStatusError` type distinguishes
transient from permanent failures.

## Execute Code Re-entrancy

Located in `internal/tools/execute_code.go`, the Python sandbox (gomonty) wraps
all downstream tools as Python callables. When the sandbox calls a tool, it
goes back through `mcpserver.CallTool` — an intentional re-entrant pattern.
Trajectories (code, tool calls, results) are written async to
`.mcp_lattice/traces/`.

## Shared Trace Types

Located in `internal/trace/trace.go`, the `ToolCallTrace` and `Trajectory` types
are shared between `internal/tools/execute_code.go` (producer) and
`internal/refine/engine.go` (consumer). Both packages import from this canonical
location. The `TraceCollector` in `execute_code.go` accumulates tool calls during
a sandbox execution, and the trajectory is written as JSON to the traces directory.

## Embedding Index

Located in `internal/embed/embed.go`, the optional embedding index supports
`find_tools(query, k, server_filter)` for semantic search-based tool retrieval.
The index is rebuilt on graph update. See [Embedding Migration Plan](../../docs/EMBEDDING_MIGRATION_PLAN.md)
for the full proposal.

## Config Persistence

Located in `internal/config/save.go`, `mcp.json` is the single source of truth.
The `skillGraph` section (descriptions + relations) is auto-populated and mutated
by the refine loops. Writes use atomic temp file + rename.

## Manager Callbacks

`Manager.SetOnRebuild` (in `internal/mcpserver/manager.go`) registers a callback
invoked after every graph rebuild. The primary consumer is the embedding index
rebuild in `cmd/serve.go` — when SkillOpt updates the graph, the callback
re-embeds all tools so `find_tools` stays current.

## What Lives Where

| Need | Location |
|---|---|
| Add/change a gateway tool | `internal/tools/<name>.go` + register in `internal/app/server.go` |
| Change LLM provider or prompt | `internal/refine/engine.go` |
| Change LLM retry behavior | `internal/refine/retry.go` |
| Change graph edge/node types | `internal/graph/graph.go` |
| Change config schema | `internal/config/config.go` + `save.go` |
| Change downstream server connection | `internal/mcpserver/server.go` + `manager.go` |
| Tool representation and signatures | `internal/mcpserver/tool.go` |
| Shared trace types (ToolCallTrace, Trajectory) | `internal/trace/trace.go` |
| Add docs/lattice generation | `internal/docs/lattice.go` |
| GitHub README fetching | `internal/docs/fetcher.go` |
| Embedding index | `internal/embed/embed.go` |
| Embedding index rebuild hook | `internal/mcpserver/manager.go` (SetOnRebuild) |
| Version check (GitHub Releases) | `internal/version/latest.go` |
