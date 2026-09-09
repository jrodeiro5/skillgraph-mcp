---
title: CodeAlmanac Wiki
topics: [concepts]
sources: []
---

# CodeAlmanac Wiki

This is the living wiki for this repository. It records the durable knowledge
the code cannot say: decisions, flows, invariants, incidents, gotchas, and
project context that future agents should not rediscover from scratch.

## Notability Bar

Write a page when it preserves non-obvious knowledge that will help a future
agent work safely in this codebase.

Good pages explain:

- a decision that took research or trial-and-error
- a cross-file flow
- an invariant or gotcha not visible from one file
- an external dependency as this repo uses it
- a product or operational constraint that shapes future work

Do not write pages that restate nearby code.

## Topic Taxonomy

Topics live in `topics.yaml`. Pages are Markdown files directly under
`almanac/`, including nested folders. The taxonomy is currently:

- `concepts` — Core vocabulary and mental models
  - `architecture` — System architecture, components, and design decisions
    - `gotchas` — Known issues, bugs, and operational pitfalls
    - `gateway` — The 9 gateway tools and routing model
    - `refinement` — Bootstrap and SkillOpt background loops
    - `config` — mcp.json schema, persistence, and runtime mutation
    - `lattice` — Generated documentation and trajectory storage
    - `graph` — In-memory capability graph, nodes, and edges

## Pages

### Architecture

- [Gateway Routing Model](architecture/gateway-routing) — the 9 gateway tools
  and the `execute_code` indirection point that keeps downstream tool schemas
  out of the agent's context until asked for.
- [Embedding-Based Retrieval](architecture/embedding-retrieval) — the
  `find_tools` semantic search index, its provider resolution, and the
  benchmark deciding whether it replaces typed-graph retrieval.
- [Graph](architecture/graph) — the in-memory directed graph with 3 node types
  and 5 edge types, its lifecycle, and the `Subgraph`/`FormatCompact` APIs.
- [Config Persistence](architecture/config) — the `mcp.json` schema, three
  transport types, environment variable expansion, and atomic writes.
- [MCP Server Manager](architecture/mcpserver) — the runtime hub that connects
  to all downstream servers, resolves tools, and maintains the capability graph.
- [Refinement Engine](architecture/refine) — the two background goroutines
  (bootstrap and SkillOpt) that populate and optimize the skillGraph section.
- [Semantic Lattice](architecture/lattice) — the `.mcp_lattice/` folder of
  generated markdown documentation, README fetching, and NPM package extraction.

### Concepts

- [Execute Code Sandbox](concepts/execute-code-sandbox) — the indirection point
  for all downstream skill tools, trajectory recording, and safety mechanisms.
- [Trajectories](concepts/trajectories) — the schema and lifecycle of execution
  traces captured by `execute_code`.

### Gotchas

- [npx Cache Corruption (npm 11.x)](guides/npx-cache-corruption) — what to do
  when a downstream server launched via `npx` in `mcp.json` dead-ends with
  `Cannot destructure property 'package' of 'node.target' as it is null`. This
  is an npm 11.x cache-corruption bug, not a skillgraph-mcp or downstream-server
  bug, and it recurs whenever a new npx-launched server is added.

### Decisions

- [Public API and Versioning](decisions/public-api-and-versioning) — ADR-0003's
  four public-API surfaces, the bump each change requires, and the delta
  between the ADR and current code (9 tools, plus `probe`/`generate-skills`).

## Links

Use normal Markdown links between pages. Put file evidence in `sources:`.
