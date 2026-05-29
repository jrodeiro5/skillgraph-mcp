package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/graph"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Manager struct {
	mu      sync.RWMutex
	servers map[string]*Server
	tools   []Tool
	graph   *graph.Graph
}

// NewManager creates a Manager by connecting to all servers in the config.
func NewManager(ctx context.Context, cfgs map[string]config.Server, graphCfg *config.SkillGraphConfig) (*Manager, error) {
	m := &Manager{
		servers: make(map[string]*Server),
		graph:   graph.New(),
	}

	for name, srv := range cfgs {
		s, err := NewServer(ctx, srv)
		if err != nil {
			// One bad downstream should not kill the entire gateway. Log and
			// continue; the remaining servers will still be reachable. The
			// agent can call `validate` to see per-server status.
			slog.Warn("failed to connect to server, skipping", "server", name, "error", err)
			continue
		}
		m.servers[name] = s
		slog.Info("connected to server", "server", name)

		// Add Skill Node
		desc := ""
		if graphCfg != nil && graphCfg.Descriptions[name] != "" {
			desc = graphCfg.Descriptions[name]
		} else if opts := srv.Options(); opts.Description != "" {
			desc = opts.Description
		} else if s.Instructions() != "" {
			desc = s.Instructions()
		}
		m.graph.AddNode(name, graph.NodeSkill, name, desc)
	}

	if len(cfgs) > 0 && len(m.servers) == 0 {
		return nil, fmt.Errorf("no downstream servers could be reached (%d configured, all failed)", len(cfgs))
	}

	tools, err := resolveTools(m.servers)
	if err != nil {
		m.Close()
		return nil, err
	}
	m.tools = tools

	// Add Tool Nodes and their HAS_TOOL relationships
	for _, t := range m.tools {
		desc := t.Description
		if graphCfg != nil && graphCfg.Descriptions[t.ResolvedName] != "" {
			desc = graphCfg.Descriptions[t.ResolvedName]
		}
		m.graph.AddNode(t.ResolvedName, graph.NodeTool, t.ResolvedName, desc)
		m.graph.AddEdge(t.ServerName, t.ResolvedName, graph.RelHasTool, "")
	}

	// Dynamic inference
	m.inferRelations(m.graph)

	// Apply Static Relations from configuration
	if graphCfg != nil {
		for _, rel := range graphCfg.Relations {
			if _, exists := m.graph.Nodes[rel.Source]; !exists {
				nodeType := graph.NodeTool
				if _, ok := m.servers[rel.Source]; ok {
					nodeType = graph.NodeSkill
				}
				m.graph.AddNode(rel.Source, nodeType, rel.Source, "")
			}
			if _, exists := m.graph.Nodes[rel.Target]; !exists {
				nodeType := graph.NodeTool
				if _, ok := m.servers[rel.Target]; ok {
					nodeType = graph.NodeSkill
				}
				m.graph.AddNode(rel.Target, nodeType, rel.Target, "")
			}
			m.graph.AddEdge(rel.Source, rel.Target, graph.RelationType(rel.Type), rel.Description)
		}
	}

	return m, nil
}

// NewManagerFromServers creates a Manager from pre-built Servers (useful for testing).
func NewManagerFromServers(servers map[string]*Server) (*Manager, error) {
	m := &Manager{
		servers: servers,
		graph:   graph.New(),
	}
	tools, err := resolveTools(m.servers)
	if err != nil {
		return nil, err
	}
	m.tools = tools

	// Add Skill and Tool Nodes for testing
	for name, srv := range servers {
		m.graph.AddNode(name, graph.NodeSkill, name, srv.Instructions())
	}
	for _, t := range m.tools {
		m.graph.AddNode(t.ResolvedName, graph.NodeTool, t.ResolvedName, t.Description)
		m.graph.AddEdge(t.ServerName, t.ResolvedName, graph.RelHasTool, "")
	}
	m.inferRelations(m.graph)

	return m, nil
}

func (m *Manager) GetGraph() *graph.Graph {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.graph
}

// RebuildGraph rebuilds the capability graph from scratch using the updated config
// and switches it atomically. This is thread-safe.
func (m *Manager) RebuildGraph(graphCfg *config.SkillGraphConfig) {
	g := graph.New()

	// 1. Add Skill Nodes for each server
	for name, s := range m.servers {
		desc := ""
		if graphCfg != nil && graphCfg.Descriptions[name] != "" {
			desc = graphCfg.Descriptions[name]
		} else if s.Instructions() != "" {
			desc = s.Instructions()
		}
		g.AddNode(name, graph.NodeSkill, name, desc)
	}

	// 2. Add Tool Nodes and their HAS_TOOL relationships
	for _, t := range m.tools {
		desc := t.Description
		if graphCfg != nil && graphCfg.Descriptions[t.ResolvedName] != "" {
			desc = graphCfg.Descriptions[t.ResolvedName]
		}
		g.AddNode(t.ResolvedName, graph.NodeTool, t.ResolvedName, desc)
		g.AddEdge(t.ServerName, t.ResolvedName, graph.RelHasTool, "")
	}

	// 3. Dynamic inference on the new graph
	m.inferRelations(g)

	// 4. Apply Static Relations from configuration
	if graphCfg != nil {
		for _, rel := range graphCfg.Relations {
			if _, exists := g.Nodes[rel.Source]; !exists {
				nodeType := graph.NodeTool
				if _, ok := m.servers[rel.Source]; ok {
					nodeType = graph.NodeSkill
				}
				g.AddNode(rel.Source, nodeType, rel.Source, "")
			}
			if _, exists := g.Nodes[rel.Target]; !exists {
				nodeType := graph.NodeTool
				if _, ok := m.servers[rel.Target]; ok {
					nodeType = graph.NodeSkill
				}
				g.AddNode(rel.Target, nodeType, rel.Target, "")
			}
			g.AddEdge(rel.Source, rel.Target, graph.RelationType(rel.Type), rel.Description)
		}
	}

	// Atomically switch graph
	m.mu.Lock()
	m.graph = g
	m.mu.Unlock()
}

func (m *Manager) inferRelations(g *graph.Graph) {
	for _, tA := range m.tools {
		for _, param := range tA.Params {
			if strings.HasSuffix(param.Name, "_id") || strings.HasSuffix(param.Name, "_number") {
				prefix := strings.TrimSuffix(strings.TrimSuffix(param.Name, "_id"), "_number")
				for _, tB := range m.tools {
					if tA.ResolvedName == tB.ResolvedName {
						continue
					}
					nameLower := strings.ToLower(tB.ResolvedName)
					prefixLower := strings.ToLower(prefix)
					isProducer := strings.Contains(nameLower, "create_"+prefixLower) ||
						strings.Contains(nameLower, "new_"+prefixLower) ||
						strings.Contains(nameLower, "get_"+prefixLower) ||
						strings.Contains(nameLower, "search_"+prefixLower) ||
						nameLower == prefixLower

					if isProducer {
						g.AddEdge(tB.ResolvedName, tA.ResolvedName, graph.RelPrerequisiteFor, fmt.Sprintf("provides %s", param.Name))
					}
				}
			}
		}
	}
}

// resolveTools resolves tool names across all servers, prefixing with
// server name only when multiple servers define a tool with the same name.
func resolveTools(servers map[string]*Server) ([]Tool, error) {
	type entry struct {
		serverName string
		tool       *mcp.Tool
	}

	byName := make(map[string][]entry)
	for name, srv := range servers {
		for _, tool := range srv.tools {
			byName[tool.Name] = append(byName[tool.Name], entry{name, tool})
		}
	}

	var resolved []Tool
	for name, entries := range byName {
		if len(entries) == 1 {
			t, err := newTool(name, name, entries[0].serverName, entries[0].tool)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, t)
		} else {
			for _, e := range entries {
				t, err := newTool(e.serverName+"_"+name, name, e.serverName, e.tool)
				if err != nil {
					return nil, err
				}
				resolved = append(resolved, t)
			}
		}
	}
	return resolved, nil
}

func (m *Manager) GetServer(name string) (*Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[name]
	if !ok {
		return nil, fmt.Errorf("unknown server: %q", name)
	}
	return s, nil
}

func (m *Manager) ListServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

func (m *Manager) AllTools() []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Tool, len(m.tools))
	copy(out, m.tools)
	return out
}

func (m *Manager) ServerTools(name string) []Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tools []Tool
	for _, t := range m.tools {
		if t.ServerName == name {
			tools = append(tools, t)
		}
	}
	return tools
}

// AddServer connects to a new downstream MCP server and hot-registers it into
// the manager without a restart. The graph is rebuilt atomically after the
// connection succeeds. Returns an error if the name is already registered.
func (m *Manager) AddServer(ctx context.Context, name string, cfg config.Server) error {
	s, err := NewServer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connecting to %q: %w", name, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.servers[name]; exists {
		_ = s.Close()
		return fmt.Errorf("server %q is already registered", name)
	}

	m.servers[name] = s
	slog.Info("hot-registered server", "server", name)

	// Re-resolve all tools — name conflicts across all servers must stay consistent.
	tools, err := resolveTools(m.servers)
	if err != nil {
		delete(m.servers, name)
		_ = s.Close()
		return fmt.Errorf("resolving tools after adding %q: %w", name, err)
	}
	m.tools = tools

	// Rebuild the graph from scratch so inferred relations are correct.
	g := graph.New()
	for sName, srv := range m.servers {
		g.AddNode(sName, graph.NodeSkill, sName, srv.Instructions())
	}
	for _, t := range m.tools {
		g.AddNode(t.ResolvedName, graph.NodeTool, t.ResolvedName, t.Description)
		g.AddEdge(t.ServerName, t.ResolvedName, graph.RelHasTool, "")
	}
	m.inferRelations(g)
	m.graph = g

	return nil
}

func (m *Manager) Close() {
	for name, s := range m.servers {
		if err := s.Close(); err != nil {
			slog.Warn("error closing server", "server", name, "error", err)
		}
	}
}
