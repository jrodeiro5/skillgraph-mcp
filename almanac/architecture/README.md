---
title: Architecture
topics: [architecture]
---

# Architecture

This cluster holds the seven pages that describe how skillgraph-mcp is built:
what it loads, how it connects to downstream servers, the graph it maintains,
the nine gateway tools that read from it, and the background loops that keep it
populated. Read them in dependency order, because each page assumes the one
before it.

## Reading Order

1. [Config Persistence](config) — `mcp.json` is what every other page loads
   from. It defines the schema, the three transport types, and the
   runtime-mutated `skillGraph` section.
2. [MCP Server Manager](mcpserver) — takes config, connects to downstream
   servers in parallel, and builds the capability graph.
3. [Graph](graph) — the in-memory capability graph the Manager maintains:
   three node types, five edge types, and the `Subgraph`/`FormatCompact` APIs
   the tools rely on.
4. [Gateway Routing Model](gateway-routing) — the nine gateway tools that read
   from the graph, and the `execute_code` indirection point that keeps tool
   schemas out of the agent's context until asked for.
5. [Embedding-Based Retrieval](embedding-retrieval) — the semantic complement to
   the typed graph (`find_tools`), plus the benchmark that decides when it
   replaces graph planning.
6. [Refinement Engine](refine) — the bootstrap and SkillOpt background loops
   that populate and optimize `skillGraph`.
7. [Semantic Lattice](lattice) — the generated `.mcp_lattice/` documentation and
   trajectory storage the loops read from and write to.

The concept pages [Execute Code Sandbox](../concepts/execute-code-sandbox) and
[Trajectories](../concepts/trajectories) sit under this cluster: the sandbox is
how `execute_code` (step 4) wraps each downstream tool as a Python function, and
trajectories are the records the SkillOpt loop (step 6) optimizes over.

## How the Pages Fit

The top of the chain is linear: config -> Manager -> graph -> gateway tools.
Refinement and the lattice are feedback loops on top of it. The loops read tool
usage (trajectories) and rewrite `skillGraph`, which the Manager rebuilds into
the graph. Embedding retrieval runs alongside the typed graph as an alternative
routing path. When a downstream server fails to connect, see
[npx Cache Corruption (npm 11.x)](../guides/npx-cache-corruption).
