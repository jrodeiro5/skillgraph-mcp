---
type: Configuration Guide
title: Configuration Reference
description: >
  Complete guide to skillgraph-mcp configuration: mcp.json schema, server types
  (Stdio, HTTP, SSE), lattice directory defaults, LLM provider environment
  variables, the skillGraph section auto-populated by SkillOpt, and security
  considerations.
resource: /internal/config/config.go
tags: [configuration, mcp.json, server-types, lattice-dir, llm-providers]
---

# Configuration Reference

skillgraph-mcp reads a standard `mcp.json` config file. The file is the single
source of truth and is persisted atomically via temp file + rename.

## Configuration Schema

```json
{
  "mcpServers": {
    "serverName": {
      "command": "npx",
      "args": ["-y", "@modelserver/name"],
      "description": "Human-readable description of what this server provides.",
      "env": { "API_KEY": "..." }
    }
  },
  "skillGraph": {
    "descriptions": {
      "serverName": "Human-readable description for routing.",
      "specific_tool_name": "Override description for a single tool."
    },
    "relations": [
      {
        "source": "search_server",
        "target": "issue_server",
        "type": "COMMON_NEXT_STEP",
        "description": "After searching, you may want to open a related issue."
      }
    ]
  }
}
```

## Server Types

The config supports three server transport types:

| Type | Config Field | Description |
|---|---|---|
| **Stdio** | `command` + `args` | Standard MCP stdio transport (default) |
| **HTTP** | `url` + optional `headers` | Streamable HTTP MCP server |
| **SSE** | `url` + optional `headers` | Server-Sent Events transport |

All server types also support `description`, `allowedTools`, and `allowedResources`
optional fields. The `headers` field (for HTTP/SSE) accepts key-value pairs, e.g.
`{"Authorization": "Bearer ${TOKEN}"}`.

## Lattice Directory

The on-disk location for SkillOpt traces, fetched downstream READMEs, and
generated semantic documentation (`skills.md`, `relations.md`, `history/`).

- **Default**: Platform-conventional cache directory (e.g.
  `~/.cache/skillgraph-mcp/lattice` on Linux)
- **Override**: `--lattice-dir` flag for CWD-relative path
- **Legacy**: `./.mcp_lattice` is still accepted via `--lattice-dir`

<!-- openwiki: broken internal link [./docs/adr/0002-default-lattice-dir-outside-cwd.md] file "./docs/adr/0002-default-lattice-dir-outside-cwd.md" does not exist. Fix the href or restore the target, then delete this comment. -->
See [ADR-0002](./docs/adr/0002-default-lattice-dir-outside-cwd.md) for the
rationale.

## LLM Provider Configuration

Environment variables for the refine/SkillOpt loop (in priority order):

| Variable | Required | Description |
|---|---|---|
| `LLM_BASE_URL` | Yes | Base URL for LLM API |
| `LLM_API_KEY` | Optional | API key (if not using default provider) |
| `LLM_MODEL` | Optional | Model name (if not using default) |
| `OPENAI_API_KEY` | Fallback | OpenAI API key |
| `DEEPSEEK_API_KEY` | Fallback | DeepSeek API key |
| `GEMINI_API_KEY` | Fallback | Gemini API key |

Without any of these, the bootstrap and optimization loops silently skip.

## The skillGraph Section

The `skillGraph` section is auto-populated and mutated by the SkillOpt loops:

- **`descriptions`**: Maps server names (or individual tool names) to human-readable
  description strings. Both `"serverName"` and `"specific_tool_name"` are valid keys.
- **`relations`**: An array of edge objects, each with:
  - `source`: Source server or tool name
  - `target`: Target server or tool name
  - `type`: One of `PREREQUISITE_FOR`, `PRODUCES`, `REQUIRES`, `COMMON_NEXT_STEP`
  - `description` (optional): Human-readable explanation of the relationship

Manual `relations` entries are merged on top of inferred ones at startup. The gateway
also infers `PREREQUISITE_FOR` edges automatically from tool parameter names ending
in `_id` or `_number` (e.g. `comment_on_issue(issue_number)` gets an edge from
`create_issue` and `search_issues`).

## Security Considerations

- **Loopback default**: `--host` defaults to `localhost` and is therefore already
  loopback-only.
- **register_server gate**: The `register_server` gateway tool **refuses to
  register** when the gateway serves HTTP on a non-loopback address.
- **No multi-tenancy**: There is no authentication, no per-tenant isolation.
  Do not deploy the binary as a public HTTP endpoint.

<!-- openwiki: broken internal link [./docs/adr/0001-loopback-default-and-register-server-gate.md] file "./docs/adr/0001-loopback-default-and-register-server-gate.md" does not exist. Fix the href or restore the target, then delete this comment. -->
See [ADR-0001](./docs/adr/0001-loopback-default-and-register-server-gate.md)
for full details.

## Stability and Versioning

The public API surface includes:

- `mcp.json` schema
- CLI flags and subcommands
- 9 gateway tool names
- `.mcp_lattice/` file format

Everything else (prompts, log format, internal paths) is explicitly NOT public.
<!-- openwiki: broken internal link [./docs/adr/0003-versioning-policy-and-public-api-surface.md] file "./docs/adr/0003-versioning-policy-and-public-api-surface.md" does not exist. Fix the href or restore the target, then delete this comment. -->
See [ADR-0003](./docs/adr/0003-versioning-policy-and-public-api-surface.md)
for the full versioning policy.
