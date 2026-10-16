---
type: CLI Reference
title: CLI Commands Reference
description: >
  Complete reference for all skillgraph-mcp CLI subcommands: serve, doctor,
  validate, probe, generate-skills, and list-skills. Includes flags, output
  formats, internal packages, and operational use cases.
resource: /cmd/root.go
tags: [cli, commands, serve, doctor, validate, probe]
---

# CLI Commands Reference

All commands are invoked via the `skillgraph-mcp` binary. The root command
(`cmd/root.go`) dispatches to subcommand handlers. All subcommands follow a
consistent pattern: parse flags via `spf13/pflag`, load config via
`config.Load()`, interact with `mcpserver`, and output results as either a
human-readable table or `--json` (via `emitJSON`).

## `serve` — Run the MCP Gateway

The default and most complex command. Starts the MCP server (stdio or HTTP).

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--config` | `mcp.json` | Path to configuration file |
| `--lattice-dir` | Platform cache dir | Directory for traces, READMEs, generated docs |
| `--transport` | `stdio` | Transport type: `stdio` or `http` |
| `--host` | `localhost` | Bind host (loopback-only by default) |
| `--port` | `8080` | Bind port (for HTTP transport) |
| `--version` | — | Print version and exit |

**Flow:**

1. Parse flags
2. Set up signal-aware context (SIGINT, SIGTERM)
3. Load config via `config.Load()` — supports Stdio, HTTP, and SSE server types
4. Create `mcpserver.NewManager` to connect to all configured servers
5. Generate the semantic lattice (`docs.GenerateLattice`)
6. Start the refinement loop (`refine.StartRefinementLoop`)
7. Build the embedding index for `find_tools` (if a provider is configured)
8. Start the MCP server (stdio or HTTP)

**Security note:** When serving over HTTP, `register_server` refuses to register
downstream servers on non-loopback hosts (see ADR-0001).

## `doctor` — Environment Diagnostics

Runs a checklist of checks for the local environment.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--json` | — | Output as JSON |
| `--skip-network` | — | Skip GitHub API checks (offline use) |

**Checks:**

- Binary version
- Latest release (GitHub API, offline-safe)
- Config file validity
- Lattice directory
- LLM provider configuration
- Runtime environment

## `validate` — Multi-Server Validation

Connects to **all** servers in **parallel** (goroutines + WaitGroup), collects
per-server results, and outputs either a table or JSON with failure counts.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--json` | — | Output as JSON |

## `probe <skill>` — Single-Skill Health Probe

Connects to exactly one server (by name) with a timeout, reports:

- Status (`ok` / `fail` / `unknown`)
- Number of tools
- Connection time in ms

Designed for **inline injection** into generated `SKILL.md` preflight blocks.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--timeout` | — | Connection timeout |
| `--json` | — | Output as JSON |

## `generate-skills` — Generate SKILL.md Files

Generates Claude Code `SKILL.md` files from config.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--offline` | — | No downstream connections |
| `--force` | — | Overwrite existing files |

Reports written/skipped files.

## `list-skills` — List Configured Skills

Connects to all servers, collects name + tool count + description, and outputs a
table or JSON.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--json` | — | Output as JSON |

## Internal Packages

| Package | Purpose |
|---|---|
| `cmd/` | CLI subcommand handlers |
| `internal/app/` | Application wiring (server creation, transport setup) |
| `internal/config/` | Config loading, server types (Stdio/HTTP/SSE), skillGraph schema |
| `internal/docs/` | Lattice generation, Claude Skills generation, README fetching |
| `internal/embed/` | Embedding-based tool retrieval for `find_tools` |
| `internal/e2e/` | End-to-end tests with fake MCP servers |
| `internal/graph/` | Semantic skill graph (3 node types, 5 edge types) |
| `internal/mcpserver/` | MCP server connection management, tool resolution, graph rebuild |
| `internal/refine/` | Background refinement loop (bootstrap + SkillOpt) with LLM retry |
| `internal/tools/` | Gateway tool implementations (9 tools) |
| `internal/trace/` | Shared trace types (ToolCallTrace, Trajectory) |
| `internal/version/` | Version info and GitHub release checks |
