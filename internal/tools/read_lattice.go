package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kurtisvg/skillful-mcp/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type readLatticeInput struct {
	FilePath string `json:"file_path" jsonschema:"relative path to the file inside .mcp_lattice (e.g., 'skills.md', 'relations.md', 'fetch_readme.md')"`
}

func RegisterReadLattice(s *mcp.Server, mgr *mcpserver.Manager) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "read_lattice",
			Description: "Read markdown documentation from the semantic lattice folder to understand detailed capabilities and server READMEs.",
		},
		newReadLattice(mgr),
	)
}

func newReadLattice(mgr *mcpserver.Manager) func(context.Context, *mcp.CallToolRequest, readLatticeInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input readLatticeInput) (*mcp.CallToolResult, any, error) {
		latticeDir, err := filepath.Abs("./.mcp_lattice")
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("failed to locate lattice folder: %w", err))
			return result, nil, nil
		}

		targetPath, err := filepath.Abs(filepath.Join(latticeDir, input.FilePath))
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("invalid path: %w", err))
			return result, nil, nil
		}

		// Security check: Prevent path traversal outside the lattice folder
		if !strings.HasPrefix(targetPath, latticeDir) {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("access denied: path traversal detected"))
			return result, nil, nil
		}

		content, err := os.ReadFile(targetPath)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("failed to read file: %w", err))
			return result, nil, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(content)}},
		}, nil, nil
	}
}
