# Benchmark Config: Embedding vs Typed-Graph Tool Retrieval

**Date:** 2026-06-02  
**Corpus:** 95 tools across 9 downstream MCP servers  
**Tasks:** 30 (10 L1 + 10 L2 + 10 L3)

## Configs

| Config | Retrieval | SkillOpt | find_tools |
|--------|-----------|----------|------------|
| **C** | Typed-edge graph (`plan_workflow`) | enabled | disabled |
| **E** | Embedding cosine (`find_tools`) | disabled | enabled |

## Model

- Embedding: `nomic-embed-text` via `OLLAMA_HOST` (default) or `text-embedding-3-small` via `OPENAI_API_KEY`
- Dimensions: 256 (Matryoshka, set `EMBED_DIMENSIONS=256`)
- LLM for task execution: same as `LLM_BASE_URL` / `OPENAI_API_KEY`

## Metrics

- **Primary:** Recall@1 (correct tool in position 1)
- **Secondary:** MRR@5 (mean reciprocal rank over top 5)
- **CI:** Wilson score interval at 95%

## Decision Rule

| Result | Action |
|--------|--------|
| E ≥ C − 0.05 on Recall@1 | Ship E, deprecate SkillOpt + typed-graph retrieval |
| C > E + 0.05 | Typed-graph survives; publish numbers |
| \|C − E\| < 0.05 | Ship E (simpler, less LLM cost) |

## Expected Hypothesis

- L1: E ≈ C (both handle direct single-tool queries)
- L2: E ≈ C (sequential flows testable by both)
- L3: C > E by 8-10pp (typed edges help multi-branch planning)

If E matches C on L3 → typed-graph is fully validated as dead weight.
