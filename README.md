# UCD - Undocumented Change Detector 🕵️‍♂️

[![Go Report Card](https://goreportcard.com/badge/github.com/tstromberg/ucd)](https://goreportcard.com/report/github.com/tstromberg/ucd)
[![Go Reference](https://pkg.go.dev/badge/github.com/tstromberg/ucd.svg)](https://pkg.go.dev/github.com/tstromberg/ucd)

UCD uses LLM magic to find hidden or undocumented changes between software versions

## Features ✨

- 🔍 Detects changes not mentioned in commit messages/changelogs
- ⚠️ Rates changes from "Benign" to "Definitely Malicious"
- 🌐 Works with Git repos or standalone diff files
- 🤖 Powered by Google's Gemini AI

## Quick Start 🚀

### Setup

```bash
# Install
go install github.com/tstromberg/ucd@latest

# Set your API key
export GEMINI_API_KEY=your_gemini_api_key
```

### Examples

```bash
# Analyze Git repo
ucd git https://github.com/example/repo.git v1.0.0 v1.1.0

# Analyze diff file
ucd diff changes.patch 1.0.0 1.1.0 [changelog.md] [commit-msgs.txt]
```

## Example Output 📊

```
Found 2 undocumented changes:

1. [Suspicious] Added network connection to external server
   Explanation: Undocumented external connection at startup

2. [Silent Security Fix] Fixed buffer overflow in parser
   Explanation: Added bounds checking for security
```

## Code Usage 💻

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tstromberg/ucd/pkg/ucd"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

func main() {
	ctx := context.Background()

	// Setup
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Analyze
	service := ucd.NewService(ucd.NewAnalyzer(client))
	result, err := service.AnalyzeGit(ctx,
		"https://github.com/example/repo.git",
		"v1.0.0",
		"v1.1.0",
	)

	// Show results
	fmt.Println(result.Format())
}
```

## Requirements 📋

- Go 1.18+
- Gemini API key
- Git (for repository analysis)

## Contributing 🤝

PRs welcome! Let's make software updates safer together.
