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
		fmt.Fprintf(&skillsBuilder, "## %s\n", name)
		if desc != "" {
			skillsBuilder.WriteString(desc + "\n\n")
		} else {
			skillsBuilder.WriteString("No description provided.\n\n")
		}

		// Attempt to fetch documentation offline in a background goroutine to prevent startup blocking.
		// On any outcome (success, failure, or "no source"), a sentinel is written so that the
		// refine bootstrap loop can stop waiting; a missing file means "still fetching".
		go func(name string, srv config.Server) {
			readmePath := filepath.Join(dir, fmt.Sprintf("%s_readme.md", name))
			sentinelPath := readmePath + ".noreadme"

			// Check if we already have it to avoid spamming network
			if _, err := os.Stat(readmePath); err == nil {
				return
			}

			fetched := false
			switch s := srv.(type) {
			case *config.HTTPServer:
				if strings.Contains(s.URL, "github.com") {
					if err := FetchReadme(ctx, s.URL, readmePath); err == nil {
						fetched = true
					} else {
						slog.Debug("fetching README failed", "server", name, "error", err)
					}
				}
			case *config.SSEServer:
				if strings.Contains(s.URL, "github.com") {
					if err := FetchReadme(ctx, s.URL, readmePath); err == nil {
						fetched = true
					} else {
						slog.Debug("fetching README failed", "server", name, "error", err)
					}
				}
			case *config.StdioServer:
				if pkg := extractNPMPackage(s.Command, s.Args); pkg != "" {
					if err := FetchNPMReadme(ctx, pkg, readmePath); err == nil {
						fetched = true
					} else {
						slog.Debug("fetching NPM README failed", "server", name, "package", pkg, "error", err)
					}
				}
			}

			if !fetched {
				// Write a tiny marker so refine knows there will never be a README.
				_ = os.WriteFile(sentinelPath, []byte("no-readme-source"), 0644)
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

// extractNPMPackage returns the NPM package name from a stdio server's
// command + args, or "" if the server isn't running through npx.
// Recognises plain "npx", absolute paths to npx, and wrapper scripts whose
// basename starts with "npx" (e.g. "npx-mcp").
func extractNPMPackage(command string, args []string) string {
	base := filepath.Base(command)
	if base != "npx" && !strings.HasPrefix(base, "npx") {
		return ""
	}
	for _, arg := range args {
		if arg == "-y" || arg == "--yes" || strings.HasPrefix(arg, "-") {
			continue
		}
		// First positional argument is the package spec (e.g. "foo", "@scope/foo", "foo@latest").
		pkg := arg
		// Drop any "@version" suffix (but keep scoped names starting with "@").
		if strings.HasPrefix(pkg, "@") {
			if idx := strings.Index(pkg[1:], "@"); idx >= 0 {
				pkg = pkg[:1+idx]
			}
		} else if idx := strings.Index(pkg, "@"); idx >= 0 {
			pkg = pkg[:idx]
		}
		return pkg
	}
	return ""
}
