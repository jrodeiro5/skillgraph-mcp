package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type registerServerInput struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

// GatewayBinding describes the transport and host the gateway itself is
// serving on. Used by register_server to gate registration when the gateway
// is exposed on a non-loopback HTTP address (see ADR-0001).
type GatewayBinding struct {
	Transport string
	Host      string
}

func RegisterRegisterServer(s *mcp.Server, mgr *mcpserver.Manager, configPath string, binding GatewayBinding) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name: "register_server",
			Description: "Hot-register a new MCP server at runtime without restarting. " +
				"Provide a server name and a config block identical to an mcpServers entry in mcp.json. " +
				"The server is connected immediately and its tools become available via use_skill.",
		},
		newRegisterServer(mgr, configPath, binding),
	)
}

func newRegisterServer(mgr *mcpserver.Manager, configPath string, binding GatewayBinding) func(context.Context, *mcp.CallToolRequest, registerServerInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input registerServerInput) (*mcp.CallToolResult, any, error) {
		if binding.Transport == "http" && !isLoopbackHost(binding.Host) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{
					Text: fmt.Sprintf(
						"register_server refused: gateway is serving HTTP on non-loopback host %q; see docs/adr/0001-loopback-default-and-register-server-gate.md",
						binding.Host,
					),
				}},
			}, nil, nil
		}

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

// isLoopbackHost reports whether host resolves exclusively to loopback
// addresses (127.0.0.0/8 or ::1). An empty host is treated as loopback
// (net.Listen on an empty host binds to all interfaces, but the gateway
// flag defaults to "localhost" and we deny-by-default on resolution
// failure rather than silently allowing). DNS resolution failures are
// treated as non-loopback (deny by default).
func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	addrs, err := net.LookupHost(host)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil || !ip.IsLoopback() {
			return false
		}
	}
	return true
}
