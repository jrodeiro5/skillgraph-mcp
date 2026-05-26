package mcpserver

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetServer(t *testing.T) {
	t.Parallel()

	m, err := NewManagerFromServers(map[string]*Server{
		"alpha": {},
		"bravo": {},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("existing server", func(t *testing.T) {
		t.Parallel()
		s, err := m.GetServer("alpha")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil server")
		}
	})

	t.Run("unknown server", func(t *testing.T) {
		t.Parallel()
		_, err := m.GetServer("nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown server")
		}
	})
}

func TestListServerNames(t *testing.T) {
	t.Parallel()

	t.Run("returns all names", func(t *testing.T) {
		t.Parallel()
		m, err := NewManagerFromServers(map[string]*Server{
			"charlie": {},
			"alpha":   {},
			"bravo":   {},
		})
		if err != nil {
			t.Fatal(err)
		}

		names := m.ListServerNames()
		if len(names) != 3 {
			t.Fatalf("got %d names, want 3", len(names))
		}
		nameSet := map[string]bool{}
		for _, n := range names {
			nameSet[n] = true
		}
		for _, expected := range []string{"alpha", "bravo", "charlie"} {
			if !nameSet[expected] {
				t.Errorf("missing expected name %q", expected)
			}
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		m, err := NewManagerFromServers(map[string]*Server{})
		if err != nil {
			t.Fatal(err)
		}
		names := m.ListServerNames()
		if len(names) != 0 {
			t.Errorf("expected empty, got %v", names)
		}
	})
}

func TestAllTools(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	s1, err := NewServerFromSession(ctx, startFakeServer(t, ctx, []string{"tool_a"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewServerFromSession(ctx, startFakeServer(t, ctx, []string{"tool_b"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManagerFromServers(map[string]*Server{"alpha": s1, "beta": s2})
	if err != nil {
		t.Fatal(err)
	}

	tools := m.AllTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestManagerServerTools(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	s1, err := NewServerFromSession(ctx, startFakeServer(t, ctx, []string{"tool_a"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewServerFromSession(ctx, startFakeServer(t, ctx, []string{"tool_b"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManagerFromServers(map[string]*Server{"alpha": s1, "beta": s2})
	if err != nil {
		t.Fatal(err)
	}

	tools := m.ServerTools("alpha")
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool for alpha, got %d", len(tools))
	}
	if tools[0].ResolvedName != "tool_a" {
		t.Errorf("expected tool_a, got %q", tools[0].ResolvedName)
	}

	tools = m.ServerTools("nonexistent")
	if len(tools) != 0 {
		t.Errorf("expected 0 tools for nonexistent, got %d", len(tools))
	}
}

func TestManagerClose(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	session := startFakeServer(t, ctx, []string{"tool"}, nil)
	srv, err := NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManagerFromServers(map[string]*Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	// Should not panic on multiple closes.
	m.Close()
	m.Close()
}

func TestManagerGraph(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	session := startFakeServer(t, ctx, nil, nil)
	srv, err := NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// Inject custom tools to test semantic inference
	srv.tools = []*mcp.Tool{
		{
			Name:        "create_issue",
			Description: "Create a new issue",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name:        "comment_on_issue",
			Description: "Add a comment",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"issue_id": map[string]any{"type": "integer"},
					"body":     map[string]any{"type": "string"},
				},
				"required": []any{"issue_id"},
			},
		},
	}

	m, err := NewManagerFromServers(map[string]*Server{"github": srv})
	if err != nil {
		t.Fatal(err)
	}

	g := m.GetGraph()
	if g == nil {
		t.Fatal("expected non-nil graph")
	}

	// Verify Skill node is present
	skillNode, ok := g.Nodes["github"]
	if !ok || skillNode.Type != "skill" {
		t.Error("missing skill node 'github'")
	}

	// Verify Tool nodes are present
	if _, ok := g.Nodes["create_issue"]; !ok {
		t.Error("missing tool node 'create_issue'")
	}
	if _, ok := g.Nodes["comment_on_issue"]; !ok {
		t.Error("missing tool node 'comment_on_issue'")
	}

	// Verify HAS_TOOL relations
	hasCreate := false
	hasComment := false
	hasInferred := false

	for _, edge := range g.Edges {
		if edge.Source == "github" && edge.Target == "create_issue" && edge.Type == "HAS_TOOL" {
			hasCreate = true
		}
		if edge.Source == "github" && edge.Target == "comment_on_issue" && edge.Type == "HAS_TOOL" {
			hasComment = true
		}
		// Verify inferred RelPrerequisiteFor relation (create_issue -> comment_on_issue)
		if edge.Source == "create_issue" && edge.Target == "comment_on_issue" && edge.Type == "PREREQUISITE_FOR" {
			hasInferred = true
		}
	}

	if !hasCreate {
		t.Error("missing edge 'github -HAS_TOOL-> create_issue'")
	}
	if !hasComment {
		t.Error("missing edge 'github -HAS_TOOL-> comment_on_issue'")
	}
	if !hasInferred {
		t.Error("missing inferred relation 'create_issue -PREREQUISITE_FOR-> comment_on_issue'")
	}
}
