# Problem

## Core Problem: Tool Bloat in Multi-Server MCP Deployments

An AI agent connecting to N MCP servers receives all tool schemas upfront — before
any user message is processed. With 10 servers exposing ~113 tools, this costs
~3,660 tokens per session, degrading model attention and inflating costs linearly
with server count.

## Gap in existing work

- Direct MCP clients (Claude Code, Gemini CLI, Codex) load all configured server
  schemas at connection time — no lazy loading mechanism exists in the protocol.
- Enterprise MCP gateways (Microsoft MCP Gateway, IBM AI Gateway) solve ops
  concerns (auth, RBAC, lifecycle) but do not address agent cognitive overhead.
- No existing system provides semantic relationships between tools across servers
  (prerequisite chains, common next steps) to guide agent tool selection.

## What skillgraph-mcp adds over skillful-mcp (upstream)

| Capability | skillful-mcp | skillgraph-mcp |
|---|---|---|
| MCP gateway (proxy N servers) | ✅ | ✅ |
| Progressive disclosure (8 gateway tools) | ✅ | ✅ |
| Semantic skill graph | ❌ | ✅ |
| plan_workflow tool | ❌ | ✅ |
| SkillOpt (trace-driven self-improvement) | ❌ | ✅ |
| CLI subcommands (doctor, validate, list-skills) | ❌ | ✅ |
| Parallel downstream connection + graceful degradation | ❌ | ✅ |
