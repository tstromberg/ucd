// Package ucd provides tools to analyze differences between software versions
// and identify undocumented changes.
package ucd

import (
	"context"
	"strings"
)

// Rating represents the severity assessment of undocumented changes.
type Rating int

const (
	// RatingUnknown is the default rating when none is specified.
	RatingUnknown Rating = iota
	// RatingBenign indicates undocumented changes appear harmless.
	RatingBenign
	// RatingSilentSecurityFix indicates changes that fix security issues without disclosure.
	RatingSilentSecurityFix
	// RatingSuspicious indicates concerning changes with unclear intent.
	RatingSuspicious
	// RatingPossiblyMalicious indicates changes that have characteristics of malicious code.
	RatingPossiblyMalicious
	// RatingDefinitelyMalicious indicates changes that are clearly malicious.
	RatingDefinitelyMalicious
)

// String returns the string representation of a rating.
func (r Rating) String() string {
	switch r {
	case RatingBenign:
		return "🟢"
	case RatingSilentSecurityFix:
		return "🛡️"
	case RatingSuspicious:
		return "🟠"
	case RatingPossiblyMalicious:
		return "🚨"
	case RatingDefinitelyMalicious:
		return "😈"
	default:
		return "Unknown"
	}
}

// ParseRating converts a string to a Rating.
func ParseRating(s string) Rating {
	switch strings.ToLower(s) {
	case "benign":
		return RatingBenign
	case "silent security fix":
		return RatingSilentSecurityFix
	case "suspicious":
		return RatingSuspicious
	case "possibly malicious":
		return RatingPossiblyMalicious
	case "definitely malicious":
		return RatingDefinitelyMalicious
	default:
		return RatingUnknown
	}
}

// Change represents a single undocumented change with its severity rating.
type Change struct {
	Description string
	Rating      Rating
	Explanation string
}

// Result contains the findings from a diff analysis.
type Result struct {
	Changes []Change
}

// AnalysisData contains all information needed for code analysis.
type AnalysisData struct {
	Diff           string
	CommitMessages string
	Changelog      string
	VersionA       string
	VersionB       string
}

// AIAnalyzer defines the interface for AI-based code analysis.
type AIAnalyzer interface {
	Analyze(ctx context.Context, data *AnalysisData) (*Result, error)
}
