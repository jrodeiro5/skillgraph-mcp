package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	flag "github.com/spf13/pflag"
)

// probeResult is what generated SKILL.md `!skillgraph-mcp probe X` blocks see.
// Designed for inline injection at skill load time — short enough that the
// agent reads it before deciding whether to call use_skill or repair first.
type probeResult struct {
	Skill         string `json:"skill"`
	Status        string `json:"status"` // "ok" | "fail" | "unknown"
	Tools         int    `json:"tools,omitempty"`
	ConnectedInMs int64  `json:"connected_in_ms,omitempty"`
	Error         string `json:"error,omitempty"`
}

func runProbe(args []string) {
	var (
		configPath string
		jsonOut    bool
		timeoutSec int
	)
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "./mcp.json", "Path to MCP config file")
	fs.BoolVar(&jsonOut, "json", false, "Emit JSON instead of one-line text")
	fs.IntVar(&timeoutSec, "timeout", 10, "Connection timeout in seconds")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "usage: skillgraph-mcp probe [--config PATH] [--json] [--timeout S] <skill-name>")
		os.Exit(2)
	}
	name := rest[0]

	servers, _, err := config.Load(configPath)
	if err != nil {
		emit(probeResult{Skill: name, Status: "fail", Error: "config load: " + err.Error()}, jsonOut)
		os.Exit(1)
	}

	cfg, ok := servers[name]
	if !ok {
		emit(probeResult{Skill: name, Status: "unknown", Error: "skill not declared in " + configPath}, jsonOut)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	start := time.Now()
	srv, err := mcpserver.NewServer(ctx, cfg)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		emit(probeResult{Skill: name, Status: "fail", Error: err.Error(), ConnectedInMs: elapsed}, jsonOut)
		os.Exit(1)
	}
	defer srv.Close()

	emit(probeResult{Skill: name, Status: "ok", Tools: len(srv.Tools()), ConnectedInMs: elapsed}, jsonOut)
}

func emit(r probeResult, jsonOut bool) {
	if jsonOut {
		emitJSON(r)
		return
	}
	switch r.Status {
	case "ok":
		fmt.Printf("%s: connected (%d tools, %dms)\n", r.Skill, r.Tools, r.ConnectedInMs)
	case "unknown":
		fmt.Printf("%s: unknown — %s\n", r.Skill, r.Error)
	default:
		fmt.Printf("%s: FAIL — %s\n", r.Skill, r.Error)
	}
}
