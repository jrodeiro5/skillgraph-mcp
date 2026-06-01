package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/graph"
)

func TestGenerateClaudeSkills(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfgs := map[string]config.Server{
		"gitnexus": &config.StdioServer{Type: "stdio", Command: "gitnexus", ServerOptions: config.ServerOptions{Description: "Code knowledge graph"}},
		"brave":    &config.StdioServer{Type: "stdio", Command: "npx"},
	}

	g := graph.New()
	g.AddNode("gitnexus", graph.NodeSkill, "gitnexus", "Code knowledge graph for impact analysis")
	g.AddNode("query", graph.NodeTool, "query", "")
	g.AddNode("context", graph.NodeTool, "context", "")
	g.AddEdge("gitnexus", "query", graph.RelHasTool, "")
	g.AddEdge("gitnexus", "context", graph.RelHasTool, "")

	if err := GenerateClaudeSkills(dir, cfgs, g, ClaudeSkillsOptions{ConfigPath: "/tmp/mcp.json"}); err != nil {
		t.Fatal(err)
	}

	// gitnexus: full content with tools
	data, err := os.ReadFile(filepath.Join(dir, "gitnexus", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"name: gitnexus",
		"allowed-tools: Bash(skillgraph-mcp probe *)",
		"!`skillgraph-mcp probe --config /tmp/mcp.json gitnexus`",
		"use_skill(\"gitnexus\")",
		"- `context`",
		"- `query`",
		"Do **not** call `readMcpResource(server: \"gitnexus\")` directly",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("gitnexus SKILL.md missing %q\nfull content:\n%s", want, got)
		}
	}

	// brave: no graph description — generator falls back to mcp.json desc, then generic.
	data, err = os.ReadFile(filepath.Join(dir, "brave", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: brave") {
		t.Errorf("brave SKILL.md missing name")
	}
}

func TestGenerateClaudeSkillsCustomBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgs := map[string]config.Server{
		"x": &config.StdioServer{Type: "stdio", Command: "x"},
	}
	opts := ClaudeSkillsOptions{BinaryPath: "/opt/bin/skillgraph-mcp", ConfigPath: "/etc/mcp.json"}
	if err := GenerateClaudeSkills(dir, cfgs, nil, opts); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "x", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "allowed-tools: Bash(/opt/bin/skillgraph-mcp probe *)") {
		t.Errorf("custom binary not in allowed-tools: %s", content)
	}
	if !strings.Contains(content, "!`/opt/bin/skillgraph-mcp probe --config /etc/mcp.json x`") {
		t.Errorf("custom binary not in preflight: %s", content)
	}
}

func TestGenerateClaudeSkillsRejectsUnsafeName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgs := map[string]config.Server{
		"../escape": &config.StdioServer{Type: "stdio", Command: "x"},
		"":          &config.StdioServer{Type: "stdio", Command: "x"},
		"ok":        &config.StdioServer{Type: "stdio", Command: "x"},
	}
	if err := GenerateClaudeSkills(dir, cfgs, nil, ClaudeSkillsOptions{}); err != nil {
		t.Fatal(err)
	}
	// Only "ok" should have been written.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "ok" {
		t.Errorf("expected only 'ok' dir, got %v", entries)
	}
}

func TestGenerateClaudeSkillsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgs := map[string]config.Server{
		"a": &config.StdioServer{Type: "stdio", Command: "x"},
	}
	for i := 0; i < 3; i++ {
		if err := GenerateClaudeSkills(dir, cfgs, nil, ClaudeSkillsOptions{}); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	// Should not have left any .tmp-* files behind.
	entries, _ := os.ReadDir(filepath.Join(dir, "a"))
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestYAMLEscape(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"", `""`},
		{"plain text", "plain text"},
		{"with: colon", `"with: colon"`},
		{"with \"quote\"", `"with \"quote\""`},
		{"-starts dash", `"-starts dash"`},
	}
	for _, tt := range tests {
		if got := yamlEscape(tt.in); got != tt.want {
			t.Errorf("yamlEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
