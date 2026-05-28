package cmd

import (
	"fmt"
	"os"

	"github.com/jrodeiro5/skillgraph-mcp/internal/version"
)

const usage = `skillgraph-mcp — MCP gateway with semantic skill graph.

Usage:
  skillgraph-mcp [serve]              Run the gateway (default).
  skillgraph-mcp list-skills          List configured skills and their tools.
  skillgraph-mcp validate             Connect to every downstream server and report failures.
  skillgraph-mcp doctor               Diagnose the local environment.
  skillgraph-mcp --version            Print version and exit.
  skillgraph-mcp --help               Show this help.

Run "skillgraph-mcp <command> --help" for command-specific flags.
`

// Execute is the CLI entry point. It dispatches on the first non-flag argument
// (the subcommand); when none is given, "serve" is used so existing invocations
// remain backwards-compatible.
func Execute() {
	args := os.Args[1:]

	// Top-level flags handled before subcommand dispatch.
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-v":
			fmt.Println(version.Version)
			return
		case "--help", "-h", "help":
			fmt.Print(usage)
			return
		}
	}

	subcommand := "serve"
	rest := args
	if len(args) > 0 && !isFlag(args[0]) {
		subcommand = args[0]
		rest = args[1:]
	}

	switch subcommand {
	case "serve":
		runServe(rest)
	case "list-skills":
		runListSkills(rest)
	case "validate":
		runValidate(rest)
	case "doctor":
		runDoctor(rest)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", subcommand, usage)
		os.Exit(2)
	}
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}
