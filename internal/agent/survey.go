package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/lazrossi/code-agent/internal/config"
	gh "github.com/lazrossi/code-agent/internal/github"
	"github.com/lazrossi/code-agent/internal/store"
)

// SurveyVerdict is the structured response from the surveyor agent.
type SurveyVerdict struct {
	Verdict string `json:"verdict"` // "approve" or "reject"
	Reason  string `json:"reason"`
}

// Survey reviews the result of a dispatch and decides whether to accept or reject it.
// It handles three cases:
//   - PR was created: review the diff against the issue
//   - No commits produced: check agent output for excuses, reject for retry
//   - Agent error/timeout: automatically reject for retry
func Survey(ctx context.Context, cfg *config.Config, st *store.Store, runID int64, repoPath string, issue *gh.Issue, dispatchResult *Result) {
	// Failed/timed out runs with no PR: nothing to review, they're already
	// eligible for retry because HasSuccessfulRun only blocks on "success"/"running".
	if dispatchResult.PRUrl == "" {
		log.Printf("  survey #%d: no PR created, skipping review (will retry)", issue.Number)
		return
	}

	// There's a PR — run the surveyor agent to review the diff
	log.Printf("  survey #%d: reviewing PR %s", issue.Number, dispatchResult.PRUrl)

	diff, err := getDiff(repoPath, issue.Number, dispatchResult)
	if err != nil {
		log.Printf("  survey #%d: cannot get diff: %v — approving by default", issue.Number, err)
		return
	}

	verdict, err := runSurveyor(ctx, cfg, issue, diff)
	if err != nil {
		log.Printf("  survey #%d: surveyor error: %v — approving by default", issue.Number, err)
		return
	}

	log.Printf("  survey #%d: verdict=%s reason=%s", issue.Number, verdict.Verdict, verdict.Reason)

	switch store.ReviewVerdict(verdict.Verdict) {
	case store.ReviewApprove:
		_ = st.ReviewRun(ctx, runID, store.ReviewApprove, verdict.Reason)

	case store.ReviewReject:
		_ = st.ReviewRun(ctx, runID, store.ReviewReject, verdict.Reason)

		// Close the PR with the rejection reason
		comment := fmt.Sprintf("Surveyor rejected this PR:\n\n%s\n\nThis issue will be retried.", verdict.Reason)
		if err := gh.ClosePR(repoPath, dispatchResult.PRUrl, comment); err != nil {
			log.Printf("  survey #%d: failed to close PR: %v", issue.Number, err)
		} else {
			log.Printf("  survey #%d: PR closed, will retry on next poll", issue.Number)
		}

	default:
		log.Printf("  survey #%d: unknown verdict %q, treating as approve", issue.Number, verdict.Verdict)
	}
}

func getDiff(repoPath string, issueNum int, result *Result) (string, error) {
	// The worktree is already cleaned up by the time survey runs,
	// so we get the diff from the remote branch.
	defaultBranch, err := gh.DefaultBranch(repoPath)
	if err != nil {
		return "", err
	}

	// Fetch to make sure we have the agent branch
	_ = gh.Fetch(repoPath)

	// Find the branch name from the result
	out, err := exec.Command("git", "-C", repoPath, "log",
		"origin/"+defaultBranch+"..origin/"+result.PRUrl, "--oneline").Output()
	_ = out

	// Use gh to get the PR diff directly — more reliable
	parts := strings.Split(strings.TrimRight(result.PRUrl, "/"), "/")
	prNum := parts[len(parts)-1]

	diffOut, err := exec.Command("gh", "pr", "diff", prNum, "--repo", repoPath).CombinedOutput()
	if err != nil {
		// Fallback: try with the repo's remote
		repoName, nameErr := gh.RepoNameFromPath(repoPath)
		if nameErr != nil {
			return "", fmt.Errorf("get diff: %w", err)
		}
		// Read org from the remote
		remoteOut, _ := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
		nwo := ""
		remote := strings.TrimSpace(string(remoteOut))
		if strings.HasPrefix(remote, "git@") {
			p := strings.SplitN(remote, ":", 2)
			if len(p) == 2 {
				nwo = strings.TrimSuffix(p[1], ".git")
			}
		} else {
			remote = strings.TrimSuffix(remote, ".git")
			p := strings.Split(remote, "/")
			if len(p) >= 2 {
				nwo = p[len(p)-2] + "/" + p[len(p)-1]
			}
		}
		if nwo == "" {
			nwo = repoName
		}

		diffOut, err = exec.Command("gh", "pr", "diff", prNum, "--repo", nwo).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("gh pr diff: %w: %s", err, diffOut)
		}
	}

	diff := string(diffOut)

	// Truncate very large diffs to avoid blowing up the prompt
	const maxDiffLen = 30000
	if len(diff) > maxDiffLen {
		diff = diff[:maxDiffLen] + "\n\n... (diff truncated)"
	}
	return diff, nil
}

func runSurveyor(ctx context.Context, cfg *config.Config, issue *gh.Issue, diff string) (*SurveyVerdict, error) {
	prompt := buildSurveyPrompt(issue, diff)

	args := []string{
		"-p",
		"--model", cfg.Agent.Model,
		"--max-turns", "1",
		"--permission-mode", "dontAsk",
		"--output-format", "json",
		"--no-session-persistence",
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("claude survey: %w: %s", err, out)
	}

	// The output is JSON from claude --output-format json.
	// Try to extract the verdict from it.
	return parseSurveyOutput(string(out))
}

func buildSurveyPrompt(issue *gh.Issue, diff string) string {
	var b strings.Builder

	b.WriteString("You are a code review surveyor. Your job is to decide whether a PR resolves the issue.\n\n")
	b.WriteString(fmt.Sprintf("## Issue #%d: %s\n\n", issue.Number, issue.Title))
	if issue.Body != "" {
		b.WriteString(issue.Body)
		b.WriteString("\n\n")
	}
	b.WriteString("## PR Diff\n\n```diff\n")
	b.WriteString(diff)
	b.WriteString("\n```\n\n")
	b.WriteString("## Your Task\n\n")
	b.WriteString("Review the diff against the issue. Decide:\n")
	b.WriteString("- **approve**: The changes meaningfully address the issue. Minor imperfections are OK.\n")
	b.WriteString("- **reject**: The changes are wrong, empty, trivially insufficient, or clearly break things.\n\n")
	b.WriteString("Respond with ONLY a JSON object (no markdown, no code fences):\n")
	b.WriteString(`{"verdict": "approve" or "reject", "reason": "one sentence explanation"}`)
	b.WriteString("\n")

	return b.String()
}

func parseSurveyOutput(output string) (*SurveyVerdict, error) {
	// Claude --output-format json wraps the response. Try to find our JSON inside.
	// First try to parse the whole output as our verdict.
	var verdict SurveyVerdict
	if err := json.Unmarshal([]byte(output), &verdict); err == nil && verdict.Verdict != "" {
		return &verdict, nil
	}

	// Claude JSON output format wraps in {"result": "..."} — try to extract.
	var wrapper struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(output), &wrapper); err == nil && wrapper.Result != "" {
		if err := json.Unmarshal([]byte(wrapper.Result), &verdict); err == nil && verdict.Verdict != "" {
			return &verdict, nil
		}

		// The result might contain the JSON embedded in text. Try to find it.
		if idx := strings.Index(wrapper.Result, `{"verdict"`); idx >= 0 {
			end := strings.Index(wrapper.Result[idx:], "}")
			if end > 0 {
				if err := json.Unmarshal([]byte(wrapper.Result[idx:idx+end+1]), &verdict); err == nil && verdict.Verdict != "" {
					return &verdict, nil
				}
			}
		}
	}

	// Last resort: scan for the JSON in the raw output
	if idx := strings.Index(output, `{"verdict"`); idx >= 0 {
		end := strings.Index(output[idx:], "}")
		if end > 0 {
			if err := json.Unmarshal([]byte(output[idx:idx+end+1]), &verdict); err == nil && verdict.Verdict != "" {
				return &verdict, nil
			}
		}
	}

	return nil, fmt.Errorf("could not parse surveyor output: %.200s", output)
}
