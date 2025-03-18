package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/fatih/color"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"github.com/tstromberg/ucd/pkg/ucd"
)

var (
	versionA    string
	versionB    string
	diffFile    string
	changesFile string
	commitsFile string
	apiKey      string
	modelName   string
	jsonOutput  bool
	debugMode   bool
)

func init() {
	flag.StringVar(&versionA, "a", "v0", "Version A (old version)")
	flag.StringVar(&versionB, "b", "v1", "Version B (new version)")
	flag.StringVar(&diffFile, "diff", "", "File containing unified diff")
	flag.StringVar(&commitsFile, "commit-messages", "", "File containing commit messages")
	flag.StringVar(&changesFile, "changelog", "", "File containing changelog entries")
	flag.StringVar(&apiKey, "api-key", "", "Google API key for Gemini")
	flag.StringVar(&modelName, "model", "gemini-2.0-flash", "Gemini model to use")
	flag.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
}

func main() {
	flag.Parse()

	// Check for API key
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			log.Fatal("API key is required. Set it with -api-key flag or GEMINI_API_KEY environment variable.")
		}
	}

	data := collectData()
	result := analyzeData(data)
	outputResult(result)
}

// collectData gathers the required information for analysis
func collectData() *ucd.AnalysisData {
	args := flag.Args()
	source := args[1]
	var data *ucd.AnalysisData
	var err error

	if args[0] == "git" {
		// Git repository mode
		data, err = collectFromGit(source)
	} else {
		// File mode
		data, err = collectFromFiles(source)
	}

	if err != nil {
		log.Fatalf("Error collecting data: %v", err)
	}

	// Debug output
	if debugMode {
		fmt.Fprintf(os.Stderr, "Diff length: %d bytes\n", len(data.Diff))
		fmt.Fprintf(os.Stderr, "Commit messages length: %d bytes\n", len(data.CommitMessages))
		fmt.Fprintf(os.Stderr, "Changelog length: %d bytes\n", len(data.Changelog))
	}

	return data
}

// collectFromGit gathers data from a Git repository
func collectFromGit(repoURL string) (*ucd.AnalysisData, error) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "Analyzing Git repository: %s between %s and %s\n",
			repoURL, versionA, versionB)
	}

	config := ucd.Config{
		RepoURL:  repoURL,
		VersionA: versionA,
		VersionB: versionB,
	}

	return ucd.Collect(config)
}

// collectFromFiles gathers data from the specified files
func collectFromFiles(diffFile string) (*ucd.AnalysisData, error) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "Analyzing diff between %s and %s\n", versionA, versionB)
	}

	diff, err := os.ReadFile(diffFile)
	if err != nil {
		return nil, fmt.Errorf("readfile: %w", err)
	}
	var commits []byte
	if commitsFile != "" {
		commits, err = os.ReadFile(commitsFile)
		if err != nil {
			return nil, fmt.Errorf("readfile: %w", err)
		}
	}

	var changelog []byte
	if changesFile != "" {
		changelog, err = os.ReadFile(changesFile)
		if err != nil {
			return nil, fmt.Errorf("readfile: %w", err)
		}
	}

	return &ucd.AnalysisData{
		VersionA:       versionA,
		VersionB:       versionB,
		Source:         diffFile,
		Diff:           string(diff),
		CommitMessages: string(commits),
		Changelog:      string(changelog),
	}, nil
}

// analyzeData processes the collected data using the AI model
func analyzeData(data *ucd.AnalysisData) *ucd.Result {
	// Set up AI client
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}
	defer client.Close()

	// Analyze the changes
	result, err := ucd.AnalyzeChanges(ctx, client, data, modelName)
	if err != nil {
		log.Fatalf("Error analyzing changes: %v", err)
	}

	return result
}

// outputResult presents the analysis findings in the requested format
func outputResult(result *ucd.Result) {
	if jsonOutput {
		outputJSON(result)
	} else {
		outputText(result)
	}
}

// outputJSON prints the result as formatted JSON.
func outputJSON(result *ucd.Result) {
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling to JSON: %v", err)
	}
	fmt.Println(string(jsonData))
}

// outputText prints the result in human-readable format with colors and emojis.
func outputText(r *ucd.Result) {
	// Setup color formatters - Apple uses subtle colors
	title := color.New(color.FgHiBlue, color.Bold)
	section := color.New(color.FgBlue, color.Bold)
	highlight := color.New()
	// color.FgBlack, color.Bold) // Apple often uses bold black for emphasis
	success := color.New(color.FgHiGreen)
	warning := color.New(color.FgYellow)
	critical := color.New(color.FgHiRed)

	// Print header in Apple style - clean and minimal
	title.Println("Undocumented Change Analysis")
	fmt.Printf("%s: %s → %s\n\n", r.Input.Source, versionA, versionB)

	// Apple-style risk indicators - prefer text over emoji for enterprise tools
	riskLevel := func(level int) string {
		switch {
		case level <= 2:
			return success.Sprintf("Low (%d/10)", level)
		case level <= 6:
			return warning.Sprintf("Medium (%d/10)", level)
		default:
			return critical.Sprintf("High (%d/10)", level)
		}
	}

	if r.Summary != nil {
		section.Println("Risk Assessment")
		fmt.Printf("• Malicious Code: %s\n", riskLevel(r.Summary.MalwareRisk))
		fmt.Printf("• Silent Security Patch: %s\n", riskLevel(r.Summary.SilentPatch))
		fmt.Printf("• Summary: %s\n\n", r.Summary.Description)
	}

	// No changes case - clean confirmation
	if len(r.UndocumentedChanges) == 0 {
		fmt.Println(success.Sprint("No undocumented changes detected."))
		return
	}

	// Sort changes by severity
	sort.Slice(r.UndocumentedChanges, func(i, j int) bool {
		return max(r.UndocumentedChanges[i].MalwareRisk, r.UndocumentedChanges[i].SilentPatch) >
			max(r.UndocumentedChanges[j].MalwareRisk, r.UndocumentedChanges[j].SilentPatch)
	})

	section.Printf("Undocumented Changes (%d)\n", len(r.UndocumentedChanges))

	for _, c := range r.UndocumentedChanges {
		fmt.Printf("• %s\n", highlight.Sprint(c.Description))

		if c.MalwareRisk > 3 {
			fmt.Printf("   • Malicious Code: %s\n     %s\n",
				riskLevel(c.MalwareRisk), c.MalwareExplanation)
		}

		if c.SilentPatch > 3 {
			fmt.Printf("   • Security Patch: %s\n     %s\n",
				riskLevel(c.SilentPatch), c.SilentExplanation)
		}
	}
}

// Helper function to get the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
