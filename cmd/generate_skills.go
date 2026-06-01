package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/docs"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	flag "github.com/spf13/pflag"
)

func runGenerateSkills(args []string) {
	var (
		configPath string
		outDir     string
		binaryPath string
		timeoutSec int
		offline    bool
		force      bool
	)
	fs := flag.NewFlagSet("generate-skills", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "./mcp.json", "Path to MCP config file")
	fs.StringVar(&outDir, "out", defaultClaudeSkillsDir(), "Directory to write SKILL.md files into")
	fs.StringVar(&binaryPath, "binary", "skillgraph-mcp", "Binary path used in SKILL.md preflight blocks (defaults to PATH lookup)")
	fs.IntVar(&timeoutSec, "timeout", 30, "Connection timeout when --offline=false")
	fs.BoolVar(&offline, "offline", false, "Skip connecting to downstreams; generate from config alone (no tool listing)")
	fs.BoolVar(&force, "force", false, "Overwrite existing SKILL.md files (default: skip)")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	servers, graphCfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	absConfig, err := filepath.Abs(configPath)
	if err != nil {
		absConfig = configPath
	}

	opts := docs.ClaudeSkillsOptions{
		BinaryPath: binaryPath,
		ConfigPath: absConfig,
		Force:      force,
	}

	if offline {
		res, err := docs.GenerateClaudeSkills(outDir, servers, nil, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate failed: %v\n", err)
			os.Exit(1)
		}
		reportGenerateResult(res, outDir, true)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	mgr, err := mcpserver.NewManager(ctx, servers, graphCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to servers: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	res, err := docs.GenerateClaudeSkills(outDir, servers, mgr.GetGraph(), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate failed: %v\n", err)
		os.Exit(1)
	}
	reportGenerateResult(res, outDir, false)
}

func reportGenerateResult(res docs.GenerateResult, outDir string, offline bool) {
	tag := ""
	if offline {
		tag = " (offline mode — no tool listings)"
	}
	fmt.Printf("Wrote %d skill files to %s%s.\n", len(res.Written), outDir, tag)
	if len(res.Skipped) > 0 {
		fmt.Printf("Skipped %d (use --force to overwrite):\n", len(res.Skipped))
		for _, s := range res.Skipped {
			fmt.Printf("  - %s\n", s)
		}
	}
}

// defaultClaudeSkillsDir resolves .claude/skills relative to the CWD — the
// recommended per-project location for Claude Code skills. Users who maintain
// a centralized skills hub can opt in with `--out ~/.claude/skills` explicitly.
func defaultClaudeSkillsDir() string {
	return filepath.Join(".claude", "skills")
}
