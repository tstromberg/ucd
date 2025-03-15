package ucd

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/google/generative-ai-go/genai"
	"k8s.io/klog/v2"
)

// skipChangePattern matches descriptions of changes that should be filtered out.
var skipChangePattern = regexp.MustCompile(`(?i)\.gitignore|README|\.github/|CI |documentation|comment|test file|workflow`)

// Assessment represents an individual change or summary assessment.
type Assessment struct {
	Description string `json:"description"`
	Rating      int    `json:"rating"`
	Explanation string `json:"explanation"`
}

// Result contains the analysis findings.
type Result struct {
	Changes []Assessment `json:"changes"`
	Summary *Assessment  `json:"summary,omitempty"`
}

// AnalyzeChanges performs AI-based analysis of code changes.
func AnalyzeChanges(ctx context.Context, client *genai.Client, data *AnalysisData, modelName string) (*Result, error) {
	if modelName == "" {
		modelName = "gemini-2.0-flash"
	}

	prompt := buildPrompt(data)

	model := client.GenerativeModel(modelName)
	klog.V(1).Infof("prompt: %s", prompt)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from AI model")
	}

	responseText := string(resp.Candidates[0].Content.Parts[0].(genai.Text))
	r, err := parseAIResponse(responseText)
	klog.V(1).Infof("result: %+v", r)
	return r, err
}

// buildPrompt constructs the prompt for the AI model.
func buildPrompt(data *AnalysisData) string {
	prompt := fmt.Sprintf(`You are a security expert and malware analyst analyzing changes between two versions of a software package.
I will provide:
1. A unified diff between version %s and %s
2. Commit messages describing changes (if available)
3. Changelog entries (if available)

Your task is to identify any behavioral changes in the code that do not appear to be related to changes mentioned in the commit messages or changelog.
When tying a behavior change to a Changelog entry or commit message, be loose with your interpretation.

Focus especially on potentially malicious changes that aren't documented. Do not mention changes that appear to be refactors for readability, documentation, or performance improvements.

UNIFIED DIFF:
%s

COMMIT MESSAGES:
%s

CHANGELOG CHANGES:
%s

For each undocumented behavioral change you identify:
1. Briefly describe the undocumented change in 15 words or less.
2. Rate each change from 0-10, (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
3. Provide a brief explanation for your rating of each change

You shouldn't care if the code is adding new functionality that may accidentally introduce a security vulnerability,
such as potential code execution risk, but do care if the undocumented behaviors appear to be malicious, for example:
adding a backdoor, downloading software, calling chmod to make programs executable, introducing malicious behaviors or add undocumented obfuscation to avoid code analysis.

Thinking how a security engineer would reason about malicious software that runs on someones computer,
you also need to take a step back and consider the overall impact of all of the undocumented changes to assess a combined impact score.

Format your response as a JSON object with:
- "changes": An array of objects, each with:
  - "description": A brief description of the undocumented change
  - "rating": 0-10 danger scale (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "explanation": Your explanation for the rating

- "summary": An object that assesses the combined impact:
  - "description": A 1-sentence description of the combined undocumented behavioral changes.
  - "rating": 0-10 danger scale (0=Benign, 5=Suspicious, 10=Extremely Dangerous.), considering all undocumented behavioral changes.
  - "explanation": A 1-sentence explanation of how you arrived at this rating.

In general, most software should score 0-1. Changes to GitHub Action configurations or
documentation changes should always be considered benign, as they have no impact to an end-user.

If there are no undocumented changes, return an empty changes array.
`, data.VersionA, data.VersionB, data.Diff, data.CommitMessages, data.Changelog)

	// Truncate if too long
	const maxPromptLength = 100000
	if len(prompt) > maxPromptLength {
		return prompt[:maxPromptLength]
	}
	return prompt
}

// parseAIResponse extracts structured information from the AI response.
func parseAIResponse(response string) (*Result, error) {
	jsonText := extractJSON(response)
	if jsonText == "" {
		return nil, fmt.Errorf("couldn't extract JSON from response: %s", response)
	}

	if jsonText == "[]" {
		return &Result{}, nil
	}

	klog.V(1).Infof("jsonText: %s", jsonText)
	// Try to unmarshal as Result structure first
	var result Result
	err := json.Unmarshal([]byte(jsonText), &result)
	return &result, err
}

// extractJSON retrieves JSON data from a response string.
func extractJSON(response string) string {
	klog.V(1).Infof("response: %s", response)

	// Try code block first (most specific)
	codeBlockRegex := regexp.MustCompile("```(?:json)?\\n?(\\{.*?\\}|\\[.*?\\])\\n?```")
	if matches := codeBlockRegex.FindStringSubmatch(response); len(matches) > 1 {
		return matches[1]
	}

	// Try JSON object
	if objMatch := regexp.MustCompile(`(?s)\{.*\}`).FindString(response); objMatch != "" {
		return objMatch
	}

	return ""
}
