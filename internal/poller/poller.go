package poller

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/lazrossi/code-agent/internal/agent"
	"github.com/lazrossi/code-agent/internal/config"
	gh "github.com/lazrossi/code-agent/internal/github"
	"github.com/lazrossi/code-agent/internal/storage"
	"github.com/lazrossi/code-agent/internal/store"
)

// PollAll polls all enabled repos for eligible issues and dispatches agents.
// Repos that share the same GitHub remote are de-duplicated so each remote
// is only polled once (the first config entry wins).
// s3 may be nil — uploads are skipped when client is nil.
func PollAll(ctx context.Context, cfg *config.Config, st *store.Store, s3 *storage.Client, dryRun bool) error {
	repos := cfg.EnabledRepos()
	if len(repos) == 0 {
		log.Println("no enabled repos configured")
		return nil
	}

	// De-duplicate by GitHub NWO (owner/repo) so we don't poll the same
	// remote twice when multiple local checkouts exist.
	seen := make(map[string]string) // nwo → first config name that claimed it
	for name, repo := range repos {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		nwo, err := gh.NWOFromPath(repo.Path)
		if err != nil {
			// Not a valid git repo or no remote — still try to poll it.
			nwo = ""
		}
		if nwo != "" {
			if first, dup := seen[nwo]; dup {
				log.Printf("skipping repo %s: same remote %s already polled via %s", name, nwo, first)
				continue
			}
			seen[nwo] = name
		}

		log.Printf("polling repo: %s (%s)", name, repo.Path)
		if err := PollRepo(ctx, cfg, st, s3, name, repo, dryRun); err != nil {
			log.Printf("error polling %s: %v", name, err)
		}

		// Rebase stale agent PRs after dispatching new work
		log.Printf("  %s: checking for stale agent PRs", name)
		if err := agent.RebaseStale(ctx, cfg, repo.Path, dryRun); err != nil {
			log.Printf("  %s: rebase error: %v", name, err)
		}
	}
	return nil
}

// PollRepo polls a single repo for eligible issues.
// s3 may be nil — uploads are skipped when client is nil.
func PollRepo(ctx context.Context, cfg *config.Config, st *store.Store, s3 *storage.Client, repoName string, repo config.RepoConfig, dryRun bool) error {
	if !gh.IsGitHubRepo(repo.Path) {
		log.Printf("  %s: skipping (not a GitHub repo)", repoName)
		return nil
	}

	ghRepoName, err := gh.RepoNameFromPath(repo.Path)
	if err != nil {
		return fmt.Errorf("resolve repo name: %w", err)
	}

	// In dry-run mode, fetch ALL open issues so we can report on everything.
	// In normal mode, pre-filter by the required label to reduce API calls.
	var labelFilter []string
	if !dryRun && cfg.GitHub.RequireLabel != "" {
		labelFilter = []string{cfg.GitHub.RequireLabel}
	}

	issues, err := gh.ListIssues(cfg.GitHub.Org, ghRepoName, labelFilter)
	if err != nil {
		return fmt.Errorf("list issues: %w", err)
	}

	if len(issues) == 0 {
		log.Printf("  %s: no open issues", repoName)
		return nil
	}

	log.Printf("  %s: %d open issues", repoName, len(issues))

	for i := range issues {
		issue := &issues[i]
		if ctx.Err() != nil {
			return ctx.Err()
		}

		reason := EligibilityReason(ctx, cfg, st, repo.Path, issue)
		eligible := reason == ""

		_ = st.UpsertIssue(ctx, ToStoreIssue(issue, repo.Path))

		labels := strings.Join(issue.LabelNames(), ", ")
		if labels == "" {
			labels = "(none)"
		}

		if dryRun {
			attempts, _ := st.CountAttempts(ctx, repo.Path, issue.Number)
			if eligible {
				log.Printf("    #%-4d %-60s [%s]  → ELIGIBLE (attempts: %d/%d)", issue.Number, issue.Title, labels, attempts, cfg.Agent.MaxAttempts)
			} else {
				log.Printf("    #%-4d %-60s [%s]  → skip: %s (attempts: %d/%d)", issue.Number, issue.Title, labels, reason, attempts, cfg.Agent.MaxAttempts)
			}
			continue
		}

		// Normal mode: only dispatch eligible issues
		if !eligible {
			continue
		}

		log.Printf("  #%d %s — dispatching agent", issue.Number, issue.Title)
		result, dispatchErr := agent.Dispatch(ctx, cfg, st, s3, repo.Path, issue)
		if dispatchErr != nil {
			log.Printf("  #%d dispatch error: %v", issue.Number, dispatchErr)
		} else {
			log.Printf("  #%d dispatch result: %s %s", issue.Number, result.Status, result.PRUrl)
		}

		// Run the surveyor on every dispatch that got far enough to have a run ID.
		// The surveyor handles all cases: PR created, no commits, errors.
		if result != nil {
			// Find the run ID for this dispatch
			runs, err := st.ListRunsForIssue(ctx, repo.Path, issue.Number)
			if err == nil && len(runs) > 0 {
				latestRun := runs[0] // most recent
				agent.Survey(ctx, cfg, st, int64(latestRun.ID), repo.Path, issue, result)
			}
		}
	}

	_ = st.UpdatePollState(ctx, repo.Path)
	return nil
}

// IsEligible checks whether an issue should be picked up by an agent.
func IsEligible(ctx context.Context, cfg *config.Config, st *store.Store, repoPath string, issue *gh.Issue) bool {
	return EligibilityReason(ctx, cfg, st, repoPath, issue) == ""
}

// EligibilityReason returns a human-readable reason why an issue is not eligible,
// or empty string if it is eligible.
func EligibilityReason(ctx context.Context, cfg *config.Config, st *store.Store, repoPath string, issue *gh.Issue) string {
	if cfg.GitHub.RequireLabel != "" && !issue.HasLabel(cfg.GitHub.RequireLabel) {
		return fmt.Sprintf("missing required label %q", cfg.GitHub.RequireLabel)
	}

	for _, skip := range cfg.GitHub.SkipLabels {
		if issue.HasLabel(skip) {
			return fmt.Sprintf("has skip label %q", skip)
		}
	}

	// Block if there's already a successful (approved) or currently-running run
	hasRun, err := st.HasSuccessfulRun(ctx, repoPath, issue.Number)
	if err != nil {
		return fmt.Sprintf("error checking runs: %v", err)
	}
	if hasRun {
		return "already has successful/running agent run"
	}

	// Block if max attempts reached (failed + rejected + timeout all count)
	if cfg.Agent.MaxAttempts > 0 {
		attempts, err := st.CountAttempts(ctx, repoPath, issue.Number)
		if err != nil {
			return fmt.Sprintf("error counting attempts: %v", err)
		}
		if attempts >= cfg.Agent.MaxAttempts {
			return fmt.Sprintf("max attempts reached (%d/%d)", attempts, cfg.Agent.MaxAttempts)
		}
	}

	return ""
}

// ToStoreIssue converts a GitHub issue to a store issue.
func ToStoreIssue(issue *gh.Issue, repoPath string) *store.Issue {
	return &store.Issue{
		Repo:   repoPath,
		Number: issue.Number,
		Title:  issue.Title,
		Body:   issue.Body,
		Labels: issue.LabelNames(),
		State:  store.IssueStateOpen,
	}
}
