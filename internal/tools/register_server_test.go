package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAddServerPersistsToConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	// Write minimal initial config.
	initial := `{"mcpServers": {}}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	raw := json.RawMessage(`{"type":"http","url":"http://localhost:9999"}`)
	if err := config.AddServer(cfgPath, "myserver", raw); err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("parse config: %v", err)
	}

	var servers map[string]json.RawMessage
	if err := json.Unmarshal(top["mcpServers"], &servers); err != nil {
		t.Fatalf("parse mcpServers: %v", err)
	}

	if _, ok := servers["myserver"]; !ok {
		t.Error("myserver not found in persisted config")
	}
}

func TestAddServerRejectsDuplicate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")

	initial := `{"mcpServers": {"existing": {"type":"http","url":"http://a"}}}`
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	raw := json.RawMessage(`{"type":"http","url":"http://b"}`)
	if err := config.AddServer(cfgPath, "existing", raw); err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}
}

func TestRegisterServerRefusesOnNonLoopbackHTTP(t *testing.T) {
	t.Parallel()

	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{})
	if err != nil {
		t.Fatalf("NewManagerFromServers: %v", err)
	}
	handler := newRegisterServer(mgr, "", GatewayBinding{Transport: "http", Host: "0.0.0.0"})

	input := registerServerInput{
		Name:   "anything",
		Config: json.RawMessage(`{"type":"http","url":"http://example/"}`),
	}
	res, _, err := handler(context.Background(), nil, input)
	if err != nil {
		t.Fatalf("handler returned err, expected IsError result: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError=true, got %+v", res)
	}
	if len(res.Content) == 0 {
		t.Fatal("expected content with refusal message")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(strings.ToLower(tc.Text), "loopback") {
		t.Fatalf("expected refusal message to mention loopback, got: %s", tc.Text)
	}
}

func TestRegisterServerAllowsLoopbackHTTP(t *testing.T) {
	t.Parallel()

	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{})
	if err != nil {
		t.Fatalf("NewManagerFromServers: %v", err)
	}
	handler := newRegisterServer(mgr, "", GatewayBinding{Transport: "http", Host: "127.0.0.1"})

	// Empty Name triggers a non-gate error path; if the gate fired, we'd
	// get an IsError CallToolResult instead.
	input := registerServerInput{Name: "", Config: json.RawMessage(`{}`)}
	res, _, err := handler(context.Background(), nil, input)
	if err == nil {
		t.Fatalf("expected non-gate validation error, got nil err and res=%+v", res)
	}
	if res != nil && res.IsError {
		t.Fatalf("gate fired on loopback host: %+v", res)
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestRegisterServerTreatsLocalhostAsLoopback(t *testing.T) {
	t.Parallel()

	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{})
	if err != nil {
		t.Fatalf("NewManagerFromServers: %v", err)
	}
	handler := newRegisterServer(mgr, "", GatewayBinding{Transport: "http", Host: "localhost"})

	input := registerServerInput{Name: "", Config: json.RawMessage(`{}`)}
	res, _, err := handler(context.Background(), nil, input)
	if err == nil {
		t.Fatalf("expected non-gate validation error, got nil err and res=%+v", res)
	}
	if res != nil && res.IsError {
		t.Fatalf("gate fired on localhost (expected loopback resolution): %+v", res)
	}
}

func TestRegisterServerAllowsStdioRegardlessOfHost(t *testing.T) {
	t.Parallel()

	mgr, err := mcpserver.NewManagerFromServers(map[string]*mcpserver.Server{})
	if err != nil {
		t.Fatalf("NewManagerFromServers: %v", err)
	}
	// Even with a non-loopback host string, stdio transport must not gate.
	handler := newRegisterServer(mgr, "", GatewayBinding{Transport: "stdio", Host: "0.0.0.0"})

	input := registerServerInput{Name: "", Config: json.RawMessage(`{}`)}
	res, _, err := handler(context.Background(), nil, input)
	if err == nil {
		t.Fatalf("expected validation error from empty name, got nil; res=%+v", res)
	}
	if res != nil && res.IsError {
		t.Fatalf("gate fired under stdio transport: %+v", res)
	}
}

func TestAddServerCreatesConfigIfAbsent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "mcp.json")
	// Do not create the file — AddServer should create it.

	raw := json.RawMessage(`{"type":"http","url":"http://localhost:8080"}`)
	if err := config.AddServer(cfgPath, "newserver", raw); err != nil {
		t.Fatalf("AddServer on missing file: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("parse created config: %v", err)
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(top["mcpServers"], &servers); err != nil {
		t.Fatalf("parse mcpServers: %v", err)
	}
	if _, ok := servers["newserver"]; !ok {
		t.Error("newserver not found in newly created config")
	}
}
