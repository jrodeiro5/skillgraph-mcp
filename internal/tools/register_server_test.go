package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
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
