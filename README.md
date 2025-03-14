# 🕵️‍♂️ UCD: Undocumented Change Detector

[![Go Report Card](https://goreportcard.com/badge/github.com/tstromberg/ucd)](https://goreportcard.com/report/github.com/tstromberg/ucd)
[![Go Reference](https://pkg.go.dev/badge/github.com/tstromberg/ucd.svg)](https://pkg.go.dev/github.com/tstromberg/ucd)

**Don't let undocumented changes sneak by!** UCD, the Undocumented Change Detector, is your AI-powered tool to find hidden code shifts between software versions. Think of it as a detective for your codebase. 🐕‍🦺

## ✨ Key Features

* **🔍 Detects Hidden Changes:**  Uncovers code modifications missed in commit messages and changelogs.
* **⚠️ Rates Risk Level:**  Classifies changes by potential risk – from minor to significant. 🟢🟡🔴
* **🌐 Git & Diff Support:** Analyze Git repositories or standard diff files.
* **🤖 Powered by Gemini AI:** Uses Google's Gemini for intelligent code analysis.
* **📦 JSON Output Option:**  Get results in JSON format for scripting and automation.

##  🚀 Quick Start

```bash
go install github.com/tstromberg/ucd@latest
export GEMINI_API_KEY=YOUR_API_KEY  # Get your API key from Google AI Studio! 🔑
ucd --a v0.25.3 --b v0.25.4 git https://github.com/chainguard-dev/apko.git
```

##  🕹️ How to Use - Examples

```bash
ucd git https://github.com/repo/example.git v1.0.0 v1.1.0   # Analyze a Git repository
ucd -diff changes.patch -a v1.0.0 -b v1.1.0               # Check a diff file
ucd -json git ...                                        # Output in JSON format
ucd -debug git ...                                       # Enable debug output
```

**Important Flags:** `-a versionA`, `-b versionB`, `-diff file`

## 📊 Example Output -  The Analysis Report

```
✨ UCD: Undocumented Change Detector ✨
Comparing v1.0.0 → v1.1.0

📊 SUMMARY:
🟡 5/10 - Moderate risk. Review changes.

🔍 UNDOCUMENTED CHANGES (2 found)
🔴 [8/10] Added network connection to external server
🟢 [2/10] Minor text update in help message
```

##  📋 Requirements

* Go 1.18+
* Gemini API Key
* Git (for Git repository analysis)

##  🤝 Contribute

Pull requests are welcome! Help make software updates more transparent. 🎉
