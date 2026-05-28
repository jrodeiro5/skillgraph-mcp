package docs

import "testing"

func TestExtractNPMPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{name: "plain npx", command: "npx", args: []string{"-y", "foo"}, want: "foo"},
		{name: "npx-mcp wrapper", command: "npx-mcp", args: []string{"-y", "@scope/foo"}, want: "@scope/foo"},
		{name: "absolute npx-mcp", command: "/home/user/.local/bin/npx-mcp", args: []string{"-y", "firecrawl-mcp"}, want: "firecrawl-mcp"},
		{name: "scoped with version", command: "npx", args: []string{"-y", "@scope/foo@1.2.3"}, want: "@scope/foo"},
		{name: "unscoped with version", command: "npx", args: []string{"-y", "foo@latest"}, want: "foo"},
		{name: "skips --yes flag", command: "npx", args: []string{"--yes", "bar"}, want: "bar"},
		{name: "skips extra flags", command: "npx", args: []string{"-y", "--silent", "bar"}, want: "bar"},
		{name: "non-npx command", command: "node", args: []string{"script.js"}, want: ""},
		{name: "no positional arg", command: "npx", args: []string{"-y"}, want: ""},
		{name: "empty args", command: "npx", args: nil, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractNPMPackage(tc.command, tc.args)
			if got != tc.want {
				t.Errorf("extractNPMPackage(%q, %v) = %q, want %q", tc.command, tc.args, got, tc.want)
			}
		})
	}
}
