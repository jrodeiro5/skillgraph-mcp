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

	res, err := GenerateClaudeSkills(dir, cfgs, g, ClaudeSkillsOptions{ConfigPath: "/tmp/mcp.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 2 {
		t.Errorf("expected 2 written, got %d (skipped %v)", len(res.Written), res.Skipped)
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
	if _, err := GenerateClaudeSkills(dir, cfgs, nil, opts); err != nil {
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
	res, err := GenerateClaudeSkills(dir, cfgs, nil, ClaudeSkillsOptions{})
	if err != nil {
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
	if len(res.Written) != 1 || res.Written[0] != "ok" {
		t.Errorf("expected Written=[ok], got %v", res.Written)
	}
	if len(res.Skipped) != 2 {
		t.Errorf("expected 2 skipped (unsafe names), got %v", res.Skipped)
	}
}

func TestGenerateClaudeSkillsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgs := map[string]config.Server{
		"a": &config.StdioServer{Type: "stdio", Command: "x"},
	}
	for i := 0; i < 3; i++ {
		if _, err := GenerateClaudeSkills(dir, cfgs, nil, ClaudeSkillsOptions{Force: true}); err != nil {
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

// TestGenerateClaudeSkillsRefusesOverwrite verifies the default (Force=false)
// path skips skills with an existing SKILL.md instead of clobbering it. This is
// the gitnexus-style case: a user's curated SKILL.md must survive a re-run.
func TestGenerateClaudeSkillsRefusesOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgs := map[string]config.Server{
		"gitnexus": &config.StdioServer{Type: "stdio", Command: "gitnexus"},
	}

	preexisting := []byte("# Hand-curated SKILL.md\n")
	if err := os.MkdirAll(filepath.Join(dir, "gitnexus"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gitnexus", "SKILL.md"), preexisting, 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GenerateClaudeSkills(dir, cfgs, nil, ClaudeSkillsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 {
		t.Errorf("expected 0 written, got %v", res.Written)
	}
	if len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0], "gitnexus") {
		t.Errorf("expected gitnexus in Skipped, got %v", res.Skipped)
	}

	got, _ := os.ReadFile(filepath.Join(dir, "gitnexus", "SKILL.md"))
	if string(got) != string(preexisting) {
		t.Errorf("existing SKILL.md was overwritten: %q", got)
	}
}

// TestGenerateClaudeSkillsForceOverwrite verifies --force/Force=true actually
// overwrites pre-existing files.
func TestGenerateClaudeSkillsForceOverwrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgs := map[string]config.Server{
		"x": &config.StdioServer{Type: "stdio", Command: "x"},
	}
	if err := os.MkdirAll(filepath.Join(dir, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x", "SKILL.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := GenerateClaudeSkills(dir, cfgs, nil, ClaudeSkillsOptions{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 1 {
		t.Errorf("expected 1 written, got %v", res.Written)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "x", "SKILL.md"))
	if string(got) == "old" {
		t.Error("force=true did not overwrite")
	}
}

// TestGenerateClaudeSkillsSkipsSymlinkedDir protects centralized hub setups
// where ~/.claude/skills/<name> is a symlink to a separate skills repo. Without
// Force, the generator must not follow the symlink and write into the hub.
func TestGenerateClaudeSkillsSkipsSymlinkedDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hubDir := t.TempDir() // simulates the centralized skills hub

	hubGitnexus := filepath.Join(hubDir, "gitnexus")
	if err := os.MkdirAll(hubGitnexus, 0o755); err != nil {
		t.Fatal(err)
	}
	hubSkill := filepath.Join(hubGitnexus, "SKILL.md")
	if err := os.WriteFile(hubSkill, []byte("# Hub-managed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(dir, "gitnexus")
	if err := os.Symlink(hubGitnexus, linkPath); err != nil {
		t.Fatal(err)
	}

	cfgs := map[string]config.Server{
		"gitnexus": &config.StdioServer{Type: "stdio", Command: "gitnexus"},
	}

	res, err := GenerateClaudeSkills(dir, cfgs, nil, ClaudeSkillsOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 {
		t.Errorf("expected 0 written, got %v", res.Written)
	}
	if len(res.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %v", res.Skipped)
	}
	got, _ := os.ReadFile(hubSkill)
	if string(got) != "# Hub-managed\n" {
		t.Errorf("hub SKILL.md was modified: %q", got)
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
