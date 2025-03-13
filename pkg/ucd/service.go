package ucd

import (
	"context"
	"fmt"
)

// Service coordinates code analysis.
type Service struct {
	aiAnalyzer AIAnalyzer
}

// NewService creates a new UCD service.
func NewService(aiAnalyzer AIAnalyzer) *Service {
	return &Service{aiAnalyzer: aiAnalyzer}
}

// AnalyzeGit analyzes differences in a Git repository.
func (s *Service) AnalyzeGit(ctx context.Context, repoURL, versionA, versionB string) (*Result, error) {
	analyzer := NewCollector(WithGit(repoURL, versionA, versionB))
	defer analyzer.Cleanup()

	data, err := analyzer.Collect()
	if err != nil {
		return nil, fmt.Errorf("collect data: %w", err)
	}

	return s.aiAnalyzer.Analyze(ctx, data)
}

// AnalyzeDiff analyzes differences from a diff file.
func (s *Service) AnalyzeDiff(ctx context.Context, diffPath, versionA, versionB string,
	changelogPath, commitMsgs string,
) (*Result, error) {
	analyzer := NewCollector(WithDiff(diffPath, versionA, versionB))

	if changelogPath != "" {
		analyzer = NewCollector(
			WithDiff(diffPath, versionA, versionB),
			WithChangelog(changelogPath),
		)
	}

	if commitMsgs != "" {
		analyzer = NewCollector(
			WithDiff(diffPath, versionA, versionB),
			WithChangelog(changelogPath),
			WithCommitMessages(commitMsgs),
		)
	}

	defer analyzer.Cleanup()

	data, err := analyzer.Collect()
	if err != nil {
		return nil, fmt.Errorf("collect data: %w", err)
	}

	return s.aiAnalyzer.Analyze(ctx, data)
}
