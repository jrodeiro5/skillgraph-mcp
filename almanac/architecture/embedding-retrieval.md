---
title: Embedding-Based Retrieval
topics: [architecture]
sources:
  - {id: embed_pkg, type: file, path: internal/embed/embed.go}
  - {id: find_tools, type: file, path: internal/tools/find_tools.go}
  - {id: build_entries, type: file, path: internal/tools/find_tools.go, note: BuildIndexEntries}
  - {id: serve_embedding, type: file, path: cmd/serve.go, note: embedding index boot}
  - {id: benchmark_config, type: file, path: benchmarks/2026-06_embed_vs_graph/config.md}
  - {id: benchmark_tasks, type: file, path: benchmarks/2026-06_embed_vs_graph/tasks.jsonl}
---

# Embedding-Based Retrieval

The gateway provides `find_tools`, a semantic search tool that returns the top-k tools most relevant to a natural language query. This complements (and can replace) the typed-graph `plan_workflow` tool.

## Architecture

The `internal/embed` package provides embedding-based tool retrieval. It embeds tool descriptions once at boot, rebuilds on config change, and exposes cosine-similarity search [@embed_pkg].

### Provider Resolution

`ResolveConfig()` detects the embedding provider using the same priority chain as the LLM provider:

1. `LLM_BASE_URL` set → Ollama (default model: `nomic-embed-text`)
2. `OLLAMA_HOST` set → Ollama (default model: `nomic-embed-text`)
3. `OPENAI_API_KEY` set → OpenAI (default model: `text-embedding-3-small`)
4. None set → `ProviderNone` (find_tools returns a helpful error)

Optional: `EMBED_DIMENSIONS` (e.g., `256`) enables Matryoshka dimension reduction for OpenAI models.

### Index Lifecycle

At boot, `cmd/serve.go` calls `embed.ResolveConfig()` and builds the embedding index synchronously [@serve_embedding]. On config change, the index rebuilds.

### find_tools Tool

The `find_tools` gateway tool accepts a natural language query, optional `k` (default 5, max 20), and optional `server_filter` [@find_tools]. It returns the top-k tools ranked by cosine similarity.

When the embedding index is unavailable (no provider configured), `find_tools` returns a helpful error message directing the user to fall back to `list_skills` + `use_skill`.

### Index Entries

`BuildIndexEntries` converts all downstream tools into `embed.ToolEntry` objects, combining tool name + signature + description into a single text string for embedding [@build_entries]. This gives the embedding model both what the tool is called and what it does.

## Benchmark: Embedding vs Typed-Graph Retrieval

A benchmark suite (`benchmarks/2026-06_embed_vs_graph/`) compares embedding-based retrieval (config E) against typed-graph retrieval (config C) across 30 tasks at three difficulty levels (L1/L2/L3) [@benchmark_config]. The tasks cover direct tool queries, sequential tool chains, and conditional branching workflows [@benchmark_tasks].

The decision rule for the benchmark:

- If embedding recall ≥ graph recall − 0.05 on Recall@1 → ship embedding, deprecate typed-graph retrieval
- If graph recall > embedding recall + 0.05 → typed-graph survives
- If |C − E| < 0.05 → ship embedding (simpler, less LLM cost)

## Gotchas

- **Embedding index is optional**: If no provider is configured, `find_tools` returns an error, not a panic. The agent falls back to `list_skills` + `use_skill`.
- **Boot-time blocking**: `Rebuild` blocks until all embeddings are fetched. For large tool counts, this can take seconds.
- **Matryoshka dimensions**: Setting `EMBED_DIMENSIONS` only applies to OpenAI providers. Ollama uses the model's native dimension.

## Related

- [Gateway Routing Model](gateway-routing) — how `find_tools` fits into the 8 gateway tools
