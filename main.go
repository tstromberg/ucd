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

	// Set up generative AI client
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY environment variable is required")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create AI client: %v", err)
	}
	defer client.Close()

	// Create service with AI analyzer
	aiAnalyzer := ucd.NewAnalyzer(client)
	service := ucd.NewService(aiAnalyzer)

	// Process command-line arguments
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "git":
		if len(os.Args) < 5 {
			log.Fatal("Usage: go run main.go git <repo-url> <version-a> <version-b>")
		}

		result, err := service.AnalyzeGit(ctx, os.Args[2], os.Args[3], os.Args[4])
		if err != nil {
			log.Fatalf("Git analysis failed: %v", err)
		}

		fmt.Println(result.Format())

	case "diff":
		if len(os.Args) < 5 {
			log.Fatal("Usage: go run main.go diff <diff-file> <version-a> <version-b> [changelog-file] [commit-messages]")
		}

		var changelog, commits string
		if len(os.Args) > 5 {
			changelog = os.Args[5]
		}
		if len(os.Args) > 6 {
			commits = os.Args[6]
		}

		result, err := service.AnalyzeDiff(ctx, os.Args[2], os.Args[3], os.Args[4], changelog, commits)
		if err != nil {
			log.Fatalf("Diff analysis failed: %v", err)
		}

		fmt.Println(result.Format())

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  go run main.go git <repo-url> <version-a> <version-b>")
	fmt.Println("  go run main.go diff <diff-file> <version-a> <version-b> [changelog-file] [commit-messages]")
}
