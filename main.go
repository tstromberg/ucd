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
	flag.StringVar(&modelName, "model", "", "Gemini model to use (default: gemini-2.0-flash)")
	flag.BoolVar(&jsonOutput, "json", false, "Output results in JSON format")
	flag.BoolVar(&debugMode, "debug", false, "Enable debug output")
}

func main() {
	flag.Parse()
	repoURL := ""
	// Check for positional args for repo URL
	args := flag.Args()
	if len(args) >= 2 && args[0] == "git" {
		repoURL = args[1]
	}

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

	var data *ucd.AnalysisData
	var err error

	if repoURL != "" {
		// Git repository mode
		config := ucd.Config{
			RepoURL:  repoURL,
			VersionA: versionA,
			VersionB: versionB,
		}

		if debugMode {
			fmt.Fprintf(os.Stderr, "Analyzing Git repository: %s between %s and %s\n",
				repoURL, versionA, versionB)
		}

		data, err = ucd.Collect(config)
		if err != nil {
			log.Fatalf("Error collecting data from Git repository: %v", err)
		}
	} else {
		// File mode - use existing logic
		diffContent, err := readFileOrStdin(diffFile)
		if err != nil {
			log.Fatalf("Error reading diff: %v", err)
		}

		commitMessages, err := readFileOrStdin(changesFile)
		if err != nil {
			log.Fatalf("Error reading commit messages: %v", err)
		}

		changelog, err := readFileOrStdin(changelogFile)
		if err != nil {
			log.Fatalf("Error reading changelog: %v", err)
		}

		// Create analysis data
		data = &ucd.AnalysisData{
			VersionA:       versionA,
			VersionB:       versionB,
			Diff:           diffContent,
			CommitMessages: commitMessages,
			Changelog:      changelog,
		}

		// Debug output
		if debugMode {
			fmt.Fprintf(os.Stderr, "Analyzing diff between %s and %s\n", versionA, versionB)
			fmt.Fprintf(os.Stderr, "Diff length: %d bytes\n", len(diffContent))
			fmt.Fprintf(os.Stderr, "Commit messages length: %d bytes\n", len(commitMessages))
			fmt.Fprintf(os.Stderr, "Changelog length: %d bytes\n", len(changelog))
		}
	}

	// Debug output for git mode
	if debugMode && repoURL != "" {
		fmt.Fprintf(os.Stderr, "Diff length: %d bytes\n", len(data.Diff))
		fmt.Fprintf(os.Stderr, "Commit messages length: %d bytes\n", len(data.CommitMessages))
		fmt.Fprintf(os.Stderr, "Changelog length: %d bytes\n", len(data.Changelog))
	}

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

	// Output in requested format
	if jsonOutput {
		outputJSON(result)
	} else {
		outputText(result)
	}
}

// readFileOrStdin reads content from a file or stdin if filename is empty.
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

// outputText prints the result in human-readable format.
func outputText(r *ucd.Result) {
	// Output summary if available
	if r.Summary != nil {
		fmt.Printf("Summary:\n--------\n")
		fmt.Printf("Risk rating: %d/10: %s\n", r.Summary.Rating, r.Summary.Description)
		//		fmt.Printf("* Explanation: %s\n\n", r.Summary.Explanation)
	}

	if len(r.Changes) == 0 {
		fmt.Println("\nNo undocumented changes found.")
		return
	}

	// Sort changes from most severe to least severe
	changes := make([]ucd.Assessment, len(r.Changes))
	copy(changes, r.Changes)
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Rating > changes[j].Rating
	})

	fmt.Printf("\n%d undocumented changes:\n", len(changes))
	fmt.Printf("--------------------------\n")

	for _, change := range changes {
		fmt.Printf("* [%d/10] %s\n", change.Rating, change.Description)
		// Uncomment to show explanations
		// fmt.Printf("   Explanation: %s\n\n", change.Explanation)
	}
}
