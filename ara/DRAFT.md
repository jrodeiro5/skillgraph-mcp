# skillgraph-mcp: Progressive Disclosure and Semantic Skill Graphs for Multi-Server MCP Deployments

**Javier Rodeiro Rodríguez**  
arXiv cs.AI — Systems / Experience Report  

---

## Abstract

As AI agents connect to increasing numbers of Model Context Protocol (MCP) servers,
tool proliferation degrades agent performance. An agent wiring ten MCP servers may
receive over 100 tool schemas in its context window before any user message is
processed — a phenomenon we term *tool bloat*. We present **skillgraph-mcp**, an
open-source MCP gateway built on top of skillful-mcp [Van Gent 2026] that addresses
tool bloat through two mechanisms: (1) *progressive disclosure*, where the agent sees
a compact skill index upfront and retrieves per-server schemas on demand, and (2) a
*semantic skill graph* encoding prerequisite and successor relationships between tools,
enabling structured workflow planning. In a deployment of ten downstream servers
exposing 113 tools, skillgraph-mcp reduces upfront context consumption from ~3,660
tokens to 287 tokens — a **92% reduction** — while preserving full tool access. The
gateway becomes beneficial at four or more servers (~9 tools); below this threshold
the gateway overhead exceeds the schema cost it replaces. We further describe
**SkillOpt**, a background loop that uses execution traces to continuously refine tool
descriptions and graph relations via LLM feedback without manual annotation.
skillgraph-mcp is production-ready, ships prebuilt binaries for six platforms, and
is available as open source at https://github.com/jrodeiro5/skillgraph-mcp.

---

## 1. Introduction

The Model Context Protocol [Anthropic 2024] has rapidly become the standard
integration layer between AI agents and external capabilities. An agent developer
today may connect 10–20 MCP servers — search, memory, browser automation, code
intelligence, databases, and more — each exposing anywhere from 1 to 30+ tools.

This creates a structural problem: MCP clients load all registered server schemas at
connection time. Every tool's name, description, and JSON Schema `inputSchema` enters
the model's context window before the user types a single word. With ten servers and
113 tools, this pre-task overhead reaches ~3,660 tokens. At 20 servers, it approaches
7,000 tokens — a significant fraction of the effective working context, consumed by
capability declarations rather than task-relevant content.

We observe that most agent tasks use a small subset of available tools. A user asking
"search for papers on attention mechanisms" needs brave-search or firecrawl — not
Playwright, gitnexus, wsl-terminal, or postgres. Yet all of their schemas are loaded
regardless.

This paper presents skillgraph-mcp, a gateway that applies *progressive disclosure*
to MCP tool loading. The agent sees a 10-line skill index upfront and requests
per-server schemas only when needed. We report empirical measurements of the token
savings, characterize when the gateway is and is not beneficial, describe the semantic
skill graph that guides tool selection, and present SkillOpt — a trace-driven
self-improvement loop that refines the graph without annotation.

---

## 2. Background and Related Work

### 2.1 Tool bloat in MCP deployments

Tool bloat was first described by Van Gent [2026] in the context of skillful-mcp,
the upstream project this work builds on. Van Gent observed that context consumption
grows linearly with registered servers and proposed a gateway architecture with eight
fixed discovery tools as the solution. skillgraph-mcp extends this with semantic
graph structure and the SkillOpt loop.

### 2.2 Enterprise MCP gateways

Microsoft MCP Gateway and IBM AI Gateway MCP address the operational concerns of
multi-server MCP deployments: Kubernetes lifecycle management, RBAC, and adapter
routing. These systems solve the *deployment* problem; they do not reduce agent
context consumption. skillgraph-mcp solves the *cognition* problem and is
complementary to these approaches.

### 2.3 Client-side progressive disclosure

NousResearch's Hermes tool-search [2025] implements progressive disclosure at the
*client* layer: three bridge tools (search_tools, get_tool_schema, call_tool) replace
all tool schemas, activating when tool context exceeds 10% of the context budget.
skillgraph-mcp implements the same concept at the *server* layer, making it
transparent to the client and compatible with any MCP client. The approaches are
complementary; skillgraph-mcp's gateway can itself be exposed through Hermes-style
bridge tools.

### 2.4 Context budget management in MCP clients

Claude Code v2.1.105 introduced `skillListingBudgetFraction`, which limits the
fraction of context reserved for skill descriptions. When exceeded, least-used skill
descriptions are collapsed to bare names. This is a *client-side* graceful degradation
mechanism operating on skill metadata; skillgraph-mcp provides *server-side* schema
hiding that prevents schemas from entering the context at all.

---

## 3. System Design

### 3.1 Gateway architecture

skillgraph-mcp acts as a single MCP server to the client and a multi-server client
to N downstream MCP servers. It exposes exactly eight gateway tools regardless of how
many downstream servers are connected:

| Tool | Purpose |
|---|---|
| `list_skills` | Compact index: skill name + one-line description |
| `use_skill` | Full tool schemas for one skill, on demand |
| `execute_code` | Python sandbox; all downstream tools callable by name |
| `plan_workflow` | Skill graph traversal for multi-step task planning |
| `get_skill_graph` | Full graph dump (nodes + edges) |
| `read_lattice` | Generated documentation (skills.md, relations.md) |
| `read_resource` | Downstream MCP resources |
| `register_server` | Runtime server registration (loopback-only, security gated) |

### 3.2 Progressive disclosure mechanics

At session start, the agent receives `list_skills` output: one line per skill with
name and description (~28 tokens/skill at 10 skills = 287 tokens total). To use a
specific server, the agent calls `use_skill(skill_name)` and receives that server's
full tool schemas (~32 tokens/tool empirically). Unused servers cost zero additional
tokens.

The crossover point — where gateway overhead equals raw schema cost — occurs at
approximately four servers (~9 tools). Below this threshold, deploying the gateway
increases rather than decreases context consumption. We document this honestly in
the project README.

### 3.3 Semantic skill graph

The in-memory directed graph (3 node types: Skill, Tool, Resource; 5 edge types:
HAS_TOOL, PREREQUISITE_FOR, PRODUCES, REQUIRES, COMMON_NEXT_STEP) encodes
relationships between tools across servers. Example: `resolve_library_id`
PREREQUISITE_FOR `query_docs` (context7 server). The agent calls `plan_workflow`
to retrieve a traversal over this graph for a given task description, receiving an
ordered list of tool calls to execute.

### 3.4 SkillOpt: trace-driven self-improvement

Two background goroutines run continuously:

**Bootstrap loop** — On startup, fetches GitHub READMEs for each downstream server,
sends tools + README to an LLM, and generates initial descriptions and graph
relations seeded into mcp.json.

**Refinement loop** — Polls trace files written by `execute_code` every 30 seconds.
Batches up to 10 trajectory files (code, tool calls, results, errors) and asks the
LLM to propose edits to tool descriptions and graph edges. Proposed edits are
validated (hallucinated tool names are rejected) and merged atomically via temp-file
rename.

The LLM provider is configurable via `LLM_BASE_URL`; the loop skips silently if
unconfigured, making the gateway functional without any LLM.

---

## 4. Evaluation

### 4.1 Experimental setup

Deployment: 10 downstream MCP servers, 113 total tools (Table 1). All measurements
taken on a local WSL2 environment. Token estimates use 4 characters per token
(conservative; actual savings likely higher for JSON Schema-heavy inputs).

The per-tool token rate (32.4 tokens/tool) is empirically measured from the
`use_skill playwright` response (Playwright MCP, 23 tools, 2,981 characters).

### 4.2 Upfront token cost: gateway vs raw loading

| Configuration | Tools exposed | Tokens |
|---|---|---|
| Raw (all schemas) | 113 | ~3,661 |
| skillgraph-mcp gateway | 10 skills | 287 |
| **Reduction** | **91%** | **92%** |

### 4.3 Scaling behavior

Gateway cost is constant (287 tokens) regardless of server count. Raw cost grows
linearly with tool count. Table 2 shows the crossover at ~4 servers:

| Servers | Tools | Raw tokens | Gateway tokens | Reduction |
|---|---|---|---|---|
| 1 | 2 | 64 | 287 | −348% |
| 4 | 9 | 291 | 287 | ≈0% (crossover) |
| 6 | 28 | 907 | 287 | 68% |
| 8 | 61 | 1,976 | 287 | 85% |
| 10 | 113 | 3,661 | 287 | 92% |

The gateway is not beneficial for users with 1–3 small servers.

### 4.4 Per-task cost model

An agent using only one server pays `287 + use_skill_cost`. For Playwright (23 tools):
287 + 745 = 1,032 tokens total, versus 3,661 for raw loading. An agent that solves a
task using only memory (9 tools, ~288 tokens): 287 + 288 = 575 tokens total.

---

## 5. Limitations and Future Work

**No task accuracy measurement.** We measure token cost reduction, not whether agents
make better tool selections. A controlled experiment comparing task success rates
with and without the gateway is left for future work.

**SkillOpt not evaluated.** The claim that SkillOpt improves routing accuracy (C03)
is a hypothesis; no before/after evaluation has been conducted.

**Fixed per-tool token rate.** The 32.4 tokens/tool rate is measured from a single
server (Playwright). Servers with complex nested JSON Schemas may have higher rates;
servers with minimal parameter definitions may have lower rates.

**Crossover threshold depends on server size.** Users with only large servers (e.g.,
two servers with 50 tools each) benefit earlier than the 4-server crossover suggests.

**Future directions:**
- Measure task success rate with/without gateway on standard agent benchmarks
- Evaluate SkillOpt description quality via blind human evaluation
- Implicit discovery mode: `use_skill` called transparently by the gateway
- rushdb integration for persistent trace indexing and semantic retrieval

---

## 6. Conclusion

skillgraph-mcp demonstrates that a lightweight gateway layer applying progressive
disclosure to MCP tool schemas reduces agent upfront context consumption by 92% in
a 10-server deployment, with linear improvement as server count grows. The semantic
skill graph and SkillOpt loop provide structural guidance and annotation-free
improvement without modifying the MCP protocol or the downstream servers. The
crossover threshold (~4 servers, ~9 tools) provides a clear decision criterion for
practitioners evaluating whether to deploy the gateway.

---

## Acknowledgements

This work builds on skillful-mcp by Kurtis Van Gent (MIT License). The progressive
disclosure architecture is due to Van Gent; this paper contributes the semantic skill
graph, SkillOpt loop, CLI diagnostics, and empirical token measurements.

---

## References

- Anthropic. *Model Context Protocol Specification*. 2024. https://modelcontextprotocol.io
- Van Gent, K. *Skills and MCP*. 2026. https://kvg.dev/posts/20260125-skills-and-mcp/
- Van Gent, K. *skillful-mcp*. 2026. https://github.com/kurtisvg/skillful-mcp
- NousResearch. *Hermes Tool Search*. 2025. https://hermes-agent.nousresearch.com
- Anthropic. *Claude Code skillListingBudgetFraction*. 2026. https://code.claude.com/docs/en/settings.md
