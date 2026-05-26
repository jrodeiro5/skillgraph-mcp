package graph

import (
	"strings"
	"testing"
)

func TestGraphOperations(t *testing.T) {
	g := New()

	// Test adding nodes
	g.AddNode("git-vcs", NodeSkill, "git-vcs", "Git Version Control System")
	g.AddNode("git_commit", NodeTool, "git_commit", "Commit staged changes")
	g.AddNode("git_push", NodeTool, "git_push", "Push to remote repository")

	if len(g.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(g.Nodes))
	}

	// Test adding edges
	g.AddEdge("git-vcs", "git_commit", RelHasTool, "")
	g.AddEdge("git-vcs", "git_push", RelHasTool, "")
	g.AddEdge("git_commit", "git_push", RelPrerequisiteFor, "Must commit before pushing")

	if len(g.Edges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(g.Edges))
	}

	// Duplicate edge check
	g.AddEdge("git-vcs", "git_commit", RelHasTool, "")
	if len(g.Edges) != 3 {
		t.Errorf("Expected edge count to remain 3 after adding duplicate, got %d", len(g.Edges))
	}

	// Test Subgraph
	sub := g.Subgraph("git_commit")
	if len(sub.Nodes) != 3 { // center, plus the two neighbors (git-vcs and git_push)
		t.Errorf("Expected subgraph to have 3 nodes, got %d", len(sub.Nodes))
	}
	if len(sub.Edges) != 2 { // edge from git-vcs -> git_commit and git_commit -> git_push
		t.Errorf("Expected subgraph to have 2 edges, got %d", len(sub.Edges))
	}

	// Test Compact Format
	compact := g.FormatCompact()
	if !strings.Contains(compact, "Capability Graph:") {
		t.Errorf("Expected format to contain title")
	}
	if !strings.Contains(compact, "[skill] git-vcs") {
		t.Errorf("Expected format to contain node details")
	}
	if !strings.Contains(compact, "- [tool] git_commit -PREREQUISITE_FOR-> [tool] git_push (Must commit before pushing)") {
		t.Errorf("Expected format to contain relation details")
	}
}
