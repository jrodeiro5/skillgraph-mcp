package graph

import (
	"fmt"
	"strings"
)

type NodeType string

const (
	NodeSkill    NodeType = "skill"
	NodeTool     NodeType = "tool"
	NodeResource NodeType = "resource"
)

type RelationType string

const (
	RelHasTool         RelationType = "HAS_TOOL"
	RelPrerequisiteFor RelationType = "PREREQUISITE_FOR"
	RelProduces        RelationType = "PRODUCES"
	RelRequires        RelationType = "REQUIRES"
	RelCommonNextStep  RelationType = "COMMON_NEXT_STEP"
)

type Node struct {
	ID          string   `json:"id"`
	Type        NodeType `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
}

type Edge struct {
	Source      string       `json:"source"`
	Target      string       `json:"target"`
	Type        RelationType `json:"type"`
	Description string       `json:"description,omitempty"`
}

type Graph struct {
	Nodes map[string]*Node `json:"nodes"`
	Edges []*Edge          `json:"edges"`
}

func New() *Graph {
	return &Graph{
		Nodes: make(map[string]*Node),
		Edges: make([]*Edge, 0),
	}
}

func (g *Graph) AddNode(id string, nType NodeType, name, description string) {
	if _, exists := g.Nodes[id]; !exists {
		g.Nodes[id] = &Node{
			ID:          id,
			Type:        nType,
			Name:        name,
			Description: description,
		}
	}
}

func (g *Graph) AddEdge(source, target string, relType RelationType, description string) {
	for _, e := range g.Edges {
		if e.Source == source && e.Target == target && e.Type == relType {
			return // avoid duplicate edges
		}
	}
	g.Edges = append(g.Edges, &Edge{
		Source:      source,
		Target:      target,
		Type:        relType,
		Description: description,
	})
}

// Subgraph returns a new Graph containing the center node and all directly connected neighbors and edges.
func (g *Graph) Subgraph(nodeID string) *Graph {
	sub := New()
	center, exists := g.Nodes[nodeID]
	if !exists {
		return sub
	}
	sub.Nodes[nodeID] = center

	for _, e := range g.Edges {
		if e.Source == nodeID || e.Target == nodeID {
			if src, ok := g.Nodes[e.Source]; ok {
				sub.Nodes[e.Source] = src
			}
			if tgt, ok := g.Nodes[e.Target]; ok {
				sub.Nodes[e.Target] = tgt
			}
			sub.Edges = append(sub.Edges, e)
		}
	}
	return sub
}

// FormatCompact returns a compact string representation of the graph suitable for an LLM context.
func (g *Graph) FormatCompact() string {
	var sb strings.Builder
	sb.WriteString("Capability Graph:\n")
	sb.WriteString("Nodes:\n")
	for _, n := range g.Nodes {
		desc := ""
		if n.Description != "" {
			desc = " - " + n.Description
		}
		fmt.Fprintf(&sb, "- [%s] %s%s\n", n.Type, n.Name, desc)
	}

	sb.WriteString("\nRelations:\n")
	for _, e := range g.Edges {
		srcNode, srcOk := g.Nodes[e.Source]
		tgtNode, tgtOk := g.Nodes[e.Target]

		var srcName, tgtName string
		if srcOk {
			srcName = fmt.Sprintf("[%s] %s", srcNode.Type, srcNode.Name)
		} else {
			srcName = e.Source
		}
		if tgtOk {
			tgtName = fmt.Sprintf("[%s] %s", tgtNode.Type, tgtNode.Name)
		} else {
			tgtName = e.Target
		}

		descPart := ""
		if e.Description != "" {
			descPart = fmt.Sprintf(" (%s)", e.Description)
		}
		fmt.Fprintf(&sb, "- %s -%s-> %s%s\n", srcName, e.Type, tgtName, descPart)
	}
	return sb.String()
}
