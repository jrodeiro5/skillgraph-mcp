package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/jrodeiro5/skillgraph-mcp/internal/trace"

	monty "github.com/ewhauser/gomonty"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type executeCodeInput struct {
	Code       string   `json:"code" jsonschema:"python code that calls downstream tools by name and returns a computed result"`
	SkillNames []string `json:"skill_names,omitempty" jsonschema:"optional: limit available tools to these skill names (reduces sandbox size for large deployments)"`
}

const executeCodeDescription = `Execute Python code in an isolated sandbox. This is the INDIRECTION POINT for all downstream skill tools — they are NOT available as MCP tools, only as Python functions here.

HOW IT WORKS:
  1. use_skill(skill_name) → returns Python function signatures
  2. execute_code("""result = function_name(args)""") → calls them

EXAMPLE WORKFLOW (gitnexus):
  (a) list_skills() → see "gitnexus: Code intelligence"
  (b) use_skill("gitnexus") → see "gitnexus_impact(target: str, direction: str) -> str"
  (c) execute_code("""result = gitnexus_impact(target="authenticate", direction="up")""")

GOTCHA: You cannot call gitnexus_impact, firecrawl_scrape, brave_web_search, or any
other downstream tool as an MCP tool directly. They do not exist as MCP tools.
They only exist as Python functions inside this sandbox.`

type contextKey string

const traceCollectorKey contextKey = "traceCollector"

type TraceCollector struct {
	mu        sync.Mutex
	ToolCalls []trace.ToolCallTrace
}

func (c *TraceCollector) Add(call trace.ToolCallTrace) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ToolCalls = append(c.ToolCalls, call)
}

// maxTraceFieldBytes caps any single Output/Error/ToolCall.Result string so a
// runaway tool dump can't poison the SkillOpt batch with a multi-hundred-KB
// trace. Truncation marker is appended; downstream LLM still sees the shape.
const maxTraceFieldBytes = 50000

func truncateTraceField(t *trace.Trajectory) {
	trunc := func(s string) string {
		if len(s) <= maxTraceFieldBytes {
			return s
		}
		return s[:maxTraceFieldBytes] + fmt.Sprintf("\n...[truncated %d bytes]", len(s)-maxTraceFieldBytes)
	}
	t.Output = trunc(t.Output)
	t.Error = trunc(t.Error)
	for i := range t.ToolCalls {
		t.ToolCalls[i].Result = trunc(t.ToolCalls[i].Result)
	}
}

func RegisterExecuteCode(s *mcp.Server, mgr *mcpserver.Manager, latticeDir string) {
	fn, _ := newExecuteCode(mgr, latticeDir)
	mcp.AddTool(
		s,
		&mcp.Tool{
			Name:        "execute_code",
			Description: executeCodeDescription,
		},
		fn,
	)
}

func newExecuteCode(mgr *mcpserver.Manager, latticeDir string) (func(context.Context, *mcp.CallToolRequest, executeCodeInput) (*mcp.CallToolResult, any, error), error) {
	fn := func(ctx context.Context, req *mcp.CallToolRequest, input executeCodeInput) (*mcp.CallToolResult, any, error) {
		if input.Code == "" {
			result := &mcp.CallToolResult{}
			result.SetError(errors.New("code must not be empty"))
			return result, nil, nil
		}

		runner, err := monty.New(input.Code, monty.CompileOptions{ScriptName: "script.py"})
		if err != nil {
			result := &mcp.CallToolResult{}
			result.SetError(fmt.Errorf("compile error: %w", err))
			return result, nil, nil
		}

		tools := mgr.AllTools()
		if len(input.SkillNames) > 0 {
			allowed := make(map[string]bool, len(input.SkillNames))
			for _, s := range input.SkillNames {
				allowed[s] = true
			}
			filtered := tools[:0]
			for _, t := range tools {
				if allowed[t.ServerName] {
					filtered = append(filtered, t)
				}
			}
			tools = filtered
		}
		fns := make(map[string]monty.ExternalFunction, len(tools))
		for _, t := range tools {
			srv, err := mgr.GetServer(t.ServerName)
			if err != nil {
				continue
			}
			fns[t.ResolvedName] = buildTool(t, srv)
		}

		collector := &TraceCollector{}
		ctxWithTrace := context.WithValue(ctx, traceCollectorKey, collector)

		var stdoutBuf bytes.Buffer
		value, runErr := runner.Run(ctxWithTrace, monty.RunOptions{
			Functions: fns,
			Print:     monty.WriterPrintCallback(&stdoutBuf),
		})

		// Fall back to captured stdout when code uses print() instead of return.
		if value == monty.None() && stdoutBuf.Len() > 0 {
			value = monty.String(stdoutBuf.String())
		}

		traj := trace.Trajectory{
			Timestamp: time.Now(),
			Code:      input.Code,
			ToolCalls: collector.ToolCalls,
		}
		if runErr != nil {
			traj.Error = runErr.Error()
		} else {
			traj.Output = value.String()
		}

		// Save trace synchronously. Writing a few KB of JSON to local disk is
		// sub-millisecond on any sane FS; the previous fire-and-forget goroutine
		// raced with t.TempDir() cleanup in tests and silently dropped traces
		// when the process exited mid-write (also a real risk on SIGTERM).
		truncateTraceField(&traj)
		tracesDir := filepath.Join(latticeDir, "traces")
		if err := os.MkdirAll(tracesDir, 0755); err == nil {
			if data, jerr := json.MarshalIndent(traj, "", "  "); jerr == nil {
				filename := fmt.Sprintf("%d_%d.json", time.Now().UnixNano(), rand.Intn(100000))
				_ = os.WriteFile(filepath.Join(tracesDir, filename), data, 0644)
			}
		}

		if runErr != nil {
			result := &mcp.CallToolResult{}
			if stdoutBuf.Len() > 0 {
				result.SetError(fmt.Errorf("runtime error: %w\npartial output:\n%s", runErr, stdoutBuf.String()))
			} else {
				result.SetError(fmt.Errorf("runtime error: %w", runErr))
			}
			return result, nil, nil
		}

		text := montyValueToText(value)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	}
	return fn, nil
}

// montyValueToText serializes a Monty value to a string for tool output.
// Dicts and lists are JSON-marshaled; all other types use .String().
func montyValueToText(v monty.Value) string {
	switch v.Kind() {
	case "dict", "list", "tuple":
		goVal := montyValueToAny(v)
		data, err := json.Marshal(goVal)
		if err != nil {
			return v.String()
		}
		return string(data)
	default:
		return v.String()
	}
}

// buildTool creates a Monty external function for a single downstream tool.
func buildTool(t mcpserver.Tool, srv *mcpserver.Server) monty.ExternalFunction {
	paramByName := make(map[string]mcpserver.ParamInfo, len(t.Params))
	for _, p := range t.Params {
		paramByName[p.Name] = p
	}

	return func(fnCtx context.Context, call monty.Call) (monty.Result, error) {
		args := make(map[string]any)

		// Map positional args to parameter names from the schema, with type validation.
		for i, val := range call.Args {
			if i < len(t.Params) {
				if err := validateMontyValue(val, t.Params[i]); err != nil {
					msg := err.Error()
					return monty.Raise(monty.Exception{Type: "TypeError", Arg: &msg}), nil
				}
				args[t.Params[i].Name] = montyValueToAny(val)
			}
		}

		// Keyword args override positional, with type validation.
		for _, pair := range call.Kwargs {
			key, ok := pair.Key.Raw().(string)
			if !ok {
				continue
			}
			if pi, ok := paramByName[key]; ok {
				if err := validateMontyValue(pair.Value, pi); err != nil {
					msg := err.Error()
					return monty.Raise(monty.Exception{Type: "TypeError", Arg: &msg}), nil
				}
			}
			args[key] = montyValueToAny(pair.Value)
		}

		var tc trace.ToolCallTrace
		tc.ToolName = t.ResolvedName
		tc.Args = args

		toolResult, err := srv.CallTool(fnCtx, &mcp.CallToolParams{
			Name:      t.OriginalName,
			Arguments: args,
		})
		if err != nil {
			tc.IsError = true
			tc.Result = fmt.Sprintf("error: %v", err)
			if collector, ok := fnCtx.Value(traceCollectorKey).(*TraceCollector); ok {
				collector.Add(tc)
			}
			return monty.Return(monty.String(tc.Result)), nil
		}

		if toolResult.IsError {
			tc.IsError = true
			tc.Result = fmt.Sprintf("error: %s", extractText(toolResult))
			if collector, ok := fnCtx.Value(traceCollectorKey).(*TraceCollector); ok {
				collector.Add(tc)
			}
			return monty.Return(monty.String(tc.Result)), nil
		}

		resVal := extractResult(toolResult)
		tc.Result = resVal.String()
		if collector, ok := fnCtx.Value(traceCollectorKey).(*TraceCollector); ok {
			collector.Add(tc)
		}
		return monty.Return(resVal), nil
	}
}

// validateMontyValue checks that a Monty value matches the expected JSON Schema
// types for a parameter. Returns nil if validation passes or types are unknown.
func validateMontyValue(v monty.Value, param mcpserver.ParamInfo) error {
	types := extractSchemaTypes(param.Schema)
	if len(types) == 0 {
		return nil
	}
	kind := v.Kind()
	for _, t := range types {
		if jsonSchemaTypeMatchesMonty(t, kind) {
			return nil
		}
	}
	return fmt.Errorf("parameter %q: expected %s, got %s", param.Name, types[0], kind)
}

// extractSchemaTypes extracts the top-level type(s) from a JSON Schema property.
func extractSchemaTypes(schema any) []string {
	m, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	switch t := m["type"].(type) {
	case string:
		return []string{t}
	case []any:
		types := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				types = append(types, s)
			}
		}
		return types
	default:
		return nil
	}
}

// jsonSchemaTypeMatchesMonty checks if a JSON Schema type is compatible with a Monty ValueKind.
func jsonSchemaTypeMatchesMonty(schemaType string, kind monty.ValueKind) bool {
	switch schemaType {
	case "string":
		return kind == "string"
	case "integer":
		return kind == "int" || kind == "big_int"
	case "number":
		return kind == "int" || kind == "big_int" || kind == "float"
	case "boolean":
		return kind == "bool"
	case "array":
		return kind == "list" || kind == "tuple"
	case "object":
		return kind == "dict"
	case "null":
		return kind == "none"
	default:
		return false
	}
}

// montyValueToAny converts a Monty Value to a Go value suitable for JSON tool arguments.
func montyValueToAny(v monty.Value) any {
	raw := v.Raw()
	switch val := raw.(type) {
	case int64:
		return val
	case float64:
		return val
	case string:
		return val
	case bool:
		return val
	case monty.Dict:
		m := make(map[string]any, len(val))
		for _, pair := range val {
			if key, ok := pair.Key.Raw().(string); ok {
				m[key] = montyValueToAny(pair.Value)
			}
		}
		return m
	case []monty.Value:
		list := make([]any, len(val))
		for i, item := range val {
			list[i] = montyValueToAny(item)
		}
		return list
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return v.String()
		}
		return json.RawMessage(data)
	}
}

// anyToMonty converts a Go value (from JSON unmarshaling) to a Monty Value.
func anyToMonty(v any) monty.Value {
	switch val := v.(type) {
	case string:
		return monty.String(val)
	case float64:
		if val == float64(int64(val)) {
			return monty.Int(int64(val))
		}
		return monty.Float(val)
	case bool:
		return monty.Bool(val)
	case nil:
		return monty.None()
	case map[string]any:
		pairs := make(monty.Dict, 0, len(val))
		for k, v := range val {
			pairs = append(pairs, monty.Pair{Key: monty.String(k), Value: anyToMonty(v)})
		}
		return monty.DictValue(pairs)
	case []any:
		items := make([]monty.Value, len(val))
		for i, item := range val {
			items[i] = anyToMonty(item)
		}
		return monty.List(items...)
	default:
		return monty.String(fmt.Sprintf("%v", val))
	}
}

// extractText pulls the first text content from a CallToolResult.
func extractText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// extractResult converts a CallToolResult to a Monty value.
// Prefers structured content when available; otherwise extracts first text content.
func extractResult(result *mcp.CallToolResult) monty.Value {
	if result.StructuredContent != nil {
		return anyToMonty(result.StructuredContent)
	}
	return monty.String(extractText(result))
}
