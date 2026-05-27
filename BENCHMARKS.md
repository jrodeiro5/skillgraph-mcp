# BENCHMARKS.md

## Mantra

**The problem is not tool capability — it is tool visibility.**

Most LLM agents fail not because the right tool doesn't exist, but because the agent can't find it. Exposing 80+ tool schemas upfront doesn't help; it actively hurts. The agent's attention fragments, routing accuracy collapses, and token cost explodes.

skillgraph-mcp is built on four beliefs:

1. **Selective exposure beats exhaustive listing.** Show the agent a small, navigable surface. Let it pull more depth on demand.
2. **Relations outperform flat lists.** A graph of typed semantic edges (`PREREQUISITE_FOR`, `PRODUCES`, `REQUIRES`, `COMMON_NEXT_STEP`) encodes workflow knowledge that no flat list can express.
3. **Descriptions are load-bearing.** A bad tool description is worse than no description. Routing is text matching. Every word in a description is a routing signal.
4. **Self-correction closes the loop.** Execution traces are ground truth. If a tool is consistently misused, the system should learn from that — and be able to undo learning that made things worse.

---

## Academic Grounding

Each architectural decision has independent academic backing from 2025–2026 research:

| Claim | Evidence | Paper |
|---|---|---|
| 80+ tools degrades LLM accuracy by 76–94% | Measured across major models | LongFuncEval — arXiv:2505.10570 |
| Context size hurts reasoning even with perfect retrieval | Validated at scale | arXiv:2510.05381 |
| Retrieve-then-expose is the correct fix (3× accuracy) | Benchmarked | RAG-MCP — arXiv:2505.03275 |
| Graph-based retrieval beats vector retrieval (+8pp Recall@1) | Benchmarked on tool routing | Agent-as-a-Graph — arXiv:2511.18194 |
| Progressive disclosure is a formal agent pattern | Surveyed | Agent Skills Survey — arXiv:2602.12430 |
| 97% of MCP tool descriptions have at least one smell | Audited 856 tools | MCP Smelly — arXiv:2602.14878 |
| Description optimization improves routing by +15% | Measured post-augmentation | MCP Smelly — arXiv:2602.14878 |
| Held-out validation gate is required for safe SkillOpt | Ablated in the paper | SkillOpt — arXiv:2605.23904 |

The one validated gap versus the literature: **typed edge topology**. Agent-as-a-Graph (arXiv:2511.18194) uses a bipartite agent↔tool graph and measures Recall@5 = 0.85. skillgraph-mcp uses a richer topology (`Skill → Tool` with typed cross-tool edges) but has not yet benchmarked whether those additional edge types add measurable value over a flat or bipartite graph. That is the subject of the ablation below.

---

## Ablation Study: Graph Topology

### Question

Do typed cross-tool relations (`PRODUCES`, `REQUIRES`, `PREREQUISITE_FOR`, `COMMON_NEXT_STEP`) improve tool routing accuracy and task completion rates compared to a flat graph (server → tool only)?

### Configurations

| Config | Description |
|---|---|
| **A — Baseline** | All tools exposed upfront, no gateway (standard MCP client behavior) |
| **B — Gateway, flat graph** | Progressive disclosure via 4 gateway tools; graph has only `HAS_TOOL` edges |
| **C — Gateway, typed graph** | Full skillgraph-mcp with all 5 edge types, no SkillOpt |
| **D — Gateway, typed graph + SkillOpt** | Full system including background description optimization |

Config A is the control. Configs B–D isolate the contribution of each architectural layer.

### Metrics

**Primary**

- **Recall@1** — on a `plan_workflow` call, does the first recommended tool match the ground-truth tool for the task? (Equivalent metric to Agent-as-a-Graph's Recall@1.)
- **Task completion rate** — fraction of benchmark tasks the agent completes without human redirection.

**Secondary**

- **Average tool calls per task** — proxy for efficiency; more calls = more exploration = weaker routing signal.
- **Mean turns to first correct tool call** — how many `use_skill` / `execute_code` cycles before the right tool is invoked.
- **Token cost per successful completion** — measures the real cost of the gateway layer.

### Task Suite

A benchmark task is a triple `(goal, ground_truth_tool_sequence, server_set)`.

Design tasks at three difficulty levels:

| Level | Description | Example |
|---|---|---|
| **L1 — single tool** | Goal maps directly to one tool | "Search the web for X" |
| **L2 — sequential** | Two tools with a data dependency | "Search for X, then save the result to memory" |
| **L3 — conditional** | Tool selection depends on prior output | "Search for X; if results are code, run it; otherwise summarize" |

Minimum suite: 30 tasks (10 per level), distributed across ≥3 downstream servers. Ground-truth sequences must be hand-annotated before running any config.

### Running the Ablation

**Step 1 — Prepare four config files.**

Each config file is a `mcp.json` variant:
- Config A: standard client (no gateway)
- Config B: gateway with `skillGraph.relations = []`
- Config C: gateway with full hand-authored typed relations
- Config D: Config C + run `LLM_BASE_URL=... skillgraph-mcp --config mcp.json` for 24h to allow SkillOpt to generate relations from traces

**Step 2 — Generate traces.**

For each config, run the task suite with a capable agent (e.g. Claude Sonnet via the Responses API):

```sh
# Pseudocode — adapt to your evaluation harness
for task in benchmark_tasks:
    result = agent.run(task.goal, mcp_server=config_X)
    record(task.id, config, result.tool_calls, result.success, result.tokens)
```

**Step 3 — Compute metrics.**

```sh
# Example: compute Recall@1 per config
jq '[.[] | select(.config == "C") | .first_tool_correct] | (map(select(. == true)) | length) / length' traces.jsonl
```

**Step 4 — Compare.**

Plot B vs C to isolate the typed-edge contribution. Plot C vs D to isolate SkillOpt. Plot A vs B to confirm the baseline benefit of progressive disclosure.

**Expected direction of results** (hypothesis, not confirmed):

- A → B: large improvement (progressive disclosure effect, validated by RAG-MCP)
- B → C: moderate improvement on L2/L3 tasks; negligible on L1 (typed edges only help for multi-step workflows)
- C → D: small improvement after SkillOpt stabilizes; regression possible if SkillOpt overfits without the hold-out gate

### Instrumentation

The gateway writes execution traces to `.mcp_lattice/traces/` automatically. Each trace is a `Trajectory` JSON with:
- `code` — the Python sandbox code the agent wrote
- `tool_calls` — each tool invoked, args, result, error flag
- `output` / `error` — final result

For the ablation, add a `config` field to each trace at the harness level (not in the gateway itself) so results can be partitioned by configuration.

### Statistical Validity

- Minimum 30 tasks per config for meaningful Recall@1 estimates (±18pp at 95% CI)
- Use the same random seed for agent temperature across configs
- Run each config on the same task order to control for task difficulty variance
- Report confidence intervals, not just point estimates

---

## Rollout Quality Monitoring

Beyond the ablation, use these signals to monitor the live system:

| Signal | Source | Healthy range |
|---|---|---|
| Batch error rate | `.mcp_lattice/traces/` | Declining or stable over 24h |
| Hold-out gate rejection rate | `slog WARN` output | < 30% of proposed edits rejected |
| Auto-rollback frequency | `slog WARN` rollback lines | 0 rollbacks/day at steady state |
| SkillOpt edit rate | `slog INFO` applied edits | Declining as descriptions stabilize |

A system that never triggers the rollback and has a declining error rate is converging correctly. A system that rollbacks frequently is either seeing non-stationarity in downstream tool behavior (expected) or has a poorly calibrated `rollbackThreshold` (adjustable constant in `engine.go`).

---

## Reproducing the Baseline Numbers

The arXiv papers cited above used different tool sets and agents. To reproduce the 76–94% accuracy drop from LongFuncEval (arXiv:2505.10570):

1. Configure a single downstream server with 80+ tools (e.g., a broad GitHub MCP server plus a search server)
2. Run Config A (all tools exposed) on the L1 task suite
3. Run Config B (gateway, flat graph) on the same suite
4. The gap between A and B on L2+ tasks should approximate the literature numbers

Note: exact numbers will vary by model. The drop is more severe for smaller models and less severe for frontier reasoning models that handle long contexts better.

---

## Contributing Benchmark Results

If you run this ablation on a real tool set, please open a PR adding your results to a `benchmarks/` directory with:

```
benchmarks/
  <date>_<model>_<server_set>/
    config.md        # what servers, which model, task descriptions
    tasks.jsonl      # the 30+ tasks with ground truth
    results.jsonl    # raw traces + metrics
    summary.md       # recall@1, completion rate, cost per task per config
```

The first set of real numbers will close the one open question in this project's academic validation.
