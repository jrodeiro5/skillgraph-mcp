# skillgraph-mcp — Domain glossary

Canonical terminology and audience definitions for this project. Update
inline as decisions crystallise during planning. Do **not** put implementation
details, specs, or scratch-pad notes here — this file is a glossary only.

For architectural decisions with non-obvious trade-offs, see `docs/adr/`.

---

## Audience

### Primary user (supported)

**An agent developer wiring 5–15 MCP servers to a local MCP client** —
Claude Code, Gemini CLI, Codex, or any client that speaks the stdio MCP
protocol. The user runs skillgraph-mcp on the same machine as the client,
configures it via `mcp.json`, and registers it as a single gateway in the
client's MCP config.

Defaults are tuned for this case: stdio transport, single process, single
user, local LLM provider (or none) for the optional SkillOpt loop.

### Out of scope (explicitly NOT supported)

- **Shared service / multi-tenant deployment.** `--transport http` exists
  for debugging and Inspector connection only — it has no authentication,
  no per-tenant isolation, and `register_server` can spawn arbitrary
  processes. Do not deploy the binary as a public HTTP endpoint.
- **Library use from another Go program.** The README's "Embedding in Go"
  section documented `import` paths under `internal/`, which Go forbids
  across module boundaries. Treat the binary as the only supported
  interface; if you need programmatic use, fork the repo.

## Terms

### Lattice directory

The on-disk location for SkillOpt traces, fetched downstream READMEs, and
generated semantic documentation (`skills.md`, `relations.md`, `history/`).
Defaults to a platform-conventional **state directory** outside the user's
working tree — see [ADR-0002](docs/adr/0002-default-lattice-dir-outside-cwd.md).
The legacy CWD-relative path `./.mcp_lattice` is still accepted via
`--lattice-dir`.

### Loopback gating

`--host` defaults to `localhost` and is therefore already loopback-only.
The `register_server` gateway tool **refuses to register** when the
gateway serves HTTP on a non-loopback address. The bind default is
unchanged from upstream; the new behaviour is the tool-level gate —
see [ADR-0001](docs/adr/0001-loopback-default-and-register-server-gate.md).

### Out-of-scope terms (do not introduce)

- "Embedding in Go" / "as a library" — Go's `internal/` rule blocks external
  imports, and we do not commit to a stable library API. Anyone wanting
  in-process integration is expected to fork.
- "Tenant", "user account", "role" — there is no multi-tenant or RBAC model.
  A single OS user runs the binary against their own MCP client.
