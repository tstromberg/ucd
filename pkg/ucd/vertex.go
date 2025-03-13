package ucd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/generative-ai-go/genai"
)

// Common errors returned by the AI module.
var (
	ErrNoResponse  = errors.New("no response from AI model")
	ErrInvalidJSON = errors.New("invalid JSON response")
)

// Analyzer performs AI-based analysis of code changes.
type Analyzer struct {
	client *genai.Client
	model  string
}

// NewAnalyzer creates a new AI Analyzer with the specified client.
func NewAnalyzer(client *genai.Client) *Analyzer {
	return &Analyzer{
		client: client,
		model:  "gemini-2.0-flash",
	}
}

// SetModel changes the model used for analysis.
func (a *Analyzer) SetModel(model string) {
	a.model = model
}

// Analyze implements the AnalyzeAIAnalyzer interface.
func (a *Analyzer) Analyze(ctx context.Context, data *AnalysisData) (*Result, error) {
	prompt := createPrompt(data)

	// Truncate if too long
	const maxPromptLength = 100000
	if len(prompt) > maxPromptLength {
		prompt = prompt[:maxPromptLength]
	}

	// Send to AI model
	model := a.client.GenerativeModel(a.model)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, ErrNoResponse
	}

	// Parse response
	responseText := resp.Candidates[0].Content.Parts[0].(genai.Text)
	return parseResponse(string(responseText))
}

// createPrompt generates the prompt for AI analysis.
func createPrompt(data *AnalysisData) string {
	return fmt.Sprintf(`You are a security expert analyzing changes between two versions of a software package.
I will provide:
1. A unified diff between version %s and %s
2. Commit messages describing changes (if available)
3. Changelog entries (if available)

Your task is to identify any behavioral changes in the code that do not appear to support the changes mentioned in the commit messages or changelog.
Focus especially on security-relevant or potentially malicious changes that aren't documented. Do not mention changes that appear to be refactors for readability or performance improvements.

UNIFIED DIFF:
%s

COMMIT MESSAGES:
%s

CHANGELOG CHANGES:
%s

For each undocumented behavioral change you identify:
1. Briefly describe the undocumented change in 15 words or less.
2. Rate each change as exactly one of: "Benign", "Silent Security Fix", "Suspicious", "Possibly Malicious", or "Definitely Malicious"
3. Provide a brief explanation for your rating of each change

Format your response as a JSON array of objects with these properties:
- "description": A brief description of the undocumented change
- "rating": One of "Benign", "Silent Security Fix", "Suspicious", "Possibly Malicious", or "Definitely Malicious"
- "explanation": Your explanation for the rating

If there are no undocumented changes, return an empty array.
`, data.VersionA, data.VersionB, data.Diff, data.CommitMessages, data.Changelog)
}

// parseResponse extracts structured information from the AI response.
func parseResponse(response string) (*Result, error) {
	// Extract JSON from response
	jsonText, err := extractJSON(response)
	if err != nil {
		return nil, err
	}

	// Check for empty array
	if jsonText == "[]" {
		return &Result{}, nil
	}

	// Parse the JSON
	var changes []struct {
		Description string `json:"description"`
		Rating      string `json:"rating"`
		Explanation string `json:"explanation"`
	}

	if err := json.Unmarshal([]byte(jsonText), &changes); err != nil {
		return nil, fmt.Errorf("%w: %v: %s", ErrInvalidJSON, err, jsonText)
	}

	// Convert to our result format
	result := &Result{
		Changes: make([]Change, 0, len(changes)),
	}

	for _, item := range changes {
		if strings.Contains(item.Description, ".gitignore") {
			continue
		}
		if strings.Contains(item.Description, "README") {
			continue
		}
		if strings.Contains(item.Description, ".github/") {
			continue
		}
		if strings.Contains(item.Description, "CI ") {
			continue
		}

		result.Changes = append(result.Changes, Change{
			Description: item.Description,
			Rating:      ParseRating(item.Rating),
			Explanation: item.Explanation,
		})
	}

	return result, nil
}

// extractJSON retrieves JSON data from a response.
func extractJSON(response string) (string, error) {
	// Try to find a JSON array directly
	jsonRegex := regexp.MustCompile(`(?s)\[.*\]`)
	jsonMatch := jsonRegex.FindString(response)
	if jsonMatch != "" {
		return jsonMatch, nil
	}

	// Try to extract from code block
	codeBlockRegex := regexp.MustCompile("```(?:json)?\n?(\\[.*?\\])\n?```")
	if matches := codeBlockRegex.FindStringSubmatch(response); len(matches) > 1 {
		return matches[1], nil
	}

	return "", ErrInvalidJSON
}
