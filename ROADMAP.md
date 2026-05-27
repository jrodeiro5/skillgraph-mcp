# ROADMAP: Skill Graph Integration in `skillgraph-mcp`

This document tracks the implementation plan for the skill graph model in the `skillgraph-mcp` fork, improving dynamic tool discovery and orchestration.

## Phase 1: Graph Data Structures (Go) [Done]
- [x] Create the `internal/graph` package.
- [x] Define node types: `Skill`, `Tool`, `Resource`.
- [x] Define relation types: `HAS_TOOL`, `PREREQUISITE_FOR`, `PRODUCES`, `REQUIRES`, `COMMON_NEXT_STEP`.
- [x] Implement in-memory `Graph` structure (node and edge CRUD).
- [x] Implement a compact serializer for LLMs (Markdown / relation lattice format).
- [x] Write unit tests in `internal/graph/graph_test.go`.

## Phase 2: Relation Inference and Configuration [Done]
- [x] Extend configuration in `internal/config/config.go` to allow user-defined static relations.
- [x] Integrate the graph into `internal/mcpserver/manager.go`.
- [x] Design automatic semantic inference engine at load time:
  - Auto-link `Skill -[HAS_TOOL]-> Tool`.
  - Infer `PRODUCES`/`REQUIRES` relations by cross-matching JSON input/output schemas (type and name matching, e.g. `xxx_id` suffix).
  - Group tools operating on the same entity by prefix (e.g. `github_create_issue` and `github_update_issue`).

## Phase 3: New MCP Tools for the Agent [Done]
- [x] Implement `get_skill_graph` tool:
  - Query the full graph or filter by a specific skill.
- [x] Implement `plan_workflow` tool:
  - Accept a descriptive string (high-level goal) and return a recommended tool execution path.
- [x] Register these tools in the main server (`internal/app/server.go`).
- [x] Write e2e and integration tests for the new tools.

## Phase 4: Offline Metadata Bootstrap [Done]
- [x] Create an offline command or script (`cmd/bootstrap_metadata` / CLI flag) that analyzes configured MCP servers.
- [x] Use a model/API to generate clear, use-case-oriented descriptions for each tool and skill (Adala-style automation).
- [x] Write these updated descriptions back to config to optimize initial routing.

## Phase 5: Semantic Lattice in Markdown [Done]
- [x] Create the `.mcp_lattice` folder.
- [x] Add a Go generator to produce `skills.md` and `relations.md`.
- [x] Expose the `read_lattice` tool (`internal/tools/read_lattice.go`).

## Phase 6: Native Documentation Downloader (go-github) [Done]
- [x] Add `github.com/google/go-github/v60` to `go.mod`.
- [x] Implement the README downloader in `internal/docs/fetcher.go`.
- [x] Wire documentation download into skill startup.
- [x] Verify download behavior and tests.

## Phase 7: Dynamic Skill Optimization (SkillOpt) [Done]
- [x] Implement trajectory logging (rollouts) in `execute_code.go` using context-based `TraceCollector`.
- [x] Save traces as JSON in `.mcp_lattice/traces/`.
- [x] Create a background daemon (`startOptimizationLoop` in `refine/engine.go`) that analyzes accumulated traces periodically.
- [x] Implement text optimization loop (SkillOpt) with prompts for DeepSeek/Gemini that suggest description and relation edits in `mcp.json`.
- [x] Validate that proposals contain no hallucinated nodes before persisting changes to config.
- [x] Write corresponding unit tests in `execute_code_test.go` and `engine_test.go`.

## Phase 8: SkillOpt Validation Gate [Done]
- [x] Add early exit in `optimizeTraces`: skip the LLM call when the batch contains no errors (neither `traj.Error` nor `is_error: true` in tool calls).
- [x] Delete processed trace files even when the LLM is skipped, to prevent unbounded accumulation.
- [x] Write `TestOptimizeTracesSkipsLLMWhenNoErrors` to verify the behavior.

## Phase 9: Multi-Provider LLM Support [Done]
- [x] Add `callOpenAICompat` in `refine/engine.go` — compatible with LiteLLM proxy, Ollama, OpenAI, and any OpenAI-compatible API.
- [x] Update `getAPIKey` to prioritize `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` over provider-specific keys.
- [x] Add support for `OPENAI_API_KEY` as a native OpenAI provider.
- [x] Update `refineServer` and `optimizeTraces` with a provider `switch` that includes the `"openai"` case.
- [x] Allow empty key for local unauthenticated servers (Ollama): omit the `Authorization` header when the key is empty.
- [x] Write `TestCallOpenAICompat`, `TestCallOpenAICompatNoKey`, and `TestGetAPIKeyLLMBaseURL`.

## Phase 10: Dynamic Server Registration [Done]

- [x] Add a `register_server` gateway tool that accepts a server config block and hot-registers it without restarting the process.
- [x] Wire live registration into `Manager` — connect to the new server, infer graph relations, and call `RebuildGraph`.
- [x] Persist the new server entry to `mcp.json` atomically so restarts are idempotent.
- [x] Write integration test: register a server at runtime, verify its tools appear in `list_skills` without a restart.

## Planned Improvements

- [x] **Hold-out validation gate**: accept SkillOpt edits only if they do not regress a reference trace set (aligned with arXiv:2605.23904, which uses a held-out validation set).
- [x] **Edit history with rollback**: save a change history for `mcp.json` to allow automatic revert if routing quality degrades.
- [x] **Graph topology ablation**: compare typed relations (`PRODUCES`, `REQUIRES`) against a flat graph to measure actual added value. Methodology, task suite design, instrumentation guide, and statistical validity requirements documented in `BENCHMARKS.md`.
