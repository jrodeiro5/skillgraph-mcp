# ADR-0003: Versioning policy and public API surface

## Status

Accepted

## Context

skillgraph-mcp ships as a binary and as a Docker image. Users wire it into
their MCP client via a `mcp.json` config and CLI flags. Past the v0.1.x
window we need an explicit statement of what users may depend on across
versions, so they can reason about upgrades.

Go's `internal/` rule already makes the library API non-public: external
imports of `internal/...` are blocked by the compiler. ADR-0001 and the
removal of the README's "Embedding in Go" section formalise that. The
question this ADR resolves is: of the user-observable surface, **what
constitutes a public API**, and what changes require a major bump?

## Decision

The **public API** of skillgraph-mcp is exactly the following surface.
Breaking changes here require a major version bump (`vX.Y.Z` → `v(X+1).0.0`).

1. **`mcp.json` schema.** The top-level structure (`mcpServers`,
   `skillGraph.descriptions`, `skillGraph.relations`), the field names and
   types inside each entry (`command`, `args`, `env`, `type`, `url`,
   `headers`, `description`, `allowedTools`, `allowedResources`), and the
   semantics of `${VAR}` interpolation.

2. **CLI flags and subcommands.** The flags listed in `README → Flags`
   (`--config`, `--lattice-dir`, `--transport`, `--host`, `--port`,
   `--version`) and the subcommands (`serve`, `doctor`, `validate`,
   `list-skills`). Their **names**, **accepted values**, and **default
   semantics** are public.

   Default *values* are also public for flags where the default has
   user-observable side effects (e.g. ADR-0002 changing `--lattice-dir`
   default would have been a breaking change pre-v1.0; post-v1.0 it
   would require a major bump). Default values of cosmetic-only flags
   (log format, timeout in seconds) are not part of the API.

3. **Gateway tool names and JSON schemas.** The 8 tools the agent sees
   (`list_skills`, `use_skill`, `read_resource`, `execute_code`,
   `register_server`, `get_skill_graph`, `plan_workflow`, `read_lattice`)
   are part of the public API: agents and skills depend on calling them by
   name. Adding a tool is a minor bump; removing or renaming one is major.
   Changing a tool's input schema in a non-backwards-compatible way is
   major.

4. **`.mcp_lattice/` file format.** Third parties (e.g. dashboards, log
   shippers) may parse `traces/*.json`, `skills.md`, `relations.md`, or
   `history/*.json`. Adding fields is minor; removing or renaming existing
   fields is major.

The following are explicitly **NOT public API** and may change in any
release:

- Anything under `internal/...`. Go forbids external imports of these
  paths; we do not commit to their stability.
- The exact LLM prompt text used by the SkillOpt and bootstrap loops.
- Log format and slog field names. Use `--json` outputs of the CLI
  subcommands if you need machine-readable output.
- Path layout inside `~/.cache/skillgraph-mcp/lattice/` beyond the
  filenames documented in (4). Subdirectory reorganisation is allowed.
- The Docker image's internal filesystem layout. Use the documented
  entrypoint flags only.

## v1.0 entry criteria

We tag v1.0 when:

1. The two implementations decided in ADR-0001 and ADR-0002 are merged.
2. CI runs `go test -race` on every push to main.
3. The flaky tests inventory is empty (`TestE2EStructuredOutput` was the
   last known one — fixed by making trace writes synchronous; see commit).
4. This ADR has been merged.
5. CHANGELOG carries an explicit "Public API surface" section linking
   here.

Until then, expect minor bumps to occasionally include surface changes
the v1.0 contract would otherwise classify as breaking. Pre-1.0 patches
remain backwards-compatible at best effort but without contractual
guarantee.

## Alternatives considered

- **Treat every CLI flag and tool name as public from v0.1.x.** Rejected:
  v0.1.x is the time to discover the right shape. Locking in early
  prevents reasonable cleanup (the lattice dir default change in ADR-0002
  is a concrete example).

- **No public API surface, everything may change.** Rejected: users
  cannot adopt a tool whose CLI flags can disappear between patches. The
  README implicitly promises stability; this ADR makes the promise
  explicit and bounded.

- **Semver-major on every CLI flag rename.** Adopted, but only post-v1.0.
  Pre-v1.0 we keep flexibility documented in CHANGELOG.

## Consequences

- ADR-0001 and ADR-0002 are pre-v1.0 changes that would be major bumps
  post-v1.0; they ship inside the v0.1.x → v0.2.0 transition.
- Future "admin" tools added by `register_server`-style hot registration
  are minor bumps, since they add to the surface without removing.
- A future migration to a different MCP protocol version is a major bump
  unless the go-sdk handles backwards compat transparently.
