package docs

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/graph"
)

// ClaudeSkillsOptions configures how the generator renders SKILL.md files.
type ClaudeSkillsOptions struct {
	// BinaryPath is the value used in `!`<bin> probe ...`` preflight blocks.
	// Defaults to "skillgraph-mcp" (assumes the binary is on $PATH).
	BinaryPath string
	// ConfigPath is passed to the probe binary so SKILL.md works regardless of
	// the CWD where Claude Code loads it. Absolute paths recommended.
	ConfigPath string
	// Force overwrites existing SKILL.md files. When false (default), the
	// generator skips skills whose target SKILL.md already exists and reports
	// them in the returned skip list. Prevents clobbering hand-curated skills
	// or symlinked content (e.g. a centralized skills hub).
	Force bool
}

// GenerateResult reports what GenerateClaudeSkills did. Skipped lists names the
// generator refused to write because a file already existed and Force was false.
type GenerateResult struct {
	Written []string
	Skipped []string
}

// GenerateClaudeSkills writes one SKILL.md per downstream skill under outDir.
// Each file declares its routing through skillgraph-mcp, runs a preflight
// health probe at load time (via the Claude Code `!` injection syntax), and
// lists the tools the gateway exposes for that skill.
//
// By default the generator refuses to overwrite an existing SKILL.md (or a
// symlink standing in for one) and reports skipped names in the result. Pass
// opts.Force=true to overwrite. This protects hand-curated skills and
// centralized skill hubs symlinked into the output directory.
//
// outDir is created if missing. Skills with names that aren't safe directory
// names are reported in the skip list.
func GenerateClaudeSkills(outDir string, cfgs map[string]config.Server, g *graph.Graph, opts ClaudeSkillsOptions) (GenerateResult, error) {
	var result GenerateResult
	if opts.BinaryPath == "" {
		opts.BinaryPath = "skillgraph-mcp"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return result, fmt.Errorf("create skills dir %s: %w", outDir, err)
	}

	names := make([]string, 0, len(cfgs))
	for name := range cfgs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if !isSafeSkillName(name) {
			slog.Warn("skipping skill with unsafe name", "skill", name)
			result.Skipped = append(result.Skipped, name+" (unsafe name)")
			continue
		}
		dir := filepath.Join(outDir, name)
		path := filepath.Join(dir, "SKILL.md")

		// Guard: refuse to clobber existing content unless Force is set. Lstat
		// (not Stat) so symlinks are detected as "exists" — a symlinked SKILL.md
		// pointing at a centralized hub would otherwise be overwritten via the
		// link target.
		if !opts.Force {
			if _, err := os.Lstat(path); err == nil {
				slog.Info("skipping existing SKILL.md (use Force to overwrite)", "path", path)
				result.Skipped = append(result.Skipped, name+" (exists)")
				continue
			}
			// Also refuse if the parent skill dir is a symlink (entire skill is
			// likely a hub-managed package).
			if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
				slog.Info("skipping skill dir which is a symlink (use Force to overwrite)", "dir", dir)
				result.Skipped = append(result.Skipped, name+" (symlink)")
				continue
			}
		}

		content := renderSkillMD(name, cfgs[name], g, opts)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return result, fmt.Errorf("create skill dir %s: %w", dir, err)
		}
		if err := atomicWrite(path, []byte(content)); err != nil {
			return result, fmt.Errorf("write %s: %w", path, err)
		}
		result.Written = append(result.Written, name)
	}
	slog.Info("generated Claude Code SKILL.md files", "dir", outDir, "written", len(result.Written), "skipped", len(result.Skipped))
	return result, nil
}

func renderSkillMD(name string, srv config.Server, g *graph.Graph, opts ClaudeSkillsOptions) string {
	desc := skillDescription(name, srv, g)
	tools := skillTools(name, g)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", name)
	fmt.Fprintf(&b, "description: %s\n", yamlEscape(firstLine(desc)))
	fmt.Fprintf(&b, "allowed-tools: Bash(%s probe *)\n", opts.BinaryPath)
	b.WriteString("---\n\n")

	fmt.Fprintf(&b, "# %s\n\n", name)
	if desc != "" {
		b.WriteString(desc)
		b.WriteString("\n\n")
	}

	b.WriteString("## Health\n\n")
	if opts.ConfigPath != "" {
		fmt.Fprintf(&b, "!`%s probe --config %s %s`\n\n", opts.BinaryPath, opts.ConfigPath, name)
	} else {
		fmt.Fprintf(&b, "!`%s probe %s`\n\n", opts.BinaryPath, name)
	}

	b.WriteString("## Routing\n\n")
	fmt.Fprintf(&b, "This skill's tools live behind the **skillgraph-mcp** gateway. To use them:\n\n")
	fmt.Fprintf(&b, "1. Call `use_skill(\"%s\")` via the `skillgraph-mcp` MCP server to inspect tool signatures.\n", name)
	b.WriteString("2. Invoke tools inside `execute_code(...)` — they are exposed as Python functions.\n\n")
	b.WriteString("Do **not** call `readMcpResource(server: \"" + name + "\")` directly; the gateway does the routing.\n\n")

	if len(tools) > 0 {
		b.WriteString("## Available tools\n\n")
		for _, t := range tools {
			fmt.Fprintf(&b, "- `%s`\n", t)
		}
		b.WriteString("\n")
	}

	b.WriteString("_Generated by `skillgraph-mcp generate-skills`. Do not edit by hand — changes will be overwritten._\n")
	return b.String()
}

// skillDescription resolves the best available description for a skill, in
// priority order: skillGraph node description (set by bootstrap/SkillOpt),
// then mcp.json `description` field, then a generic fallback.
func skillDescription(name string, srv config.Server, g *graph.Graph) string {
	if g != nil {
		if node, ok := g.Nodes[name]; ok && node.Description != "" {
			return node.Description
		}
	}
	if opts := srv.Options(); opts.Description != "" {
		return opts.Description
	}
	return fmt.Sprintf("Downstream MCP skill `%s` exposed via the skillgraph-mcp gateway.", name)
}

func skillTools(name string, g *graph.Graph) []string {
	if g == nil {
		return nil
	}
	var tools []string
	seen := map[string]bool{}
	for _, e := range g.Edges {
		if e.Source == name && e.Type == graph.RelHasTool && !seen[e.Target] {
			tools = append(tools, e.Target)
			seen[e.Target] = true
		}
	}
	sort.Strings(tools)
	return tools
}

// isSafeSkillName rejects names that would escape outDir or produce invalid
// directory paths. Skill names come from mcp.json keys, which are user input.
func isSafeSkillName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return false
	}
	return true
}

// yamlEscape produces a value safe for a single-line YAML scalar. The
// description field is the only user-derived string in the frontmatter; quote
// it if it contains characters that would break parsing.
func yamlEscape(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#&*!|>'\"%@`") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "?") {
		return "\"" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\"") + "\""
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// atomicWrite writes to a temp file and renames into place — prevents partial
// SKILL.md files when the generator is killed mid-write.
func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
