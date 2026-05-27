package docs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/jrodeiro5/skillgraph-mcp/internal/config"
	"github.com/jrodeiro5/skillgraph-mcp/internal/graph"
)

// GenerateLattice generates the `.mcp_lattice` folder containing skills, relations, and downloaded READMEs.
func GenerateLattice(ctx context.Context, dir string, cfgs map[string]config.Server, g *graph.Graph) error {
	// Create folder
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 1. Generate skills.md
	var skillsBuilder strings.Builder
	skillsBuilder.WriteString("# Active Skills and Capability Index\n\n")
	skillsBuilder.WriteString("This file lists all active servers and skills configured in this gateway.\n\n")

	for name, srv := range cfgs {
		desc := ""
		if opts := srv.Options(); opts.Description != "" {
			desc = opts.Description
		}
		skillsBuilder.WriteString(fmt.Sprintf("## %s\n", name))
		if desc != "" {
			skillsBuilder.WriteString(desc + "\n\n")
		} else {
			skillsBuilder.WriteString("No description provided.\n\n")
		}

		// Attempt to fetch documentation offline in a background goroutine to prevent startup blocking
		go func(name string, srv config.Server) {
			readmePath := filepath.Join(dir, fmt.Sprintf("%s_readme.md", name))

			// Check if we already have it to avoid spamming network
			if _, err := os.Stat(readmePath); err == nil {
				return
			}

			// Try to fetch GitHub or NPM readme
			switch s := srv.(type) {
			case *config.HTTPServer:
				if strings.Contains(s.URL, "github.com") {
					_ = FetchReadme(ctx, s.URL, readmePath)
				}
			case *config.SSEServer:
				if strings.Contains(s.URL, "github.com") {
					_ = FetchReadme(ctx, s.URL, readmePath)
				}
			case *config.StdioServer:
				if s.Command == "npx" {
					for _, arg := range s.Args {
						if arg != "-y" && !strings.HasPrefix(arg, "-") {
							// Simple package detection (supports @scope/name or name)
							_ = FetchNPMReadme(ctx, arg, readmePath)
							break
						}
					}
				}
			}
		}(name, srv)
	}

	if err := os.WriteFile(filepath.Join(dir, "skills.md"), []byte(skillsBuilder.String()), 0644); err != nil {
		return err
	}

	// 2. Generate relations.md
	var relationsBuilder strings.Builder
	relationsBuilder.WriteString("# Capability Relationship Graph\n\n")
	relationsBuilder.WriteString("This file represents the dependency and planning map between skills and tools.\n\n")
	if g != nil {
		relationsBuilder.WriteString("```\n")
		relationsBuilder.WriteString(g.FormatCompact())
		relationsBuilder.WriteString("```\n")
	}

	if err := os.WriteFile(filepath.Join(dir, "relations.md"), []byte(relationsBuilder.String()), 0644); err != nil {
		return err
	}

	slog.Info("generated semantic lattice documentation", "dir", dir)
	return nil
}
