---
title: Graph
topics: [architecture, concepts, graph]
sources:
  - {id: graph_pkg, type: file, path: internal/graph/graph.go}
---

# Graph

The in-memory directed graph that represents the capability topology of all
connected downstream MCP servers. Every gateway tool reads from this single
data structure [@graph_pkg].

## Node and Edge Types

The graph has three node types: `skill` (downstream server), `tool` (individual
MCP tool exposed by a server), and `resource` (readable resource endpoint).

Five edge types describe relationships between nodes:

- `HAS_TOOL` — a skill exposes a tool
- `PREREQUISITE_FOR` — one tool must be called before another
- `PRODUCES` — one tool's output feeds another's input
- `REQUIRES` — one tool depends on another's result
- `COMMON_NEXT_STEP` — a recommended follow-up tool

## Graph Lifecycle

`graph.New()` creates an empty graph. `config.Load()` and `mcpserver.Manager.NewManager()`
populate it from the `mcp.json` configuration. The graph is rebuilt atomically
when the skill graph configuration changes (via SkillOpt or manual edits).

## Subgraph and Compact Format

`Subgraph(nodeID)` returns a new graph containing only the center node and its
directly connected neighbors — used by `get_skill_graph` to return focused views.

`FormatCompact()` produces a single-line-per-entry string representation suitable
for LLM context windows. Newlines, tabs, and carriage returns in descriptions
are collapsed to spaces so multi-line tool docs don't break the layout.

## Gotchas

- The graph is in-memory only. Persistence lives in `mcp.json`'s `skillGraph` section.
- `graph.New()` and `config.Load()` are the two highest-blast-radius symbols —
  everything rebuilds from them.
- Rebuilding the graph is expensive for large tool counts because it re-resolves
  all downstream tools and re-infers relations.
