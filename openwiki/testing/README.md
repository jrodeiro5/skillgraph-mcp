---
type: Testing Guide
title: Testing & Benchmarks
description: >
  Testing strategy for skillgraph-mcp: unit tests, E2E tests with fake MCP
  servers and mock LLM endpoints, and the embedding vs. typed-graph benchmark
  comparing retrieval strategies across 30 tasks.
resource: /Makefile
tags: [testing, benchmarks, e2e, unit-tests, embedding]
---

# Testing & Benchmarks

skillgraph-mcp uses a layered testing strategy: unit tests for all internal
packages, E2E tests with fake infrastructure, and structured benchmarks for
retrieval strategy comparison.

## Unit Tests

Run all unit tests:

```bash
go test ./...
```

Test a single package:

```bash
go test ./internal/graph/...
```

Run a specific test:

```bash
go test ./internal/refine/... -run TestOptimizeTracesExecution
```

Unit tests cover:

- Graph construction and traversal
- Config loading and validation
- Tool registration and execution
- Embedding index operations
- Version checking

## E2E Tests

Run end-to-end tests:

```bash
go test ./internal/e2e/...
```

E2E tests spin up **fake MCP servers** and **mock LLM endpoints** to validate:

- Full gateway lifecycle (start → tool call → shutdown)
- Multi-server connectivity
- SkillOpt loop behavior with synthetic traces
- Embedding index rebuild on graph update

Helper utilities are in `internal/e2e/helpers_test.go`.

## Embedding vs. Graph Benchmark

A controlled experiment comparing two retrieval strategies:

### Configuration

| Config | Retrieval | SkillOpt | find_tools |
|---|---|---|---|
| **C** | Typed-edge graph (`plan_workflow`) | enabled | disabled |
| **E** | Embedding cosine (`find_tools`) | disabled | enabled |

### Corpus

- **95 tools** across **9 downstream MCP servers**
- **30 tasks** (10 L1 direct, 10 L2 sequential, 10 L3 conditional/multi-step)

### Metrics

- **Recall@1** (primary)
- **MRR@5** (secondary)
- **Wilson score interval** at 95%

### Decision Rule

If E ≥ C − 0.05 on Recall@1, ship E and deprecate typed-graph retrieval.

Benchmark data is stored in `benchmarks/2026-06_embed_vs_graph/`:

- `config.md`: Experiment configuration
- `tasks.jsonl`: Task corpus

## Linting

Run the full lint suite:

```bash
golangci-lint run
```

Includes: errcheck, govet, staticcheck, unused + goimports.

## Build

```bash
go build -o skillgraph-mcp .
```
