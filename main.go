package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"google.golang.org/genai"

	"github.com/tstromberg/ucd/pkg/ucd"
)

var (
	versionA    string
	versionB    string
	diffFile    string
	changesFile string
	programName string
	programDesc string
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
	flag.StringVar(&programName, "name", "", "name of program for context")
	flag.StringVar(&programDesc, "description", "", "description of program for context")

	flag.StringVar(&modelName, "model", "gemini-2.5-flash-preview-04-17", "Gemini model to use")
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
	if len(args) < 2 {
		log.Fatalf("syntax: ucd [file|git] [source]")
	}

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
		RepoURL:     repoURL,
		VersionA:    versionA,
		VersionB:    versionB,
		ProgramName: programName,
		ProgramDesc: programDesc,
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
		ProgramName:    programName,
		ProgramDesc:    programDesc,
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

	// Create client with API key using ClientConfig
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI, // Explicitly set backend
	})
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

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

func outputText(r *ucd.Result) {
	title := color.New(color.FgHiBlue, color.Bold)
	section := color.New(color.FgBlue, color.Bold)
	success := color.New(color.FgHiGreen)
	warning := color.New(color.FgYellow)
	critical := color.New(color.FgHiRed)

	// Risk level indicators with emoji
	riskLevel := func(level int) string {
		icon := "✓"
		if level > 6 {
			icon = "‼️"
		} else if level > 2 {
			icon = "⚠️"
		}

		text := fmt.Sprintf("%s  %2d/10  ", icon, level)

		switch {
		case level <= 2:
			return success.Sprint(text + "Low")
		case level <= 6:
			return warning.Sprint(text + "Medium")
		default:
			return critical.Sprint(text + "High")
		}
	}

	// Header
	if programName != "" {
		title.Printf("\n📊 %s – Change Analysis Report\n", programName)
	} else {
		title.Println("\n📊 Change Analysis Report")
	}
	fmt.Printf("   %s: %s → %s\n", r.Input.Source, versionA, versionB)
	fmt.Println(strings.Repeat("─", 80))

	// Risk Assessment
	if r.Summary != nil {
		section.Println("\n🔍 Risk Assessment")
		fmt.Printf("   Malicious Code Risk:  %s\n", riskLevel(r.Summary.MalwareRisk))
		fmt.Printf("   Silent Security Patch Risk: %s\n", riskLevel(r.Summary.SilentPatch))
		fmt.Printf("\n   Summary: \n     %s\n", wordwrap(r.Summary.Description, 70, "     "))
		fmt.Println(strings.Repeat("─", 80))
	}

	// Changes
	if len(r.UndocumentedChanges) == 0 {
		fmt.Println("\n✅ No undocumented changes detected.\n")
		return
	}

	section.Printf("\n🔎 Undocumented Changes (%d)\n\n", len(r.UndocumentedChanges))

	for i, c := range r.UndocumentedChanges {
		fmt.Printf("%d. %s\n", i+1, wordwrap(c.Description, 70, "   "))

		if c.MalwareRisk > 3 {
			fmt.Printf("\n   Malicious Code Risk: %s\n", riskLevel(c.MalwareRisk))
			fmt.Printf("   %s\n", wordwrap(c.MalwareExplanation, 65, "   "))
		}

		if c.SilentPatch > 3 {
			fmt.Printf("\n   Silent Security Patch Risk: %s\n", riskLevel(c.SilentPatch))
			fmt.Printf("   %s\n", wordwrap(c.SilentExplanation, 65, "   "))
		}
		fmt.Println()
	}
}

func wordwrap(text string, width int, indent ...string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	indentation := "     " // default indentation
	if len(indent) > 0 {
		indentation = indent[0]
	}

	var lines []string
	current := words[0]

	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	lines = append(lines, current)

	return strings.Join(lines, "\n"+indentation)
}

// Helper function to get the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
