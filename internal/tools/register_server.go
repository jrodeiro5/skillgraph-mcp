package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type registerServerInput struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

func RegisterRegisterServer(s *mcp.Server, mgr *mcpserver.Manager, configPath string) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name: "register_server",
			Description: "Hot-register a new MCP server at runtime without restarting. " +
				"Provide a server name and a config block identical to an mcpServers entry in mcp.json. " +
				"The server is connected immediately and its tools become available via use_skill.",
		},
		newRegisterServer(mgr, configPath),
	)
}

func newRegisterServer(mgr *mcpserver.Manager, configPath string) func(context.Context, *mcp.CallToolRequest, registerServerInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input registerServerInput) (*mcp.CallToolResult, any, error) {
		if input.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		if len(input.Config) == 0 {
			return nil, nil, fmt.Errorf("config is required")
		}

		cfg, err := config.UnmarshalServer(input.Config)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid server config: %w", err)
		}

		if err := mgr.AddServer(ctx, input.Name, cfg); err != nil {
			return nil, nil, fmt.Errorf("registering server: %w", err)
		}

		if configPath != "" {
			if err := config.AddServer(configPath, input.Name, input.Config); err != nil {
				// Non-fatal: in-memory registration succeeded; log the persistence failure.
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{
						Text: fmt.Sprintf(
							"Server %q registered (in-memory only — failed to persist to config: %v). "+
								"It will not survive a restart.",
							input.Name, err,
						),
					}},
				}, nil, nil
			}
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Server %q registered and persisted to config.", input.Name),
			}},
		}, nil, nil
	}
}
