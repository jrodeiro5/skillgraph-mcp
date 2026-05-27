package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// AddServer adds a new server entry to the mcpServers section of the config file at path.
// raw must be a valid JSON object representing a server config block.
// Returns an error if the name already exists in the config.
func AddServer(path, name string, raw json.RawMessage) error {
	var top map[string]json.RawMessage

	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading config: %w", err)
		}
		if err := json.Unmarshal(data, &top); err != nil {
			return fmt.Errorf("parsing config: %w", err)
		}
	}
	if top == nil {
		top = make(map[string]json.RawMessage)
	}

	var servers map[string]json.RawMessage
	if v, ok := top["mcpServers"]; ok {
		if err := json.Unmarshal(v, &servers); err != nil {
			return fmt.Errorf("parsing mcpServers: %w", err)
		}
	}
	if servers == nil {
		servers = make(map[string]json.RawMessage)
	}

	if _, exists := servers[name]; exists {
		return fmt.Errorf("server %q already exists in config", name)
	}
	servers[name] = raw

	serialized, err := json.Marshal(servers)
	if err != nil {
		return fmt.Errorf("marshaling mcpServers: %w", err)
	}
	top["mcpServers"] = serialized

	finalBytes, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, finalBytes, 0644); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}
	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming temp config: %w", err)
	}
	return nil
}

// SaveSkillGraph writes the updated SkillGraphConfig to the config file at path,
// preserving the original mcpServers config and layout.
func SaveSkillGraph(path string, newConfig *SkillGraphConfig) error {
	var raw map[string]json.RawMessage

	// Read existing config if it exists
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading config to save skill graph: %w", err)
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing existing config to save skill graph: %w", err)
		}
	}

	if raw == nil {
		raw = make(map[string]json.RawMessage)
	}

	// Marshal new config
	newGraphBytes, err := json.Marshal(newConfig)
	if err != nil {
		return fmt.Errorf("marshaling new skill graph config: %w", err)
	}
	raw["skillGraph"] = newGraphBytes

	// If mcpServers is missing, initialize it to avoid empty config errors
	if _, ok := raw["mcpServers"]; !ok {
		raw["mcpServers"] = json.RawMessage([]byte("{}"))
	}

	// Write back indented JSON
	finalBytes, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling final config: %w", err)
	}

	// Write to temp file first to ensure atomic write
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, finalBytes, 0644); err != nil {
		return fmt.Errorf("writing temp config: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("renaming temp config to target: %w", err)
	}

	return nil
}
