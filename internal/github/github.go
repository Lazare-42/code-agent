package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Issue represents a GitHub issue from `gh`.
type Issue struct {
	Number int     `json:"number"`
	Title  string  `json:"title"`
	Body   string  `json:"body"`
	Labels []Label `json:"labels"`
	State  string  `json:"state"`
	URL    string  `json:"url"`
}

// Label represents a GitHub label.
type Label struct {
	Name string `json:"name"`
}

// LabelNames returns the label names as a string slice.
func (i *Issue) LabelNames() []string {
	names := make([]string, len(i.Labels))
	for idx, l := range i.Labels {
		names[idx] = l.Name
	}
	return names
}

// HasLabel checks if the issue has a specific label.
func (i *Issue) HasLabel(name string) bool {
	for _, l := range i.Labels {
		if l.Name == name {
			return true
		}
	}
	return false
}

// ListIssues lists open issues for a repo in the given org.
func ListIssues(org, repoName string, labels []string) ([]Issue, error) {
	args := []string{"issue", "list",
		"--repo", org + "/" + repoName,
		"--state", "open",
		"--json", "number,title,body,labels,state,url",
		"--limit", "100",
	}
	for _, l := range labels {
		args = append(args, "--label", l)
	}

	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("gh issue list: %w: %s", err, stderrFrom(err))
	}

	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("parse issues: %w", err)
	}
	return issues, nil
}

// CreateIssue creates a new GitHub issue using gh CLI.
// Returns the issue number and URL.
func CreateIssue(nwo, title, body string, labels []string) (int, string, error) {
	args := []string{"issue", "create",
		"--repo", nwo,
		"--title", title,
		"--body", body,
	}
	for _, l := range labels {
		args = append(args, "--label", l)
	}

	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		return 0, "", fmt.Errorf("gh issue create: %w: %s", err, stderrFrom(err))
	}
	url := strings.TrimSpace(string(out))

	// Extract issue number from URL (last path segment)
	parts := strings.Split(strings.TrimRight(url, "/"), "/")
	numStr := parts[len(parts)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, url, fmt.Errorf("parse issue number from %s: %w", url, err)
	}
	return num, url, nil
}

// DefaultBranch detects the default branch of a local repo.
// Strategy: symbolic-ref → gh API → check main/master locally.
func DefaultBranch(repoPath string) (string, error) {
	// 1. Try symbolic-ref (fastest, works if origin/HEAD is set)
	out, err := exec.Command("git", "-C", repoPath, "symbolic-ref", "refs/remotes/origin/HEAD").Output()
	if err == nil {
		if branch := strings.TrimPrefix(strings.TrimSpace(string(out)), "refs/remotes/origin/"); branch != "" {
			return branch, nil
		}
	}

	// 2. Ask gh for the default branch via the API
	out, err = exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err == nil {
		if nwo := nwoFromRemote(strings.TrimSpace(string(out))); nwo != "" {
			out, err = exec.Command("gh", "repo", "view", nwo, "--json", "defaultBranchRef", "-q", ".defaultBranchRef.name").Output()
			if err == nil {
				if branch := strings.TrimSpace(string(out)); branch != "" {
					return branch, nil
				}
			}
		}
	}

	// 3. Last resort: check if main or master exists locally
	for _, candidate := range []string{"main", "master"} {
		if err := exec.Command("git", "-C", repoPath, "rev-parse", "--verify", candidate).Run(); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("cannot determine default branch for %s", repoPath)
}

// Fetch fetches from origin without touching the working tree.
func Fetch(repoPath string) error {
	if out, err := exec.Command("git", "-C", repoPath, "fetch", "origin").CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch: %w: %s", err, out)
	}
	return nil
}

// AddWorktree creates a git worktree at dir on the given branch (created from base).
// The branch is created new — it must not already exist locally.
func AddWorktree(repoPath, dir, branch, base string) error {
	out, err := exec.Command("git", "-C", repoPath, "worktree", "add", "-b", branch, dir, base).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %w: %s", err, out)
	}
	return nil
}

// AddWorktreeExisting checks out an existing branch into a worktree.
func AddWorktreeExisting(repoPath, dir, branch string) error {
	out, err := exec.Command("git", "-C", repoPath, "worktree", "add", dir, branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add (existing): %w: %s", err, out)
	}
	return nil
}

// RemoveWorktree removes a git worktree and deletes the local branch.
func RemoveWorktree(repoPath, dir, branch string) {
	_ = exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", dir).Run()
	_ = exec.Command("git", "-C", repoPath, "branch", "-D", branch).Run()
}

// RemoveWorktreeOnly removes a worktree without deleting the branch.
func RemoveWorktreeOnly(repoPath, dir string) {
	_ = exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", dir).Run()
}

// BranchExists checks if a branch exists on origin.
func BranchExists(repoPath, branch string) bool {
	return exec.Command("git", "-C", repoPath, "rev-parse", "--verify", "origin/"+branch).Run() == nil
}

// LocalBranchExists checks if a branch exists locally.
func LocalBranchExists(repoPath, branch string) bool {
	return exec.Command("git", "-C", repoPath, "rev-parse", "--verify", branch).Run() == nil
}

// HasNewCommits checks if there are commits on the current branch beyond the base.
func HasNewCommits(worktreePath, baseBranch string) (bool, error) {
	out, err := exec.Command("git", "-C", worktreePath, "log", "origin/"+baseBranch+"..HEAD", "--oneline").Output()
	if err != nil {
		return false, fmt.Errorf("git log: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// PushBranch pushes the current branch to origin.
func PushBranch(worktreePath, branch string) error {
	out, err := exec.Command("git", "-C", worktreePath, "push", "-u", "origin", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push: %w: %s", err, out)
	}
	return nil
}

// CreatePR creates a pull request using gh.
func CreatePR(worktreePath, title, body, baseBranch string) (string, error) {
	cmd := exec.Command("gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--base", baseBranch,
	)
	cmd.Dir = worktreePath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoNameFromPath extracts the short repo name (without owner) from the origin remote.
func RepoNameFromPath(repoPath string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("get remote url: %w", err)
	}
	nwo := nwoFromRemote(strings.TrimSpace(string(out)))
	if nwo == "" {
		return "", fmt.Errorf("cannot parse remote URL: %s", out)
	}
	parts := strings.SplitN(nwo, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected nwo format: %s", nwo)
	}
	return parts[1], nil
}

// WorktreeDir returns a temporary worktree path under the system temp dir.
func WorktreeDir(repoPath, branch string) string {
	repoName := filepath.Base(repoPath)
	// Sanitize branch for filesystem: agent/42-foo → agent-42-foo
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(os.TempDir(), "code-agent-worktrees", repoName, safeBranch)
}

// nwoFromRemote extracts "owner/repo" from a git remote URL.
func nwoFromRemote(remote string) string {
	// SSH: git@github.com:owner/repo.git
	if strings.HasPrefix(remote, "git@") {
		parts := strings.SplitN(remote, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSuffix(parts[1], ".git")
		}
	}
	// HTTPS: https://github.com/owner/repo.git
	remote = strings.TrimSuffix(remote, ".git")
	parts := strings.Split(remote, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return ""
}

// PR represents an open pull request from `gh pr list`.
type PR struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	HeadRef   string    `json:"headRefName"`
	BaseRef   string    `json:"baseRefName"`
	URL       string    `json:"url"`
	Mergeable string    `json:"mergeable"`
	CreatedAt time.Time `json:"createdAt"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
}

// ListOpenPRs lists all open PRs for a repo.
func ListOpenPRs(nwo string) ([]PR, error) {
	out, err := exec.Command("gh", "pr", "list",
		"--repo", nwo,
		"--state", "open",
		"--json", "number,title,headRefName,baseRefName,url,mergeable,author,createdAt",
		"--limit", "100",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w: %s", err, stderrFrom(err))
	}

	var prs []PR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("parse PRs: %w", err)
	}
	return prs, nil
}

// IsGitHubRepo checks if a repo's origin remote points to github.com.
func IsGitHubRepo(repoPath string) bool {
	out, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), "github.com")
}

// NWOFromPath returns the "owner/repo" string for a local repo.
func NWOFromPath(repoPath string) (string, error) {
	out, err := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("get remote url: %w", err)
	}
	nwo := nwoFromRemote(strings.TrimSpace(string(out)))
	if nwo == "" {
		return "", fmt.Errorf("cannot parse remote URL: %s", out)
	}
	return nwo, nil
}

// RebaseWorktree rebases a worktree branch onto origin/<base>.
// Returns true if the rebase succeeded (or was a no-op), false if there were conflicts.
func RebaseWorktree(wtDir, baseBranch string) (bool, error) {
	out, err := exec.Command("git", "-C", wtDir, "rebase", "origin/"+baseBranch).CombinedOutput()
	if err != nil {
		// Abort the failed rebase
		_ = exec.Command("git", "-C", wtDir, "rebase", "--abort").Run()
		return false, fmt.Errorf("rebase failed: %s", out)
	}
	return true, nil
}

// ForcePushBranch force-pushes (with lease) the current branch.
func ForcePushBranch(wtDir, branch string) error {
	out, err := exec.Command("git", "-C", wtDir, "push", "--force-with-lease", "origin", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push --force-with-lease: %w: %s", err, out)
	}
	return nil
}

// IsBehindBase checks if a branch is behind origin/<base> (i.e. base has new commits).
func IsBehindBase(repoPath, branch, baseBranch string) (bool, error) {
	// Count commits on origin/<base> that aren't in origin/<branch>
	out, err := exec.Command("git", "-C", repoPath, "rev-list",
		"--count", "origin/"+branch+"..origin/"+baseBranch).Output()
	if err != nil {
		return false, fmt.Errorf("rev-list: %w", err)
	}
	count := strings.TrimSpace(string(out))
	return count != "0", nil
}

// GetDiff returns the diff between the current branch and origin/<base> in a worktree.
func GetDiff(worktreePath, baseBranch string) (string, error) {
	out, err := exec.Command("git", "-C", worktreePath, "diff", "origin/"+baseBranch+"..HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// ClosePR closes a pull request by URL with a comment.
func ClosePR(repoPath, prURL, comment string) error {
	// Extract PR number from URL (last path segment)
	parts := strings.Split(strings.TrimRight(prURL, "/"), "/")
	prNum := parts[len(parts)-1]

	if comment != "" {
		_ = exec.Command("gh", "-C", repoPath, "pr", "comment", prNum, "--body", comment).Run()
	}
	out, err := exec.Command("gh", "-C", repoPath, "pr", "close", prNum).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr close: %w: %s", err, out)
	}
	return nil
}

// CommentPR adds a comment to a PR identified by number.
func CommentPR(nwo string, prNumber int, body string) error {
	out, err := exec.Command("gh", "pr", "comment", fmt.Sprintf("%d", prNumber),
		"--repo", nwo, "--body", body).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr comment: %w: %s", err, out)
	}
	return nil
}

// LabelPR adds labels to a PR.
func LabelPR(nwo string, prNumber int, labels ...string) error {
	args := []string{"pr", "edit", fmt.Sprintf("%d", prNumber), "--repo", nwo}
	for _, l := range labels {
		args = append(args, "--add-label", l)
	}
	out, err := exec.Command("gh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr edit: %w: %s", err, out)
	}
	return nil
}

// PRHasLabel checks if a PR already has a specific label.
func PRHasLabel(nwo string, prNumber int, label string) bool {
	out, err := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", prNumber),
		"--repo", nwo, "--json", "labels", "-q", ".labels[].name").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == label {
			return true
		}
	}
	return false
}

// PRDetail represents detailed info for a single PR.
type PRDetail struct {
	Number         int       `json:"number"`
	Title          string    `json:"title"`
	Body           string    `json:"body"`
	HeadRef        string    `json:"headRefName"`
	BaseRef        string    `json:"baseRefName"`
	URL            string    `json:"url"`
	State          string    `json:"state"`
	Additions      int       `json:"additions"`
	Deletions      int       `json:"deletions"`
	ReviewDecision string    `json:"reviewDecision"`
	CreatedAt      time.Time `json:"createdAt"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	Files []struct {
		Path      string `json:"path"`
		Additions int    `json:"additions"`
		Deletions int    `json:"deletions"`
	} `json:"files"`
}

// GetPRDetail returns detailed information for a single PR.
func GetPRDetail(nwo string, number int) (*PRDetail, error) {
	out, err := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", number),
		"--repo", nwo,
		"--json", "number,title,body,headRefName,baseRefName,url,state,additions,deletions,files,reviewDecision,author,createdAt",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view: %w: %s", err, stderrFrom(err))
	}

	var detail PRDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, fmt.Errorf("parse PR detail: %w", err)
	}
	return &detail, nil
}

// ReviewComment represents a line-level review comment on a PR.
type ReviewComment struct {
	ID          int       `json:"id"`
	Path        string    `json:"path"`
	Line        *int      `json:"line"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
	InReplyToID *int      `json:"in_reply_to_id"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

// ListPRReviewComments returns line-level review comments for a PR.
func ListPRReviewComments(nwo string, number int) ([]ReviewComment, error) {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pulls/%d/comments", nwo, number),
		"--paginate",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api pr comments: %w: %s", err, stderrFrom(err))
	}

	var comments []ReviewComment
	if err := json.Unmarshal(out, &comments); err != nil {
		return nil, fmt.Errorf("parse review comments: %w", err)
	}
	return comments, nil
}

// SubmitPRReview submits a review on a PR (approve, request-changes, or comment).
func SubmitPRReview(nwo string, number int, event, body string) error {
	var flag string
	switch event {
	case "approve":
		flag = "--approve"
	case "request_changes":
		flag = "--request-changes"
	case "comment":
		flag = "--comment"
	default:
		return fmt.Errorf("unknown review event: %s", event)
	}

	args := []string{"pr", "review", fmt.Sprintf("%d", number), "--repo", nwo, flag}
	if body != "" {
		args = append(args, "--body", body)
	}

	out, err := exec.Command("gh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr review: %w: %s", err, out)
	}
	return nil
}

// GetPRDiff returns the diff for a PR.
func GetPRDiff(nwo string, number int) (string, error) {
	out, err := exec.Command("gh", "pr", "diff", fmt.Sprintf("%d", number),
		"--repo", nwo,
	).Output()
	if err != nil {
		return "", fmt.Errorf("gh pr diff: %w: %s", err, stderrFrom(err))
	}
	return string(out), nil
}

func stderrFrom(err error) string {
	if ee, ok := err.(*exec.ExitError); ok {
		return string(ee.Stderr)
	}
	return ""
}
