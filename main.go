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
	versionA      string
	versionB      string
	diffFile      string
	changesFile   string
	changelogFile string
	apiKey        string
	modelName     string
	jsonOutput    bool
	debugMode     bool
)

func init() {
	flag.StringVar(&versionA, "a", "", "Version A (old version)")
	flag.StringVar(&versionB, "b", "", "Version B (new version)")
	flag.StringVar(&diffFile, "diff", "", "File containing unified diff")
	flag.StringVar(&changesFile, "commit-messages", "", "File containing commit messages")
	flag.StringVar(&changelogFile, "changelog", "", "File containing changelog entries")
	flag.StringVar(&apiKey, "api-key", "", "Google API key for Gemini")
	flag.StringVar(&modelName, "model", "gemini-2.0-flash", "Gemini model to use")
	flag.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
}

func main() {
	flag.Parse()

	validateInputs()
	data := collectData()
	result := analyzeData(data)
	outputResult(result)
}

// validateInputs checks required parameters and environment variables
func validateInputs() {
	// Check for API key
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			log.Fatal("API key is required. Set it with -api-key flag or GEMINI_API_KEY environment variable.")
		}
	}

	// Validate required parameters
	if versionA == "" || versionB == "" {
		log.Fatal("Both versions A and B are required.")
	}
}

// collectData gathers the required information for analysis
func collectData() *ucd.AnalysisData {
	args := flag.Args()
	repoURL := ""
	if len(args) >= 2 && args[0] == "git" {
		repoURL = args[1]
	}

	var data *ucd.AnalysisData
	var err error

	if repoURL != "" {
		// Git repository mode
		data, err = collectFromGit(repoURL)
	} else {
		// File mode
		data, err = collectFromFiles()
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
func collectFromFiles() (*ucd.AnalysisData, error) {
	if debugMode {
		fmt.Fprintf(os.Stderr, "Analyzing diff between %s and %s\n", versionA, versionB)
	}

	diffContent, err := readFileOrStdin(diffFile)
	if err != nil {
		return nil, fmt.Errorf("reading diff: %w", err)
	}

	commitMessages, err := readFileOrStdin(changesFile)
	if err != nil {
		return nil, fmt.Errorf("reading commit messages: %w", err)
	}

	changelog, err := readFileOrStdin(changelogFile)
	if err != nil {
		return nil, fmt.Errorf("reading changelog: %w", err)
	}

	return &ucd.AnalysisData{
		VersionA:       versionA,
		VersionB:       versionB,
		Diff:           diffContent,
		CommitMessages: commitMessages,
		Changelog:      changelog,
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

// readFileOrStdin reads content from a file or returns empty string if filename is empty.
func readFileOrStdin(filename string) (string, error) {
	if filename == "" {
		return "", nil
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", filename, err)
	}
	return string(content), nil
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
	title := color.New(color.Bold, color.FgCyan).PrintlnFunc()
	sectionTitle := color.New(color.Bold, color.FgBlue).PrintlnFunc()
	highlight := color.New(color.Bold, color.FgYellow).SprintfFunc()
	good := color.New(color.FgGreen).SprintfFunc()
	warning := color.New(color.FgYellow).SprintfFunc()
	danger := color.New(color.FgRed).SprintfFunc()

	title("✨ UCD: Undocumented Change Detector ✨")
	fmt.Printf("Comparing %s → %s\n\n", versionA, versionB)

	// Output summary if available
	if r.Summary != nil {
		sectionTitle("📊 SUMMARY")

		// Choose emoji based on rating
		var ratingDisplay string
		switch {
		case r.Summary.Rating <= 2:
			ratingDisplay = good("🟢 %d/10", r.Summary.Rating)
		case r.Summary.Rating <= 6:
			ratingDisplay = warning("🟡 %d/10", r.Summary.Rating)
		default:
			ratingDisplay = danger("🔴 %d/10", r.Summary.Rating)
		}

		fmt.Printf("%s - %s\n\n", ratingDisplay, r.Summary.Description)
	}

	if len(r.Changes) == 0 {
		fmt.Println(good("✅ No undocumented changes found."))
		return
	}

	// Sort changes from most severe to least severe
	changes := make([]ucd.Assessment, len(r.Changes))
	copy(changes, r.Changes)
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Rating > changes[j].Rating
	})

	sectionTitle(fmt.Sprintf("🔍 UNDOCUMENTED CHANGES (%d found)", len(changes)))

	for _, change := range changes {
		// Choose emoji based on rating
		var icon string
		switch {
		case change.Rating <= 2:
			icon = "✅"
		case change.Rating <= 6:
			icon = "⚠️"
		default:
			icon = "🚨"
		}

		// Choose color based on rating
		var ratingDisplay string
		switch {
		case change.Rating <= 2:
			ratingDisplay = good("%d/10", change.Rating)
		case change.Rating <= 6:
			ratingDisplay = warning("%d/10", change.Rating)
		default:
			ratingDisplay = danger("%d/10", change.Rating)
		}

		fmt.Printf("%s [%s] %s\n", icon, ratingDisplay, highlight(change.Description))
		//	fmt.Printf("   %s\n\n", change.Explanation)
	}
}
