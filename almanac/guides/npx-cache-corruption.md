---
title: npx Cache Corruption (npm 11.x)
topics: [architecture, gotchas]
sources:
  - {id: npx_bug_doc, type: file, path: docs/GITNEXUS_NPX_BUG.md}
  - {id: readme_gotcha, type: file, path: README.md, note: Gotchas section}
---

# npx Cache Corruption (npm 11.x)

When a downstream MCP server launched via `npx <package>` in `mcp.json` fails with:

```
npm error Cannot destructure property 'package' of 'node.target' as it is null.
```

This is an npm 11.x cache-corruption bug — not a skillgraph or downstream-server bug [@npx_bug_doc].

## Why It Happens

npm 11.x introduced a regression where `_npx/<hash>` cache entries can be corrupted, causing `node.target` to be null when npm tries to resolve the package. The error is non-deterministic and depends on the cached state. Once triggered, the `~/.npm/_npx/<hash>/` cache stores the half-installed tree and subsequent runs re-crash with the same cryptic error.

## Fixes (in preference order)

1. **Use globally-installed binaries with absolute paths** (recommended):
   ```bash
   ~/.local/share/pnpm/bin/gitnexus analyze
   ```
   Or ensure `~/.local/share/pnpm/bin` is on your `PATH`, then:
   ```bash
   gitnexus analyze
   ```

2. **Install globally**: `npm install -g gitnexus && gitnexus analyze`

3. **Nuke the npx cache**: `rm -rf ~/.npm/_npx && npx gitnexus analyze`

4. **Downgrade npm**: `npm install -g npm@10`

## Impact on skillgraph-mcp

The gateway handles the failed connection gracefully — `validate` shows the downstream as failed and the rest of the graph comes up. However, the agent-facing error message points nowhere useful. Surface the class of problem and the three fixes at the docs level so users self-serve [@readme_gotcha].

## Prevention

Prefer globally-installed binaries with absolute paths in `mcp.json` to avoid the entire class of problem. This is the recommended approach for all downstream MCP servers (gitnexus, brave-search, context7, etc.) [@npx_bug_doc].

## Related

- [Gateway Routing Model](../architecture/gateway-routing) — how downstream servers connect
