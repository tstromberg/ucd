package ucd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Common errors returned by the package.
var (
	ErrNoChangelog    = errors.New("no changelog file found")
	ErrGitNotFound    = errors.New("git command not found")
	ErrDiffNotFound   = errors.New("diff file not found")
	ErrMissingVersion = errors.New("version identifiers are required")
)

// Collector collects code change data for analysis.
type Collector struct {
	// Data source configuration
	repoURL       string
	diffPath      string
	changelogPath string
	commitMsgs    string

	// Version identifiers
	versionA string
	versionB string

	// Git repository handling
	tempDir string
	cleanup func()
}

// NewCollector creates a new code Collector with options.
func NewCollector(opts ...Option) *Collector {
	a := &Collector{}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Option configures an Collector.
type Option func(*Collector)

// WithGit sets up Git repository analysis.
func WithGit(repoURL, versionA, versionB string) Option {
	return func(a *Collector) {
		a.repoURL = repoURL
		a.versionA = versionA
		a.versionB = versionB
	}
}

// WithDiff sets up direct diff file analysis.
func WithDiff(diffPath, versionA, versionB string) Option {
	return func(a *Collector) {
		a.diffPath = diffPath
		a.versionA = versionA
		a.versionB = versionB
	}
}

// WithChangelog adds a changelog file.
func WithChangelog(path string) Option {
	return func(a *Collector) {
		a.changelogPath = path
	}
}

// WithCommitMessages adds commit messages directly.
func WithCommitMessages(messages string) Option {
	return func(a *Collector) {
		a.commitMsgs = messages
	}
}

// Cleanup releases resources used by the Collector.
func (a *Collector) Cleanup() {
	if a.cleanup != nil {
		a.cleanup()
		a.cleanup = nil
	}
}

// Collect gathers all necessary data for analysis.
func (a *Collector) Collect() (*AnalysisData, error) {
	defer a.Cleanup()

	if a.versionA == "" || a.versionB == "" {
		return nil, ErrMissingVersion
	}

	var diff, commitMsgs, changelog string
	var err error

	if a.repoURL != "" {
		// Git repository analysis mode
		if err := a.setupRepo(); err != nil {
			return nil, err
		}

		diff, commitMsgs, changelog, err = a.collectFromGit()
	} else {
		// Direct file analysis mode
		diff, commitMsgs, changelog, err = a.collectFromFiles()
	}

	if err != nil {
		return nil, err
	}

	return &AnalysisData{
		Diff:           diff,
		CommitMessages: commitMsgs,
		Changelog:      changelog,
		VersionA:       a.versionA,
		VersionB:       a.versionB,
	}, nil
}

// setupRepo prepares a local git repository.
func (a *Collector) setupRepo() error {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("%w: %v", ErrGitNotFound, err)
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "ucd-git-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	a.tempDir = tempDir
	a.cleanup = func() { os.RemoveAll(tempDir) }

	// Clone the repository
	cmd := exec.Command("git", "clone", "--quiet", a.repoURL, tempDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s: %w", bytes.TrimSpace(out), err)
	}

	return nil
}

// collectFromGit extracts data from a Git repository.
func (a *Collector) collectFromGit() (diff, commitMsgs, changelog string, err error) {
	// Generate diff
	cmd := exec.Command("git", "diff", a.versionA, a.versionB)
	cmd.Dir = a.tempDir
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("git diff failed: %w", err)
	}
	diff = string(out)

	// Extract commit messages
	cmd = exec.Command("git", "log", "--pretty=format:%s", a.versionA+".."+a.versionB)
	cmd.Dir = a.tempDir
	out, err = cmd.Output()
	if err != nil {
		return "", "", "", fmt.Errorf("git log failed: %w", err)
	}
	commitMsgs = string(out)

	// Extract changelog
	changelog, _ = a.getChangelogFromGit() // Non-fatal if fails
	if changelog == "" {
		changelog = "No CHANGELOG found."
	}

	return diff, commitMsgs, changelog, nil
}

// getChangelogFromGit extracts changelog differences from a Git repository.
func (a *Collector) getChangelogFromGit() (string, error) {
	// Look for common changelog filenames
	patterns := []string{
		"CHANGELOG.md", "CHANGELOG.txt", "CHANGELOG",
		"changelog.md", "changelog.txt", "changelog",
		"CHANGES.md", "changes.md",
	}

	var changelogFile string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(a.tempDir, pattern))
		if len(matches) > 0 {
			changelogFile = matches[0]
			break
		}
	}

	if changelogFile == "" {
		return "", ErrNoChangelog
	}

	relPath, err := filepath.Rel(a.tempDir, changelogFile)
	if err != nil {
		relPath = filepath.Base(changelogFile)
	}

	// Get changelog at version A
	cmdA := exec.Command("git", "show", a.versionA+":"+relPath)
	cmdA.Dir = a.tempDir
	contentA, err := cmdA.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get changelog at version %s: %w", a.versionA, err)
	}

	// Get changelog at version B
	cmdB := exec.Command("git", "show", a.versionB+":"+relPath)
	cmdB.Dir = a.tempDir
	contentB, err := cmdB.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get changelog at version %s: %w", a.versionB, err)
	}

	// Create a diff of the changelog
	cmd := exec.Command("diff", "-u", "--label", a.versionA, "--label", a.versionB, "-", "-")
	cmd.Stdin = io.MultiReader(bytes.NewReader(contentA), strings.NewReader("\n"), bytes.NewReader(contentB))
	output, _ := cmd.Output() // Ignore error since diff returns non-zero for differences

	return string(output), nil
}

// collectFromFiles extracts data from provided files.
func (a *Collector) collectFromFiles() (diff, commitMsgs, changelog string, err error) {
	// Read diff file
	if a.diffPath == "" {
		return "", "", "", ErrDiffNotFound
	}

	diff, err = readFile(a.diffPath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read diff file: %w", err)
	}

	// Use provided commit messages
	commitMsgs = a.commitMsgs

	// Read changelog if provided
	if a.changelogPath != "" {
		changelog, err = readFile(a.changelogPath)
		if err != nil {
			changelog = "Failed to read CHANGELOG."
		}
	} else {
		changelog = "No CHANGELOG provided."
	}

	return diff, commitMsgs, changelog, nil
}

// readFile reads the content of a file.
func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
