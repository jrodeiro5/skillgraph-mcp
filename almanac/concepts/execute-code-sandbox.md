---
title: Execute Code Sandbox
topics: [concepts, architecture, gateway]
sources:
  - {id: execute_code, type: file, path: internal/tools/execute_code.go}
  - {id: trace_pkg, type: file, path: internal/trace/trace.go}
---

# Execute Code Sandbox

The \`execute_code\` gateway tool (defined in \`internal/tools/execute_code.go\`) [@execute_code] is the indirection point for all downstream
skill tools. Instead of exposing 80+ MCP tool schemas to the agent, the
gateway wraps each downstream tool as a Python function and makes them
callable only inside an isolated Python sandbox.

## How It Works

1. The agent calls `use_skill("gitnexus")` to see Python function signatures
2. The agent writes Python code inside `execute_code` that calls those functions
3. The sandbox (gomonty) executes the code, routing each function call back
   through `mcpserver.CallTool` to the actual downstream MCP server

This re-entrant pattern means the agent never sees raw MCP tool schemas —
it only sees Python function names and signatures.

## Trajectory Recording

Every `execute_code` call produces a `Trajectory` record containing:

- `timestamp` — when the call was made
- `code` — the Python code that was executed
- `toolCalls` — list of tool calls made during execution (name, args, result, error flag)
- `output` — the final output if no error
- `error` — the error message if the call failed

These trajectories are written to `.mcp_lattice/traces/<timestamp>_<rand>.json`
synchronously (not async) to avoid race conditions with process exit [@trace_pkg].

## Safety

- Code is compiled and executed in an isolated gomonty runtime
- Tool calls are validated against the tool's JSON schema before being sent
  to the downstream server
- Output strings are truncated to 50KB to prevent runaway tool dumps from
  poisoning the SkillOpt batch
- The `skill_names` parameter lets callers restrict which downstream servers
  are available to a specific execution

## Gotchas

- The Python sandbox uses gomonty, which wraps all downstream tools as Python
  callables. When the sandbox calls a tool, it goes back through
  `mcpserver.CallTool` — this is intentional but creates a re-entrant loop.
- Trajectory files are written synchronously. This is sub-millisecond on any
  sane filesystem but means a crash during a large tool response could
  leave a partial trace file.
- The `ToolCallTrace` and `Trajectory` types are duplicated in
  `tools/execute_code.go` and `refine/engine.go`. They are compatible via
  JSON marshaling but should not be consolidated (it would create an import cycle).

## Related

- [Trajectories](trajectories) — each `execute_code` call produces a trajectory record.
- [Gateway Routing Model](../architecture/gateway-routing) — `execute_code` is one of the 9 gateway tools.
