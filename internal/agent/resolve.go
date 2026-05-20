package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lazrossi/code-agent/internal/config"
	gh "github.com/lazrossi/code-agent/internal/github"
	"github.com/lazrossi/code-agent/internal/storage"
	"github.com/lazrossi/code-agent/internal/store"
)

// runOnPRBranch is the shared worktree checkout → claude → push lifecycle for PR operations.
// s3 may be nil — uploads are skipped when client is nil.
func runOnPRBranch(ctx context.Context, cfg *config.Config, st *store.Store, s3 *storage.Client,
	repoPath string, nwo string, prNumber int, wtSuffix string, prompt string) (*Result, error) {

	start := time.Now()
	trace := NewTrace()

	fail := func(err error) (*Result, error) {
		return &Result{Status: store.RunStatusFailed, Error: err.Error(), Elapsed: time.Since(start)}, err
	}

	detail, err := gh.GetPRDetail(nwo, prNumber)
	if err != nil {
		return fail(err)
	}
	branch := detail.HeadRef

	trace.Record("fetch", "")
	if err := gh.Fetch(repoPath); err != nil {
		return fail(err)
	}

	trace.Record("worktree", branch+"-"+wtSuffix)
	wtDir := gh.WorktreeDir(repoPath, branch+"-"+wtSuffix)
	_ = os.RemoveAll(wtDir)

	if !gh.LocalBranchExists(repoPath, branch) {
		out, err := exec.Command("git", "-C", repoPath, "branch", branch, "origin/"+branch).CombinedOutput()
		if err != nil {
			return fail(fmt.Errorf("create tracking branch: %w: %s", err, out))
		}
	}

	if err := gh.AddWorktreeExisting(repoPath, wtDir, branch); err != nil {
		return fail(err)
	}
	defer func() {
		trace.Record("cleanup", wtDir)
		gh.RemoveWorktreeOnly(repoPath, wtDir)
	}()

	run := &store.AgentRun{
		Repo:     repoPath,
		IssueNum: prNumber,
		Branch:   branch,
		Status:   store.RunStatusRunning,
	}
	runID, err := st.CreateRun(ctx, run)
	if err != nil {
		return fail(err)
	}

	finish := func(r *Result) {
		_ = st.FinishRun(ctx, runID, store.RunResult{
			Status:      r.Status,
			PRUrl:       detail.URL,
			ErrorMsg:    r.Error,
			AgentOutput: r.Output,
			DurationS:   r.Elapsed.Seconds(),
		})
		if s3 != nil {
			_ = s3.PutRunLog(ctx, runID, []byte(r.Output))
			_ = s3.PutRunTrace(ctx, runID, trace.JSON())
		}
	}

	trace.Record("claude_start", "")
	agentCtx, cancel := context.WithTimeout(ctx, cfg.Agent.Timeout.Duration)
	defer cancel()

	output, agentErr := runClaude(agentCtx, cfg, wtDir, repoPath, prompt, LogPath(runID))
	elapsed := time.Since(start)
	trace.Record("claude_end", fmt.Sprintf("exit_err=%v", agentErr))

	if agentErr != nil {
		result := &Result{Elapsed: elapsed, Output: output}
		if agentCtx.Err() == context.DeadlineExceeded {
			result.Status = store.RunStatusTimeout
			result.Error = "agent timed out"
		} else {
			result.Status = store.RunStatusFailed
			result.Error = agentErr.Error()
		}
		finish(result)
		return result, agentErr
	}

	hasCommits, err := gh.HasNewCommits(wtDir, detail.BaseRef)
	if err != nil || !hasCommits {
		result := &Result{Status: store.RunStatusFailed, Error: "agent produced no commits", Output: output, Elapsed: elapsed}
		finish(result)
		return result, fmt.Errorf("no commits produced")
	}

	trace.Record("push", branch)
	if err := gh.PushBranch(wtDir, branch); err != nil {
		result := &Result{Status: store.RunStatusFailed, Error: err.Error(), Output: output, Elapsed: elapsed}
		finish(result)
		return result, err
	}

	result := &Result{Status: store.RunStatusSuccess, PRUrl: detail.URL, Output: output, Elapsed: elapsed}
	finish(result)

	log.Printf("PR #%d: agent done, pushed to %s", prNumber, branch)
	return result, nil
}

// ResolveComments runs claude -p to address PR review comments, then pushes to the same branch.
// s3 may be nil — uploads are skipped when client is nil.
func ResolveComments(ctx context.Context, cfg *config.Config, st *store.Store, s3 *storage.Client,
	repoPath string, prNumber int, nwo string, comments []gh.ReviewComment) (*Result, error) {

	prompt := buildResolvePrompt(nwo, prNumber, comments, repoPath)
	return runOnPRBranch(ctx, cfg, st, s3, repoPath, nwo, prNumber, "resolve", prompt)
}

// RunOnPR runs claude -p with a user-provided prompt on a PR branch, then pushes.
// s3 may be nil — uploads are skipped when client is nil.
func RunOnPR(ctx context.Context, cfg *config.Config, st *store.Store, s3 *storage.Client,
	repoPath string, prNumber int, nwo string, userPrompt string) (*Result, error) {

	prompt := buildPRAgentPrompt(nwo, prNumber, userPrompt, repoPath)
	return runOnPRBranch(ctx, cfg, st, s3, repoPath, nwo, prNumber, "agent", prompt)
}

func buildResolvePrompt(nwo string, prNumber int, comments []gh.ReviewComment, repoPath string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("You are working on PR #%d in %s.\n\n", prNumber, nwo))
	b.WriteString("Review comments have been left on this PR. Your job is to address ALL the feedback.\n\n")

	b.WriteString("## Review Comments\n\n")
	for _, c := range comments {
		b.WriteString(fmt.Sprintf("### %s", c.Path))
		if c.Line != nil {
			b.WriteString(fmt.Sprintf(" (line %d)", *c.Line))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("**%s** commented:\n", c.User.Login))
		b.WriteString(c.Body)
		b.WriteString("\n\n")
	}

	// Include CLAUDE.md if present
	claudeMD := filepath.Join(repoPath, "CLAUDE.md")
	if data, err := os.ReadFile(claudeMD); err == nil && len(data) > 0 {
		b.WriteString("## Repository Guidelines (CLAUDE.md)\n\n")
		b.WriteString(string(data))
		b.WriteString("\n\n")
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("1. Read and understand each review comment above.\n")
	b.WriteString("2. Make the necessary code changes to address the feedback.\n")
	b.WriteString("3. Ensure your changes compile and pass any existing tests.\n")
	b.WriteString("4. Commit your changes with a clear message describing what feedback was addressed.\n")

	return b.String()
}

func buildPRAgentPrompt(nwo string, prNumber int, userPrompt string, repoPath string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("You are working on PR #%d in %s.\n", prNumber, nwo))
	b.WriteString("The PR branch is already checked out. Make changes and commit them.\n\n")

	// Include CLAUDE.md if present
	claudeMD := filepath.Join(repoPath, "CLAUDE.md")
	if data, err := os.ReadFile(claudeMD); err == nil && len(data) > 0 {
		b.WriteString("## Repository Guidelines (CLAUDE.md)\n\n")
		b.WriteString(string(data))
		b.WriteString("\n\n")
	}

	b.WriteString("## Task\n\n")
	b.WriteString(userPrompt)
	b.WriteString("\n\n")

	b.WriteString("## Instructions\n\n")
	b.WriteString("1. Understand the task above in the context of this PR.\n")
	b.WriteString("2. Make the necessary code changes.\n")
	b.WriteString("3. Ensure your changes compile and pass any existing tests.\n")
	b.WriteString("4. Commit your changes with a clear commit message.\n")

	return b.String()
}
