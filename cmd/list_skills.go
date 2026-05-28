package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	flag "github.com/spf13/pflag"
)

func runListSkills(args []string) {
	var (
		configPath string
		jsonOut    bool
		timeoutSec int
	)
	fs := flag.NewFlagSet("list-skills", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "./mcp.json", "Path to MCP config file")
	fs.BoolVar(&jsonOut, "json", false, "Emit JSON instead of a table")
	fs.IntVar(&timeoutSec, "timeout", 30, "Connection timeout in seconds")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	servers, graphCfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	mgr, err := mcpserver.NewManager(ctx, servers, graphCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to servers: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	type skillRow struct {
		Name        string `json:"name"`
		ToolCount   int    `json:"tool_count"`
		Description string `json:"description"`
	}

	names := mgr.ListServerNames()
	sort.Strings(names)
	rows := make([]skillRow, 0, len(names))
	for _, name := range names {
		desc := ""
		if graphCfg != nil && graphCfg.Descriptions != nil {
			desc = graphCfg.Descriptions[name]
		}
		if desc == "" {
			if srv, ok := servers[name]; ok {
				desc = srv.Options().Description
			}
		}
		rows = append(rows, skillRow{
			Name:        name,
			ToolCount:   len(mgr.ServerTools(name)),
			Description: desc,
		})
	}

	if jsonOut {
		emitJSON(rows)
		return
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SKILL\tTOOLS\tDESCRIPTION")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%d\t%s\n", r.Name, r.ToolCount, truncate(r.Description, 80))
	}
	_ = tw.Flush()
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
