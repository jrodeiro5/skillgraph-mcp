package refine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kurtisvg/skillful-mcp/internal/config"
	"github.com/kurtisvg/skillful-mcp/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCleanMarkdownJSON(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"```json\n{\"key\": \"val\"}\n```", "{\"key\": \"val\"}"},
		{"```\n{\"key\": \"val\"}\n```", "{\"key\": \"val\"}"},
		{"   {\"key\": \"val\"}   ", "{\"key\": \"val\"}"},
	}

	for i, tc := range cases {
		got := cleanMarkdownJSON(tc.input)
		if got != tc.expected {
			t.Errorf("case %d: expected %q, got %q", i, tc.expected, got)
		}
	}
}

func TestCallDeepSeek(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ds-test-key" {
			t.Errorf("expected ds-test-key token, got %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"content": `{"status": "deepseek-ok"}`,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override endpoint
	oldDS := deepseekEndpoint
	deepseekEndpoint = mockServer.URL
	defer func() { deepseekEndpoint = oldDS }()

	res, err := callDeepSeek(context.Background(), "ds-test-key", "system", "user")
	if err != nil {
		t.Fatalf("callDeepSeek failed: %v", err)
	}

	if res != `{"status": "deepseek-ok"}` {
		t.Errorf("expected deepseek-ok content, got %s", res)
	}
}

func TestCallGemini(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key != "gemini-test-key" {
			t.Errorf("expected gemini-test-key, got %s", key)
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]string{
							{"text": `{"status": "gemini-ok"}`},
						},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override endpoint
	oldGemini := geminiEndpoint
	geminiEndpoint = mockServer.URL
	defer func() { geminiEndpoint = oldGemini }()

	res, err := callGemini(context.Background(), "gemini-test-key", "system", "user")
	if err != nil {
		t.Fatalf("callGemini failed: %v", err)
	}

	if res != `{"status": "gemini-ok"}` {
		t.Errorf("expected gemini-ok content, got %s", res)
	}
}

func startFakeServer(t *testing.T, ctx context.Context, toolNames []string) *mcp.ClientSession {
	t.Helper()

	srv := mcp.NewServer(&mcp.Implementation{Name: "fake-server", Version: "1.0"}, nil)
	for _, name := range toolNames {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        name,
			Description: "tool " + name,
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"param1": map[string]any{
						"type":        "string",
						"description": "a test parameter description",
					},
				},
			},
		}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
			}, nil, nil
		})
	}

	serverT, clientT := mcp.NewInMemoryTransports()
	go func() { _ = srv.Run(ctx, serverT) }()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestRefineServerExecution(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "refine-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "mcp.json")
	latticeDir := filepath.Join(tmpDir, ".mcp_lattice")
	serverName := "calc-server"

	// Create initial config
	initialCfg := `{
		"mcpServers": {
			"calc-server": {
				"command": "node",
				"args": ["dummy.js"]
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(initialCfg), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Create dummy README in lattice dir
	if err := os.MkdirAll(latticeDir, 0755); err != nil {
		t.Fatalf("failed to create lattice dir: %v", err)
	}
	readmePath := filepath.Join(latticeDir, fmt.Sprintf("%s_readme.md", serverName))
	if err := os.WriteFile(readmePath, []byte("# Calculator\nAdds numbers."), 0644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}

	// Load servers from config
	servers, _, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading servers: %v", err)
	}

	// Create a real session in memory
	session := startFakeServer(t, ctx, []string{"test_tool"})
	defer session.Close()

	srv, err := mcpserver.NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatalf("failed to wrap session: %v", err)
	}

	// Setup Manager
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{
		"calc-server": srv,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Setup mock LLM server for DeepSeek
	mockLLMOutput := `{
		"descriptions": {
			"calc-server": "A clean calculator skill",
			"test_tool": "refined test tool description"
		},
		"relations": []
	}`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"content": mockLLMOutput,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override DeepSeek endpoint
	oldDS := deepseekEndpoint
	deepseekEndpoint = mockServer.URL
	defer func() { deepseekEndpoint = oldDS }()

	// Call refineServer directly
	err = refineServer(ctx, "deepseek", "dummy-key", configPath, mgr, latticeDir, serverName, servers)
	if err != nil {
		t.Fatalf("refineServer failed: %v", err)
	}

	// Verify configuration file was updated with the descriptions
	_, updatedCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	if updatedCfg.Descriptions["calc-server"] != "A clean calculator skill" {
		t.Errorf("expected calc-server description 'A clean calculator skill', got '%s'", updatedCfg.Descriptions["calc-server"])
	}

	if updatedCfg.Descriptions["test_tool"] != "refined test tool description" {
		t.Errorf("expected test_tool description, got '%s'", updatedCfg.Descriptions["test_tool"])
	}

	// Verify memory graph was rebuilt in manager
	n, exists := mgr.GetGraph().Nodes["calc-server"]
	if !exists {
		t.Fatalf("calc-server node missing from rebuilt graph")
	}

	if n.Description != "A clean calculator skill" {
		t.Errorf("expected node description 'A clean calculator skill', got '%s'", n.Description)
	}
}

func TestOptimizeTracesExecution(t *testing.T) {
	ctx := context.Background()

	tmpDir, err := os.MkdirTemp("", "refine-optimize-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "mcp.json")
	latticeDir := filepath.Join(tmpDir, ".mcp_lattice")
	serverName := "calc-server"

	// Create initial config
	initialCfg := `{
		"mcpServers": {
			"calc-server": {
				"command": "node",
				"args": ["dummy.js"]
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(initialCfg), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Create dummy README in lattice dir
	if err := os.MkdirAll(latticeDir, 0755); err != nil {
		t.Fatalf("failed to create lattice dir: %v", err)
	}
	readmePath := filepath.Join(latticeDir, fmt.Sprintf("%s_readme.md", serverName))
	if err := os.WriteFile(readmePath, []byte("# Calculator\nAdds numbers."), 0644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}

	// Load servers from config
	servers, _, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading servers: %v", err)
	}

	// Create a real session in memory
	session := startFakeServer(t, ctx, []string{"test_tool"})
	defer session.Close()

	srv, err := mcpserver.NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatalf("failed to wrap session: %v", err)
	}

	// Setup Manager
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{
		"calc-server": srv,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	// Create a fake trace file
	tracesDir := filepath.Join(latticeDir, "traces")
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(tracesDir, "trace1.json")
	traceData := `{
		"timestamp": "2026-05-26T22:00:00Z",
		"code": "test_tool(param1=42)",
		"tool_calls": [
			{
				"tool_name": "test_tool",
				"args": {"param1": 42},
				"result": "error: expected string, got int",
				"is_error": true
			}
		],
		"error": "TypeError: expected string, got int"
	}`
	if err := os.WriteFile(tracePath, []byte(traceData), 0644); err != nil {
		t.Fatal(err)
	}

	// Setup mock LLM server for DeepSeek
	mockLLMOutput := `{
		"descriptions": {
			"test_tool": "refined test tool description: parameter param1 must be a string."
		},
		"relations": [
			{
				"source": "calc-server",
				"target": "test_tool",
				"type": "PREREQUISITE_FOR",
				"description": "calc-server setup is prerequisite for test_tool"
			}
		]
	}`

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"content": mockLLMOutput,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override DeepSeek endpoint
	oldDS := deepseekEndpoint
	deepseekEndpoint = mockServer.URL
	defer func() { deepseekEndpoint = oldDS }()

	// Call optimizeTraces
	err = optimizeTraces(ctx, "deepseek", "dummy-key", configPath, mgr, latticeDir, servers, []string{tracePath})
	if err != nil {
		t.Fatalf("optimizeTraces failed: %v", err)
	}

	// Verify configuration file was updated with the descriptions and relations
	_, updatedCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	if updatedCfg.Descriptions["test_tool"] != "refined test tool description: parameter param1 must be a string." {
		t.Errorf("expected test_tool description, got '%s'", updatedCfg.Descriptions["test_tool"])
	}

	if len(updatedCfg.Relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(updatedCfg.Relations))
	}
	if updatedCfg.Relations[0].Source != "calc-server" || updatedCfg.Relations[0].Target != "test_tool" || updatedCfg.Relations[0].Type != "PREREQUISITE_FOR" {
		t.Errorf("got relation %+v", updatedCfg.Relations[0])
	}

	// Verify memory graph was rebuilt in manager
	n, exists := mgr.GetGraph().Nodes["test_tool"]
	if !exists {
		t.Fatalf("test_tool node missing from rebuilt graph")
	}

	if n.Description != "refined test tool description: parameter param1 must be a string." {
		t.Errorf("expected node description, got '%s'", n.Description)
	}

	// Verify trace file was deleted
	if _, err := os.Stat(tracePath); !os.IsNotExist(err) {
		t.Errorf("expected trace file to be deleted, but it still exists")
	}
}
