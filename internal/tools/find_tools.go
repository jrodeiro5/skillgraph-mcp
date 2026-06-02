package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrodeiro5/skillgraph-mcp/internal/embed"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const findToolsDescription = `Semantic search over all downstream skill tools. Returns the top-k tools most relevant to your query.

Use this INSTEAD of browsing list_skills + use_skill manually when you know what you want to do but not which skill or tool does it.

After finding the right tool, call execute_code to invoke it.

Example queries:
  "scrape a webpage and return markdown"
  "find callers of a Go function"
  "take a screenshot of a URL"
  "search the web for recent news"
  "run a SQL query against a database"`

type findToolsInput struct {
	Query        string `json:"query"         jsonschema:"natural language description of what you want to do"`
	K            int    `json:"k,omitempty"   jsonschema:"number of results to return, default 5"`
	ServerFilter string `json:"server_filter,omitempty" jsonschema:"limit results to a specific skill/server name"`
}

// RegisterFindTools registers the find_tools gateway tool.
// idx may be nil — in that case the tool returns a helpful error instead of panicking.
func RegisterFindTools(s *mcp.Server, mgr *mcpserver.Manager, idx *embed.Index) {
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "find_tools",
			Description: findToolsDescription,
		},
		newFindTools(mgr, idx),
	)
}

func newFindTools(mgr *mcpserver.Manager, idx *embed.Index) func(context.Context, *mcp.CallToolRequest, findToolsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input findToolsInput) (*mcp.CallToolResult, any, error) {
		if idx == nil || idx.Len() == 0 {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("embedding index not available (no embedding provider configured — set LLM_BASE_URL/OLLAMA_HOST or OPENAI_API_KEY). Fall back to list_skills + use_skill"))
			return result, nil, nil
		}

		k := input.K
		if k <= 0 {
			k = 5
		}
		if k > 20 {
			k = 20
		}

		results, err := idx.Find(ctx, input.Query, k, input.ServerFilter)
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("find_tools: %w", err))
			return result, nil, nil
		}

		if len(results) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{
					Text: "No matching tools found. Try a different query or call list_skills to browse all skills.",
				}},
			}, nil, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Top %d tools for %q:\n\n", len(results), input.Query))
		for i, r := range results {
			sb.WriteString(fmt.Sprintf("%d. %s (skill: %s, score: %.3f)\n", i+1, r.ToolName, r.Server, r.Score))
			if r.Description != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", r.Description))
			}
		}
		sb.WriteString("\nTo call a tool: execute_code with the tool name as a Python function.")

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}},
		}, nil, nil
	}
}

// BuildIndexEntries converts the manager's AllTools into embed.ToolEntry slice.
// Each entry embeds tool name + signature + description for richer semantic matching.
func BuildIndexEntries(mgr *mcpserver.Manager) []embed.ToolEntry {
	tools := mgr.AllTools()
	entries := make([]embed.ToolEntry, 0, len(tools))
	for _, t := range tools {
		// Combine name + signature + description so embeddings capture
		// both what the tool is called and what it does.
		sig := t.Signature()
		text := t.ResolvedName
		if sig != "" {
			text = sig
		}
		if t.Description != "" && !strings.Contains(sig, t.Description) {
			text = text + "\n" + t.Description
		}
		entries = append(entries, embed.ToolEntry{
			Server:      t.ServerName,
			ToolName:    t.ResolvedName,
			Description: text,
		})
	}
	return entries
}
