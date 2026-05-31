package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jrodeiro5/skillgraph-mcp/internal/mcpserver"
	"github.com/jrodeiro5/skillgraph-mcp/internal/tools"
	"github.com/jrodeiro5/skillgraph-mcp/internal/version"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates an MCP server with all tools registered.
// configPath is the path to mcp.json used for persisting dynamic server registrations.
// transport and host describe how the gateway itself is being served, so
// register_server can gate registration when bound to a non-loopback HTTP
// address (see ADR-0001).
func NewServer(mgr *mcpserver.Manager, latticeDir, configPath, transport, host string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "skillgraph-mcp",
		Version: version.Version,
	}, nil)

	tools.RegisterListSkills(s, mgr)
	tools.RegisterUseSkill(s, mgr)
	tools.RegisterReadResource(s, mgr)
	tools.RegisterExecuteCode(s, mgr, latticeDir)
	tools.RegisterGetSkillGraph(s, mgr)
	tools.RegisterPlanWorkflow(s, mgr)
	tools.RegisterReadLattice(s, mgr, latticeDir)
	tools.RegisterRegisterServer(s, mgr, configPath, tools.GatewayBinding{Transport: transport, Host: host})

	return s
}

// ServeStdio runs the MCP server over stdin/stdout.
func ServeStdio(ctx context.Context, s *mcp.Server) error {
	return s.Run(ctx, &mcp.StdioTransport{})
}

// ServeHTTP runs the MCP server over HTTP with graceful shutdown.
func ServeHTTP(ctx context.Context, s *mcp.Server, host, port string) error {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return s
	}, nil)
	addr := net.JoinHostPort(host, port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Warn("http server shutdown error", "error", err)
		}
	}()
	slog.Info("listening", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
