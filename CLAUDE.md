# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Working Principles

### 1. Think Before Coding

Before implementing: state assumptions explicitly — if uncertain, ask. If multiple interpretations exist, present them; don't pick silently. If a simpler approach exists, say so and push back when warranted. If something is unclear, stop, name what's confusing, and ask.

### 2. Simplicity First

Minimum code that solves the problem. No features beyond what was asked, no abstractions for single-use code, no "flexibility" that wasn't requested, no error handling for impossible scenarios. If you write 200 lines and it could be 50, rewrite it. Ask: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

### 3. Surgical Changes

Touch only what you must. Don't improve adjacent code, comments, or formatting. Don't refactor things that aren't broken. Match existing style even if you'd do it differently. If you notice unrelated dead code, mention it — don't delete it. When your changes create orphans (imports, variables, functions made unused by your edits), remove them. Don't remove pre-existing dead code unless asked. Every changed line should trace directly to the user's request.

### 4. Goal-Driven Execution

Transform tasks into verifiable goals before starting:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"

For multi-step tasks, state a brief plan with explicit verify steps before writing any code.

## Commands

```bash
# Build
go build ./...

# Test all packages
go test ./...

# Test a single package
go test ./internal/graph/...

# Run a specific test
go test ./internal/refine/... -run TestOptimizeTracesExecution

# Run E2E tests (spins up fake MCP servers and mock LLM endpoints)
go test ./internal/e2e/...

# Lint (errcheck, govet, staticcheck, unused + goimports)
golangci-lint run

# Build binary
go build -o skillgraph-mcp .
```

Environment variables for the refine/SkillOpt loop (in priority order): `LLM_BASE_URL` (+ optional `LLM_API_KEY`, `LLM_MODEL`), `OPENAI_API_KEY`, `DEEPSEEK_API_KEY`, `GEMINI_API_KEY`. Without any of these, the bootstrap and optimization loops silently skip.

## Architecture

skillgraph-mcp is an **MCP gateway**: the agent connects to this server and sees 7 tools, while this server proxies to N downstream MCP servers. The core idea is progressive disclosure — the agent starts with 4 discovery tools and traverses a semantic skill graph to find specific tools on demand, rather than receiving 80+ tool schemas upfront.

### Package dependency order (leaf → root)

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

`graph` and `config` have no outward dependencies and are safe to edit independently. Everything else flows up through `mcpserver.Manager`.

### Key architectural decisions

**Tool registration** (`internal/app/server.go` + `internal/tools/*.go`): Each of the 7 gateway tools is registered as a closure capturing `*mcpserver.Manager`. The pattern is consistent — `Register*` functions in each tool file. Adding a new tool means adding a file in `internal/tools/` and one `Register*` call in `app/server.go`.

**Two background goroutines** (both launched from `internal/refine/engine.go`):
1. Bootstrap loop — fetches GitHub READMEs into `.mcp_lattice/`, then sends server tools + README to LLM to generate initial descriptions and graph relations, merges into `mcp.json`.
2. SkillOpt loop — polls `.mcp_lattice/traces/*.json` every 30s, batches up to 10 trajectory files, asks LLM to propose description/relation edits, validates (rejects hallucinated tool names), merges atomically into `mcp.json`, then calls `mgr.RebuildGraph()`.

The refine package directly calls `mcpserver.RebuildGraph`, `mcpserver.AllTools`, and `mcpserver.ServerTools` — it is tightly coupled to `Manager`'s concrete type. Any refactor of `Manager` will break `refine/engine.go`.

**execute_code re-entrancy** (`internal/tools/execute_code.go`): The Python sandbox (gomonty) wraps all downstream tools as Python callables. When the sandbox calls a tool, it goes back through `mcpserver.CallTool` — intentional re-entrant pattern. Trajectories (code, tool calls, results) are written async to `.mcp_lattice/traces/`.

**Config persistence** (`internal/config/save.go`): `mcp.json` is the single source of truth. The `skillGraph` section (descriptions + relations) is auto-populated and mutated by the refine loops. Atomic write via temp file + rename.

**Graph** (`internal/graph/graph.go`): In-memory directed graph with 3 node types (Skill, Tool, Resource) and 5 edge types (HAS_TOOL, PREREQUISITE_FOR, PRODUCES, REQUIRES, COMMON_NEXT_STEP). `graph.New` and `config.Load` are the two highest-blast-radius symbols — everything rebuilds from them.

### What lives where

| Need | Location |
|---|---|
| Add/change a gateway tool | `internal/tools/<name>.go` + register in `internal/app/server.go` |
| Change LLM provider or prompt | `internal/refine/engine.go` |
| Change graph edge/node types | `internal/graph/graph.go` |
| Change config schema | `internal/config/config.go` + `save.go` |
| Change downstream server connection | `internal/mcpserver/server.go` + `manager.go` |
| Add docs/lattice generation | `internal/docs/lattice.go` |

### Known footguns

- **Blank child env**: STDIO child processes receive only the env vars in their `env` block — not the parent shell's env. Tests that rely on child processes needing `PATH` will fail unless those vars are passed explicitly.
- **`.mcp_lattice/` is CWD-relative**: Hardcoded as `"./.mcp_lattice"` in both `execute_code.go` and `refine/engine.go`. Running the binary from different directories scatters traces.
- **`mcp.json` mutated at runtime**: The `skillGraph` section is rewritten by background goroutines. Never parse it as a static file while the server is running.
- **`ToolCallTrace`/`Trajectory` duplicated**: Defined identically in `tools/execute_code.go` and `refine/engine.go`. They're compatible via JSON marshaling. Do not consolidate without also fixing the import cycle that would result.
- **CHANGELOG.md links point to upstream**: Historical commit/PR links in `CHANGELOG.md` point to `github.com/kurtisvg/skillful-mcp` (the original upstream repo). This is correct — those events happened there. Do not rewrite them.

<!-- gitnexus:start -->
# GitNexus — Code Intelligence

This project is indexed by GitNexus as **skillgraph-mcp** (902 symbols, 1706 relationships, 52 execution flows). Use the GitNexus MCP tools to understand code, assess impact, and navigate safely.

> If any GitNexus tool warns the index is stale, run `npx gitnexus analyze` in terminal first.

## Always Do

- **MUST run impact analysis before editing any symbol.** Before modifying a function, class, or method, run `gitnexus_impact({target: "symbolName", direction: "upstream"})` and report the blast radius (direct callers, affected processes, risk level) to the user.
- **MUST run `gitnexus_detect_changes()` before committing** to verify your changes only affect expected symbols and execution flows.
- **MUST warn the user** if impact analysis returns HIGH or CRITICAL risk before proceeding with edits.
- When exploring unfamiliar code, use `gitnexus_query({query: "concept"})` to find execution flows instead of grepping. It returns process-grouped results ranked by relevance.
- When you need full context on a specific symbol — callers, callees, which execution flows it participates in — use `gitnexus_context({name: "symbolName"})`.

## Never Do

- NEVER edit a function, class, or method without first running `gitnexus_impact` on it.
- NEVER ignore HIGH or CRITICAL risk warnings from impact analysis.
- NEVER rename symbols with find-and-replace — use `gitnexus_rename` which understands the call graph.
- NEVER commit changes without running `gitnexus_detect_changes()` to check affected scope.

## Resources

| Resource | Use for |
|----------|---------|
| `gitnexus://repo/skillgraph-mcp/context` | Codebase overview, check index freshness |
| `gitnexus://repo/skillgraph-mcp/clusters` | All functional areas |
| `gitnexus://repo/skillgraph-mcp/processes` | All execution flows |
| `gitnexus://repo/skillgraph-mcp/process/{name}` | Step-by-step execution trace |

## CLI

| Task | Read this skill file |
|------|---------------------|
| Understand architecture / "How does X work?" | `.claude/skills/gitnexus/gitnexus-exploring/SKILL.md` |
| Blast radius / "What breaks if I change X?" | `.claude/skills/gitnexus/gitnexus-impact-analysis/SKILL.md` |
| Trace bugs / "Why is X failing?" | `.claude/skills/gitnexus/gitnexus-debugging/SKILL.md` |
| Rename / extract / split / refactor | `.claude/skills/gitnexus/gitnexus-refactoring/SKILL.md` |
| Tools, resources, schema reference | `.claude/skills/gitnexus/gitnexus-guide/SKILL.md` |
| Index, status, clean, wiki CLI commands | `.claude/skills/gitnexus/gitnexus-cli/SKILL.md` |

<!-- gitnexus:end -->
