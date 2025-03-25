# UCD: Undocumented Change Detector

UCD helps security teams detect hidden code changes between software versions using Google's Gemini AI.

It works by comparing code diffs against the stated commit messages and CHANGELOG entries, and then scoring them for maliciousness or attempts to covertly patch a critical security vulnerability.

[![Go Report Card](https://goreportcard.com/badge/github.com/tstromberg/ucd)](https://goreportcard.com/report/github.com/tstromberg/ucd)
[![Go Reference](https://pkg.go.dev/badge/github.com/tstromberg/ucd.svg)](https://pkg.go.dev/github.com/tstromberg/ucd)

> **Note:** Experimental project. Results should be manually verified.

## Example Output

For example, when analyzing the recent supply-chain attack that introduced a [malicious commit to reviewdog/action-setup](https://github.com/reviewdog/action-setup/commit/f0d342), ucd reports:

```log
Undocumented Change Analysis
/Users/t/Downloads/f0d342.patch.txt: v0 → v1

Risk Assessment
• Malicious Code: High (9/10)
• Silent Security Patch: Low (2/10)
• Summary: The install script now contains code to potentially read process memory and extract environment variables, which represents a potential supply chain risk.

Undocumented Changes (1)
• The install script now executes a base64-encoded Python script, `$TEMP/runner_script.py`, which attempts to find the PID of a `Runner.Worker` process, reads its memory maps, and outputs readable regions to standard output, after which the output is further processed by `grep`, `sort`, and `base64` before being stored in the `VALUES` environment variable if running on a github-hosted Linux runner.
   • Malicious Code: High (9/10)
     The script attempts to read memory regions of a process named `Runner.Worker`, which could be used to extract secrets, environment variables, or other sensitive information, and the `VALUES` variable is printed to standard output.
```

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
