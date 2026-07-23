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
`almanac/`, including nested folders. The taxonomy is currently `concepts` ->
`architecture` -> `gotchas`, reflecting a small wiki where architecture
knowledge and its operational pitfalls are the main content so far.

## Pages

- [Gateway Routing Model](architecture/gateway-routing) — the 8 gateway tools
  and the `execute_code` indirection point that keeps downstream tool schemas
  out of the agent's context until asked for.
- [Embedding-Based Retrieval](architecture/embedding-retrieval) — the
  `find_tools` semantic search index, its provider resolution, and the
  benchmark deciding whether it replaces typed-graph retrieval.
- [npx Cache Corruption (npm 11.x)](guides/npx-cache-corruption) — what to do
  when a downstream MCP server launched via `npx` in `mcp.json` dead-ends with
  `Cannot destructure property 'package' of 'node.target' as it is null`. This
  is an npm 11.x cache-corruption bug, not a skillgraph-mcp or downstream-server
  bug, and it recurs whenever a new npx-launched server is added.

## Links

Use normal Markdown links between pages. Put file evidence in `sources:`.
