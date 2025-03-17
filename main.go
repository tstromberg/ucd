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
	// Setup formatters
	titleFmt := color.New(color.Bold, color.FgCyan).PrintlnFunc()
	sectionFmt := color.New(color.Bold, color.FgBlue).PrintlnFunc()
	highlight := color.New(color.Bold, color.FgYellow).SprintFunc()
	good := color.New(color.FgGreen).SprintFunc()
	warning := color.New(color.FgYellow).SprintFunc()
	danger := color.New(color.FgRed).SprintFunc()

	titleFmt("✨ UCD: Undocumented Change Detector ✨")
	fmt.Printf("Comparing %s → %s\n\n", versionA, versionB)

	// Helper functions for consistent formatting
	getRatingDisplay := func(rating int, isMalware bool) (string, func(a ...interface{}) string) {
		if isMalware {
			// Malware emojis
			switch {
			case rating <= 2:
				return "🔒", good
			case rating <= 6:
				return "⚠️", warning
			default:
				return "🚨", danger
			}
		} else {
			// Security patch emojis
			switch {
			case rating <= 2:
				return "🛡️ ", good
			case rating <= 6:
				return "🔧", warning
			default:
				return "🔓", danger
			}
		}
	}

	// Output summary if available
	if r.Summary != nil {
		sectionFmt("📊 RISK SUMMARY")

		// Display malware risk
		malwareEmoji, malwareColor := getRatingDisplay(r.Summary.MalwareRisk, true)
		fmt.Printf("%s %s - malware\n",
			malwareColor(malwareEmoji),
			malwareColor(fmt.Sprintf("%d/10", r.Summary.MalwareRisk)))

		// Display security patch risk
		securityEmoji, securityColor := getRatingDisplay(r.Summary.SilentPatch, false)
		fmt.Printf("%s %s - silent security patches\n",
			securityColor(securityEmoji),
			securityColor(fmt.Sprintf("%d/10", r.Summary.SilentPatch)))

		fmt.Printf("\n%s\n\n", r.Summary.Description)
	}

	if len(r.Changes) == 0 {
		fmt.Println(good("✅ No undocumented behavioral changes found."))
		return
	}

	// Sort changes by maximum severity
	changes := r.Changes
	sort.Slice(changes, func(i, j int) bool {
		iMax := max(changes[i].MalwareRisk, changes[i].SilentPatch)
		jMax := max(changes[j].MalwareRisk, changes[j].SilentPatch)
		return iMax > jMax
	})

	sectionFmt(fmt.Sprintf("🔍 UNDOCUMENTED BEHAVIOR CHANGES (%d found)", len(changes)))

	for _, change := range changes {
		// Print basic change information
		fmt.Printf("- %s\n", highlight(change.Description))

		// Show malware risk if significant
		if change.MalwareRisk > 5 {
			malwareEmoji, malwareColor := getRatingDisplay(change.MalwareRisk, true)
			fmt.Printf("  %s %s %s\n",
				malwareEmoji,
				malwareColor(fmt.Sprintf("%d/10 malware risk:", change.MalwareRisk)),
				change.MalwareExplanation)
		}

		// Show security patch risk if significant
		if change.SilentPatch > 5 {
			securityEmoji, securityColor := getRatingDisplay(change.SilentPatch, false)
			fmt.Printf("  %s %s %s\n",
				securityEmoji,
				securityColor(fmt.Sprintf("%d/10 hidden security patch:", change.SilentPatch)),
				change.SilentExplanation)
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
