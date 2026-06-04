package embed

import (
	"context"
	"testing"
)

func TestRebuildNilEmbedder(t *testing.T) {
	t.Parallel()
	tools := []ToolEntry{
		{Server: "s1", ToolName: "search_code", Description: "Search code in repo"},
		{Server: "s1", ToolName: "list_files", Description: "List files in directory"},
		{Server: "s2", ToolName: "run_tests", Description: "Execute test suite"},
	}
	idx := NewIndex(nil)
	if err := idx.Rebuild(context.Background(), tools); err != nil {
		t.Fatalf("Rebuild with nil embedder: %v", err)
	}
	if got := idx.Len(); got != len(tools) {
		t.Fatalf("Len() = %d, want %d", got, len(tools))
	}
	idx.mu.RLock()
	for i, e := range idx.entries {
		if e.Vector != nil {
			t.Errorf("entries[%d].Vector = %v, want nil (nil embedder)", i, e.Vector)
		}
	}
	idx.mu.RUnlock()
}

func TestFindKeywordFallback(t *testing.T) {
	t.Parallel()
	tools := []ToolEntry{
		{Server: "s1", ToolName: "search_code", Description: "Search source code in the repository"},
		{Server: "s1", ToolName: "list_files", Description: "List files in a directory"},
		{Server: "s1", ToolName: "run_tests", Description: "Execute the test suite"},
	}
	idx := NewIndex(nil)
	if err := idx.Rebuild(context.Background(), tools); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	results, err := idx.Find(context.Background(), "search code", 3, "")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Find returned no results")
	}
	if results[0].ToolName != "search_code" {
		t.Errorf("top result = %q, want %q", results[0].ToolName, "search_code")
	}
}

func TestFindKeywordNoMatch(t *testing.T) {
	t.Parallel()
	tools := []ToolEntry{
		{Server: "s1", ToolName: "list_files", Description: "List files in a directory"},
		{Server: "s1", ToolName: "run_tests", Description: "Execute the test suite"},
	}
	idx := NewIndex(nil)
	if err := idx.Rebuild(context.Background(), tools); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	results, err := idx.Find(context.Background(), "xyzzy quux frobnitz", 5, "")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	for _, r := range results {
		if r.Score > 0 {
			t.Errorf("expected score 0 for no-match query, got %f for %q", r.Score, r.ToolName)
		}
	}
}

func TestKeywordScore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		query string
		entry ToolEntry
		want  float32
	}{
		{
			name:  "exact single term match",
			query: "search",
			entry: ToolEntry{ToolName: "search_code", Description: "Search source code"},
			want:  1.0,
		},
		{
			name:  "multi-term partial match",
			query: "search files",
			entry: ToolEntry{ToolName: "search_code", Description: "Search source code"},
			want:  0.5, // "search" hits, "files" does not
		},
		{
			name:  "no match",
			query: "xyzzy quux",
			entry: ToolEntry{ToolName: "list_files", Description: "List files in a directory"},
			want:  0.0,
		},
		{
			name:  "both terms match",
			query: "list files",
			entry: ToolEntry{ToolName: "list_files", Description: "List files in a directory"},
			want:  1.0,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := keywordScore(tc.query, tc.entry)
			if got != tc.want {
				t.Errorf("keywordScore(%q, %+v) = %f, want %f", tc.query, tc.entry, got, tc.want)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  []string
	}{
		{"search_code", []string{"search", "code"}},
		{"brave-search", []string{"brave", "search"}},
		{"find.files", []string{"find", "files"}},
		{"  spaces  ", []string{"spaces"}},
		{"a/b(c,d)", []string{"a", "b", "c", "d"}},
		{"", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			got := tokenize(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("tokenize(%q) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("tokenize(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestFindServerFilter(t *testing.T) {
	t.Parallel()
	tools := []ToolEntry{
		{Server: "alpha", ToolName: "search_alpha", Description: "Search in alpha"},
		{Server: "alpha", ToolName: "list_alpha", Description: "List in alpha"},
		{Server: "beta", ToolName: "search_beta", Description: "Search in beta"},
	}
	idx := NewIndex(nil)
	if err := idx.Rebuild(context.Background(), tools); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	results, err := idx.Find(context.Background(), "search", 10, "alpha")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	for _, r := range results {
		if r.Server != "alpha" {
			t.Errorf("result server = %q, want %q", r.Server, "alpha")
		}
	}
	// beta tool must not appear
	for _, r := range results {
		if r.ToolName == "search_beta" {
			t.Error("search_beta should not appear when filtering to alpha")
		}
	}
}

func TestIndexLen(t *testing.T) {
	t.Parallel()
	idx := NewIndex(nil)
	if got := idx.Len(); got != 0 {
		t.Fatalf("Len() before Rebuild = %d, want 0", got)
	}
	tools := []ToolEntry{
		{Server: "s1", ToolName: "a", Description: "tool a"},
		{Server: "s1", ToolName: "b", Description: "tool b"},
		{Server: "s1", ToolName: "c", Description: "tool c"},
	}
	if err := idx.Rebuild(context.Background(), tools); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if got := idx.Len(); got != 3 {
		t.Fatalf("Len() after Rebuild = %d, want 3", got)
	}
	// Rebuild with fewer tools replaces index.
	if err := idx.Rebuild(context.Background(), tools[:1]); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
	if got := idx.Len(); got != 1 {
		t.Fatalf("Len() after second Rebuild = %d, want 1", got)
	}
}
