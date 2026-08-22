---
title: Config Persistence
topics: [architecture, config]
sources:
  - {id: config_pkg, type: file, path: internal/config/config.go}
  - {id: save_pkg, type: file, path: internal/config/save.go}
---

# Config Persistence

The `mcp.json` file is the single source of truth for gateway configuration.
It has two sections: `mcpServers` (the downstream servers) and `skillGraph`
(descriptions and relations, auto-populated by background refinement loops).

## Schema

Three transport types are supported:

| Type | Required Field | Example |
|------|---------------|---------|
| `stdio` | `command` | `{"type": "stdio", "command": "npx", "args": ["-y", "package"]}` |
| `http` | `url` | `{"type": "http", "url": "https://example.com/mcp"}` |
| `sse` | `url` | `{"type": "sse", "url": "https://example.com/sse"}` |

Optional fields shared by all transports: `description`, `allowedTools`,
`allowedResources`.

## Environment Variable Expansion

Config values can reference environment variables using `${VAR}` syntax.
`Load()` expands all `${VAR}` references in all string values, failing if
a referenced variable is not set [@config_pkg].

## Atomic Writes

`SaveSkillGraph()` and `AddServer()` (defined in `internal/config/save.go`) both use a temp file + rename pattern
to ensure `mcp.json` is never left in a partial state during crashes [@save_pkg].

## The skillGraph Section

The `skillGraph` section contains:

- `descriptions` — a map of server/tool name to description, populated by
  the bootstrap and SkillOpt refinement loops
- `relations` — a list of static relations (source, target, type, description)

This section is mutated at runtime by background goroutines. Never parse
`mcp.json` as a static file while the server is running — always use
`config.Load()` which returns the current state.

## Gotchas

- `mcp.json` is mutated at runtime by background goroutines. Never parse it
  as a static file while the server is running.
- The `skillGraph` section is the auto-populated part of the config. Manual
  edits to this section will be overwritten by the next bootstrap or SkillOpt
  cycle.
- Atomic writes use `<path>.tmp` as the temp file. If the process crashes
  during a write, the temp file remains and must be cleaned up manually.

## Related

- [Refinement Engine](refine) — the bootstrap and SkillOpt loops populate and optimize the `skillGraph` section described here.
- [MCP Server Manager](mcpserver) — `NewManager()` loads config and populates the graph.
