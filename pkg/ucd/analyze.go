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
	Description        string `json:"description"`
	MalwareRisk        int    `json:"malware_risk"`
	MalwareExplanation string `json:"malware_explanation"`
	SilentPatch        int    `json:"silent_patch"`
	SilentExplanation  string `json:"silent_explanation"`
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

	prompt, err := buildPrompt(data)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

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
func buildPrompt(data *AnalysisData) (string, error) {
	prompt := fmt.Sprintf(`You are a security expert and malware analyst analyzing changes between two versions of a software package.
I will provide:
1. A unified diff between version %s and %s
2. Commit messages describing changes (if available)
3. Changelog entries (if available)

Your task is to identify any behavioral changes in the code that do not appear to be related to changes mentioned in the commit messages or changelog.
Be loose and liberal with your interpretation when relating code changes to a changelog entry or commit message.

We are trying to uncover two types of changes:

- Malicious changes being snuck into the supply change: for example, credential theft, backdoors, exfiltration, or data wipers
- Silently fixed security vulnerabilities (CVE's), for example: directory traversal, buffer overflows

For each undocumented behavioral change you identify:
1. Briefly describe the undocumented change in 15 words or less.
2. Give the undocumented change a malice rating, from 0-10 (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
   * Don't worry if the code is adding new functionality that may accidentally introduce a security vulnerability,
such as potential code execution risk, but do care if the undocumented behaviors appear to be malicious, for example:
adding a backdoor, downloading software, calling chmod to make programs executable, introducing malicious behaviors or add undocumented obfuscation to avoid code analysis.
3. Give the undocumented change a silent security fix rating, based on how likely and critical you think the security patch might have been.
   If the code authors mention "security fix" or "CVE" in the changelog or commit messages relating to the delta between these two versions,
   it is less likely to see hidden silent security fixes.
4. For each rating, provide a 1-sentence explanation of how you arrived to your conclusion.

Thinking how a security engineer would reason about a combination of security threats or analyze software, you
you also need to take a step back and consider the overall impact of all of the undocumented changes to assess a
combined "malice" and "silent security fix" score.

In general, most software should score 0-1.

Here are undocumented behavioral changes to ignore:
- Changes to .github/workflows/ files - as they do not impact the behavior of the software
- Changes to documentation (.md files, for example) - as they do not impact the behavior of the software
- Performance improvements
- Changes that may be related to code refactoring

Focus on behavioral changes that could be construed as malicious or a fix for an undocumented critical security vulnerability.

Format your response as a JSON object with:
- "changes": An array of JSON objects, each with:
  - "description": A brief description of the undocumented change
  - "malware_risk": 0-10 danger scale of this change (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "malware_explanation": Your explanation for your malware risk rating.
  - "silent_patch": 0-10 likelihood of a silent critical security patch (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "silent_explanation": Your explanation for your silent_Patch rating.

- "summary": A JSON object that assesses the combined impact:
  - "description": A 1-sentence description of the combined undocumented behavioral changes.
  - "malware_risk": 0-10 danger scale of all combined changes considered together (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "malware_explanation": Your explanation for your combined malware risk rating.
  - "silent_patch": 0-10 likelihood of a silent critical security patch introduced in this version change (0=Benign, 5=Suspicious, 10=Extremely Dangerous)
  - "silent_explanation": Your explanation for your combined silent_patch rating.

If there are no undocumented behavior changes, return an empty changes array. Your response must be in JSON form to be understood.

Here are the details to analyze:

UNIFIED DIFF:
%s

COMMIT MESSAGES:
%s

CHANGELOG CHANGES:
%s
`, data.VersionA, data.VersionB, data.Diff, data.CommitMessages, data.Changelog)

	// Truncate if too long
	const maxPromptLength = 2000000
	if len(prompt) > maxPromptLength {
		return "", fmt.Errorf("too much data to analyze (%d length)", maxPromptLength)
	}
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
	return &result, err
}

// extractJSON retrieves JSON data from a response string.
func extractJSON(response string) string {
	klog.Infof("response: %s", response)

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
