package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/lazrossi/code-agent/internal/config"
	gh "github.com/lazrossi/code-agent/internal/github"
	"github.com/lazrossi/code-agent/internal/storage"
	"github.com/lazrossi/code-agent/internal/store"
)

// Result represents the outcome of an agent dispatch.
type Result struct {
	Status  store.RunStatus
	PRUrl   string
	Error   string
	Output  string
	Elapsed time.Duration
}

// Dispatch runs the full agent lifecycle for one issue using a git worktree:
// fetch → worktree add → claude -p → push → PR → worktree remove.
// The main repo checkout is never modified.
// s3 may be nil — uploads are skipped when client is nil.
func Dispatch(ctx context.Context, cfg *config.Config, st *store.Store, s3 *storage.Client, repoPath string, issue *gh.Issue) (*Result, error) {
	start := time.Now()
	trace := NewTrace()

	fail := func(err error) (*Result, error) {
		return &Result{Status: store.RunStatusFailed, Error: err.Error(), Elapsed: time.Since(start)}, err
	}

	// Detect the default branch
	defaultBranch, err := gh.DefaultBranch(repoPath)
	if err != nil {
		return fail(err)
	}
	log.Printf("repo %s: default branch is %q", repoPath, defaultBranch)

	// Fetch latest from origin (does not touch working tree)
	trace.Record("fetch", "")
	if err := gh.Fetch(repoPath); err != nil {
		return fail(err)
	}

	// Determine branch name and worktree path
	branch := fmt.Sprintf("agent/%d-%s", issue.Number, slugify(issue.Title))
	wtDir := gh.WorktreeDir(repoPath, branch)

	if gh.BranchExists(repoPath, branch) {
		return &Result{
			Status:  store.RunStatusSkipped,
			Error:   fmt.Sprintf("branch %s already exists on origin", branch),
			Elapsed: time.Since(start),
		}, nil
	}

	// Clean up stale local branch if it exists (e.g. from a previous failed run)
	if gh.LocalBranchExists(repoPath, branch) {
		_ = exec.Command("git", "-C", repoPath, "branch", "-D", branch).Run()
	}

	// Create worktree with new branch based on origin/<default>
	trace.Record("worktree", wtDir)
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		return fail(fmt.Errorf("create worktree dir: %w", err))
	}
	// Remove any leftover directory from a previous failed run
	_ = os.RemoveAll(wtDir)

	if err := gh.AddWorktree(repoPath, wtDir, branch, "origin/"+defaultBranch); err != nil {
		return fail(err)
	}

	// Always clean up the worktree when done
	defer func() {
		trace.Record("cleanup", wtDir)
		gh.RemoveWorktree(repoPath, wtDir, branch)
	}()

	// Record run as started
	run := &store.AgentRun{
		Repo:     repoPath,
		IssueNum: issue.Number,
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

	// Run claude -p inside the worktree
	trace.Record("claude_start", "")
	agentCtx, cancel := context.WithTimeout(ctx, cfg.Agent.Timeout.Duration)
	defer cancel()

	output, agentErr := runClaude(agentCtx, cfg, wtDir, repoPath, BuildPrompt(issue, repoPath), LogPath(runID))
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

	// Check if the agent produced commits
	hasCommits, err := gh.HasNewCommits(wtDir, defaultBranch)
	if err != nil || !hasCommits {
		result := &Result{Status: store.RunStatusFailed, Error: "agent produced no commits", Output: output, Elapsed: elapsed}
		finish(result)
		return result, fmt.Errorf("no commits produced")
	}

	// Push branch from the worktree
	trace.Record("push", branch)
	if err := gh.PushBranch(wtDir, branch); err != nil {
		result := &Result{Status: store.RunStatusFailed, Error: err.Error(), Output: output, Elapsed: elapsed}
		finish(result)
		return result, err
	}

	// Create PR
	prTitle := fmt.Sprintf("fix: resolve #%d — %s", issue.Number, issue.Title)
	prBody := fmt.Sprintf("Automated fix for #%d by code-agent.\n\nFixes #%d", issue.Number, issue.Number)
	prURL, err := gh.CreatePR(wtDir, prTitle, prBody, defaultBranch)
	if err != nil {
		result := &Result{Status: store.RunStatusFailed, Error: err.Error(), Output: output, Elapsed: elapsed}
		finish(result)
		return result, err
	}
	trace.Record("pr", prURL)

	result := &Result{Status: store.RunStatusSuccess, PRUrl: prURL, Output: output, Elapsed: elapsed}
	finish(result)

	log.Printf("PR created: %s", prURL)
	return result, nil
}

// LogPath returns the log file path for a given run ID.
func LogPath(runID int64) string {
	dir := filepath.Join(config.Home(), "logs")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, fmt.Sprintf("run-%d.log", runID))
}

// runClaude invokes claude -p in the worktree directory.
// It reads the CLAUDE.md from the original repo path (already baked into the prompt).
// Output is streamed to a log file for live viewing.
func runClaude(ctx context.Context, cfg *config.Config, wtDir, repoPath string, prompt string, logFile string) (string, error) {
	args := []string{
		"-p",
		"--model", cfg.Agent.Model,
		"--max-turns", fmt.Sprintf("%d", cfg.Agent.MaxTurns),
		"--permission-mode", "dontAsk",
		"--output-format", "json",
		"--no-session-persistence",
	}

	if rulesPath := AgentRulesPath(); rulesPath != "" {
		args = append(args, "--append-system-prompt-file", rulesPath)
	}

	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = wtDir

	// Stream output to log file for live tailing
	if logFile != "" {
		f, err := os.Create(logFile)
		if err != nil {
			return "", fmt.Errorf("create log file: %w", err)
		}
		defer f.Close()

		var buf strings.Builder
		w := io.MultiWriter(f, &buf)
		cmd.Stdout = w
		cmd.Stderr = w
		err = cmd.Run()
		return buf.String(), err
	}

	out, err := cmd.CombinedOutput()
	return string(out), err
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == ' ' {
			return r
		}
		return ' '
	}, s)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}
	return s
}
