# GitNexus npx Cache Bug

## Symptom

Running `npx gitnexus analyze` fails with:

```
npm error Cannot destructure property 'package' of 'node.target' as it is null.
```

This is an npm 11.x cache-corruption bug — not a gitnexus or skillgraph-mcp issue.

## Fix

gitnexus is installed globally via pnpm. Always run it directly:

```bash
~/.local/share/pnpm/bin/gitnexus analyze
```

Or ensure `~/.local/share/pnpm/bin` is on your `PATH`, then:

```bash
gitnexus analyze
```

## Why npx breaks

npm 11.x introduced a regression where `_npx/<hash>` cache entries can be corrupted, causing `node.target` to be null when npm tries to resolve the package. The error is non-deterministic and depends on the cached state.

## Alternative fixes (if not using pnpm)

1. **Install globally**: `npm install -g gitnexus && gitnexus analyze`
2. **Nuke the npx cache**: `rm -rf ~/.npm/_npx && npx gitnexus analyze`
3. **Downgrade npm**: `npm install -g npm@10`

Prefer the pnpm approach — it avoids the entire class of problem.
