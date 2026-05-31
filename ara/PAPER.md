# skillgraph-mcp: Progressive Disclosure and Semantic Skill Graphs for Multi-Server MCP Deployments

## Metadata

| Field | Value |
|---|---|
| **Author** | Javier Rodeiro Rodríguez |
| **Target venue** | arXiv cs.AI |
| **Paper type** | Systems / experience report |
| **Status** | Draft — abstract written, measurements captured |
| **Upstream fork** | [kurtisvg/skillful-mcp](https://github.com/kurtisvg/skillful-mcp) (MIT) |
| **Repository** | https://github.com/jrodeiro5/skillgraph-mcp |

## Layer Index

| Layer | Path | Status |
|---|---|---|
| Problem | `logic/problem.md` | draft |
| Claims | `logic/claims.md` | seeded |
| Architecture | `logic/solution/architecture.md` | pending |
| Heuristics | `logic/solution/heuristics.md` | seeded |
| Exploration tree | `trace/exploration_tree.yaml` | seeded |
| Evidence tables | `evidence/tables/` | T01 captured |
| Sessions | `trace/sessions/` | session_001 |

## Abstract (current draft)

As AI agents connect to increasing numbers of Model Context Protocol (MCP) servers,
tool proliferation degrades agent performance. An agent wiring ten MCP servers may
receive over 100 tool schemas in its context window before any user message is
processed — a phenomenon we term *tool bloat*. We present **skillgraph-mcp**, an
open-source MCP gateway built on top of skillful-mcp (Van Gent, 2026) that addresses
tool bloat through two mechanisms: (1) *progressive disclosure*, where the agent sees
a compact skill index upfront and retrieves per-server schemas on demand, and (2) a
*semantic skill graph* encoding prerequisite and successor relationships between tools,
enabling structured workflow planning. In our deployment of ten downstream servers
exposing 113 tools, skillgraph-mcp reduces upfront context consumption from ~3,660
tokens to 287 tokens — a **92% reduction** — while preserving full tool access.
Per-server schema retrieval costs 745 tokens for a 23-tool server (Playwright MCP),
meaning an agent that activates only one server pays 1,032 tokens total versus 3,660
for naive loading. We further describe **SkillOpt**, a background loop that uses
execution traces to continuously refine tool descriptions and graph relations via LLM
feedback, without manual annotation. skillgraph-mcp is production-ready, ships
prebuilt binaries for six platforms, and is available as open source.
