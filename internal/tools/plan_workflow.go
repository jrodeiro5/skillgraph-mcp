package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrodeiro5/skillgraph-mcp/internal/graph"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type planWorkflowInput struct {
	Goal string `json:"goal" jsonschema:"the high-level goal you want to achieve, e.g. 'comment on a created issue'"`
}

func RegisterPlanWorkflow(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "plan_workflow",
			Description: "Suggest a sequence of tools and skills to achieve a given goal by analyzing dependencies in the capability graph.",
		},
		newPlanWorkflow(mgr),
	)
}

func newPlanWorkflow(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, planWorkflowInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input planWorkflowInput) (*mcp.CallToolResult, any, error) {
		g := mgr.GetGraph()
		if g == nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("graph not initialized"))
			return result, nil, nil
		}

		targets := matchGoalToNodes(g, input.Goal)
		if len(targets) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No matching tools found for this goal. Try `list_skills` to browse available skills."}},
			}, nil, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Workflow plan for: %q\n\n", input.Goal))

		seen := make(map[string]bool)
		for _, target := range targets {
			if target.Type != graph.NodeTool || seen[target.ID] {
				continue
			}
			seen[target.ID] = true

			chain := buildPrerequisiteChain(g, target.ID)
			chain = append(chain, target)

			sb.WriteString(fmt.Sprintf("## Path to %s\n", target.Name))
			for i, node := range chain {
				skill := findParentSkill(g, node.ID)
				sb.WriteString(fmt.Sprintf("  %d. **%s**", i+1, node.Name))
				if skill != nil {
					sb.WriteString(fmt.Sprintf(" — via `use_skill(%q)`", skill.Name))
				}
				if node.Description != "" {
					sb.WriteString(fmt.Sprintf("\n     %s", node.Description))
				}
				sb.WriteString("\n")
			}

			var nexts []string
			for _, e := range g.Edges {
				if e.Source != target.ID {
					continue
				}
				if e.Type != graph.RelCommonNextStep && e.Type != graph.RelProduces {
					continue
				}
				if n, ok := g.Nodes[e.Target]; ok {
					label := n.Name
					if e.Description != "" {
						label += " (" + e.Description + ")"
					}
					nexts = append(nexts, "- "+label)
				}
			}
			if len(nexts) > 0 {
				sb.WriteString("  After this:\n")
				for _, ns := range nexts {
					sb.WriteString("    " + ns + "\n")
				}
			}
			sb.WriteString("\n")
		}

		var skills []string
		for _, n := range targets {
			if n.Type == graph.NodeSkill {
				entry := fmt.Sprintf("- `use_skill(%q)`", n.Name)
				if n.Description != "" {
					entry += ": " + n.Description
				}
				skills = append(skills, entry)
			}
		}
		if len(skills) > 0 {
			sb.WriteString("## Skills to explore\n")
			for _, s := range skills {
				sb.WriteString(s + "\n")
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	}
}

// commonStopwords are filtered out before keyword matching to reduce noise.
var commonStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "how": true, "to": true,
	"with": true, "from": true, "that": true, "this": true, "into": true,
	"use": true, "using": true, "can": true, "want": true, "need": true,
	"get": true, "set": true, "all": true, "any": true, "its": true,
	"are": true, "was": true, "has": true, "have": true, "had": true,
	"will": true, "what": true, "then": true, "also": true, "when": true,
	"make": true, "give": true, "show": true, "find": true, "list": true,
}

// matchGoalToNodes returns all nodes whose name or description shares a
// meaningful keyword (length > 3, not a stopword) with the goal string.
func matchGoalToNodes(g *graph.Graph, goal string) []*graph.Node {
	words := strings.Fields(strings.ToLower(goal))
	var keywords []string
	for _, w := range words {
		if len(w) > 3 && !commonStopwords[w] {
			keywords = append(keywords, w)
		}
	}
	if len(keywords) == 0 {
		// Fall back to all non-stopwords of any length if filtering left nothing.
		for _, w := range words {
			if !commonStopwords[w] {
				keywords = append(keywords, w)
			}
		}
	}

	var results []*graph.Node
	for _, n := range g.Nodes {
		nameLower := strings.ToLower(n.Name)
		descLower := strings.ToLower(n.Description)
		for _, kw := range keywords {
			if strings.Contains(nameLower, kw) || strings.Contains(descLower, kw) {
				results = append(results, n)
				break
			}
		}
	}
	return results
}

// buildPrerequisiteChain walks backwards through PREREQUISITE_FOR and REQUIRES
// edges from targetID and returns the dependency chain in execution order
// (deepest prerequisite first, target excluded).
func buildPrerequisiteChain(g *graph.Graph, targetID string) []*graph.Node {
	visited := map[string]bool{targetID: true}
	queue := []string{targetID}
	var chain []*graph.Node

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, e := range g.Edges {
			var prereqID string
			switch {
			case e.Target == cur && e.Type == graph.RelPrerequisiteFor:
				prereqID = e.Source
			case e.Source == cur && e.Type == graph.RelRequires:
				prereqID = e.Target
			}
			if prereqID == "" || visited[prereqID] {
				continue
			}
			n, ok := g.Nodes[prereqID]
			if !ok || n.Type != graph.NodeTool {
				continue
			}
			visited[prereqID] = true
			chain = append(chain, n)
			queue = append(queue, prereqID)
		}
	}

	// Reverse so deepest prerequisites appear first.
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// findParentSkill returns the Skill node connected to nodeID via a HAS_TOOL edge.
func findParentSkill(g *graph.Graph, nodeID string) *graph.Node {
	for _, e := range g.Edges {
		if e.Target == nodeID && e.Type == graph.RelHasTool {
			if n, ok := g.Nodes[e.Source]; ok && n.Type == graph.NodeSkill {
				return n
			}
		}
	}
	return nil
}
