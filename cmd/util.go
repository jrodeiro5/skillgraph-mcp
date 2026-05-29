package cmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// defaultLatticeDir resolves the default --lattice-dir value: a per-user
// cache path outside any working tree (see ADR-0002). Falls back to the
// CWD-relative ./.mcp_lattice if os.UserCacheDir fails (e.g. $HOME unset).
func defaultLatticeDir() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		slog.Warn("os.UserCacheDir() failed, falling back to CWD-relative lattice dir", "error", err)
		return "./.mcp_lattice"
	}
	return filepath.Join(cache, "skillgraph-mcp", "lattice")
}

// emitJSON writes v to stdout as indented JSON. Used by subcommands that
// support --json output.
func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
		os.Exit(1)
	}
}
