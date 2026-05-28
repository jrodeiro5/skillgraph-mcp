package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/version"

	flag "github.com/spf13/pflag"
)

type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "warn" | "fail"
	Detail string `json:"detail,omitempty"`
}

func runDoctor(args []string) {
	var (
		configPath string
		latticeDir string
		jsonOut    bool
	)
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "./mcp.json", "Path to MCP config file")
	fs.StringVar(&latticeDir, "lattice-dir", "./.mcp_lattice", "Directory for traces and lattice docs")
	fs.BoolVar(&jsonOut, "json", false, "Emit JSON instead of a checklist")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	checks := []checkResult{
		checkVersion(),
		checkConfig(configPath),
		checkLatticeDir(latticeDir),
		checkLLMProvider(),
		checkRuntime(),
	}

	failures := 0
	for _, c := range checks {
		if c.Status == "fail" {
			failures++
		}
	}

	if jsonOut {
		emitJSON(struct {
			Checks   []checkResult `json:"checks"`
			Failures int           `json:"failures"`
		}{checks, failures})
	} else {
		for _, c := range checks {
			fmt.Printf("%s  %s", statusIcon(c.Status), c.Name)
			if c.Detail != "" {
				fmt.Printf("  — %s", c.Detail)
			}
			fmt.Println()
		}
	}

	if failures > 0 {
		os.Exit(1)
	}
}

func statusIcon(s string) string {
	switch s {
	case "ok":
		return "[OK]"
	case "warn":
		return "[WARN]"
	case "fail":
		return "[FAIL]"
	default:
		return "[??]"
	}
}

func checkVersion() checkResult {
	return checkResult{Name: "binary version", Status: "ok", Detail: version.Version}
}

func checkConfig(path string) checkResult {
	if _, err := os.Stat(path); err != nil {
		return checkResult{Name: "config file", Status: "fail", Detail: fmt.Sprintf("not found at %s", path)}
	}
	servers, _, err := config.Load(path)
	if err != nil {
		return checkResult{Name: "config file", Status: "fail", Detail: err.Error()}
	}
	return checkResult{Name: "config file", Status: "ok", Detail: fmt.Sprintf("%s (%d servers)", path, len(servers))}
}

func checkLatticeDir(dir string) checkResult {
	abs, _ := filepath.Abs(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return checkResult{Name: "lattice dir writable", Status: "fail", Detail: err.Error()}
	}
	probe := filepath.Join(dir, ".doctor-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return checkResult{Name: "lattice dir writable", Status: "fail", Detail: err.Error()}
	}
	_ = os.Remove(probe)
	return checkResult{Name: "lattice dir writable", Status: "ok", Detail: abs}
}

func checkLLMProvider() checkResult {
	// Mirror the priority order used by refine.getAPIKey.
	if os.Getenv("LLM_BASE_URL") != "" {
		model := os.Getenv("LLM_MODEL")
		if model == "" {
			model = "(provider default)"
		}
		return checkResult{Name: "LLM provider", Status: "ok", Detail: fmt.Sprintf("LLM_BASE_URL=%s model=%s", os.Getenv("LLM_BASE_URL"), model)}
	}
	for _, key := range []string{"OPENAI_API_KEY", "DEEPSEEK_API_KEY", "GEMINI_API_KEY"} {
		if os.Getenv(key) != "" {
			return checkResult{Name: "LLM provider", Status: "ok", Detail: key + " set"}
		}
	}
	return checkResult{Name: "LLM provider", Status: "warn", Detail: "no LLM env var detected — SkillOpt will be skipped"}
}

func checkRuntime() checkResult {
	// Surface useful runtime info: OS/arch and the presence of npx (common dependency for downstream servers).
	osArch := fmt.Sprintf("%s/%s go%s", runtime.GOOS, runtime.GOARCH, strings.TrimPrefix(runtime.Version(), "go"))
	if _, err := exec.LookPath("npx"); err != nil {
		if _, err2 := exec.LookPath("npx-mcp"); err2 != nil {
			return checkResult{Name: "runtime", Status: "warn", Detail: osArch + " — npx/npx-mcp not in PATH (downstream npm servers will fail)"}
		}
	}
	return checkResult{Name: "runtime", Status: "ok", Detail: osArch}
}
