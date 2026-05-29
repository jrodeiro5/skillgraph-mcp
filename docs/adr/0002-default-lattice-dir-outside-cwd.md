# ADR-0002: Default `--lattice-dir` outside the working tree

## Status

Accepted

## Context

`.mcp_lattice/` holds three categories of data:

| Category   | Examples                                  | Persistence needed |
|------------|-------------------------------------------|--------------------|
| Cache      | `*_readme.md` (fetched downstream docs)   | No — regeneratable |
| Trace      | `traces/*.json` (SkillOpt rollouts)       | No — replayed once |
| Snapshot   | `history/skillgraph_*.json` (rollback)    | Yes, short window  |

Today the binary defaults `--lattice-dir` to `./.mcp_lattice` (CWD-relative).
For the documented target user — a developer wiring skillgraph-mcp into
Claude Code — the agent invokes the binary from each project's working
directory, so a `.mcp_lattice/` folder appears inside **every project the
user touches**. This was a footgun in the maintainer's own setup (we already
fixed it locally with `--lattice-dir /absolute/path`).

We want a portable default that:
- Lives outside any user repo so it never lands in commits.
- Persists across sessions and across `cwd` changes.
- Uses platform-standard locations a tool support engineer would expect.
- Does not require per-platform code in the binary.

## Decision

When `--lattice-dir` is not provided, resolve the default via
`os.UserCacheDir()` joined with `skillgraph-mcp/lattice`:

| OS       | Resulting default                                                   |
|----------|---------------------------------------------------------------------|
| Linux    | `$XDG_CACHE_HOME/skillgraph-mcp/lattice` or `~/.cache/skillgraph-mcp/lattice` |
| macOS    | `$HOME/Library/Caches/skillgraph-mcp/lattice`                       |
| Windows  | `%LocalAppData%\skillgraph-mcp\lattice`                             |

If `os.UserCacheDir()` returns an error (e.g. `$HOME` unset), fall back to
`./.mcp_lattice` and log a warning so the user knows where data went.

The CWD-relative path remains accessible via `--lattice-dir ./.mcp_lattice`
for users who want it (debug, tests, ephemeral runs).

## Alternatives considered

- **`$XDG_STATE_HOME` / `os.UserStateDir` semantics.** The XDG spec
  distinguishes state from cache (state is "data not essential, but persists
  between sessions"). But Go's `os` package does not abstract a `UserStateDir`,
  so we would need 30+ lines of per-platform code that drifts from std lib
  conventions. `UserCacheDir` is good enough — the OS rarely purges these
  directories, and if it does, the next run regenerates everything except
  the `history/` rollback window (acceptable trade-off).

- **`os.UserConfigDir()` (`~/.config/skillgraph-mcp` on Linux).** Conceptually
  wrong — config is what the user writes (their `mcp.json`), not what the
  binary generates.

- **Hardcoded `~/.mcp_lattice/`.** Cross-platform inconsistent (doesn't
  match macOS or Windows conventions); pollutes `$HOME`.

- **Keep `./.mcp_lattice` default and rely on docs.** Rejected for the same
  reason as the security defaults in ADR-0001: the documented target user
  will not read Gotchas before running the binary, and the cost of getting
  it wrong is `.mcp_lattice/` inside every git repo they touch.

## Consequences

- Existing users who relied on the CWD-relative default see traces moving to
  the cache dir on first run after the change. Documented in CHANGELOG.
- `validate` and `doctor` show the resolved path so users always know where
  data lives. (`doctor` already does this.)
- A `cache` directory implies the data is disposable. Long-running SkillOpt
  snapshots in `history/` are correct in cache only because we never rely on
  them surviving — they are best-effort rollback within a single session.
- If we later need true persistent state, we move just `history/` to a
  separate flag (`--history-dir`) defaulting to `os.UserConfigDir() + /skillgraph-mcp/history`.
