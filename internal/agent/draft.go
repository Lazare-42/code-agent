package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lazrossi/code-agent/internal/config"
)

// DraftIssueResult holds the generated issue draft.
type DraftIssueResult struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels"`
}

// DraftIssue runs claude -p to generate a structured GitHub issue from a free-form description.
func DraftIssue(ctx context.Context, cfg *config.Config, repo, description string) (*DraftIssueResult, error) {
	prompt := buildDraftPrompt(repo, description)

	args := []string{
		"-p",
		"--model", cfg.Agent.Model,
		"--max-turns", "1",
		"--output-format", "json",
		"--no-session-persistence",
		prompt,
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude draft: %w: %s", err, string(out))
	}

	// The output-format json wraps the result; extract the text content.
	text := extractJSONText(string(out))

	var result DraftIssueResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse draft response: %w (raw: %s)", err, text)
	}

	if result.Title == "" {
		return nil, fmt.Errorf("draft returned empty title")
	}

	return &result, nil
}

// extractJSONText tries to extract the JSON object from claude's output.
// With --output-format json, claude wraps the response in a JSON envelope
// with a "result" field containing the text. We try to parse that first,
// then fall back to finding a raw JSON object in the output.
func extractJSONText(raw string) string {
	// Try claude's JSON envelope format first
	var envelope struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err == nil && envelope.Result != "" {
		return envelope.Result
	}

	// Fall back: find the first { ... } block in the output
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		return raw[start : end+1]
	}

	return raw
}

func buildDraftPrompt(repo, description string) string {
	return fmt.Sprintf(`You are a GitHub issue writer. Given a repository name and a free-form description, generate a well-structured GitHub issue. Output ONLY valid JSON with these fields:
{"title": "...", "body": "...", "labels": ["..."]}

The title should be concise (under 80 chars). The body should use markdown with sections like ## Problem, ## Expected Behavior, ## Suggested Fix where appropriate. Labels should be chosen from common ones: bug, enhancement, documentation, security, performance, dependencies, etc.

Repository: %s
Description: %s`, repo, description)
}
