package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/lazrossi/code-agent/internal/config"
	gh "github.com/lazrossi/code-agent/internal/github"
)

const staleLabel = "stale"

// RebaseStale finds open PRs that are behind their base branch and rebases them.
// PRs older than max_pr_age are marked stale and handed back to the author.
// Uses worktrees so the main checkout is never touched.
func RebaseStale(ctx context.Context, cfg *config.Config, repoPath string, dryRun bool) error {
	if !gh.IsGitHubRepo(repoPath) {
		return nil
	}

	nwo, err := gh.NWOFromPath(repoPath)
	if err != nil {
		return fmt.Errorf("get nwo: %w", err)
	}

	// Fetch latest refs
	if err := gh.Fetch(repoPath); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	prs, err := gh.ListOpenPRs(nwo)
	if err != nil {
		return fmt.Errorf("list open PRs: %w", err)
	}

	if len(prs) == 0 {
		return nil
	}

	maxAge := cfg.Agent.MaxPRAge.Duration
	now := time.Now()

	for _, pr := range prs {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		age := now.Sub(pr.CreatedAt)

		// Mark old PRs as stale and hand back to author
		if maxAge > 0 && age > maxAge {
			if dryRun {
				log.Printf("    PR #%d (%s by @%s): open %s — would mark stale (max age %s, dry run)",
					pr.Number, pr.HeadRef, pr.Author.Login, fmtAge(age), fmtAge(maxAge))
				continue
			}
			if err := markStale(nwo, pr, age, maxAge); err != nil {
				log.Printf("    PR #%d: failed to mark stale: %v", pr.Number, err)
			}
			continue
		}

		behind, err := gh.IsBehindBase(repoPath, pr.HeadRef, pr.BaseRef)
		if err != nil {
			log.Printf("    PR #%d (%s): cannot check if behind: %v", pr.Number, pr.HeadRef, err)
			continue
		}

		if !behind {
			continue
		}

		if dryRun {
			log.Printf("    PR #%d (%s): behind %s — would rebase (dry run)", pr.Number, pr.HeadRef, pr.BaseRef)
			continue
		}

		log.Printf("    PR #%d (%s): behind %s — rebasing", pr.Number, pr.HeadRef, pr.BaseRef)

		if err := rebasePR(ctx, repoPath, pr); err != nil {
			log.Printf("    PR #%d rebase failed: %v", pr.Number, err)
		} else {
			log.Printf("    PR #%d rebased and pushed", pr.Number)
		}
	}

	return nil
}

// markStale adds the "stale" label and a comment pinging the author.
// Skips if the PR already has the stale label (avoids duplicate comments).
func markStale(nwo string, pr gh.PR, age, maxAge time.Duration) error {
	if gh.PRHasLabel(nwo, pr.Number, staleLabel) {
		log.Printf("    PR #%d (%s): already marked stale", pr.Number, pr.HeadRef)
		return nil
	}

	log.Printf("    PR #%d (%s by @%s): open %s — marking stale", pr.Number, pr.HeadRef, pr.Author.Login, fmtAge(age))

	comment := fmt.Sprintf(
		"@%s This PR has been open for **%s** (threshold: %s). Please rebase, update, or close it.",
		pr.Author.Login, fmtAge(age), fmtAge(maxAge))

	if err := gh.LabelPR(nwo, pr.Number, staleLabel); err != nil {
		return fmt.Errorf("add label: %w", err)
	}
	if err := gh.CommentPR(nwo, pr.Number, comment); err != nil {
		return fmt.Errorf("comment: %w", err)
	}
	return nil
}

func rebasePR(ctx context.Context, repoPath string, pr gh.PR) error {
	wtDir := gh.WorktreeDir(repoPath, pr.HeadRef+"-rebase")

	// Clean up any stale worktree
	_ = os.RemoveAll(wtDir)

	// Check out the PR branch into a worktree (existing branch, not -b)
	if err := gh.AddWorktreeExisting(repoPath, wtDir, pr.HeadRef); err != nil {
		return fmt.Errorf("worktree add: %w", err)
	}
	defer gh.RemoveWorktreeOnly(repoPath, wtDir)

	ok, err := gh.RebaseWorktree(wtDir, pr.BaseRef)
	if err != nil || !ok {
		return fmt.Errorf("rebase conflicts on %s: %v", pr.HeadRef, err)
	}

	// Force-push the rebased branch
	if err := gh.ForcePushBranch(wtDir, pr.HeadRef); err != nil {
		return fmt.Errorf("push: %w", err)
	}

	return nil
}

// fmtAge formats a duration as a human-readable age string.
func fmtAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
