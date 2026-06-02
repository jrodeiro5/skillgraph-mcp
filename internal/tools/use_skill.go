package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type useSkillInput struct {
	SkillName string `json:"skill_name" jsonschema:"name of the skill to inspect"`
}

func RegisterUseSkill(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "use_skill",
			Description: "Show the tool signatures and resources for a specific skill (downstream server). Returns function signatures you can call inside execute_code. IMPORTANT: these tools are NOT MCP tools you can call directly — they are Python functions only accessible inside the execute_code sandbox. After calling use_skill, write execute_code(code) to invoke them.",
		},
		newUseSkill(mgr),
	)
}

func newUseSkill(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, useSkillInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input useSkillInput) (*mcp.CallToolResult, any, error) {
		srv, err := mgr.GetServer(input.SkillName)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(err)
			return result, nil, nil
		}

		var lines []string
		for _, t := range mgr.ServerTools(input.SkillName) {
			lines = append(lines, t.Signature())
		}

		if resources := srv.Resources(); len(resources) > 0 {
			lines = append(lines, "\nResources:")
			for _, r := range resources {
				if r.Description != "" {
					lines = append(lines, fmt.Sprintf("- %s: %s", r.URI, r.Description))
				} else {
					lines = append(lines, fmt.Sprintf("- %s", r.URI))
				}
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil, nil
	}
}
