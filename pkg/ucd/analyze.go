package ucd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"google.golang.org/genai"
	"k8s.io/klog/v2"
)

// skipChangePattern matches descriptions of changes that should be filtered out.
var skipChangePattern = regexp.MustCompile(`(?i)\.gitignore|README|\.github/|CI |documentation|comment|test file|workflow`)

// Assessment represents an individual change or summary assessment.
type Assessment struct {
	Description        string `json:"description"`
	MalwareRisk        int    `json:"malware_risk"`
	MalwareExplanation string `json:"malware_explanation"`
	SilentPatch        int    `json:"silent_patch"`
	SilentExplanation  string `json:"silent_explanation"`
}

// Result contains the analysis findings.
type Result struct {
	Input               *AnalysisData `json:"input"`
	UndocumentedChanges []Assessment  `json:"undocumented_changes"`
	Summary             *Assessment   `json:"summary,omitempty"`
}

// AnalyzeChanges performs AI-based analysis of code changes.
func AnalyzeChanges(ctx context.Context, client *genai.Client, data *AnalysisData, modelName string) (*Result, error) {
	if modelName == "" {
		modelName = "gemini-2.5-pro-preview-05-06"
	}

	prompt, err := buildPrompt(data)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	// Create generation config for the new API
	genConfig := &genai.GenerateContentConfig{
		Temperature: genai.Ptr[float32](0.0),
		Seed:        genai.Ptr[int32](0),
	}

	klog.V(1).Infof("prompt: %s", prompt)

	// Generate content using the new API structure
	// client.Models provides access to model-specific methods like GenerateContent.
	// genai.Text(prompt) returns []*genai.Content, which is the expected type for the 'contents' parameter.
	resp, err := client.Models.GenerateContent(ctx, modelName, genai.Text(prompt), genConfig)
	if err != nil {
		return nil, fmt.Errorf("generate content: %w", err)
	}

	// Check for valid response and parts
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response or valid parts from AI model")
	}

	// Extract the text from the response using strings.Builder for efficiency
	var responseTextBuilder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		// In the new API, each part is a *genai.Part, which has a Text field (string).
		if part.Text != "" {
			responseTextBuilder.WriteString(part.Text)
		}
	}
	responseText := responseTextBuilder.String()
	r, err := parseAIResponse(responseText)
	if err != nil {
		return nil, fmt.Errorf("parse failure: %w", err)
	}
	r.Input = data
	klog.V(1).Infof("result: %+v", r)
	return r, err
}

const promptTemplateStr = `
You are a security expert and malware analyst studying the changes between two versions of an
open-source program that you are not familiar with.

{{with .ProgramName}}The name of the program we are analyzing is "{{.}}", you may know about it already.{{end}}
{{with.ProgramDesc}}The description of the program we are analyzing is: "{{.}}"{{end}}

I will provide:

1. A unified diff of changes between version {{.VersionA}} and {{.VersionB}} collected from {{.Source}}
2. Commit messages describing changes (if available)
3. Changelog entries (if available)

Your task is to determine if there are behavior changes present in the unified diff that are not documented
by either the commit messages or changelog.

- Be loose in your interpretation of how a diff change
may be related to a commit message or changelog entry.
- Don't include undocumented code health improvements that often appear alongside feature changes.
  * For example, don't include documentation updates, changes that can come up in code refactoring, CI/CD configuration changes, or performance improvements.
- Ignore changes to files within the .github directory, as they will not impact the users of this tool.
- Unless you know of a specific security threat for a package version, assume that dependency version bumps are not part of a silent security fix.
- Be particularly on the lookout for possible supply-chain security attacks that would impact an open-source project. For exampel:
  * The introduction of a silent network backdoor
  * The addition of obfuscated or encoded text that does not match the surrounding code
  * Execution of external commands, especially ones that fetch URLs or decode strings
  * Cryptomining attacks
  * Credential theft

Format your response as a JSON object with:

- "undocumented_changes": An array of JSON objects for each undocumented behavioral change that could impact a user of this program, each with:
  - "description": A terse, concise, and technical 1-sentence description of the undocumented behavioral change
  - "malware_risk": 0-10 danger scale of this undocumented change being malicious in nature. For example, could this undocumented change
        represent the addition of code for credential exfiltration, a backdoor, or a data wiper? (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "malware_explanation":  A terse, concise, and technical 1-sentence explanation for the given malware_risk rating.
  - "silent_patch": 0-10 likelihood of this undocumented change representing a hidden critical security patch (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "silent_explanation": Your explanation for your silent_patch rating.

- "summary": A JSON object that assesses the full combined impact of the undocumented behavioral changes you've found:
  - "description": A terse, concise, and technical 1-sentence description of the combined undocumented behavioral changes.
  - "malware_risk": 0-10 danger scale of all combined changes considered together (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "malware_explanation": A terse, concise, and technical 1-sentence explanation for your combined malware risk rating.
  - "silent_patch": 0-10 likelihood of a silent critical security patch introduced in this version change (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "silent_explanation":  A terse, concise, and technical explanation for your combined silent_patch rating.

Do not include changes mentioned in the Changelog or commit messages.

If there are no undocumented behavior changes, return an empty changes array. Your response must be in JSON form to be understood.

Here are the details to analyze:

UNIFIED DIFF:
{{.Diff}}

COMMIT MESSAGES:
{{.CommitMessages}}

CHANGELOG CHANGES:
{{.Changelog}}

Ensure that the returned data is in valid JSON form.
`

var promptTmpl *template.Template

func init() {
	var err error
	promptTmpl, err = template.New("prompt").Parse(promptTemplateStr)
	if err != nil {
		// This would be a panic because it's a programming error if the template is invalid
		panic(fmt.Sprintf("failed to parse prompt template: %v", err))
	}
}

// buildPrompt constructs the prompt for the AI model.
func buildPrompt(data *AnalysisData) (string, error) {
	var buf bytes.Buffer
	if err := promptTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}

	prompt := buf.String()

	// Truncate if too long
	const maxPromptLength = 2000000
	if len(prompt) > maxPromptLength {
		return "", fmt.Errorf("too much data to analyze (%d length)", maxPromptLength)
	}
	//	fmt.Printf("prompt: %s\n", prompt)
	return prompt, nil
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
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %v\ncontent: %s", err, jsonText)
	}
	return &result, err
}

// extractJSON retrieves JSON data from a response string.
func extractJSON(response string) string {
	//	klog.Infof("response: %s", response)

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
