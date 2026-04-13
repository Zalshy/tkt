package cmd

import (
	"fmt"
	"strings"
)

// parseIDs splits a raw comma-separated ID string into a deduplicated,
// ordered slice of trimmed ID strings.
// Returns error if any segment is empty after trimming (e.g. "33,,34").
func parseIDs(raw string) ([]string, error) {
	segments := strings.Split(raw, ",")
	seen := make(map[string]bool, len(segments))
	result := make([]string, 0, len(segments))
	for _, s := range segments {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return nil, fmt.Errorf("parseIDs: empty segment in %q", raw)
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result, nil
}
