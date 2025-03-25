# UCD: Undocumented Change Detector

UCD helps security teams detect hidden code changes between software versions using Google's Gemini AI.

It works by comparing code diffs against the stated commit messages and CHANGELOG entries, and then scoring them for maliciousness or attempts to covertly patch a critical security vulnerability.

[![Go Report Card](https://goreportcard.com/badge/github.com/tstromberg/ucd)](https://goreportcard.com/report/github.com/tstromberg/ucd)
[![Go Reference](https://pkg.go.dev/badge/github.com/tstromberg/ucd.svg)](https://pkg.go.dev/github.com/tstromberg/ucd)

> **Note:** Experimental project. Results should be manually verified.

## Example Output

For example, when analyzing the recent supply-chain attack that introduced a [malicious commit to reviewdog/action-setup](https://github.com/reviewdog/action-setup/commit/f0d342), ucd reports:

![screenshot](images/screenshot.png?raw=true "screenshot")

## Install

```bash
go install github.com/tstromberg/ucd@latest
```

## Usage

```bash
# Set API key
export GEMINI_API_KEY=YOUR_API_KEY

# Analyze a Git repository
ucd git https://github.com/org/repo.git

# Compare specific versions
ucd -a v0.25.3 -b v0.25.4 git https://github.com/org/repo.git

# Analyze a local diff file
ucd file changes.patch

# Output in JSON format
ucd -json git https://github.com/org/repo.git
```

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
* Gemini API Key
* Git (for repository analysis)
