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

	var (
		diff, commitMsgs, changelog string
		err                         error
	)

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

// runCommand is a helper function that executes a command and returns its output with better error handling.
func runCommand(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Include stderr in error message for better debugging
		return "", fmt.Errorf("%s command failed: %v\nStderr: %s", name, err, stderr.String())
	}

	return stdout.String(), nil
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
	_, err = runCommand("", "git", "clone", "--quiet", cfg.RepoURL, tempDir)
	if err != nil {
		return "", "", "", err
	}

	// Generate diff
	diff, err = runCommand(tempDir, "git", "diff", cfg.VersionA, cfg.VersionB)
	if err != nil {
		return "", "", "", err
	}

	// Extract commit messages
	commitMsgs, err = runCommand(tempDir, "git", "log", "--pretty=format:%s",
		fmt.Sprintf("%s..%s", cfg.VersionA, cfg.VersionB))
	if err != nil {
		return "", "", "", err
	}

	// Extract changelog (non-fatal if it fails)
	changelog, _ = getChangelogFromGit(tempDir, cfg.VersionA, cfg.VersionB)
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

	// Find the first matching changelog file
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

	// Get relative path for git commands
	relPath, err := filepath.Rel(repoDir, changelogFile)
	if err != nil {
		relPath = filepath.Base(changelogFile)
	}

	// Get changelog contents at both versions
	contentA, err := runCommand(repoDir, "git", "show", versionA+":"+relPath)
	if err != nil {
		return "", fmt.Errorf("failed to get changelog at version %s: %w", versionA, err)
	}

	contentB, err := runCommand(repoDir, "git", "show", versionB+":"+relPath)
	if err != nil {
		return "", fmt.Errorf("failed to get changelog at version %s: %w", versionB, err)
	}

	// Create a diff of the changelog using the diff command
	// Note: We don't use runCommand here because diff returns non-zero exit code for differences
	cmd := exec.Command("diff", "-u", "--label", versionA, "--label", versionB, "-", "-")
	cmd.Stdin = io.MultiReader(
		strings.NewReader(contentA),
		strings.NewReader("\n"),
		strings.NewReader(contentB),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Ignore error since diff returns non-zero for differences
	_ = cmd.Run()

	// If there's something in stderr, it might indicate a real error
	if stderr.Len() > 0 {
		return stdout.String(), fmt.Errorf("diff command warning: %s", stderr.String())
	}

	return stdout.String(), nil
}

// collectFromFiles extracts data from provided files.
func collectFromFiles(cfg Config) (diff, commitMsgs, changelog string, err error) {
	// Read diff file
	if cfg.DiffPath == "" {
		return "", "", "", fmt.Errorf("diff file path not provided")
	}

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
			changelog = fmt.Sprintf("Failed to read CHANGELOG: %v", err)
		} else {
			changelog = string(changelogData)
		}
	} else {
		changelog = "No CHANGELOG provided."
	}

	return diff, commitMsgs, changelog, nil
}
