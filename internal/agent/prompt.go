package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gh "github.com/lazrossi/code-agent/internal/github"
)

// BuildPrompt constructs the prompt for claude -p from an issue.
func BuildPrompt(issue *gh.Issue, repoPath string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Resolve GitHub issue #%d in this repository.\n\n", issue.Number))
	b.WriteString(fmt.Sprintf("## Issue: %s\n\n", issue.Title))
	if issue.Body != "" {
		b.WriteString(issue.Body)
		b.WriteString("\n\n")
	}

	// Include repo's CLAUDE.md if it exists
	claudeMD := filepath.Join(repoPath, "CLAUDE.md")
	if data, err := os.ReadFile(claudeMD); err == nil && len(data) > 0 {
		b.WriteString("## Repository Guidelines (CLAUDE.md)\n\n")
		b.WriteString(string(data))
		b.WriteString("\n\n")
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("1. Analyze the issue and understand what needs to be done.\n")
	b.WriteString("2. Make the necessary code changes to resolve the issue.\n")
	b.WriteString("3. Ensure your changes compile and pass any existing tests.\n")
	b.WriteString("4. Commit your changes with a clear commit message referencing the issue number.\n")
	b.WriteString(fmt.Sprintf("5. Use 'Fixes #%d' in your commit message.\n", issue.Number))

	return b.String()
}

// AgentRulesPath returns the path to the agent-rules.md file.
// Checks CODE_AGENT_HOME, next to binary, then working directory.
func AgentRulesPath() string {
	if home := os.Getenv("CODE_AGENT_HOME"); home != "" {
		if p := filepath.Join(home, "agent-rules.md"); fileExists(p) {
			return p
		}
	}

	if exe, err := os.Executable(); err == nil {
		if p := filepath.Join(filepath.Dir(exe), "scripts", "agent-rules.md"); fileExists(p) {
			return p
		}
	}

	if p, err := filepath.Abs("scripts/agent-rules.md"); err == nil && fileExists(p) {
		return p
	}

	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
