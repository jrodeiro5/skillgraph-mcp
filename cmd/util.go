package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

// emitJSON writes v to stdout as indented JSON. Used by subcommands that
// support --json output.
func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
		os.Exit(1)
	}
}
