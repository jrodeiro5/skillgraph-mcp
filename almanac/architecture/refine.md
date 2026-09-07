---
title: Refinement Engine
topics: [architecture, refinement]
sources:
  - {id: refine_pkg, type: file, path: internal/refine/engine.go}
  - {id: trace_pkg, type: file, path: internal/trace/trace.go}
  - {id: lattice_dir_util, type: file, path: cmd/util.go}
---

# Refinement Engine

The refinement engine (defined in `internal/refine/engine.go` [@refine_pkg]) runs two background goroutines that populate and
optimize the `skillGraph` section of `mcp.json`:

1. **Bootstrap loop** — fetches GitHub READMEs into `.mcp_lattice/`, then
   sends server tools + README to an LLM to generate initial descriptions
   and graph relations, merges into `mcp.json`.
2. **SkillOpt loop** — polls `.mcp_lattice/traces/*.json` every 30s, batches
   up to 10 trajectory files, asks the LLM to propose description/relation
   edits, validates (rejects hallucinated tool names), merges atomically
   into `mcp.json`, then calls `mgr.RebuildGraph()`.

## Bootstrap Phase

For each server without existing descriptions, the bootstrap loop:

1. Waits up to 15 seconds for a README file (or a "no readme" sentinel)
2. Extracts all tools and their parameter signatures
3. Sends the tools list + README to the LLM with a system prompt that
   enforces output matching the `SkillGraphConfig` schema
4. Validates the LLM output — rejects any relations referencing tool names
   that don't exist (hallucination check)
5. Merges the result into `mcp.json` and rebuilds the in-memory graph

Concurrent LLM calls are capped at 3 with staggered start times to avoid
blasting upstream providers with N simultaneous requests.

## SkillOpt Optimization Loop

Every 30 seconds, the SkillOpt loop:

1. Reads all trace files from `.mcp_lattice/traces/`
2. Purges traces older than 7 days (TTL)
3. Sorts chronologically and splits into hold-out (oldest third) and
   optimization sets
4. Computes the current error rate; if it increased by >1.5x since the
   last edit, rolls back to the latest snapshot
5. Calls the LLM with the current graph, tools, and recent traces
6. Validates and merges proposed edits, saving a pre-edit snapshot first
7. Regenerates lattice documentation

## LLM Provider Resolution

Providers are detected in priority order:

1. `LLM_BASE_URL` set → OpenAI-compatible endpoint. Auth reads `LLM_API_KEY`; the model is `LLM_MODEL` if set, otherwise `gpt-5.4-nano`
2. `OPENAI_API_KEY` → OpenAI
3. `DEEPSEEK_API_KEY` → DeepSeek (`deepseek-v4-flash`)
4. `GEMINI_API_KEY` → Gemini (`gemini-3.1-flash-lite`)

Without any of these, both loops silently skip.

## Gotchas

- The refine package directly calls `mcpserver.RebuildGraph`, `mcpserver.AllTools`,
  and `mcpserver.ServerTools` — it is tightly coupled to `Manager`'s concrete type.
- `Trajectory` and `ToolCallTrace` are defined once in `internal/trace/trace.go`
 [@trace_pkg] and imported by both `tools/execute_code.go` and `refine/engine.go`. The
 duplication is already resolved, so there is no import cycle to fix.
- The lattice directory is resolved by `defaultLatticeDir()` (`cmd/util.go`)
  to a per-user cache path by default, and the CWD-relative `./.mcp_lattice`
  is reachable only via `--lattice-dir` [@lattice_dir_util]. Traces
  therefore no longer scatter across working directories.
- The bootstrap loop's 15-second README timeout is a hard limit. If a README
  isn't available within that window, the loop proceeds with tools only.
- The SkillOpt hold-out gate rejects description edits that regress routing
  accuracy on historical traces. With no hold-out data, every edit is accepted.

## Related

- [Config Persistence](config) — the loops populate and optimize the `skillGraph` section of `mcp.json`.
- [Semantic Lattice](lattice) — the bootstrap writes READMEs and SkillOpt reads traces from `.mcp_lattice/`.
- [MCP Server Manager](mcpserver) — the loops call `mgr.RebuildGraph()` after merging edits.
- [Trajectories](../concepts/trajectories) — the SkillOpt loop parses trajectory files to propose edits.
