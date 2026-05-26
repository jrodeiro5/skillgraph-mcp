package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveSkillGraph(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config-save-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "mcp.json")

	// 1. Test saving to empty/new file
	cfg := &SkillGraphConfig{
		Descriptions: map[string]string{
			"test-server": "A test server description",
		},
		Relations: []StaticRelation{
			{
				Source: "toolA",
				Target: "toolB",
				Type:   "PREREQUISITE_FOR",
			},
		},
	}

	if err := SaveSkillGraph(configPath, cfg); err != nil {
		t.Fatalf("failed to save skill graph: %v", err)
	}

	servers, loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}

	if len(servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(servers))
	}

	if loadedCfg.Descriptions["test-server"] != "A test server description" {
		t.Errorf("expected description 'A test server description', got '%s'", loadedCfg.Descriptions["test-server"])
	}

	// 2. Test saving while preserving mcpServers with environment variables
	os.Setenv("TEST_ENV_VAR", "expanded-value")
	defer os.Unsetenv("TEST_ENV_VAR")

	initialJSON := `{
  "mcpServers": {
    "my-server": {
      "command": "node",
      "args": ["${TEST_ENV_VAR}", "arg2"]
    }
  }
}`

	if err := os.WriteFile(configPath, []byte(initialJSON), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Save new skill graph
	newCfg := &SkillGraphConfig{
		Descriptions: map[string]string{
			"my-server": "refined desc",
		},
		Relations: []StaticRelation{
			{Source: "A", Target: "B", Type: "PRODUCES"},
		},
	}

	if err := SaveSkillGraph(configPath, newCfg); err != nil {
		t.Fatalf("failed to save updated skill graph: %v", err)
	}

	// Read raw file content to verify ${TEST_ENV_VAR} was NOT expanded in the file
	rawBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read raw config: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rawBytes, &raw); err != nil {
		t.Fatalf("failed to parse raw config: %v", err)
	}

	var srvs map[string]any
	if err := json.Unmarshal(raw["mcpServers"], &srvs); err != nil {
		t.Fatalf("failed to parse mcpServers: %v", err)
	}

	mySrv, ok := srvs["my-server"].(map[string]any)
	if !ok {
		t.Fatalf("my-server config is missing or invalid")
	}

	args, ok := mySrv["args"].([]any)
	if !ok || len(args) != 2 {
		t.Fatalf("my-server args are invalid or missing")
	}

	if args[0] != "${TEST_ENV_VAR}" {
		t.Errorf("expected raw string '${TEST_ENV_VAR}', got %v (expansion occurred in file!)", args[0])
	}

	// 3. Test Load on the saved file (expansion should occur now)
	loadedServers, loadedConfig, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to Load updated config: %v", err)
	}

	srv, ok := loadedServers["my-server"].(*StdioServer)
	if !ok {
		t.Fatalf("expected my-server to be a StdioServer")
	}

	if srv.Args[0] != "expanded-value" {
		t.Errorf("expected expanded arg 'expanded-value', got '%s'", srv.Args[0])
	}

	if loadedConfig.Descriptions["my-server"] != "refined desc" {
		t.Errorf("expected description 'refined desc', got '%s'", loadedConfig.Descriptions["my-server"])
	}
}
