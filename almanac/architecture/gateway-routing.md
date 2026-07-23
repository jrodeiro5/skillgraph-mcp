---
title: Gateway Routing Model
topics: [architecture]
sources:
  - {id: execute_code, type: file, path: internal/tools/execute_code.go}
  - {id: find_tools, type: file, path: internal/tools/find_tools.go}
  - {id: list_skills, type: file, path: internal/tools/list_skills.go}
  - {id: use_skill, type: file, path: internal/tools/use_skill.go}
  - {id: server_register, type: file, path: internal/app/server.go}
---

# Gateway Routing Model

The skillgraph-mcp gateway exposes 8 tools to the agent while proxying to N downstream MCP servers. The core idea is progressive disclosure: the agent starts with a small set of discovery tools and traverses a semantic skill graph to find specific tools on demand, rather than receiving 80+ tool schemas upfront.

## The 8 Gateway Tools

The gateway exposes exactly 8 tools, registered in `internal/app/server.go` [@server_register]:

1. `list_skills` — lists all downstream MCP servers with their instructions
2. `use_skill` — returns Python function signatures for a specific server
3. `read_resource` — reads skill:// resources from downstream servers
4. `execute_code` — executes Python code in a sandbox (the indirection point)
5. `register_server` — registers a new downstream server at runtime
6. `get_skill_graph` — returns the semantic skill graph
7. `plan_workflow` — plans a multi-step workflow using the graph
8. `read_lattice` — reads trajectory traces from `.mcp_lattice/traces/`

Each tool is registered as a closure capturing `*mcpserver.Manager`. Adding a new tool means adding a file in `internal/tools/` and one `Register*` call in `internal/app/server.go`.

## The Indirection Point: execute_code

`execute_code` is the indirection point for all downstream skill tools. Downstream tools are NOT available as MCP tools — they are only available as Python functions inside the execute_code sandbox [@execute_code].

The workflow is:

1. `list_skills()` → see available servers
2. `use_skill("gitnexus")` → see Python function signatures (e.g., `gitnexus_impact(target: str, direction: str) -> str`)
3. `execute_code("""result = gitnexus_impact(target="authenticate", direction="up")""")` → call the function

This pattern prevents tool bloat: the agent never receives 80+ tool schemas upfront. Instead, it discovers tools on demand through the gateway.

The Python sandbox uses [gomonty](https://github.com/ewhauser/gomonty) to wrap all downstream tools as Python callables. When the sandbox calls a tool, it goes back through `mcpserver.CallTool` — this is an intentional re-entrant pattern.

## Discovery Tools

`list_skills` enumerates all downstream MCP servers and their instructions [@list_skills]. `use_skill` returns the Python function signatures for a specific server, which can then be called inside `execute_code` [@use_skill].

## Related

- [Embedding-Based Retrieval](embedding-retrieval) — how `find_tools` [@find_tools] uses semantic search to find tools without reading all signatures
- [npx Cache Corruption (npm 11.x)](../guides/npx-cache-corruption) — when a downstream server registered here fails to connect, `validate` shows it as failed and the rest of the graph still comes up; a cryptic `node.target` npm error at connect time is this bug, not a routing bug
