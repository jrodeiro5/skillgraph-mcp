# HANDOFF.md — skillgraph-mcp Project State

> **For AI agents picking up this project.** Read before touching any code.
> Last updated: 2026-06-04

---

## What This Project Is

**skillgraph-mcp** is a Go MCP gateway that solves tool bloat. Instead of exposing 100+ tools directly to an agent, it proxies N downstream MCP servers behind 9 semantic meta-tools. Agents discover tools progressively: `list_skills → use_skill → execute_code`.

The core insight: **downstream tools are never directly callable MCP tools.** They only exist as Python functions inside the `execute_code` sandbox. This forces the agent through a single choke point — observable, gatable, instrumentable.

---

## Installation State

```
~/.claude.json               ← registers skillgraph as user-level MCP (all projects)
  command: ~/.local/bin/skillgraph-mcp-wrapper.sh
  args: ["--config", "~/.config/skillgraph-mcp/mcp.json"]

~/.local/bin/skillgraph-mcp-wrapper.sh
  ← sources ~/.bash_env + `pass show api/...` for all API keys
  ← exec ~/.local/bin/skillgraph-mcp

~/.config/skillgraph-mcp/mcp.json
  ← 10 downstream servers: brave-search, chrome-devtools, context7,
    firecrawl, gitnexus, magic, memory, playwright, sequential-thinking, wsl-terminal

~/.cache/skillgraph-mcp/lattice/
  ← traces/ (empty — no execute_code usage yet)
  ← history/ (SkillOpt rollback snapshots)

~/.claude/skills/
  ← SKILL.md files generated for all 10 downstream servers
  ← each instructs agents: use_skill("X") → execute_code(), never call directly
```

---

## Architecture

### Package dependency order (leaf → root)

```
graph   config
  ↑        ↑
  mcpserver
  ↑        ↑
tools    refine  docs  embed
  ↑        ↑
     app
      ↑
     cmd → main
```

### The 9 gateway meta-tools

| Tool | Purpose |
|------|---------|
| `list_skills` | Enumerate servers + `skill://` resources from servers that ship them |
| `use_skill(skill_name)` | Python function signatures. Shows `find_tools` hint if >10 tools |
| `execute_code(code, skill_names?)` | Python sandbox. `skill_names` limits stubs to specific skills |
| `find_tools(query, k, server_filter?)` | Semantic search. Falls back to keyword scoring without embedding provider |
| `read_resource(skill_name, uri)` | Proxy MCP resource reads |
| `register_server(config)` | Hot-register new downstream server at runtime |
| `get_skill_graph()` | Current capability relationship graph |
| `plan_workflow(objective)` | Suggest tool execution sequence via graph traversal |
| `read_lattice(path)` | Read semantic lattice documentation |

---

## Recent Changes (2026-06-04 session)

### What was shipped to `main`

```
c60ee26  test: embed unit tests — keyword fallback, tokenize, score, server filter (7 tests)
0235bce  feat: 20+ server scale — keyword fallback + skill isolation + skill:// resources
5fbb31e  feat: auto-rebuild embed index on RebuildGraph callback
c0a0346  feat: embedding-based tool retrieval + fix skill/MCP confusion
```

### Key code changes

**`internal/embed/embed.go`**
- `Rebuild()` skips vector gen when embedder nil (stores `Vector: nil`)
- `Find()` detects nil embedder/nil vectors → `keywordScore()` fallback
- Added `keywordScore(query, entry)` + `tokenize(s)` — no deps

**`internal/tools/execute_code.go`**
- `executeCodeInput` gains `SkillNames []string` (optional) — filters tool stubs
- `montyValueToText()` — JSON-marshals dict/list/tuple (was returning `"dict"` string)
- Description rewritten with `GOTCHA` warning against direct tool calls

**`internal/tools/find_tools.go`**
- Works with nil index (keyword path)
- `BuildIndexEntries()` embeds full signature + description

**`internal/tools/list_skills.go`**
- Enumerates `skill://` URI resources from each server (Sam Morrow spec)

**`internal/tools/use_skill.go`**
- Appends `find_tools` hint when skill has >10 tools

**`internal/mcpserver/manager.go`**
- `SetOnRebuild(fn func())` — callback after each `RebuildGraph()`

---

## The SkillOpt Story

### Two separate systems — don't confuse them

**1. skillgraph's internal SkillOpt** (`internal/refine/engine.go`)
- Optimizes **tool descriptions** in `mcp.json#skillGraph.descriptions`
- Background loop, polls traces every 30s, only fires on error batches
- Hold-out gate at `passesHoldoutGate()` (oldest third of traces = validation set)
- **Currently idle**: no traces exist yet — agents haven't used `execute_code` through live gateway

**2. Microsoft SkillOpt** (Python, arXiv 2605.23904, `pip install skillopt`)
- Optimizes **SKILL.md documents** via trajectory rollouts + validation gate
- +19.1pp accuracy inside Claude Code (from paper)
- Supports: Azure OpenAI, OpenAI-compatible, Anthropic, Qwen, MiniMax

### Bedrock backend built this session

**Location:** `~/SkillOpt` (fork of `microsoft/SkillOpt`, branch `feat/bedrock-backend`)

**What it adds:**
- `skillopt/model/bedrock_backend.py` — full boto3 Converse API backend
- Wired into `common.py`, `backend_config.py`, `__init__.py`, `router.py`
- `pyproject.toml`: `bedrock` optional extra → `pip install skillopt[bedrock]`

**Auth:** standard boto3 credential chain. Key env vars:
```bash
AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_BEDROCK_REGION
# or any boto3 provider (instance profiles, ~/.aws/credentials)
```

**Default model:** `amazon.nova-pro-v1:0`

**PR NOT submitted yet.** User must run:
```bash
cd ~/SkillOpt && gh pr create --repo microsoft/SkillOpt
```

---

## Unification Plan

The vision: one project, one `pip install skillgraph-mcp`. The Go gateway and Python SkillOpt training communicate via filesystem — no shared code needed.

### Data contracts

```
Traces (Go writes, Python reads):
  ~/.cache/skillgraph-mcp/lattice/traces/*.json
  Schema: {timestamp, code, toolCalls:[{toolName, args, result, isError}], output, error}

Skill documents (Python writes, Claude Code reads):
  ~/.claude/skills/<server>/SKILL.md
  Generated by: skillgraph-mcp generate-skills (seed)
  Improved by:  SkillOpt train (optimized)

Tool descriptions (Go internal SkillOpt):
  ~/.config/skillgraph-mcp/mcp.json#skillGraph.descriptions
```

### What needs to be built (ordered)

#### 1. `SkillGraphEnv` adapter (~60 lines Python)

**File:** `~/SkillOpt/skillopt/envs/skillgraph_env.py`

```python
class SkillGraphEnv(BaseEnv):
    """Reads skillgraph-mcp traces as SkillOpt rollouts."""

    def rollout(self, skill_content: str, out_dir: str) -> list[RolloutResult]:
        traces = self._load_traces()  # reads *.json from traces_dir config key
        results = []
        for t in traces:
            has_error = bool(t.get("error")) or any(
                c["isError"] for c in t.get("toolCalls", [])
            )
            hard = 0 if has_error else 1
            results.append(RolloutResult(
                id=t["timestamp"],
                hard=hard,
                soft=float(hard),
                fail_reason=t.get("error", ""),
            ))
        return results

    def _skill_init_content(self) -> str:
        # seed from existing SKILL.md generated by skillgraph-mcp generate-skills
        return Path(self.cfg.skill_init_path).read_text()
```

Config YAML for training:
```yaml
env: skillgraph
traces_dir: ~/.cache/skillgraph-mcp/lattice/traces
skill_init_path: ~/.claude/skills/gitnexus/SKILL.md
output_dir: outputs/gitnexus_nova
```

#### 2. `skillgraph-mcp train` subcommand (~100 lines Go)

**File:** `cmd/train.go`

```
skillgraph-mcp train \
  --skill gitnexus \
  --backend bedrock_chat \
  --model amazon.nova-pro-v1:0 \
  --epochs 4
```

Wires: traces_dir → skill_init → invokes `python -m skillopt.train` → copies `best_skill.md` → `~/.claude/skills/<skill>/SKILL.md`

#### 3. Python wheel with embedded Go binary

**Distribution model:** `ruff`/`esbuild` pattern — Go binary inside Python wheel.

```bash
pip install skillgraph-mcp   # single command, gets everything
```

Structure:
```
skillgraph_mcp/
├── bin/linux-amd64/skillgraph-mcp     ← pre-compiled Go binary
├── bin/darwin-arm64/skillgraph-mcp
├── bin/win-amd64/skillgraph-mcp.exe
├── training/skillgraph_env.py
├── training/bedrock_backend.py
└── cli.py                             ← platform selector, delegates to binary
```

GitHub Actions: matrix build (linux/amd64, darwin/arm64, win/amd64) → `pip wheel` → PyPI.

#### 4. Submit Bedrock PR

```bash
cd ~/SkillOpt && gh pr create --repo microsoft/SkillOpt \
  --title "feat: add Amazon Bedrock backend (Converse API, Nova models)"
```

#### 5. C-vs-E benchmark

30 tasks scaffolded at `benchmarks/2026-06_embed_vs_graph/tasks.jsonl`.
Config at `benchmarks/2026-06_embed_vs_graph/config.md`.
Harness not built. Primary metric: Recall@1 (find_tools vs graph traversal).

---

## Running the Project

```bash
# Build + test
go build ./...
go test ./...

# Install binary
make install   # → ~/.local/bin/skillgraph-mcp

# Start with API keys
set -a && source ~/.bash_env
BRAVE_SEARCH_API_KEY=$(pass show api/brave-search) \
CONTEXT7_API_KEY=$(pass show api/context7) \
FIRECRAWL_API_KEY=$(pass show api/firecrawl) \
MAGIC_API_KEY=$(pass show api/magic) \
skillgraph-mcp serve --config ~/.config/skillgraph-mcp/mcp.json

# Validate all servers connect
skillgraph-mcp validate --config ~/.config/skillgraph-mcp/mcp.json

# Regenerate SKILL.md files
skillgraph-mcp generate-skills \
  --config ~/.config/skillgraph-mcp/mcp.json \
  --out ~/.claude/skills

# Diagnose
skillgraph-mcp doctor
```

---

## Key Files

| File | Role |
|------|------|
| `internal/embed/embed.go` | Embedding index + keyword fallback |
| `internal/embed/embed_test.go` | 7 unit tests for keyword path |
| `internal/tools/execute_code.go` | Python sandbox, skill_names filter, montyValueToText |
| `internal/tools/find_tools.go` | Semantic/keyword search tool |
| `internal/tools/list_skills.go` | Server enumeration + skill:// resources |
| `internal/tools/use_skill.go` | Tool signature inspection + hint |
| `internal/refine/engine.go` | Bootstrap + SkillOpt loop (optimizes mcp.json descriptions) |
| `internal/docs/claude_skills.go` | SKILL.md template generator |
| `internal/trace/trace.go` | `Trajectory` struct — JSON schema for traces |
| `internal/mcpserver/manager.go` | Server registry, RebuildGraph, SetOnRebuild |
| `cmd/serve.go` | Main serve loop, embed index boot, onRebuild wiring |
| `cmd/generate_skills.go` | generate-skills subcommand |
| `cmd/util.go` | `defaultLatticeDir()` → `~/.cache/skillgraph-mcp/lattice` |

---

## Known Footguns

- **Blank child env**: STDIO children only get env vars in their `env` block in mcp.json. `${VAR}` interpolation handles this — don't add `inherit: true`.
- **SkillOpt idle**: no traces yet → optimization loop never fires. Needs real `execute_code` usage first.
- **`skill://` resources**: list_skills enumerates them but no downstream server ships them yet. Forward-compatible.
- **gitnexus index goes stale**: run `gitnexus analyze` after Go changes. Hook reminds you.
- **npx cache corruption**: npm 11.x bug. Use absolute binary paths in mcp.json, not `npx`.
- **`~/SkillOpt` local clone**: can be deleted after PR submitted. Not part of this project.

---

## Research Context

| Paper / Source | What It Says | Relevance |
|----------------|-------------|-----------|
| arXiv 2605.23904 (MS SkillOpt) | Trains SKILL.md docs via rollout+gate. +19.1pp on Claude Code. | `SkillGraphEnv` adapter connects our traces to this training loop |
| arXiv 2505.03275 (RAG-MCP) | Embedding-based retrieval cuts token use ~50% vs flat tool list | What `find_tools` implements |
| Sam Morrow blog (sam-morrow.com) | Progressive discovery via skill:// resources, tool-cli, Code Mode | `skill://` in list_skills; `outputSchema` on tools not yet done |

---

*HANDOFF.md — auto-generated from session 2026-06-04. Run `gitnexus query --repo skillgraph-mcp "architecture"` for current symbol graph.*
