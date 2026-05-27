package tools

import (
	"context"
	"fmt"

	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type getSkillGraphInput struct {
	SkillName string `json:"skill_name,omitempty" jsonschema:"optional name of a skill to focus the graph query on"`
}

func RegisterGetSkillGraph(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "get_skill_graph",
			Description: "Get the capability relationship graph between skills, tools, and resources, showing dependencies and recommended flows.",
		},
		newGetSkillGraph(mgr),
	)
}

func newGetSkillGraph(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, getSkillGraphInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input getSkillGraphInput) (*mcp.CallToolResult, any, error) {
		g := mgr.GetGraph()
		if g == nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("graph not initialized"))
			return result, nil, nil
		}

		var graphStr string
		if input.SkillName != "" {
			sub := g.Subgraph(input.SkillName)
			graphStr = sub.FormatCompact()
		} else {
			graphStr = g.FormatCompact()
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: graphStr}},
		}, nil, nil
	}
}
