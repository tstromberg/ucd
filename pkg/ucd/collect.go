package ucd

import (
	"bytes"
	"fmt"
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
	ProgramName   string
	ProgramDesc   string

	// Optional version identifiers
	VersionA string
	VersionB string
}

// AnalysisData contains collected code change information.
type AnalysisData struct {
	Source         string
	Diff           string
	CommitMessages string
	Changelog      string
	VersionA       string
	VersionB       string
	ProgramName    string
	ProgramDesc    string
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
		Source:         cfg.RepoURL,
		ProgramName:    cfg.ProgramName,
		ProgramDesc:    cfg.ProgramDesc,
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
	//	defer os.RemoveAll(tempDir)

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
	// Find the first matching changelog file
	var changelogFile string
	for _, pattern := range []string{"CHANGELOG*", "changelog*", "CHANGES.md", "changes.md", "RELNOTES*"} {
		if matches, _ := filepath.Glob(filepath.Join(repoDir, pattern)); len(matches) > 0 {
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

	// Create temporary files for both versions
	fileA, err := os.CreateTemp("", "changelog-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(fileA.Name())

	fileB, err := os.CreateTemp("", "changelog-*")
	if err != nil {
		os.Remove(fileA.Name())
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(fileB.Name())

	// Write contents to temporary files
	if err := os.WriteFile(fileA.Name(), []byte(contentA), 0o644); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := os.WriteFile(fileB.Name(), []byte(contentB), 0o644); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	// Run diff command on temp files
	cmd := exec.Command("diff", "-u", "--label", versionA, "--label", versionB, fileA.Name(), fileB.Name())
	out, _ := cmd.CombinedOutput()

	// Extract only the lines starting with "+" and remove the "+" prefix
	var newLines []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			// Skip the diff header line (starting with +++)
			newLines = append(newLines, line[1:])
		}
	}

	return strings.Join(newLines, "\n"), nil
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
