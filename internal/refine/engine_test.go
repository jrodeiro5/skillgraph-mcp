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

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/jrodeiro5/skillgraph-mcp/internal/trace"
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
		_ = json.NewEncoder(w).Encode(resp)
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
		_ = json.NewEncoder(w).Encode(resp)
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
		_ = json.NewEncoder(w).Encode(resp)
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

func TestOptimizeTracesSkipsLLMWhenNoErrors(t *testing.T) {
	ctx := context.Background()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp.json")
	latticeDir := filepath.Join(tmpDir, ".mcp_lattice")

	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{"s":{"command":"x"}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	servers, _, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	session := startFakeServer(t, ctx, []string{"tool_a"})
	defer session.Close()
	srv, err := mcpserver.NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	// Write a trace with no errors at all.
	tracesDir := filepath.Join(latticeDir, "traces")
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(tracesDir, "success.json")
	successTrace := `{
		"timestamp": "2026-05-27T10:00:00Z",
		"code": "tool_a(param1=\"hello\")",
		"tool_calls": [{"tool_name":"tool_a","args":{"param1":"hello"},"result":"ok","is_error":false}],
		"output": "ok"
	}`
	if err := os.WriteFile(tracePath, []byte(successTrace), 0644); err != nil {
		t.Fatal(err)
	}

	// LLM mock that fails the test if called.
	llmCalled := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmCalled = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	oldDS := deepseekEndpoint
	deepseekEndpoint = mockServer.URL
	defer func() { deepseekEndpoint = oldDS }()

	if _, err := optimizeTraces(ctx, "deepseek", "key", configPath, mgr, latticeDir, servers, []string{tracePath}, nil); err != nil {
		t.Fatalf("optimizeTraces failed: %v", err)
	}

	if llmCalled {
		t.Error("LLM was called for a batch with no errors")
	}
	if _, err := os.Stat(tracePath); !os.IsNotExist(err) {
		t.Error("expected trace file to be cleaned up")
	}
}

func TestCallOpenAICompat(t *testing.T) {
	// Isolate from any LLM_BASE_URL the shell might have set (e.g. Mistral / Ollama)
	// — the function reads it at call time and would bypass the mock server.
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")
	var gotModel string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer oai-test-key" {
			t.Errorf("expected Bearer oai-test-key, got %s", r.Header.Get("Authorization"))
		}
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"status": "openai-ok"}`}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	old := openaiCompatEndpoint
	openaiCompatEndpoint = mockServer.URL
	defer func() { openaiCompatEndpoint = old }()

	res, err := callOpenAICompat(context.Background(), "oai-test-key", "system", "user")
	if err != nil {
		t.Fatalf("callOpenAICompat failed: %v", err)
	}
	if res != `{"status": "openai-ok"}` {
		t.Errorf("expected openai-ok content, got %s", res)
	}
	if gotModel == "" {
		t.Error("expected model to be sent in request body")
	}
}

func TestCallOpenAICompatNoKey(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")
	// Local models (Ollama) may not require an API key — no Authorization header should be sent.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header for empty key, got %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"ok":true}`}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	old := openaiCompatEndpoint
	openaiCompatEndpoint = mockServer.URL
	defer func() { openaiCompatEndpoint = old }()

	res, err := callOpenAICompat(context.Background(), "", "system", "user")
	if err != nil {
		t.Fatalf("callOpenAICompat (no key) failed: %v", err)
	}
	if res != `{"ok":true}` {
		t.Errorf("unexpected response: %s", res)
	}
}

func TestGetAPIKeyLLMBaseURL(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "http://localhost:11434/v1")
	t.Setenv("LLM_API_KEY", "local-key")
	// Clear competing vars so this test is isolated.
	t.Setenv("DEEPSEEK_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")

	provider, key := getAPIKey()
	if provider != "openai" {
		t.Errorf("expected provider openai, got %s", provider)
	}
	if key != "local-key" {
		t.Errorf("expected key local-key, got %s", key)
	}
}

func TestSnapshotCreatedBeforeEdit(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp.json")
	latticeDir := filepath.Join(tmpDir, ".mcp_lattice")

	initial := `{"mcpServers":{"s":{"command":"x"}},"skillGraph":{"descriptions":{"s":"original"}}}`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	servers, _, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	session := startFakeServer(t, ctx, []string{"tool_a"})
	defer session.Close()
	srv, err := mcpserver.NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	tracesDir := filepath.Join(latticeDir, "traces")
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		t.Fatal(err)
	}
	tracePath := filepath.Join(tracesDir, "err.json")
	errTrace := `{"timestamp":"2026-05-27T10:00:00Z","code":"tool_a()","tool_calls":[{"tool_name":"tool_a","args":{},"result":"error: bad","is_error":true}],"error":"bad"}`
	if err := os.WriteFile(tracePath, []byte(errTrace), 0644); err != nil {
		t.Fatal(err)
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"descriptions":{"tool_a":"improved"},"relations":[]}`}},
			},
		})
	}))
	defer mockServer.Close()
	old := deepseekEndpoint
	deepseekEndpoint = mockServer.URL
	defer func() { deepseekEndpoint = old }()

	if _, err := optimizeTraces(ctx, "deepseek", "key", configPath, mgr, latticeDir, servers, []string{tracePath}, nil); err != nil {
		t.Fatalf("optimizeTraces: %v", err)
	}

	// A snapshot must exist in <latticeDir>/history/ before the edit was applied.
	snapshots, err := filepath.Glob(filepath.Join(latticeDir, "history", "skillgraph_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) == 0 {
		t.Fatal("expected at least one snapshot in history/, got none")
	}

	// Snapshot must contain the original description, not the new one.
	data, err := os.ReadFile(snapshots[0])
	if err != nil {
		t.Fatal(err)
	}
	var snap config.SkillGraphConfig
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("snapshot not valid JSON: %v", err)
	}
	if snap.Descriptions["s"] != "original" {
		t.Errorf("snapshot should capture pre-edit state, got descriptions: %v", snap.Descriptions)
	}
}

func TestSnapshotRollingLimit(t *testing.T) {
	tmpDir := t.TempDir()
	historyDir := filepath.Join(tmpDir, "history")

	cfg := config.SkillGraphConfig{Descriptions: map[string]string{"x": "v"}}
	for i := 0; i < 7; i++ {
		if err := saveSkillGraphSnapshot(historyDir, cfg); err != nil {
			t.Fatalf("snapshot %d: %v", i, err)
		}
	}

	files, err := filepath.Glob(filepath.Join(historyDir, "skillgraph_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 5 {
		t.Errorf("expected 5 snapshots (rolling limit), got %d", len(files))
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
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Override DeepSeek endpoint
	oldDS := deepseekEndpoint
	deepseekEndpoint = mockServer.URL
	defer func() { deepseekEndpoint = oldDS }()

	// Call optimizeTraces
	_, err = optimizeTraces(ctx, "deepseek", "dummy-key", configPath, mgr, latticeDir, servers, []string{tracePath}, nil)
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

func TestSplitHoldout(t *testing.T) {
	tests := []struct {
		files   []string
		wantHO  int
		wantOpt int
	}{
		{[]string{"a", "b", "c"}, 0, 3},                               // < 4 → no hold-out
		{[]string{"a", "b", "c", "d"}, 1, 3},                          // 4 files → 1 hold-out
		{[]string{"a", "b", "c", "d", "e", "f"}, 2, 4},                // 6 files → 2 hold-out
		{[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}, 3, 6}, // 9 → 3 hold-out
	}
	for _, tt := range tests {
		ho, opt := splitHoldout(tt.files)
		if len(ho) != tt.wantHO {
			t.Errorf("splitHoldout(%d files): holdOut len = %d, want %d", len(tt.files), len(ho), tt.wantHO)
		}
		if len(opt) != tt.wantOpt {
			t.Errorf("splitHoldout(%d files): opt len = %d, want %d", len(tt.files), len(opt), tt.wantOpt)
		}
	}
}

func TestWordSet(t *testing.T) {
	words := wordSet("Search for documents by query parameter")
	for _, want := range []string{"search", "documents", "query", "parameter"} {
		if !words[want] {
			t.Errorf("wordSet: expected %q in result", want)
		}
	}
	// Stop words and short words should be excluded.
	for _, skip := range []string{"for", "with", "this", "that", "by"} {
		if words[skip] {
			t.Errorf("wordSet: %q should be excluded", skip)
		}
	}
}

func TestPassesHoldoutGate(t *testing.T) {
	successTrace := trace.Trajectory{
		Code: "search for documents using query string",
		ToolCalls: []trace.ToolCallTrace{
			{ToolName: "search_tool", IsError: false},
		},
	}

	t.Run("no hold-out data always accepts", func(t *testing.T) {
		if !passesHoldoutGate("search_tool", "unrelated description", "also unrelated", nil) {
			t.Error("expected accept with no hold-out data")
		}
	})

	t.Run("improved description accepted", func(t *testing.T) {
		// Proposed has more overlap with the trace than current.
		proposed := "search for documents by query"
		current := "processes stuff"
		if !passesHoldoutGate("search_tool", proposed, current, []trace.Trajectory{successTrace}) {
			t.Error("expected improved description to pass gate")
		}
	})

	t.Run("regressing description rejected", func(t *testing.T) {
		// Proposed has zero overlap; current has overlap with trace words.
		proposed := "processes data pipelines"
		current := "search for documents by query parameter"
		if passesHoldoutGate("search_tool", proposed, current, []trace.Trajectory{successTrace}) {
			t.Error("expected regressing description to be rejected by gate")
		}
	})

	t.Run("irrelevant tool always accepted", func(t *testing.T) {
		// Hold-out has no successful calls to other_tool.
		if !passesHoldoutGate("other_tool", "anything", "anything", []trace.Trajectory{successTrace}) {
			t.Error("expected accept when tool has no relevant hold-out traces")
		}
	})
}

func TestHoldoutGateFiltersRegressionInOptimize(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp.json")
	latticeDir := filepath.Join(tmpDir, ".mcp_lattice")

	initial := `{"mcpServers":{"s":{"command":"x"}},"skillGraph":{"descriptions":{"tool_a":"search for documents by query"}}}`
	if err := os.WriteFile(configPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	servers, _, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}

	session := startFakeServer(t, ctx, []string{"tool_a"})
	defer session.Close()
	srv, err := mcpserver.NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	tracesDir := filepath.Join(latticeDir, "traces")
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Optimization trace: error, so LLM is called.
	optPath := filepath.Join(tracesDir, "opt.json")
	optTrace := `{"timestamp":"2026-05-27T10:00:00Z","code":"tool_a()","tool_calls":[{"tool_name":"tool_a","args":{},"result":"error","is_error":true}],"error":"bad"}`
	if err := os.WriteFile(optPath, []byte(optTrace), 0644); err != nil {
		t.Fatal(err)
	}

	// Hold-out trace: success calling tool_a with search-related code.
	hoPath := filepath.Join(tracesDir, "holdout.json")
	hoTrace := `{"timestamp":"2026-05-27T09:00:00Z","code":"search for documents by query","tool_calls":[{"tool_name":"tool_a","args":{},"result":"ok","is_error":false}],"output":"ok"}`
	if err := os.WriteFile(hoPath, []byte(hoTrace), 0644); err != nil {
		t.Fatal(err)
	}

	// LLM proposes a description that has zero overlap with the hold-out trace.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"descriptions":{"tool_a":"processes unrelated data pipelines"},"relations":[]}`}},
			},
		})
	}))
	defer mockServer.Close()
	old := deepseekEndpoint
	deepseekEndpoint = mockServer.URL
	defer func() { deepseekEndpoint = old }()

	if _, err := optimizeTraces(ctx, "deepseek", "key", configPath, mgr, latticeDir, servers, []string{optPath}, []string{hoPath}); err != nil {
		t.Fatalf("optimizeTraces: %v", err)
	}

	// The regressing description must NOT have been written.
	_, updated, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Descriptions["tool_a"] != "search for documents by query" {
		t.Errorf("hold-out gate failed: description was changed to %q", updated.Descriptions["tool_a"])
	}
}

func TestComputeErrRate(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	ok := write("ok.json", `{"timestamp":"2026-05-27T10:00:00Z","code":"x","tool_calls":[{"tool_name":"t","args":{},"result":"ok","is_error":false}],"output":"ok"}`)
	bad := write("bad.json", `{"timestamp":"2026-05-27T10:00:01Z","code":"x","tool_calls":[{"tool_name":"t","args":{},"result":"err","is_error":true}],"error":"fail"}`)
	junk := write("junk.json", `not json`)

	rate := computeErrRate([]string{ok, bad, junk})
	// junk is skipped; 1 error out of 2 valid traces = 0.5
	if rate != 0.5 {
		t.Errorf("expected 0.5, got %v", rate)
	}
	if computeErrRate(nil) != 0 {
		t.Error("empty files should return 0")
	}
}

func TestRollbackToLatestSnapshot(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mcp.json")
	historyDir := filepath.Join(tmpDir, "history")

	// Write current config with "edited" description.
	current := `{"mcpServers":{"s":{"command":"x"}},"skillGraph":{"descriptions":{"tool_a":"edited description"}}}`
	if err := os.WriteFile(configPath, []byte(current), 0644); err != nil {
		t.Fatal(err)
	}

	// Save a snapshot with the "original" description.
	original := config.SkillGraphConfig{
		Descriptions: map[string]string{"tool_a": "original description"},
	}
	if err := saveSkillGraphSnapshot(historyDir, original); err != nil {
		t.Fatal(err)
	}

	session := startFakeServer(t, ctx, []string{"tool_a"})
	defer session.Close()
	srv, err := mcpserver.NewServerFromSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	// Rollback should restore the original description.
	if err := rollbackToLatestSnapshot(configPath, historyDir, mgr); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	_, updated, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Descriptions["tool_a"] != "original description" {
		t.Errorf("expected original description after rollback, got %q", updated.Descriptions["tool_a"])
	}

	// Graph should also be rebuilt with the restored description.
	if n, ok := mgr.GetGraph().Nodes["tool_a"]; ok {
		if n.Description != "original description" {
			t.Errorf("graph node description not updated: %q", n.Description)
		}
	}
}

func TestRollbackNoSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	err := rollbackToLatestSnapshot(filepath.Join(tmpDir, "mcp.json"), filepath.Join(tmpDir, "history"), nil)
	if err == nil {
		t.Error("expected error when no snapshots exist")
	}
}
