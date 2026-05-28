package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadLatticePathTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	latticeDir := filepath.Join(root, "lattice")
	if err := os.Mkdir(latticeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Sibling directory whose absolute path shares the lattice prefix.
	// Without the trailing separator in the prefix check this slips through.
	siblingDir := filepath.Join(root, "lattice_evil")
	if err := os.Mkdir(siblingDir, 0o755); err != nil {
		t.Fatal(err)
	}

	insideFile := filepath.Join(latticeDir, "ok.md")
	if err := os.WriteFile(insideFile, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(siblingDir, "secret.md")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	handler := newReadLattice(nil, latticeDir)

	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{name: "inside lattice", path: "ok.md", wantError: false},
		{name: "sibling with prefix collision", path: "../lattice_evil/secret.md", wantError: true},
		{name: "traversal up", path: "../../etc/passwd", wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, _, err := handler(context.Background(), nil, readLatticeInput{FilePath: tc.path})
			if err != nil {
				t.Fatalf("handler returned err: %v", err)
			}
			if res.IsError != tc.wantError {
				t.Fatalf("IsError = %v, want %v", res.IsError, tc.wantError)
			}
		})
	}
}
