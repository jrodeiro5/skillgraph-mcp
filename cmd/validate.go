package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"

	flag "github.com/spf13/pflag"
)

type validateResult struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	ToolCount int    `json:"tool_count,omitempty"`
	Error     string `json:"error,omitempty"`
}

func runValidate(args []string) {
	var (
		configPath string
		jsonOut    bool
		timeoutSec int
	)
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fs.StringVar(&configPath, "config", "./mcp.json", "Path to MCP config file")
	fs.BoolVar(&jsonOut, "json", false, "Emit JSON instead of a table")
	fs.IntVar(&timeoutSec, "timeout", 20, "Per-server connection timeout in seconds")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	servers, _, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Connect to every server in parallel and collect per-server results.
	results := make([]validateResult, 0, len(servers))
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	for name, srvCfg := range servers {
		wg.Add(1)
		go func(name string, cfg config.Server) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
			defer cancel()

			r := validateResult{Name: name}
			srv, err := mcpserver.NewServer(ctx, cfg)
			if err != nil {
				r.Status = "fail"
				r.Error = err.Error()
			} else {
				r.Status = "ok"
				r.ToolCount = len(srv.Tools())
				_ = srv.Close()
			}

			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(name, srvCfg)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })

	failures := 0
	for _, r := range results {
		if r.Status != "ok" {
			failures++
		}
	}

	if jsonOut {
		emitJSON(struct {
			Servers  []validateResult `json:"servers"`
			Failures int              `json:"failures"`
		}{results, failures})
	} else {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "SERVER\tSTATUS\tTOOLS\tDETAIL")
		for _, r := range results {
			detail := r.Error
			if r.Status == "ok" {
				detail = ""
			}
			fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.Name, r.Status, r.ToolCount, truncate(detail, 80))
		}
		_ = tw.Flush()
		fmt.Printf("\n%d/%d servers OK\n", len(results)-failures, len(results))
	}

	if failures > 0 {
		os.Exit(1)
	}
}
