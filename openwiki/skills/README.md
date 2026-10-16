---
type: Skills Catalog
title: Skills Ecosystem
description: >
  Complete catalog of 14+ built-in skills for skillgraph-mcp: code intelligence
  (gitnexus), search (brave-search, firecrawl, context7), browser automation
  (chrome-devtools, playwright), diagramming (architecture, uml, mindmap),
  development (golang-patterns, golang-testing), and infrastructure tools.
resource: /.agents/skills/
tags: [skills, catalog, gitnexus, brave-search, firecrawl, architecture, uml]
---

# Skills Ecosystem

skillgraph-mcp ships with 14+ built-in skills that extend the gateway's
capabilities. Skills are registered via `mcp.json` and discovered through the
`list_skills` and `use_skill` gateway tools.

## Skills by Category

### Code Intelligence

| Skill | Description | Sub-skills |
|---|---|---|
| **gitnexus** | Code intelligence via tree-sitter | 6 specialized workflows |

**GitNexus sub-skills:**

- `gitnexus-cli`: CLI commands (analyze, index, clean, wiki generation)
- `gitnexus-debugging`: Bug tracing and error investigation
- `gitnexus-exploring`: Codebase navigation and architecture understanding
- `gitnexus-guide`: GitNexus tool reference and workflow guide
- `gitnexus-impact-analysis`: Safety analysis before code changes
- `gitnexus-refactoring`: Safe rename, extract, split, move operations

### Search & Extraction

| Skill | Description |
|---|---|
| **brave-search** | Web search via Brave API |
| **firecrawl** | Web scraping and content extraction |
| **context7** | Library/framework documentation resolver |

### Browser Automation

| Skill | Description |
|---|---|
| **chrome-devtools** | Chrome DevTools protocol (DOM, network, profiling) |
| **playwright** | Browser automation (headless WSL2) |

### Diagramming

| Skill | Description | Features |
|---|---|---|
| **architecture** | Layered system architecture diagrams (HTML/CSS) | 12 styles × 12 layouts |
| **uml** | UML diagrams (PlantUML) | 14 diagram types, 9500+ stencils |
| **mindmap** | Hierarchical mind maps (PlantUML) | 7 example patterns |

### Development

| Skill | Description |
|---|---|
| **golang-patterns** | Idiomatic Go patterns and best practices |
| **golang-testing** | Go testing (TDD, benchmarks, fuzzing) |
| **sequential-thinking** | Structured sequential reasoning |

### Infrastructure & Utilities

| Skill | Description |
|---|---|
| **magic** | AI-powered UI component generation (21st.dev) |
| **memory** | Cross-session persistent memory for agents |
| **wsl-terminal** | WSL2 terminal access |

## Skill Registration

Skills are registered in `mcp.json` under `mcpServers`. The gateway discovers
them via `list_skills` and loads their schemas on demand via `use_skill`.

## Skills Lock File

`skills-lock.json` tracks 5 primary skills with sources:

- **architecture**, **mindmap**, **uml** → from `markdown-viewer/skills` (GitHub)
- **golang-patterns**, **golang-testing** → from `affaan-m/everything-claude-code`
  (autoskills-registry)

## Dual Architecture

Skills use either:

1. **Direct HTML/CSS** (architecture skill) — renders diagrams as styled HTML
2. **PlantUML syntax** (uml, mindmap) — generates diagrams via PlantUML

Most skills route through `skillgraph-mcp` gateway with the `use_skill()`
pattern.
