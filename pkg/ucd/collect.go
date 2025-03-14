package ucd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config holds the configuration for code change collection.
type Config struct {
	// Data source configuration
	RepoURL       string
	DiffPath      string
	ChangelogPath string
	CommitMsgs    string

	// Optional version identifiers
	VersionA string
	VersionB string
}

// AnalysisData contains collected code change information.
type AnalysisData struct {
	Diff           string
	CommitMessages string
	Changelog      string
	VersionA       string
	VersionB       string
}

// Collect gathers all necessary data for analysis based on the provided config.
func Collect(cfg Config) (*AnalysisData, error) {
	// Set default versions if not specified
	if cfg.VersionA == "" {
		cfg.VersionA = "v0"
	}
	if cfg.VersionB == "" {
		cfg.VersionB = "v1"
	}

	var diff, commitMsgs, changelog string
	var err error

	if cfg.RepoURL != "" {
		// Git repository analysis mode
		diff, commitMsgs, changelog, err = collectFromGit(cfg)
	} else {
		// Direct file analysis mode
		diff, commitMsgs, changelog, err = collectFromFiles(cfg)
	}

	if err != nil {
		return nil, err
	}

	return &AnalysisData{
		Diff:           diff,
		CommitMessages: commitMsgs,
		Changelog:      changelog,
		VersionA:       cfg.VersionA,
		VersionB:       cfg.VersionB,
	}, nil
}

// collectFromGit extracts data from a Git repository.
func collectFromGit(cfg Config) (diff, commitMsgs, changelog string, err error) {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", "", fmt.Errorf("git command not found: %v", err)
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "ucd-git-*")
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the repository
	cmd := exec.Command("git", "clone", "--quiet", cfg.RepoURL, tempDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", "", fmt.Errorf("git clone failed: %s: %w", bytes.TrimSpace(out), err)
	}

	// Generate diff
	cmd = exec.Command("git", "diff", cfg.VersionA, cfg.VersionB)
	cmd.Dir = tempDir
	out, err = cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("git diff failed: %w", err)
	}
	diff = string(out)

	// Extract commit messages
	cmd = exec.Command("git", "log", "--pretty=format:%s", cfg.VersionA+".."+cfg.VersionB)
	cmd.Dir = tempDir
	out, err = cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("git log failed: %w", err)
	}
	commitMsgs = string(out)

	// Extract changelog
	changelog, _ = getChangelogFromGit(tempDir, cfg.VersionA, cfg.VersionB) // Non-fatal if fails
	if changelog == "" {
		changelog = "No CHANGELOG found."
	}

	return diff, commitMsgs, changelog, nil
}

// getChangelogFromGit extracts changelog differences from a Git repository.
func getChangelogFromGit(repoDir, versionA, versionB string) (string, error) {
	// Look for common changelog filenames
	patterns := []string{
		"CHANGELOG.md", "CHANGELOG.txt", "CHANGELOG",
		"changelog.md", "changelog.txt", "changelog",
		"CHANGES.md", "changes.md",
	}

	var changelogFile string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(repoDir, pattern))
		if len(matches) > 0 {
			changelogFile = matches[0]
			break
		}
	}

	if changelogFile == "" {
		return "", fmt.Errorf("no changelog file found")
	}

	relPath, err := filepath.Rel(repoDir, changelogFile)
	if err != nil {
		relPath = filepath.Base(changelogFile)
	}

	// Get changelog at version A
	cmdA := exec.Command("git", "show", versionA+":"+relPath)
	cmdA.Dir = repoDir
	contentA, err := cmdA.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get changelog at version %s: %w", versionA, err)
	}

	// Get changelog at version B
	cmdB := exec.Command("git", "show", versionB+":"+relPath)
	cmdB.Dir = repoDir
	contentB, err := cmdB.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get changelog at version %s: %w", versionB, err)
	}

	// Create a diff of the changelog
	cmd := exec.Command("diff", "-u", "--label", versionA, "--label", versionB, "-", "-")
	cmd.Stdin = io.MultiReader(bytes.NewReader(contentA), strings.NewReader("\n"), bytes.NewReader(contentB))
	output, _ := cmd.Output() // Ignore error since diff returns non-zero for differences

	return string(output), nil
}

// collectFromFiles extracts data from provided files.
func collectFromFiles(cfg Config) (diff, commitMsgs, changelog string, err error) {
	// Read diff file
	if cfg.DiffPath == "" {
		return "", "", "", fmt.Errorf("diff file not found")
	}

	// Inline the readFile function
	diffData, err := os.ReadFile(cfg.DiffPath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read diff file: %w", err)
	}
	diff = string(diffData)

	// Use provided commit messages
	commitMsgs = cfg.CommitMsgs

	// Read changelog if provided
	if cfg.ChangelogPath != "" {
		changelogData, err := os.ReadFile(cfg.ChangelogPath)
		if err != nil {
			changelog = "Failed to read CHANGELOG."
		} else {
			changelog = string(changelogData)
		}
	} else {
		changelog = "No CHANGELOG provided."
	}

	return diff, commitMsgs, changelog, nil
}
