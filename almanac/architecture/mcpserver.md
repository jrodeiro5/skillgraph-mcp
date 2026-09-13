---
title: MCP Server Manager
topics: [architecture, manager]
sources:
  - {id: manager_pkg, type: file, path: internal/mcpserver/manager.go}
  - {id: server_pkg, type: file, path: internal/mcpserver/server.go}
---

# MCP Server Manager

The `Manager` is the runtime hub that connects to all configured downstream
MCP servers, resolves their tools, and maintains the capability graph.

## Connection Strategy

The server entry point is defined in `internal/mcpserver/server.go` [@server_pkg].

`NewManager()` (defined in \`internal/mcpserver/manager.go\`) [@manager_pkg] connects to all servers in parallel so total startup is
O(slowest server) instead of O(sum of servers). This is important to stay
under MCP client initialize timeouts (Claude Code's is 30s).

If any server fails to connect, the gateway logs a warning and continues —
one bad downstream should not kill the entire gateway. If all servers fail,
the gateway exits with an error.

## Tool Resolution

Tools are resolved across all servers with name conflict handling:

- If only one server defines a tool, it keeps its original name (e.g., `impact`)
- If multiple servers define the same tool, each gets prefixed with the
  server name (e.g., `gitnexus_impact`, `brave_search`)

Tool names are also "pythonized" — any character outside `[A-Za-z0-9_]`
is replaced with `_`, and names starting with digits get a leading underscore.
This is purely a sandbox-facing concern; the wire-format name is preserved
in `OriginalName`.

## Dynamic Inference

The manager infers `PREREQUISITE_FOR` edges automatically by analyzing
tool parameter names. If a tool has a parameter ending in `_id` or `_number`,
and another tool's name starts with `create_`, `new_`, `get_`, or `search_`
for the same prefix, a prerequisite edge is created.

## Hot Registration

`AddServer()` connects to a new downstream MCP server and hot-registers it
without restarting the gateway. The graph is rebuilt atomically after the
connection succeeds. This is used by the `register_server` gateway tool.

## Thread Safety

All `Manager` methods that read or modify state use a `sync.RWMutex`.
`RebuildGraph()` switches the graph atomically, calling an optional
`onRebuild` callback (used to rebuild the embedding index) after the swap.

## Gotchas

- The `Manager` is tightly coupled to `refine/engine.go`. Any refactor of
  `Manager` will break the refinement loops.
- `NewManagerFromMCPServers()` and `NewManagerFromServers()` are test helpers
  that bypass the normal connection flow. Use them only in tests.
- `Close()` must be called to clean up all server connections. The `cmd/serve.go`
  entry point uses `defer mgr.Close()` to ensure cleanup on exit.

## Related

- [Graph](graph) — the `Manager` maintains the capability graph and rebuilds it via `RebuildGraph()`.
- [Refinement Engine](refine) — the refine package is tightly coupled to `Manager` and calls `RebuildGraph()`.
- [Config Persistence](config) — `NewManager()` populates the graph from config.
