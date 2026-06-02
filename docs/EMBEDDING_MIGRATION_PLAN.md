# Embedding-Based Routing — Migration Plan

**Status:** Proposal, not committed.
**Date:** 2026-06-02.
**Author note:** Replaces / demotes the LLM-authored typed-graph approach with semantic retrieval. Aligns skillgraph-mcp with Zapier MCP, RAG-MCP, and 2026 arXiv consensus.

---

## 1. Context

The current skillgraph-mcp gateway uses a hand-authored / LLM-authored typed graph (`HAS_TOOL`, `PREREQUISITE_FOR`, `PRODUCES`, `REQUIRES`, `COMMON_NEXT_STEP`) plus a background SkillOpt loop that mutates descriptions and relations from execution traces.

Observed behaviour on this user's setup (2026-05-29 → 2026-06-02, 220 traces, 314 tool calls):

| Metric | Value |
|---|---|
| Tools configured (descriptions) | 86 |
| Tools actually called | 34 (40%) |
| Tools never called | 62 (72% of configured) |
| Tools called with 100% error rate | 4 (`brave_llm_context`, `browser_click`, `browser_type`, `browser_run_code_unsafe`) |
| Servers configured (post aguara/excalidraw removal) | 9 |
| SkillOpt cycles run | >100 |
| SkillOpt improvements measurably validated | 0 (no benchmark exists) |

The gateway pays prompt-token cost for 62 dead-weight descriptions. SkillOpt mutates descriptions against an unmeasured baseline.

## 2. 2026 arXiv consensus on tool routing

| Paper | ID | Mechanism | Key number |
|---|---|---|---|
| Tool-Schema Compression Under Constrained Context | [2605.26165](https://arxiv.org/abs/2605.26165) | Dynamic selection + schema compression | Recommends both as complementary |
| Outcome-Aware Tool Selection for Semantic Routers | [2603.13426](https://arxiv.org/abs/2603.13426) | Embedding router + outcome feedback | Outcome-aware beats pure cosine |
| Latency-Quality Routing for Equivalent Tools | [2605.14241](https://arxiv.org/abs/2605.14241) | Cost/latency routing among substitutes | New routing axis |
| Tool-to-Agent Retrieval | [2511.01854](https://arxiv.org/abs/2511.01854) | Hierarchical: server → tool | Beats flat retrieval |
| RAG-MCP (foundational) | [2505.03275](https://arxiv.org/abs/2505.03275) | Embed tools, top-k pre-LLM | −50% prompt tokens, 3× accuracy |
| MCP-Zero | [2506.01056](https://arxiv.org/abs/2506.01056) | Active discovery, zero upfront list | Extends RAG to tool selection |

Unanimous direction: **embed tools, retrieve by NL query, expose only top-k**. No 2026 paper proposes an LLM-authored typed graph as the primary retrieval mechanism.

## 3. Zapier MCP comparison

Zapier exposes 9 000+ apps / 40 000+ actions through **14 static meta-tools**, not a flat list. Pattern:

1. `discover_zapier_actions(query)` — NL search over the catalog
2. `enable_zapier_action(id)` — add to the current agent's surface
3. `execute_zapier_read_action` / `execute_zapier_write_action` — invoke

Same shape as skillgraph's `use_skill` + `execute_code`. **Difference: Zapier's discover step is semantic search, not graph traversal.**

## 4. Proposed architecture

### 4.1 Keep

- Gateway pattern, 8-ish meta-tools, progressive disclosure (Phase 3 of ROADMAP).
- `execute_code` Python sandbox + trace capture.
- Trace TTL + size cap (already patched today).
- Atomic config persistence.
- Holdout gate, snapshot rollback, multi-provider LLM support.

### 4.2 Add

**Embedding index.** Per-tool description + per-server summary, embedded once at boot, refreshed when `mcp.json` changes. In-memory FAISS or sqlite-vss; pgvector only if multi-process. ~5 MB for 200 tools.

**New gateway tool `find_tools`.** Signature:

```
find_tools(query: str, k: int = 5, server_filter: str | None = None) -> [{tool, server, score, description}]
```

Replaces the role currently played by `plan_workflow` (which routes textually, not semantically) and demotes `list_skills` to a debug aid.

**Two-tier hierarchy** ([2511.01854](https://arxiv.org/abs/2511.01854)):

1. Embed server-level summaries; route query → top-2 servers.
2. Embed tool descriptions scoped to those servers; return top-3 per server.

Cheaper than flat embedding over all tools.

**Cost/latency metadata** ([2605.14241](https://arxiv.org/abs/2605.14241)). Each tool gets optional `cost_hint` / `latency_hint`. When two tools score within `ε` on semantic similarity, prefer cheaper one. Example: `brave_web_search` over `firecrawl_scrape` when both can answer.

**Outcome-aware re-ranking** ([2603.13426](https://arxiv.org/abs/2603.13426)). Track, per (query-embedding-cluster, tool) pair, whether the call succeeded. Adjust ranking by historical success rate. Replaces SkillOpt's LLM-judged description edits with a real loss function.

### 4.3 Remove or demote

- **SkillOpt loop** (`internal/refine/engine.go::startOptimizationLoop`) — demote to disabled-by-default. Outcome-aware re-ranker takes its job and uses actual click-through, not LLM judgment.
- **Typed-edge graph** — keep the data structure for debug/visualization; stop using it as the primary retrieval mechanism. `read_lattice` remains for humans.
- **Bootstrap README-to-relations** — still useful for *descriptions*; stop generating typed edges from it.

## 5. File-level impact (skillgraph-mcp repo)

| File | Change |
|---|---|
| `internal/embed/embed.go` (new) | Wrap an embedding provider (`text-embedding-3-small`, Voyage, Ollama). Same provider priority chain as `refine/engine.go::getAPIKey`. |
| `internal/embed/index.go` (new) | In-memory matrix + cosine; rebuild on config change. ~150 LOC. |
| `internal/tools/find_tools.go` (new) | New gateway tool. Hierarchical: server top-2 → tool top-3. ~100 LOC. |
| `internal/app/server.go` | Register `find_tools`; keep others. +1 line. |
| `internal/mcpserver/manager.go` | Notify embed index on `RebuildGraph` / `AddServer`. |
| `internal/tools/plan_workflow.go` | Either delete or delegate to `find_tools` with `k=10`. Decide after benchmark. |
| `internal/refine/engine.go` | Add a feature flag to disable SkillOpt; keep bootstrap description fetch. |
| `internal/tools/execute_code.go` | Add post-call outcome logging into a `success_counts` sqlite db. |
| `internal/embed/rerank.go` (new) | Outcome-aware re-rank pass over top-k. ~80 LOC. |
| `BENCHMARKS.md` | Replace 4-config ablation with 2-config: **C** (current typed-graph) vs **E** (embedding-only). |

## 6. Verification

**End-to-end test (manual).**

1. `go build -o skillgraph-mcp .`
2. `LLM_BASE_URL=http://localhost:11434 ./skillgraph-mcp --config $AGENT_HUB/config/skillgraph-config.json --lattice-dir ~/.mcp_lattice`
3. From a client, call `find_tools(query="scrape a webpage and extract markdown", k=5)`. Expected: `firecrawl_scrape` in top-3.
4. Call `find_tools(query="find callers of a function", k=5)`. Expected: `gitnexus.impact` or `gitnexus.context` in top-3.
5. Call `find_tools(query="take a screenshot of a URL", k=5)`. Expected: `chrome-devtools.take_screenshot` or `playwright.browser_take_screenshot` in top-3.
6. Re-run with `--disable-skillopt`; confirm no SkillOpt slog lines after 5 min.

**Benchmark (real validation).**

1. Hand-author 30 tasks in `benchmarks/2026-06_embed_vs_graph/tasks.jsonl` matching the L1/L2/L3 difficulty levels from `BENCHMARKS.md` § Task Suite.
2. Run Config **C** (current) and **E** (embedding) against same task list, same model, same seed.
3. Compute Recall@1, task completion, mean turns to first correct tool. Report 95% CI.
4. Decision rule:
   - **E ≥ C — 0.05 on Recall@1**: ship E, mark typed-graph as deprecated.
   - **C > E + 0.05**: typed-graph thesis survives; first real validation; publish numbers.
   - **|C − E| < 0.05**: ship E anyway (simpler, less LLM cost).

## 7. Rollout sequencing

1. **Week 1.** Build `internal/embed/` + `find_tools.go`. No behaviour change for existing clients.
2. **Week 2.** Hand-author the 30-task benchmark suite. Wire harness.
3. **Week 3.** Run Config C vs E. Record numbers in `benchmarks/2026-06_embed_vs_graph/`.
4. **Week 4.** Based on results: deprecate SkillOpt + typed-edge retrieval, or revise.

No step ships a regression; `find_tools` is additive until the benchmark decides.

## 8. What is explicitly *not* in this plan

- No change to `read_resource`, `read_lattice`, `register_server`, `get_skill_graph` semantics.
- No change to atomic-write / snapshot rollback / holdout gate machinery.
- No GUI, no Web UI, no telemetry pipeline.
- No re-architecture of `Manager`. The patch surface is additive plus one feature flag.

## 9. Upstream lineage (added 2026-06-02)

The project forks [`kurtisvg/skillful-mcp`](https://github.com/kurtisvg/skillful-mcp), tagline:

> Too many MCP tools slowing your agent down? Might be a Skill Issue 😉

Upstream is **4 gateway tools**: `list_skills`, `use_skill`, `read_resource`, `execute_code`. No graph. No SkillOpt. No refinement loop. No README fetcher. ~120KB repo. 62 stars at time of writing.

Upstream's pitch is **pure progressive disclosure**: agent lists skills → inspects one → calls tools by name inside the Python sandbox. That is the entire mechanism.

The fork (this repo) added, in order:

| Layer | Files | LOC | Validated? |
|---|---|---|---|
| Typed-edge graph (`HAS_TOOL`, `PRODUCES`, `REQUIRES`, `PREREQUISITE_FOR`, `COMMON_NEXT_STEP`) | `internal/graph/*` | ~600 | No |
| `plan_workflow`, `get_skill_graph`, `read_lattice`, `register_server` | `internal/tools/*` | ~500 | No |
| Bootstrap README fetcher + LLM-authored descriptions/relations | `internal/refine/engine.go::refineServer`, `internal/docs/*` | ~900 | Partially (descriptions help; relations not measured) |
| SkillOpt trace-based refinement loop + holdout gate + snapshot rollback | `internal/refine/engine.go::startOptimizationLoop` | ~700 | No (no benchmark) |
| `.mcp_lattice` markdown generator | `internal/docs/lattice.go` | ~200 | Cosmetic |

Total addition over upstream: **~2 900 LOC, none of it benchmarked**.

### Reframing

The original "skill issue" tagline is honest: **agents have a tool-discovery problem, not a tool-relationship problem**. Upstream solves discovery with progressive disclosure; this fork bet that *typed relationships between tools* would further help. The 2026 papers cited above say that bet was the wrong axis — the right next move from upstream is **embeddings**, not **typed edges**.

If the embedding migration (§4-6) lands and the C-vs-E benchmark goes the expected way (E ≥ C), the honest project shape becomes:

```
upstream (4 tools, progressive disclosure)
   +
embedding-backed find_tools (~250 LOC)
   +
outcome-aware re-ranker (~80 LOC, kicks in after ~500 traces)
```

≈ **350 LOC of real value on top of upstream**, versus the current ~2 900 LOC of unvalidated machinery.

The rest of this fork (graph, SkillOpt, lattice) becomes either:

- **Debug/visualization affordance** (graph + lattice — fine to keep, demote in README).
- **Deprecated, with a clear pointer to the benchmark numbers** (SkillOpt).

This is not a criticism of the engineering — it is what 14 months of arXiv consensus says is the right architecture in 2026.

## 10. Open questions

1. **Embedding provider default.** `text-embedding-3-small` (cloud, 0.02$/1M) or local Ollama nomic-embed (free, slower)? Lean Ollama for parity with `LLM_BASE_URL` priority.
2. **Persistence.** In-memory only is fine for 200 tools; if user grows to thousands, swap for sqlite-vss. Defer.
3. **Cold start.** Embed all configured tools at boot (sync, ~2 s for 200 tools) or lazy on first `find_tools`? Sync at boot — keeps `find_tools` p99 < 50 ms.
4. **Re-ranker training data.** Need at least ~500 outcome rows before outcome-aware re-rank fires. Until then, pure cosine. Threshold goes in config.
