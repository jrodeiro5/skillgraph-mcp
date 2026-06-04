package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listSkillsInput struct{}

func RegisterListSkills(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "list_skills",
			Description: "List all available skills (downstream MCP servers) by name. IMPORTANT: skills are NOT directly callable as MCP tools. They are accessed only via the execute_code sandbox. Workflow: list_skills → use_skill(name) to see tool signatures → execute_code(python) to call them.",
		},
		newListSkills(mgr),
	)
}

func newListSkills(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, listSkillsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input listSkillsInput) (*mcp.CallToolResult, any, error) {
		var lines []string
		for _, name := range mgr.ListServerNames() {
			srv, err := mgr.GetServer(name)
			if err != nil {
				continue
			}
			if instr := srv.Instructions(); instr != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", name, instr))
			} else {
				lines = append(lines, fmt.Sprintf("- %s", name))
			}
			// Enumerate skill:// resources from servers that ship them (Sam Morrow emerging spec).
			for _, r := range srv.Resources() {
				if strings.HasPrefix(r.URI, "skill://") {
					if r.Description != "" {
						lines = append(lines, fmt.Sprintf("  skill-resource: %s — %s", r.URI, r.Description))
					} else {
						lines = append(lines, fmt.Sprintf("  skill-resource: %s", r.URI))
					}
				}
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil, nil
	}
}
