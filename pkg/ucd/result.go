package ucd

import (
	"fmt"
	"sort"
	"strings"
)

// HasChanges returns true if undocumented changes were found.
func (r *Result) HasChanges() bool {
	return len(r.Changes) > 0
}

// SortByRating sorts changes from most severe to least severe.
func (r *Result) SortByRating() {
	sort.Slice(r.Changes, func(i, j int) bool {
		return r.Changes[i].Rating > r.Changes[j].Rating
	})
}

// Format generates a human-readable summary of the analysis results.
func (r *Result) Format() string {
	if !r.HasChanges() {
		return "No undocumented changes found."
	}

	// Make a copy to avoid modifying the original
	result := &Result{
		Changes: make([]Change, len(r.Changes)),
	}
	copy(result.Changes, r.Changes)
	result.SortByRating()

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d undocumented changes:\n\n", len(result.Changes))

	for _, change := range result.Changes {
		fmt.Fprintf(&sb, "%s %s\n", change.Rating, change.Description)
		//		fmt.Fprintf(&sb, "   Explanation: %s\n\n", change.Explanation)
	}

	return sb.String()
}
