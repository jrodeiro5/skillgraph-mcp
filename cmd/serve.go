package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jrodeiro5/skillgraph-mcp/internal/app"
	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/docs"
	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/jrodeiro5/skillgraph-mcp/internal/refine"
	"github.com/jrodeiro5/skillgraph-mcp/internal/version"

	flag "github.com/spf13/pflag"
)

type serveOptions struct {
	configPath string
	latticeDir string
	transport  string
	host       string
	port       string
	version    bool
}

func parseServeFlags(args []string) serveOptions {
	var opts serveOptions
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.StringVar(&opts.configPath, "config", "./mcp.json", "Path to MCP config file")
	fs.StringVar(&opts.latticeDir, "lattice-dir", defaultLatticeDir(), "Directory for traces, READMEs, and lattice docs (default: user cache dir)")
	fs.StringVar(&opts.transport, "transport", "stdio", "Upstream transport: stdio or http")
	fs.StringVar(&opts.host, "host", "localhost", "HTTP host (when transport=http)")
	fs.StringVar(&opts.port, "port", "8080", "HTTP port (when transport=http)")
	fs.BoolVar(&opts.version, "version", false, "Print version and exit")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	return opts
}

func runServe(args []string) {
	opts := parseServeFlags(args)

	if opts.version {
		fmt.Println(version.Version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	servers, graphCfg, err := config.Load(opts.configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("loaded config", "servers", len(servers))
	for name, srv := range servers {
		switch s := srv.(type) {
		case *config.StdioServer:
			slog.Info("configured server", "name", name, "transport", "stdio", "command", s.Command, "args", s.Args)
		case *config.HTTPServer:
			slog.Info("configured server", "name", name, "transport", "http", "url", s.URL)
		case *config.SSEServer:
			slog.Info("configured server", "name", name, "transport", "sse", "url", s.URL)
		}
	}

	mgr, err := mcpserver.NewManager(ctx, servers, graphCfg)
	if err != nil {
		slog.Error("failed to connect to servers", "error", err)
		os.Exit(1)
	}
	defer mgr.Close()

	slog.Info("connected to servers", "servers", mgr.ListServerNames())

	if err := docs.GenerateLattice(ctx, opts.latticeDir, servers, mgr.GetGraph()); err != nil {
		slog.Warn("failed to generate semantic lattice", "error", err)
	}

	refine.StartRefinementLoop(ctx, opts.configPath, mgr, opts.latticeDir, servers)

	s := app.NewServer(mgr, opts.latticeDir, opts.configPath, opts.transport, opts.host)
	var serveErr error
	switch opts.transport {
	case "stdio":
		serveErr = app.ServeStdio(ctx, s)
	case "http":
		serveErr = app.ServeHTTP(ctx, s, opts.host, opts.port)
	default:
		slog.Error("unknown transport (use 'stdio' or 'http')", "transport", opts.transport)
		os.Exit(1)
	}
	if serveErr != nil {
		slog.Error("server error", "error", serveErr)
		os.Exit(1)
	}
}
