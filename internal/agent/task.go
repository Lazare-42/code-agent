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

// RunTask dispatches an agent with a free-form prompt on a repo, independent of any issue.
// If the agent produces commits, a PR is created with the title "task: {title}".
// s3 may be nil — uploads are skipped when client is nil.
func RunTask(ctx context.Context, cfg *config.Config, st *store.Store, s3 *storage.Client, repoPath, title, prompt string) (*Result, error) {
	start := time.Now()
	trace := NewTrace()

	fail := func(err error) (*Result, error) {
		return &Result{Status: store.RunStatusFailed, Error: err.Error(), Elapsed: time.Since(start)}, err
	}

	defaultBranch, err := gh.DefaultBranch(repoPath)
	if err != nil {
		return fail(err)
	}

	trace.Record("fetch", "")
	if err := gh.Fetch(repoPath); err != nil {
		return fail(err)
	}

	branch := fmt.Sprintf("task/%s", slugify(title))
	wtDir := gh.WorktreeDir(repoPath, branch)

	if gh.BranchExists(repoPath, branch) {
		return &Result{
			Status:  store.RunStatusSkipped,
			Error:   fmt.Sprintf("branch %s already exists on origin", branch),
			Elapsed: time.Since(start),
		}, nil
	}

	if gh.LocalBranchExists(repoPath, branch) {
		_ = exec.Command("git", "-C", repoPath, "branch", "-D", branch).Run()
	}

	trace.Record("worktree", wtDir)
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		return fail(fmt.Errorf("create worktree dir: %w", err))
	}
	_ = os.RemoveAll(wtDir)

	if err := gh.AddWorktree(repoPath, wtDir, branch, "origin/"+defaultBranch); err != nil {
		return fail(err)
	}
	defer func() {
		trace.Record("cleanup", wtDir)
		gh.RemoveWorktree(repoPath, wtDir, branch)
	}()

	run := &store.AgentRun{
		Repo:       repoPath,
		IssueNum:   0,
		Branch:     branch,
		Status:     store.RunStatusRunning,
		TaskTitle:  &title,
		TaskPrompt: &prompt,
	}
	runID, err := st.CreateRun(ctx, run)
	if err != nil {
		return fail(err)
	}

	finish := func(r *Result) {
		_ = st.FinishRun(ctx, runID, store.RunResult{
			Status:      r.Status,
			PRUrl:       r.PRUrl,
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

	fullPrompt := buildTaskPrompt(title, prompt, repoPath)
	output, agentErr := runClaude(agentCtx, cfg, wtDir, repoPath, fullPrompt, LogPath(runID))
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

	hasCommits, err := gh.HasNewCommits(wtDir, defaultBranch)
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

	prTitle := fmt.Sprintf("task: %s", title)
	prBody := fmt.Sprintf("Standalone task dispatched by code-agent.\n\n## Prompt\n\n%s", prompt)
	prURL, err := gh.CreatePR(wtDir, prTitle, prBody, defaultBranch)
	if err != nil {
		result := &Result{Status: store.RunStatusFailed, Error: err.Error(), Output: output, Elapsed: elapsed}
		finish(result)
		return result, err
	}
	trace.Record("pr", prURL)

	result := &Result{Status: store.RunStatusSuccess, PRUrl: prURL, Output: output, Elapsed: elapsed}
	finish(result)

	log.Printf("Task PR created: %s", prURL)
	return result, nil
}

func buildTaskPrompt(title, userPrompt, repoPath string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("## Task: %s\n\n", title))
	b.WriteString(userPrompt)
	b.WriteString("\n\n")

	claudeMD := filepath.Join(repoPath, "CLAUDE.md")
	if data, err := os.ReadFile(claudeMD); err == nil && len(data) > 0 {
		b.WriteString("## Repository Guidelines (CLAUDE.md)\n\n")
		b.WriteString(string(data))
		b.WriteString("\n\n")
	}

	b.WriteString("## Instructions\n\n")
	b.WriteString("1. Understand the task above.\n")
	b.WriteString("2. Make the necessary code changes.\n")
	b.WriteString("3. Ensure your changes compile and pass any existing tests.\n")
	b.WriteString("4. Commit your changes with a clear commit message.\n")

	return b.String()
}
