package cli

import (
	"encoding/json"
	"os"
)

// printJSON marshals v to indented JSON on stdout.
// Returns empty array "[]" instead of null when v is a nil slice.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
