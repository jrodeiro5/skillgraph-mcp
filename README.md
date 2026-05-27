# skillgraph-mcp

[![Go](https://img.shields.io/github/go-mod/go-version/jrodeiro5/skillgraph-mcp)](https://go.dev/)
[![CI](https://github.com/jrodeiro5/skillgraph-mcp/actions/workflows/test.yml/badge.svg)](https://github.com/jrodeiro5/skillgraph-mcp/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jrodeiro5/skillgraph-mcp)](https://goreportcard.com/report/github.com/jrodeiro5/skillgraph-mcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Too many MCP tools slowing your agent down? Might be a Skill Issue 😉

**skillgraph-mcp** eliminates tool bloat by building a semantic capability relationship graph and turning your MCP servers into Agent Skills in an MCP-native way.

- 🔍 **Progressive Disclosure** — agent sees 7 lightweight tools; downstream schemas loaded on demand
- ⚡ **Code Mode** — trigger and combine multiple tool calls with Python
- 🔒 **Secure sandbox** — code executes in a sandbox, not your shell
- 🔌 **Any MCP client** — works with Gemini CLI, Claude Code, Codex, and more

## Table of contents

- [Why?](#-why)
- [How it works](#-how-it-works)
- [Getting started](#-getting-started)
- [Configuration](#-configuration)
- [Embedding in Go](#-embedding-in-go)
- [Gotchas](#️-gotchas)
- [Troubleshooting](#-troubleshooting)

## ❓ Why?

Connecting an agent to too many tools (or MCP servers) creates
[tool bloat][tool-bloat]. An agent with access to 5 servers might have 80+ tools
loaded into its context window before the user says a word. Accuracy drops,
latency increases, and adding capabilities makes the agent worse.

skillgraph-mcp fixes this through **progressive disclosure**. The agent sees 7
lightweight gateway tools and discovers specific downstream schemas on-demand,
collapsing thousands of tokens down to a lightweight index.

> **Not an infrastructure gateway.** Projects like [Microsoft MCP Gateway](https://github.com/microsoft/mcp-gateway) and [IBM AI Gateway MCP](https://www.ibm.com/docs/en/api-connect) solve the ops problem — Kubernetes deployment, RBAC, adapter lifecycle. skillgraph-mcp solves the agent cognition problem: which tools exist, how they relate, and how to get better at routing them over time. They are complementary, not competing.

[tool-bloat]: https://kvg.dev/posts/20260125-skills-and-mcp/

## 💡 How it works

```
Agent  <--MCP-->  skillgraph-mcp  <--MCP-->  Database Server
                                <--MCP-->  Filesystem Server
                                <--MCP-->  API Server
```

skillgraph-mcp reads a standard `mcp.json` config, connects to each downstream
server, and exposes seven tools:

| Tool              | Description                                                                      |
|-------------------|----------------------------------------------------------------------------------|
| `list_skills`     | Returns the names of all configured downstream servers                           |
| `use_skill`       | Lists the tools and resources available in a specific skill                      |
| `read_resource`   | Reads a resource from a specific skill                                           |
| `execute_code`    | Runs Python code in a secure [gomonty](https://github.com/ewhauser/gomonty) sandbox |
| `get_skill_graph` | Returns the capability relationship graph                                        |
| `plan_workflow`   | Receives a high-level goal and returns a recommended path of execution           |
| `read_lattice`    | Reads files from the generated `.mcp_lattice` semantic documentation index       |

The typical agent workflow:

1. Call `list_skills` to see what's available.
2. Call `get_skill_graph` or `plan_workflow` to inspect relations and plan the steps.
3. Call `use_skill` to inspect a skill's tools and their input schemas.
4. Use `execute_code` to orchestrate tool calls in a single round-trip.

### 🔄 Self-Evolving Optimization (SkillOpt)

`skillgraph-mcp` implements a self-evolving skill optimization mechanism inspired by Microsoft's **SkillOpt** framework (arXiv:2605.23904):
* **Trajectory Logging:** When the agent runs Python code via `execute_code`, the gateway records execution trajectories (rollouts) including arguments, results, and runtime errors under `.mcp_lattice/traces/` (relative to the CWD where skillgraph-mcp is launched).
* **Background Refinement:** A background daemon periodically reads these traces and — only when errors are present — prompts an LLM to propose text-space optimization edits (add/replace/delete tool descriptions or graph relations) in `mcp.json`.
* **Zero Latency:** Updates are verified and applied atomically, ensuring that future routing and discovery prompts are optimized dynamically without model retraining.

To enable the SkillOpt and bootstrap refinement loops, set one of these environment variables:

| Variable | Provider | Notes |
|---|---|---|
| `LLM_BASE_URL` | Any OpenAI-compatible API | LiteLLM proxy, Ollama, etc. Set `LLM_API_KEY` and `LLM_MODEL` alongside |
| `OPENAI_API_KEY` | OpenAI | Uses `gpt-4o` by default; override with `LLM_MODEL` |
| `DEEPSEEK_API_KEY` | DeepSeek | Uses `deepseek-chat` |
| `GEMINI_API_KEY` | Google Gemini | Uses `gemini-1.5-pro` |

`LLM_BASE_URL` takes priority. When set, `LLM_API_KEY` is optional (Ollama and other local servers don't require auth).

**Examples:**
```sh
# Ollama (local, no auth)
LLM_BASE_URL=http://localhost:11434/v1 LLM_MODEL=llama3.1 skillgraph-mcp --config mcp.json

# LiteLLM proxy (routes to any backend)
LLM_BASE_URL=http://localhost:4000 LLM_API_KEY=sk-... LLM_MODEL=anthropic/claude-3-5-haiku skillgraph-mcp --config mcp.json

# OpenAI directly
OPENAI_API_KEY=sk-... skillgraph-mcp --config mcp.json
```


### Example Code Mode Usage

After discovering tools via `use_skill`, the agent can call them directly by
name inside `execute_code` — chaining outputs from one tool into another:

```python
# Query users, then send each one a welcome email
users = query(sql="SELECT name, email FROM users WHERE welcomed = false")
for user in users:
    send_email(to=user["email"], subject="Welcome!", body="Hi " + user["name"])
"Sent " + str(len(users)) + " welcome emails"
```

All downstream tools are available as functions with positional and keyword
arguments. If two skills define a tool with the same name, the function is
prefixed with the skill name (e.g. `database_search`, `docs_search`). Tool
names returned by `use_skill` always match the function names in `execute_code`.

## 🚀 Getting started

### Install

<details open>
<summary><strong>Download a binary</strong></summary>

```sh
VERSION="0.1.0"
OS="linux"       # or: darwin, windows
ARCH="amd64"     # or: arm64

curl -L "https://github.com/jrodeiro5/skillgraph-mcp/releases/download/v${VERSION}/skillgraph-mcp_${VERSION}_${OS}_${ARCH}" -o skillgraph-mcp
chmod +x skillgraph-mcp
```

Or download from the [releases page](https://github.com/jrodeiro5/skillgraph-mcp/releases/latest).

</details>

<details>
<summary><strong>Docker</strong></summary>

```sh
docker run --rm \
  -v /path/to/mcp.json:/mcp.json \
  ghcr.io/jrodeiro5/skillgraph-mcp:latest \
  --config /mcp.json --transport http --port 8080
```
</details>

<details>
<summary><strong>Go install</strong> (requires Go 1.26+)</summary>

```sh
go install github.com/jrodeiro5/skillgraph-mcp@latest
```
</details>

<details>
<summary><strong>Build from source</strong></summary>

```sh
git clone https://github.com/jrodeiro5/skillgraph-mcp.git
cd skillgraph-mcp
go build -o skillgraph-mcp .
```
</details>

### Create a config

Create an `mcp.json` file with your downstream servers:

```json
{
  "mcpServers": {
    "postgres": {
      "command": "npx",
      "args": ["-y", "@toolbox-sdk/server", "--prebuilt=postgres"],
      "description": "Postgres database tools — query, inspect schemas, and manage tables. Use when the user needs to read or write data, explore table structures, or run SQL.",
      "env": {
        "POSTGRES_HOST": "${POSTGRES_HOST}",
        "POSTGRES_USER": "${POSTGRES_USER}",
        "POSTGRES_PASSWORD": "${POSTGRES_PASSWORD}",
        "POSTGRES_DATABASE": "${POSTGRES_DATABASE}"
      }
    },
    "github-issues": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/issues",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub issue management — create, search, update, and comment on issues. Use when the user mentions bugs, feature requests, or issue triage."
    }
  }
}
```

### Run

```sh
skillgraph-mcp --config mcp.json
```

Or over HTTP:

```sh
skillgraph-mcp --config mcp.json --transport http --port 8080
```

### Connect to your agent

<details>
<summary><strong>Gemini CLI</strong> (<code>~/.gemini/settings.json</code>)</summary>

```json
{
  "mcpServers": {
    "skillgraph": {
      "command": "/path/to/skillgraph-mcp",
      "args": ["--config", "/path/to/mcp.json"]
    }
  }
}
```
</details>

<details>
<summary><strong>Claude Code</strong> (<code>.claude/settings.json</code>)</summary>

```json
{
  "mcpServers": {
    "skillgraph": {
      "command": "/path/to/skillgraph-mcp",
      "args": ["--config", "/path/to/mcp.json"]
    }
  }
}
```
</details>

<details>
<summary><strong>Codex CLI</strong> (<code>~/.codex/config.toml</code>)</summary>

```toml
[mcp_servers.skillgraph]
command = "/path/to/skillgraph-mcp"
args = ["--config", "/path/to/mcp.json"]
```
</details>

Any MCP-compatible client works — just point it at the `skillgraph-mcp` binary.

### Advanced example: GitHub MCP Server

The [GitHub MCP server](https://github.com/github/github-mcp-server) exposes
19+ toolsets — a perfect candidate for skill decomposition. Instead of one
massive server, split it into focused skills by feature group. The agent sees
4 skills instead of 40+ tools, and calls `use_skill` only when it needs a
specific capability.

```json
{
  "mcpServers": {
    "github-issues": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/issues",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub issue management — create, search, update, and comment on issues. Use when the user mentions bugs, feature requests, or issue triage."
    },
    "github-labels": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/labels",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub label management — create, assign, and remove labels. Use when organizing or categorizing issues and pull requests."
    },
    "github-prs": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/pull_requests",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub pull request workflows — review, merge, and manage PRs. Use when the user asks about code review, PR status, or merging changes."
    },
    "github-actions": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/actions",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub Actions CI/CD — trigger, monitor, and debug workflows. Use when the user asks about build status, failed checks, or re-running pipelines."
    }
  }
}
```

## 📝 Configuration

Each entry in `mcpServers` is a downstream server that becomes a skill. The key
is the skill name. The value depends on the transport type.

All string values support `${VAR}` environment variable expansion. Missing
variables cause a startup error.

### Common options

All server types support these optional fields:

| Field              | Description                                               |
|--------------------|-----------------------------------------------------------|
| `description`      | Override the server's instructions shown by `list_skills`  |
| `allowedTools`     | Only expose these tool names (default: all)                |
| `allowedResources` | Only expose these resource URIs (default: all)             |

Excluded tools are invisible everywhere — they won't appear in `use_skill`,
can't be called via `execute_code`, and won't cause name-conflict prefixing.

### STDIO server

Spawns the server as a child process. Only env vars explicitly listed in `env`
are passed to the child — the parent environment is not inherited.

| Field     | Required | Description                             |
|-----------|----------|-----------------------------------------|
| `command` | yes      | Executable to run                       |
| `args`    | no       | Arguments array                         |
| `env`     | no       | Environment variables for the child process |

```json
{
  "mcpServers": {
    "postgres": {
      "command": "npx",
      "args": ["-y", "@toolbox-sdk/server", "--prebuilt=postgres"],
      "description": "Postgres database tools — query, inspect schemas, and manage tables. Use when the user needs to read or write data, explore table structures, or run SQL.",
      "env": {
        "POSTGRES_HOST": "${POSTGRES_HOST}",
        "POSTGRES_USER": "${POSTGRES_USER}",
        "POSTGRES_PASSWORD": "${POSTGRES_PASSWORD}",
        "POSTGRES_DATABASE": "${POSTGRES_DATABASE}"
      }
    }
  }
}
```

### HTTP server

Connects via Streamable HTTP.

| Field     | Required | Description                     |
|-----------|----------|---------------------------------|
| `type`    | yes      | Must be `"http"`                |
| `url`     | yes      | Server endpoint URL             |
| `headers` | no       | HTTP headers (e.g. auth tokens) |

```json
{
  "mcpServers": {
    "remote-api": {
      "type": "http",
      "url": "https://api.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${API_KEY}"
      }
    }
  }
}
```

### SSE server

Connects via Server-Sent Events.

| Field     | Required | Description      |
|-----------|----------|------------------|
| `type`    | yes      | Must be `"sse"`  |
| `url`     | yes      | SSE endpoint URL |
| `headers` | no       | HTTP headers     |

### Skill graph (manual overrides)

The optional `skillGraph` section in `mcp.json` lets you manually define descriptions and relationships that the agent uses for routing and planning. The LLM refinement loops also write here — you can seed it by hand or let it auto-populate.

```json
{
  "mcpServers": { "...": {} },
  "skillGraph": {
    "descriptions": {
      "github-issues": "GitHub issue management — create, search, update, and triage issues.",
      "github_create_issue": "Opens a new issue. Requires a repository name and title."
    },
    "relations": [
      {
        "source": "github_search_issues",
        "target": "github_create_issue",
        "type": "COMMON_NEXT_STEP",
        "description": "After searching, you may want to open a related issue"
      },
      {
        "source": "github_create_issue",
        "target": "github_add_label",
        "type": "PREREQUISITE_FOR",
        "description": "Label the issue after creating it"
      }
    ]
  }
}
```

**Descriptions** override the tool/server text shown by `list_skills`, `use_skill`, and `plan_workflow`. Both server names and individual tool names are valid keys.

**Relations** define typed edges in the capability graph. Valid `type` values:

| Type | Meaning |
|---|---|
| `PREREQUISITE_FOR` | source must run before target |
| `PRODUCES` | source output is consumed by target |
| `REQUIRES` | source needs target's output as input |
| `COMMON_NEXT_STEP` | target is commonly called after source |

#### Automatic relation inference

skillgraph-mcp also infers `PREREQUISITE_FOR` edges automatically at startup. If a tool has a parameter ending in `_id` or `_number`, it looks for other tools whose names contain `create_<prefix>`, `get_<prefix>`, or `search_<prefix>` and adds a prerequisite edge from them. For example, `comment_on_issue(issue_number: int)` will automatically get a `PREREQUISITE_FOR` edge from `create_issue` and `search_issues` — no config needed.

Manual `relations` entries are merged on top of inferred ones.

### Flags

| Flag              | Default          | Description                           |
|-------------------|------------------|---------------------------------------|
| `--config`        | `./mcp.json`     | Path to the config file               |
| `--lattice-dir`   | `./.mcp_lattice` | Directory for traces, READMEs, and lattice docs |
| `--transport`     | `stdio`          | Upstream transport: `stdio` or `http` |
| `--host`          | `localhost`      | HTTP listen host                      |
| `--port`          | `8080`           | HTTP listen port                      |
| `--version`       |                  | Print version and exit                |

## 🔌 Embedding in Go

skillgraph-mcp can be used as a library inside a Go program without running the CLI binary. Import the internal packages directly:

```go
import (
    "github.com/jrodeiro5/skillgraph-mcp/internal/config"
    "github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Load config and connect to downstream servers
servers, graphCfg, err := config.Load("mcp.json")
mgr, err := mcpserver.NewManager(ctx, servers, graphCfg)
defer mgr.Close()

// Inspect the graph
graph := mgr.GetGraph()
tools := mgr.AllTools()               // all resolved tools across all skills
dbTools := mgr.ServerTools("postgres") // tools from one skill

// Proxy a tool call directly
srv, _ := mgr.GetServer("postgres")
result, _ := srv.CallTool(ctx, &mcp.CallToolParams{
    Name:      "query",
    Arguments: map[string]any{"sql": "SELECT 1"},
})

// Rebuild graph after a config change
mgr.RebuildGraph(updatedGraphCfg)
```

If you already have an `*mcp.ClientSession` (e.g. from your own transport), wrap it without going through the config file:

```go
srv, err := mcpserver.NewServerFromSession(ctx, session, config.ServerOptions{
    Description:  "My custom skill",
    AllowedTools: []string{"search", "create"},
})
mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"my-skill": srv})
```

## ⚠️ Gotchas

**STDIO child processes get a blank environment.** When `command` servers are launched, they inherit only the variables explicitly listed in the `env` block — not the parent shell's environment. If a child process needs `PATH`, `HOME`, or any other system variable, you must pass it explicitly:

```json
"env": {
  "PATH": "${PATH}",
  "HOME": "${HOME}",
  "MY_API_KEY": "${MY_API_KEY}"
}
```

**`.mcp_lattice/` is relative to CWD by default.** Traces, READMEs, and lattice files are written to `.mcp_lattice/` in the working directory where skillgraph-mcp is launched — not next to the config file. Use `--lattice-dir /absolute/path` to pin it to a fixed location.

**`mcp.json` is mutated at runtime.** The `skillGraph` section (descriptions and relations) is auto-populated and updated by the background refinement loops. Keep a copy if you want to preserve a known-good baseline, or use git to track changes.

## 🔧 Troubleshooting

**SkillOpt loop never runs** — Check that an LLM env var is set (`LLM_BASE_URL`, `OPENAI_API_KEY`, `DEEPSEEK_API_KEY`, or `GEMINI_API_KEY`). Without one, both background loops are skipped silently on startup.

**Child server won't start** — Run skillgraph-mcp with `--transport http` and check logs. The most common cause is a missing env var (remember: child processes get a blank env, not the parent shell's env).

**Tool name conflicts** — If two skills define a tool with the same name, skillgraph-mcp prefixes both with their skill name (e.g. `github_search`, `docs_search`). The prefixed names are what `use_skill` returns and what `execute_code` expects.

**Graph not updating after config edit** — The graph is loaded once at startup. Restart skillgraph-mcp to pick up manual edits to `mcp.json`'s `skillGraph` section.
