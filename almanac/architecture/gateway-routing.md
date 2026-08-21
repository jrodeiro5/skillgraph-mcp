---
title: Gateway Routing Model
topics: [architecture, gateway]
sources:
  - {id: execute_code, type: file, path: internal/tools/execute_code.go}
  - {id: find_tools, type: file, path: internal/tools/find_tools.go}
  - {id: list_skills, type: file, path: internal/tools/list_skills.go}
  - {id: use_skill, type: file, path: internal/tools/use_skill.go}
  - {id: get_skill_graph, type: file, path: internal/tools/get_skill_graph.go}
  - {id: plan_workflow, type: file, path: internal/tools/plan_workflow.go}
  - {id: read_lattice, type: file, path: internal/tools/read_lattice.go}
  - {id: read_resource, type: file, path: internal/tools/read_resource.go}
  - {id: register_server, type: file, path: internal/tools/register_server.go}
  - {id: server_register, type: file, path: internal/app/server.go}
---

# Gateway Routing Model

The skillgraph-mcp gateway exposes 9 tools to the agent while proxying to N downstream MCP servers. The core idea is progressive disclosure: the agent starts with a small set of discovery tools and traverses a semantic skill graph to find specific tools on demand, rather than receiving 80+ tool schemas upfront.

## The 9 Gateway Tools

The gateway exposes exactly 9 tools, registered in `internal/app/server.go` [@server_register]:

1. `list_skills` — lists all downstream MCP servers with their instructions
2. `use_skill` — returns Python function signatures for a specific server
3. `read_resource` — reads skill:// resources from downstream servers [@read_resource]
4. `execute_code` — executes Python code in a sandbox (the indirection point)
5. `find_tools` — semantic search over all downstream tools (requires embedding provider)
6. `register_server` — registers a new downstream server at runtime
7. `get_skill_graph` — returns the semantic skill graph
8. `plan_workflow` — plans a multi-step workflow using the graph
9. `read_lattice` — reads documentation from the `.mcp_lattice` folder

Each tool is registered as a closure capturing `*mcpserver.Manager`. Adding a new tool means adding a file in `internal/tools/` and one `Register*` call in `internal/app/server.go`.

## The Indirection Point: execute_code

`execute_code` is the indirection point for all downstream skill tools. Downstream tools are NOT available as MCP tools — they are only available as Python functions inside the `execute_code` sandbox [@execute_code].

The workflow is:

1. `list_skills()` → see available servers
2. `use_skill("gitnexus")` → see Python function signatures (e.g., `gitnexus_impact(target: str, direction: str) -> str`)
3. `execute_code("""result = gitnexus_impact(target="authenticate", direction="up")""")` → call the function

This pattern prevents tool bloat: the agent never receives 80+ tool schemas upfront. Instead, it discovers tools on demand through the gateway.

The Python sandbox uses [gomonty](https://github.com/ewhauser/gomonty) to wrap all downstream tools as Python callables. When the sandbox calls a tool, it goes back through `mcpserver.CallTool` — this is an intentional re-entrant pattern.

## Discovery Tools

`list_skills` enumerates all downstream MCP servers and their instructions [@list_skills]. `use_skill` returns the Python function signatures for a specific server, which can then be called inside `execute_code` [@use_skill].

`find_tools` provides semantic search over all downstream tools, returning the top-k most relevant results for a natural language query [@find_tools]. It requires an embedding provider (Ollama or OpenAI) and falls back to keyword scoring when no provider is configured.

## Graph-Based Planning

`get_skill_graph` returns the full or subgraph of the capability graph [@get_skill_graph]. `plan_workflow` uses the graph to plan a sequence of tool calls to achieve a goal [@plan_workflow].

## Lattice Documentation

`read_lattice` reads markdown documentation from the `.mcp_lattice` folder, which contains generated skills and relations documentation [@read_lattice].

## Security: register_server

`register_server` hot-registers a new downstream MCP server at runtime without restarting the gateway [@register_server]. When the gateway serves HTTP on a non-loopback host, this tool refuses registration to prevent remote code execution — see [ADR-0001](../../docs/adr/0001-loopback-default-and-register-server-gate.md).

## Related

- [Embedding-Based Retrieval](embedding-retrieval) — how `find_tools` uses semantic search to find tools without reading all signatures
- [npx Cache Corruption (npm 11.x)](../guides/npx-cache-corruption) — when a downstream server registered here fails to connect, `validate` shows it as failed and the rest of the graph still comes up; a cryptic `node.target` npm error at connect time is this bug, not a routing bug
