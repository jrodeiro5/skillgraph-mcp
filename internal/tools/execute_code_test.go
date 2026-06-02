package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/jrodeiro5/skillgraph-mcp/internal/trace"

	monty "github.com/ewhauser/gomonty"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestExecuteCodeDescriptionRefersToUseSkill(t *testing.T) {
	t.Parallel()
	if !strings.Contains(executeCodeDescription, "use_skill") {
		t.Error("description should refer to use_skill for tool discovery")
	}
	if !strings.Contains(executeCodeDescription, "execute_code") {
		t.Error("description should mention execute_code")
	}
	if !strings.Contains(executeCodeDescription, "GOTCHA") {
		t.Error("description should mention GOTCHA")
	}
}

func TestExecuteCodeBasicMath(t *testing.T) {
	t.Parallel()
	runner, err := monty.New("40 + 2", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(t.Context(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "42" {
		t.Errorf("result = %q, want '42'", value.String())
	}
}

func TestExecuteCodeStringExpression(t *testing.T) {
	t.Parallel()
	runner, err := monty.New("'hello' + ' ' + 'world'", monty.CompileOptions{ScriptName: "script.py"})
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	value, err := runner.Run(t.Context(), monty.RunOptions{})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if value.String() != "hello world" {
		t.Errorf("result = %q, want 'hello world'", value.String())
	}
}

func TestExecuteCodeSyntaxError(t *testing.T) {
	t.Parallel()
	_, err := monty.New("def (invalid syntax", monty.CompileOptions{ScriptName: "script.py"})
	if err == nil {
		t.Fatal("expected compile error for invalid syntax")
	}
}

// --- validateMontyValue tests ---

func TestValidateMontyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   monty.Value
		param   mcpserver.ParamInfo
		wantErr bool
	}{
		{
			name:  "string_match",
			value: monty.String("hello"),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "string"}},
		},
		{
			name:  "integer_match",
			value: monty.Int(42),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "integer"}},
		},
		{
			name:  "number_accepts_int",
			value: monty.Int(42),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "number"}},
		},
		{
			name:  "number_accepts_float",
			value: monty.Float(3.14),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "number"}},
		},
		{
			name:  "boolean_match",
			value: monty.Bool(true),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "boolean"}},
		},
		{
			name:  "array_match",
			value: monty.List(monty.Int(1)),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "array"}},
		},
		{
			name:  "object_match",
			value: monty.DictValue(monty.Dict{{Key: monty.String("k"), Value: monty.String("v")}}),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": "object"}},
		},
		{
			name:    "type_mismatch",
			value:   monty.Int(42),
			param:   mcpserver.ParamInfo{Name: "sql", Schema: map[string]any{"type": "string"}},
			wantErr: true,
		},
		{
			name:  "nullable_none",
			value: monty.None(),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": []any{"string", "null"}}},
		},
		{
			name:  "nullable_string",
			value: monty.String("hi"),
			param: mcpserver.ParamInfo{Name: "x", Schema: map[string]any{"type": []any{"string", "null"}}},
		},
		{
			name:  "no_schema",
			value: monty.Int(42),
			param: mcpserver.ParamInfo{Name: "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateMontyValue(tt.value, tt.param)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateMontyValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMontyValueErrorMentionsParam(t *testing.T) {
	t.Parallel()
	err := validateMontyValue(monty.Int(42), mcpserver.ParamInfo{Name: "sql", Schema: map[string]any{"type": "string"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sql") {
		t.Errorf("error should mention parameter name, got: %v", err)
	}
}

// --- Integration: type validation via Monty runner ---

func TestExecuteCodeTypeValidation(t *testing.T) {
	t.Parallel()

	// Build tool functions from a manager with a typed tool.
	ctx := t.Context()
	ds := mcp.NewServer(&mcp.Implementation{Name: "typed"}, nil)
	type GreetInput struct {
		Name string `json:"name" jsonschema:"the name to greet"`
	}
	mcp.AddTool(
		ds,
		&mcp.Tool{Name: "greet", Description: "Greet someone"},
		func(ctx context.Context, req *mcp.CallToolRequest, input GreetInput) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "hello " + input.Name}},
			}, nil, nil
		},
	)

	dsServerT, dsClientT := mcp.NewInMemoryTransports()
	go func() { _ = ds.Run(ctx, dsServerT) }()
	dsClient := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	dsSession, err := dsClient.Connect(ctx, dsClientT, nil)
	if err != nil {
		t.Fatal(err)
	}

	srv, err := mcpserver.NewServerFromSession(ctx, dsSession)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"s": srv})
	if err != nil {
		t.Fatal(err)
	}

	tools := mgr.AllTools()
	fns := make(map[string]monty.ExternalFunction, len(tools))
	for _, tool := range tools {
		srv, err := mgr.GetServer(tool.ServerName)
		if err != nil {
			t.Fatal(err)
		}
		fns[tool.ResolvedName] = buildTool(tool, srv)
	}

	t.Run("valid_string_arg", func(t *testing.T) {
		t.Parallel()
		runner, err := monty.New(`greet("world")`, monty.CompileOptions{ScriptName: "test.py"})
		if err != nil {
			t.Fatal(err)
		}
		value, err := runner.Run(t.Context(), monty.RunOptions{Functions: fns})
		if err != nil {
			t.Fatalf("runtime error: %v", err)
		}
		if value.String() != "hello world" {
			t.Errorf("result = %q, want 'hello world'", value.String())
		}
	})

	t.Run("invalid_int_for_string", func(t *testing.T) {
		t.Parallel()
		runner, err := monty.New(`greet(42)`, monty.CompileOptions{ScriptName: "test.py"})
		if err != nil {
			t.Fatal(err)
		}
		_, err = runner.Run(t.Context(), monty.RunOptions{Functions: fns})
		if err == nil {
			t.Fatal("expected runtime error for type mismatch")
		}
		if !strings.Contains(err.Error(), "TypeError") {
			t.Errorf("expected TypeError, got: %v", err)
		}
	})
}

// --- extractResult tests ---

func TestExtractResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   *mcp.CallToolResult
		wantKind monty.ValueKind
	}{
		{
			name:     "text_content",
			result:   &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}},
			wantKind: "string",
		},
		{
			name: "structured_content_preferred",
			result: &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: `{"temp": 22}`}},
				StructuredContent: map[string]any{"temp": float64(22)},
			},
			wantKind: "dict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractResult(tt.result)
			if got.Kind() != tt.wantKind {
				t.Errorf("Kind() = %s, want %s", got.Kind(), tt.wantKind)
			}
		})
	}
}

// --- anyToMonty tests ---

func TestAnyToMonty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    any
		wantKind monty.ValueKind
	}{
		{
			name:     "string",
			input:    "hello",
			wantKind: "string",
		},
		{
			name:     "float",
			input:    3.14,
			wantKind: "float",
		},
		{
			name:     "int_from_float",
			input:    float64(42),
			wantKind: "int",
		},
		{
			name:     "bool",
			input:    true,
			wantKind: "bool",
		},
		{
			name:     "nil",
			input:    nil,
			wantKind: "none",
		},
		{
			name:     "map",
			input:    map[string]any{"key": "val"},
			wantKind: "dict",
		},
		{
			name:     "slice",
			input:    []any{"a", "b"},
			wantKind: "list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := anyToMonty(tt.input)
			if got.Kind() != tt.wantKind {
				t.Errorf("Kind() = %s, want %s", got.Kind(), tt.wantKind)
			}
		})
	}
}

// TestExecuteCodeReturnDict verifies that returning a dict from execute_code
// produces valid JSON, not just the string "dict".
func TestExecuteCodeReturnDict(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	ds := mcp.NewServer(&mcp.Implementation{Name: "dict-server"}, nil)
	type EmptyInput struct{}
	mcp.AddTool(
		ds,
		&mcp.Tool{Name: "get_user", Description: "Get a user"},
		func(ctx context.Context, req *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{}, map[string]any{"name": "Alice", "age": float64(30)}, nil
		},
	)

	mgr, err := mcpserver.NewManagerFromMCPServers(ctx, map[string]*mcp.Server{"dict-server": ds})
	if err != nil {
		t.Fatalf("NewManagerFromServers: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })

	executeCode, err := newExecuteCode(mgr, t.TempDir())
	if err != nil {
		t.Fatalf("newExecuteCode: %v", err)
	}

	req := &mcp.CallToolRequest{}
	res, _, err := executeCode(ctx, req, executeCodeInput{Code: "result = get_user()\nreturn result"})
	if err != nil {
		t.Fatalf("execute_code error: %v", err)
	}

	got := res.Content[0].(*mcp.TextContent).Text
	if got == "dict" {
		t.Fatal("execute_code returned \"dict\" instead of JSON — dict serialization broken")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v (got: %q)", err, got)
	}
	if parsed["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", parsed["name"])
	}
}

// TestExecuteCodeReturnList verifies that returning a list from execute_code
// produces valid JSON array, not just the string "list".
func TestExecuteCodeReturnList(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	ds := mcp.NewServer(&mcp.Implementation{Name: "list-server"}, nil)
	type EmptyInput struct{}
	mcp.AddTool(
		ds,
		&mcp.Tool{Name: "list_items", Description: "List items"},
		func(ctx context.Context, req *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{}, []any{"alpha", "beta", "gamma"}, nil
		},
	)

	mgr, err := mcpserver.NewManagerFromMCPServers(ctx, map[string]*mcp.Server{"list-server": ds})
	if err != nil {
		t.Fatalf("NewManagerFromServers: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })

	executeCode, err := newExecuteCode(mgr, t.TempDir())
	if err != nil {
		t.Fatalf("newExecuteCode: %v", err)
	}

	req := &mcp.CallToolRequest{}
	res, _, err := executeCode(ctx, req, executeCodeInput{Code: "items = list_items()\nreturn items"})
	if err != nil {
		t.Fatalf("execute_code error: %v", err)
	}

	got := res.Content[0].(*mcp.TextContent).Text
	if got == "list" {
		t.Fatal("execute_code returned \"list\" instead of JSON — list serialization broken")
	}
	var parsed []any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not valid JSON array: %v (got: %q)", err, got)
	}
	if len(parsed) != 3 || parsed[0] != "alpha" {
		t.Errorf("items = %v, want [alpha beta gamma]", parsed)
	}
}

// TestExecuteCodeReturnListOfDicts verifies nested list-of-dicts serialization
// (the pattern returned by tools like brave_web_search).
func TestExecuteCodeReturnListOfDicts(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	ds := mcp.NewServer(&mcp.Implementation{Name: "results-server"}, nil)
	type EmptyInput struct{}
	mcp.AddTool(
		ds,
		&mcp.Tool{Name: "search", Description: "Search results"},
		func(ctx context.Context, req *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{}, []any{
				map[string]any{"url": "https://example.com", "title": "Example"},
				map[string]any{"url": "https://other.com", "title": "Other"},
			}, nil
		},
	)

	mgr, err := mcpserver.NewManagerFromMCPServers(ctx, map[string]*mcp.Server{"results-server": ds})
	if err != nil {
		t.Fatalf("NewManagerFromServers: %v", err)
	}
	t.Cleanup(func() { mgr.Close() })

	executeCode, err := newExecuteCode(mgr, t.TempDir())
	if err != nil {
		t.Fatalf("newExecuteCode: %v", err)
	}

	req := &mcp.CallToolRequest{}
	res, _, err := executeCode(ctx, req, executeCodeInput{Code: "results = search()\nreturn results"})
	if err != nil {
		t.Fatalf("execute_code error: %v", err)
	}

	got := res.Content[0].(*mcp.TextContent).Text
	var parsed []map[string]any
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("result is not valid JSON: %v (got: %q)", err, got)
	}
	if len(parsed) != 2 || parsed[0]["url"] != "https://example.com" {
		t.Errorf("unexpected results: %v", parsed)
	}
}

func TestExecuteCodeTrajectoryLogging(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	ds := mcp.NewServer(&mcp.Implementation{Name: "logging-server"}, nil)
	type GreetInput struct {
		Name string `json:"name"`
	}
	mcp.AddTool(
		ds,
		&mcp.Tool{Name: "greet", Description: "Greet someone"},
		func(ctx context.Context, req *mcp.CallToolRequest, input GreetInput) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "hello " + input.Name}},
			}, nil, nil
		},
	)

	dsServerT, dsClientT := mcp.NewInMemoryTransports()
	go func() { _ = ds.Run(ctx, dsServerT) }()
	dsClient := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	dsSession, err := dsClient.Connect(ctx, dsClientT, nil)
	if err != nil {
		t.Fatal(err)
	}

	srv, err := mcpserver.NewServerFromSession(ctx, dsSession)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{"logging-server": srv})
	if err != nil {
		t.Fatal(err)
	}

	// Clean/prepare traces directory
	latticeDir := t.TempDir()
	tracesDir := filepath.Join(latticeDir, "traces")
	_ = os.RemoveAll(tracesDir)

	fn, err := newExecuteCode(mgr, latticeDir)
	if err != nil {
		t.Fatal(err)
	}
	req := &mcp.CallToolRequest{}
	input := executeCodeInput{
		Code: `greet("world")`,
	}

	res, _, err := fn(ctx, req, input)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("executeCode returned error: %s", extractText(res))
	}

	// Wait up to 2 seconds for background logging goroutine to write the trace file
	var files []string
	for i := 0; i < 20; i++ {
		files, _ = filepath.Glob(filepath.Join(tracesDir, "*.json"))
		if len(files) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if len(files) == 0 {
		t.Fatal("expected trajectory trace file to be created, but found none")
	}

	// Read and verify trace file content
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}

	var traj trace.Trajectory
	if err := json.Unmarshal(data, &traj); err != nil {
		t.Fatalf("failed to unmarshal trajectory: %v", err)
	}

	if traj.Code != `greet("world")` {
		t.Errorf("got code = %q, want 'greet(\"world\")'", traj.Code)
	}
	if len(traj.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(traj.ToolCalls))
	}
	if traj.ToolCalls[0].ToolName != "greet" {
		t.Errorf("got tool name = %q, want 'greet'", traj.ToolCalls[0].ToolName)
	}
	if traj.ToolCalls[0].Args["name"] != "world" {
		t.Errorf("got tool args name = %v, want 'world'", traj.ToolCalls[0].Args["name"])
	}
	if traj.ToolCalls[0].Result != "hello world" {
		t.Errorf("got tool call result = %q, want 'hello world'", traj.ToolCalls[0].Result)
	}
	if traj.Output != "hello world" {
		t.Errorf("got traj output = %q, want 'hello world'", traj.Output)
	}
	if traj.Error != "" {
		t.Errorf("expected no error, got %q", traj.Error)
	}
}
