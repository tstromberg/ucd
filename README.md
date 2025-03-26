# UCD: Undocumented Change Detector 🕵️‍♀️

[![Go Report Card](https://goreportcard.com/badge/github.com/tstromberg/ucd)](https://goreportcard.com/report/github.com/tstromberg/ucd)
[![Go Reference](https://pkg.go.dev/badge/github.com/tstromberg/ucd.svg)](https://pkg.go.dev/github.com/tstromberg/ucd)

UCD is your AI-powered security sidekick that helps detect sneaky code changes between software versions using Google's Gemini AI.

It works by comparing code diffs against commit messages and CHANGELOG entries, then scoring them for potential maliciousness or attempts to silently patch security vulnerabilities. Think of it as a lie detector for your code!

> **Note:** Experimental project. Raw unpolished interface.

## Example Output

For example, when analyzing the recent supply-chain attack that introduced a [malicious commit to reviewdog/action-setup](https://github.com/reviewdog/action-setup/commit/f0d342), ucd reports:

![screenshot](images/screenshot.png?raw=true "screenshot")

Here's what benign undocumented changes look like for the `apko` git repo:

![screenshot](images/screenshot2.png?raw=true "screenshot")

## Install

```bash
go install github.com/tstromberg/ucd@latest
```

## Usage

```bash
# Set API key (or pass it with -api-key flag)
export GEMINI_API_KEY=YOUR_API_KEY

# Analyze a Git repository
ucd git https://github.com/org/repo.git

# Compare specific versions
ucd -a v0.25.3 -b v0.25.4 git https://github.com/org/repo.git

# Analyze a local diff file
ucd file changes.patch

# Add commit messages and changelog for better results
ucd -commit-messages commits.txt -changelog changes.md file changes.patch

# Use a different Gemini model
ucd -model gemini-2.0-pro git https://github.com/org/repo.git

# Get debug information
ucd -debug git https://github.com/org/repo.git

# Output in JSON format for further processing
ucd -json git https://github.com/org/repo.git
```

## Available Options

| Flag | Description |
|------|-------------|
| `-a` | Version A (old version), defaults to "v0" |
| `-b` | Version B (new version), defaults to "v1" |
| `-diff` | File containing unified diff |
| `-commit-messages` | File containing commit messages |
| `-changelog` | File containing changelog entries |
| `-api-key` | Google API key for Gemini (alternatively use GEMINI_API_KEY env var) |
| `-model` | Gemini model to use (default: "gemini-2.0-flash") |
| `-json` | Output results in JSON format |
| `-debug` | Enable debug output |

## Go API Example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
	"github.com/tstromberg/ucd/pkg/ucd"
	"google.golang.org/api/option"
)

func main() {
	// Collect data
	data, err := ucd.Collect(ucd.Config{
		RepoURL:  "https://github.com/example/repo",
		VersionA: "v1.0.0",
		VersionB: "v1.1.0",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Analyze changes
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey("YOUR_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	result, err := ucd.AnalyzeChanges(ctx, client, data, "gemini-2.0-flash")
	if err != nil {
		log.Fatal(err)
	}

	// Process results
	fmt.Printf("Found %d undocumented changes\n", len(result.UndocumentedChanges))
}
```

## Requirements

* Go 1.18+
* Gemini API Key (get one from [Google AI Studio](https://ai.google.dev/))
* Git (for repository analysis)
