# UCD: Undocumented Change Detector

[![Go Report Card](https://goreportcard.com/badge/github.com/tstromberg/ucd)](https://goreportcard.com/report/github.com/tstromberg/ucd)
[![Go Reference](https://pkg.go.dev/badge/github.com/tstromberg/ucd.svg)](https://pkg.go.dev/github.com/tstromberg/ucd)

UCD is an experimental AI-powered tool that identifies hidden code changes between software versions. It helps security teams detect undocumented modifications that might introduce security risks.

> **Note:** This is an experimental project. Analysis results may vary in accuracy and should be manually verified.

## Features

* Detects hidden code changes missed in documentation
* Assesses risk for potential malware and silent security patches
* Supports Git repositories and diff files
* Uses Google's Gemini AI for analysis
* Provides JSON output for integration with other tools

## Quick Start

```bash
go install github.com/tstromberg/ucd@latest
export GEMINI_API_KEY=YOUR_API_KEY  # From Google AI Studio
ucd --a v1.0.0 --b v1.1.0 git https://github.com/repo/example.git
```

## Usage

```bash
# Analyze Git repository
ucd --a v0.25.3 --b v0.25.4 git https://github.com/org/repo.git

# Analyze diff file
ucd diff changes.patch

# Use with additional options
ucd --json --model gemini-2.0-flash git --a v1.0 --b v1.1 https://github.com/org/repo.git
```

## Key Options

```
--a string          Old version (default "v0")
--b string          New version (default "v1")
--diff string       Unified diff file
--commit-messages   Commit messages file
--changelog         Changelog file
--api-key string    Gemini API key
--model string      AI model (default "gemini-2.0-flash")
--json              Output as JSON
--debug             Enable debug output
```

## Requirements

* Go 1.18+
* Gemini API Key
* Git (for repository analysis)

## Contributing

Contributions welcome! As this is an experimental tool, we value feedback and improvements.
