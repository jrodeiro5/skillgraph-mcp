---
title: Trajectories
topics: [concepts, refinement]
sources:
  - {id: trace_pkg, type: file, path: internal/trace/trace.go}
---

# Trajectories

A trajectory is a record of a single `execute_code` invocation, capturing
the Python code, all tool calls made during execution, and the final output
or error.

## Schema

The schema is defined in `internal/trace/trace.go` [@trace_pkg].

```go
type ToolCallTrace struct {
    ToolName string         `json:"tool_name"`
    Args     map[string]any `json:"args"`
    Result   string         `json:"result"`
    IsError  bool           `json:"is_error"`
}

type Trajectory struct {
    Timestamp time.Time       `json:"timestamp"`
    Code      string          `json:"code"`
    ToolCalls []ToolCallTrace `json:"tool_calls"`
    Output    string          `json:"output"`
    Error     string          `json:"error,omitempty"`
}
```

## Lifecycle

1. Written synchronously to `.mcp_lattice/traces/<timestamp>_<rand>.json`
   after each `execute_code` call
2. Parsed by the SkillOpt optimization loop every 30 seconds
3. Garbage-collected after 7 days (TTL)

## Gotchas

- The `ToolCallTrace` and `Trajectory` types are defined identically in both
  `tools/execute_code.go` and `refine/engine.go`. They are compatible via
  JSON marshaling but should not be consolidated (it would create an import cycle).
- Trajectory files are named with nanosecond timestamps and a random number,
  so lexicographic sort equals chronological sort.
- The SkillOpt loop batches up to 10 trajectory files per optimization cycle.
  If more than 10 are available, only the 10 most recent are processed.

## Related

- [Execute Code Sandbox](execute-code-sandbox) — trajectories are produced by each `execute_code` call.
- [Refinement Engine](../architecture/refine) — the SkillOpt loop parses trajectory files to propose edits.
