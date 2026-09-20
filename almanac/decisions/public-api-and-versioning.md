---
title: Public API and Versioning
topics: [decisions, versioning]
sources:
  - {id: adr_0003, type: file, path: docs/adr/0003-versioning-and-public-api-surface.md}
  - {id: root_go, type: file, path: cmd/root.go}
  - {id: changelog, type: file, path: CHANGELOG.md}
---

# Public API and Versioning

ADR-0003 defines which parts of skillgraph-mcp's user-observable surface are
stable across versions, and what kind of version bump each change requires. It
exists because the project ships as a binary and a Docker image that users wire
into their MCP client through `mcp.json` and CLI flags; without an explicit
statement of what they may depend on, every upgrade is a gamble.

## Context

Go's `internal/` rule already makes the library API non-public: the compiler
blocks external imports of `internal/...`, so nothing under `internal/` can be
relied on across releases. ADR-0001 (loopback-only `register_server`) and
ADR-0002 (cache-directory lattice default) formalised two such boundaries. The
question ADR-0003 resolves is what remains of the *observable* surface — the
parts agents, skills, and third-party tooling depend on by name.

## Decision

The public API is exactly four surfaces; breaking changes here require a major
bump (`vX.Y.Z` → `v(X+1).0.0`) [@adr_0003]:

1. `mcp.json` schema — the `mcpServers` and `skillGraph` (`descriptions`,
   `relations`) structure, the field names and types inside each entry, and the
   semantics of `${VAR}` interpolation.
2. CLI flags and subcommands — their names, accepted values, and default
   semantics. Default values of cosmetic-only flags (log format, timeout in
   seconds) are not public.
3. Gateway tool names and JSON schemas — the tools the agent calls by name.
   Adding a tool is a minor bump; removing or renaming one is major; changing a
   tool's input schema in a non-backwards-compatible way is major.
4. `.mcp_lattice/` file format — `traces/*.json`, `skills.md`, `relations.md`,
   and `history/*.json`. Adding fields is minor; removing or renaming existing
   fields is major.

Explicitly **not** public API (may change in any release): anything under
`internal/...`, the exact LLM prompt text used by the refine loops, log format
and `slog` field names, the internal layout of `~/.cache/skillgraph-mcp/`
beyond the filenames documented in surface 4, and the Docker image's internal
filesystem layout.

## Status

Accepted. `v1.0.0` was tagged 2026-05-29, but the ADR's own v1.0 entry criteria
are only partially met. CI runs `go test -race` on every push to main, both
prior ADRs are implemented, the last known flaky test was fixed by making
trajectory writes synchronous, and this ADR is merged — but criterion 5, a
"Public API surface" section in `CHANGELOG.md`, is not yet present
[@changelog].

## Delta From the ADR Against Current Code

ADR-0003 was written before `find_tools` was added, so its tool list contains
only 8 tools. Current code registers 9 (the nine `Register*` calls in
`internal/app/server.go`): `list_skills`, `use_skill`, `read_resource`,
`execute_code`, `find_tools`, `register_server`, `get_skill_graph`,
`plan_workflow`, and `read_lattice`. Per the ADR's own rule, `find_tools` is
now part of the public API.

The ADR also enumerates only `serve`, `doctor`, `validate`, and `list-skills`.
`cmd/root.go` registers two more subcommands: `probe` and `generate-skills`
[@root_go].

## Consequences

- Adding a gateway tool or a lattice file field is a minor bump.
- Removing or renaming a gateway tool, changing a tool's input schema
  incompatibly, or dropping a lattice field is a major bump.
- Future work that adds a gateway tool must update this page and ADR-0003, not
  just the registration in `internal/app/server.go`.

## Related

- [Gateway Routing Model](../architecture/gateway-routing) — the 9 gateway tools whose names are part of this surface.
- [Config Persistence](../architecture/config) — the `mcp.json` schema is the first public-API surface.
- [Semantic Lattice](../architecture/lattice) — the lattice file format is the fourth public-API surface.
- [Refinement Engine](../architecture/refine) — the refine loops' LLM prompt text is explicitly not part of the public API.
