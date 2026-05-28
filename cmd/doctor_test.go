package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckConfig(t *testing.T) {
	t.Parallel()

	t.Run("missing file", func(t *testing.T) {
		r := checkConfig("/tmp/does-not-exist-skillgraph.json")
		if r.Status != "fail" {
			t.Errorf("status = %q, want 'fail'", r.Status)
		}
	})

	t.Run("valid file", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mcp.json")
		if err := os.WriteFile(p, []byte(`{"mcpServers": {}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		r := checkConfig(p)
		if r.Status != "ok" {
			t.Errorf("status = %q (detail=%s), want 'ok'", r.Status, r.Detail)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "mcp.json")
		if err := os.WriteFile(p, []byte(`not json`), 0o644); err != nil {
			t.Fatal(err)
		}
		r := checkConfig(p)
		if r.Status != "fail" {
			t.Errorf("status = %q, want 'fail'", r.Status)
		}
	})
}

func TestCheckLatticeDir(t *testing.T) {
	t.Parallel()

	t.Run("writable", func(t *testing.T) {
		r := checkLatticeDir(filepath.Join(t.TempDir(), "lattice"))
		if r.Status != "ok" {
			t.Errorf("status = %q, detail=%s", r.Status, r.Detail)
		}
	})
}

func TestCheckLLMProvider(t *testing.T) {
	// Cannot run in parallel: mutates env.
	saved := map[string]string{}
	for _, k := range []string{"LLM_BASE_URL", "LLM_MODEL", "OPENAI_API_KEY", "DEEPSEEK_API_KEY", "GEMINI_API_KEY"} {
		saved[k] = os.Getenv(k)
		_ = os.Unsetenv(k)
	}
	defer func() {
		for k, v := range saved {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
	}()

	if r := checkLLMProvider(); r.Status != "warn" {
		t.Errorf("no env: status = %q, want 'warn'", r.Status)
	}

	_ = os.Setenv("GEMINI_API_KEY", "x")
	if r := checkLLMProvider(); r.Status != "ok" {
		t.Errorf("gemini: status = %q, want 'ok'", r.Status)
	}
	_ = os.Unsetenv("GEMINI_API_KEY")

	_ = os.Setenv("LLM_BASE_URL", "http://localhost:11434/v1")
	r := checkLLMProvider()
	if r.Status != "ok" {
		t.Errorf("LLM_BASE_URL: status = %q, want 'ok'", r.Status)
	}
}
