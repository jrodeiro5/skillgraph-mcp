package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/kurtisvg/skillful-mcp/internal/graph"
	"github.com/kurtisvg/skillful-mcp/internal/mcpserver"
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

		goalLower := strings.ToLower(input.Goal)
		words := strings.Fields(goalLower)

		// Find target nodes matching goal keywords
		var targets []*graph.Node
		for _, n := range g.Nodes {
			nameLower := strings.ToLower(n.Name)
			descLower := strings.ToLower(n.Description)

			matched := false
			for _, w := range words {
				if len(w) > 2 && (strings.Contains(nameLower, w) || strings.Contains(descLower, w)) {
					matched = true
					break
				}
			}
			if matched {
				targets = append(targets, n)
			}
		}

		if len(targets) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No specific workflow path found in the capability graph for this goal. Try using different keywords."}},
			}, nil, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Suggested Workflow Plan for: %q\n\n", input.Goal))

		// For each target tool, trace its prerequisites and recommendations
		seen := make(map[string]bool)
		for _, target := range targets {
			if target.Type != graph.NodeTool {
				continue
			}
			if seen[target.ID] {
				continue
			}
			seen[target.ID] = true

			sb.WriteString(fmt.Sprintf("--- Action Path for Tool: %s ---\n", target.Name))
			if target.Description != "" {
				sb.WriteString(fmt.Sprintf("Description: %s\n", target.Description))
			}

			// Find prerequisites (nodes that are PREREQUISITE_FOR or HAS_TOOL)
			var prerequisites []string
			var nextSteps []string

			for _, e := range g.Edges {
				if e.Target == target.ID {
					if e.Type == graph.RelPrerequisiteFor {
						prerequisites = append(prerequisites, fmt.Sprintf("- Run %s first (%s)", e.Source, e.Description))
					}
				}
				if e.Source == target.ID {
					if e.Type == graph.RelPrerequisiteFor {
						nextSteps = append(nextSteps, fmt.Sprintf("- Follow with %s (%s)", e.Target, e.Description))
					} else if e.Type == graph.RelCommonNextStep {
						nextSteps = append(nextSteps, fmt.Sprintf("- Commonly followed by %s (%s)", e.Target, e.Description))
					}
				}
			}

			if len(prerequisites) > 0 {
				sb.WriteString("Prerequisites / Inputs required:\n")
				for _, p := range prerequisites {
					sb.WriteString(p + "\n")
				}
			} else {
				sb.WriteString("Prerequisites: None detected (can be run directly or as starter step)\n")
			}

			sb.WriteString(fmt.Sprintf("Execution: Call %s()\n", target.Name))

			if len(nextSteps) > 0 {
				sb.WriteString("Recommended next steps:\n")
				for _, ns := range nextSteps {
					sb.WriteString(ns + "\n")
				}
			}
			sb.WriteString("\n")
		}

		// Also suggest any skills that match
		var suggestedSkills []string
		for _, target := range targets {
			if target.Type == graph.NodeSkill {
				suggestedSkills = append(suggestedSkills, fmt.Sprintf("- %s: %s", target.Name, target.Description))
			}
		}
		if len(suggestedSkills) > 0 {
			sb.WriteString("Relevant Skills / Server Areas to explore:\n")
			for _, sk := range suggestedSkills {
				sb.WriteString(sk + "\n")
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	}
}
