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
	)
	fs := flag.NewFlagSet("generate-skills", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "./mcp.json", "Path to MCP config file")
	fs.StringVar(&outDir, "out", defaultClaudeSkillsDir(), "Directory to write SKILL.md files into")
	fs.StringVar(&binaryPath, "binary", "skillgraph-mcp", "Binary path used in SKILL.md preflight blocks (defaults to PATH lookup)")
	fs.IntVar(&timeoutSec, "timeout", 30, "Connection timeout when --offline=false")
	fs.BoolVar(&offline, "offline", false, "Skip connecting to downstreams; generate from config alone (no tool listing)")
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
	}

	if offline {
		if err := docs.GenerateClaudeSkills(outDir, servers, nil, opts); err != nil {
			fmt.Fprintf(os.Stderr, "generate failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Wrote %d skill files to %s (offline mode — no tool listings).\n", len(servers), outDir)
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

	if err := docs.GenerateClaudeSkills(outDir, servers, mgr.GetGraph(), opts); err != nil {
		fmt.Fprintf(os.Stderr, "generate failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote skill files for %d connected skills to %s.\n", len(mgr.ListServerNames()), outDir)
}

// defaultClaudeSkillsDir resolves ~/.claude/skills (per the Claude Code Skills
// docs — personal skills location). Falls back to the CWD-relative
// .claude/skills if $HOME is unset.
func defaultClaudeSkillsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".claude", "skills")
	}
	return filepath.Join(home, ".claude", "skills")
}
