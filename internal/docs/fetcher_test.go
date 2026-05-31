package docs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		url     string
		owner   string
		repo    string
		subpath string
		branch  string
		wantErr bool
	}{
		{
			url:     "https://github.com/google/go-github",
			owner:   "google",
			repo:    "go-github",
			wantErr: false,
		},
		{
			url:     "https://github.com/modelcontextprotocol/servers/tree/main/src/postgres",
			owner:   "modelcontextprotocol",
			repo:    "servers",
			branch:  "main",
			subpath: "src/postgres",
			wantErr: false,
		},
		{
			url:     "https://gitlab.com/google/go-github",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		owner, repo, subpath, branch, err := ParseGitHubURL(tt.url)
		if tt.wantErr {
			if err == nil {
				t.Errorf("expected error for %s, got nil", tt.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for %s: %v", tt.url, err)
			continue
		}
		if owner != tt.owner || repo != tt.repo || subpath != tt.subpath || branch != tt.branch {
			t.Errorf("ParseGitHubURL(%s) = (%q, %q, %q, %q), want (%q, %q, %q, %q)",
				tt.url, owner, repo, subpath, branch, tt.owner, tt.repo, tt.subpath, tt.branch)
		}
	}
}

func TestFetchNPMReadme(t *testing.T) {
	// Fetch README of a lightweight, highly stable package from registry
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "readme.md")

	ctx := context.Background()
	err := FetchNPMReadme(ctx, "is-even", outputPath)
	if err != nil {
		t.Skipf("Skipping NPM fetch test (likely offline or network issue): %v", err)
		return
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output readme: %v", err)
	}

	if len(content) == 0 {
		t.Error("expected non-empty readme content")
	}
}
