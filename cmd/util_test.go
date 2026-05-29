package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultLatticeDir(t *testing.T) {
	got := defaultLatticeDir()
	if got == "" {
		t.Fatal("defaultLatticeDir returned empty string")
	}

	// When UserCacheDir succeeds we expect an absolute path ending in
	// skillgraph-mcp/lattice. Compute the expected value the same way the
	// helper does so the assertion is portable across Linux/macOS/Windows.
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Skipf("os.UserCacheDir() failed in this environment: %v", err)
	}

	want := filepath.Join(cache, "skillgraph-mcp", "lattice")
	if got != want {
		t.Errorf("defaultLatticeDir() = %q, want %q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("defaultLatticeDir() = %q, want absolute path", got)
	}
	suffix := filepath.Join("skillgraph-mcp", "lattice")
	if !strings.HasSuffix(got, suffix) {
		t.Errorf("defaultLatticeDir() = %q, want suffix %q", got, suffix)
	}
}

func TestDefaultLatticeDirFallback(t *testing.T) {
	// os.UserCacheDir only fails on Linux when both XDG_CACHE_HOME and HOME
	// are empty. macOS and Windows have other resolution paths we cannot
	// portably break, so restrict the fallback assertion to Linux.
	if runtime.GOOS != "linux" {
		t.Skipf("fallback only forceable on linux, got %s", runtime.GOOS)
	}
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	got := defaultLatticeDir()
	if got != "./.mcp_lattice" {
		t.Errorf("defaultLatticeDir() fallback = %q, want %q", got, "./.mcp_lattice")
	}
}
