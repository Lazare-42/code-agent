package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/lazrossi/code-agent/internal/agent"
	"github.com/lazrossi/code-agent/internal/config"
	gh "github.com/lazrossi/code-agent/internal/github"
	"github.com/lazrossi/code-agent/internal/poller"
	"github.com/lazrossi/code-agent/internal/storage"
	"github.com/lazrossi/code-agent/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
)

type handlers struct {
	cfg   *config.Config
	store *store.Store
	s3    *storage.Client
}

func (h *handlers) listRepos(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	type repoInfo struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Enabled bool   `json:"enabled"`
	}

	repos := make([]repoInfo, 0, len(h.cfg.Repos))
	for name, repo := range h.cfg.Repos {
		repos = append(repos, repoInfo{
			Name:    name,
			Path:    repo.Path,
			Enabled: repo.Enabled,
		})
	}

	return jsonResult(repos)
}

func (h *handlers) listIssues(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	repoFilter, _ := args["repo"].(string)

	type issueInfo struct {
		Repo     string   `json:"repo"`
		Number   int      `json:"number"`
		Title    string   `json:"title"`
		Labels   []string `json:"labels"`
		Eligible bool     `json:"eligible"`
		Reason   string   `json:"reason,omitempty"`
	}

	var allIssues []issueInfo

	for name, repo := range h.cfg.EnabledRepos() {
		if repoFilter != "" && name != repoFilter {
			continue
		}

		ghRepoName, err := gh.RepoNameFromPath(repo.Path)
		if err != nil {
			continue
		}

		var labelFilter []string
		if h.cfg.GitHub.RequireLabel != "" {
			labelFilter = []string{h.cfg.GitHub.RequireLabel}
		}

		issues, err := gh.ListIssues(h.cfg.GitHub.Org, ghRepoName, labelFilter)
		if err != nil {
			continue
		}

		for i := range issues {
			issue := &issues[i]
			reason := poller.EligibilityReason(ctx, h.cfg, h.store, repo.Path, issue)

			allIssues = append(allIssues, issueInfo{
				Repo:     name,
				Number:   issue.Number,
				Title:    issue.Title,
				Labels:   issue.LabelNames(),
				Eligible: reason == "",
				Reason:   reason,
			})
		}
	}

	return jsonResult(allIssues)
}

func (h *handlers) dispatchAgent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	repoName, _ := args["repo"].(string)
	if repoName == "" {
		return mcp.NewToolResultError("repo parameter is required"), nil
	}

	issueNum, err := getIDArg(args, "issue_number")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	repo, ok := h.cfg.Repos[repoName]
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf("repo %q not found in config", repoName)), nil
	}
	if !repo.Enabled {
		return mcp.NewToolResultError(fmt.Sprintf("repo %q is disabled", repoName)), nil
	}

	ghRepoName, err := gh.RepoNameFromPath(repo.Path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get repo name: %v", err)), nil
	}

	issues, err := gh.ListIssues(h.cfg.GitHub.Org, ghRepoName, nil)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list issues: %v", err)), nil
	}

	var target *gh.Issue
	for i := range issues {
		if issues[i].Number == int(issueNum) {
			target = &issues[i]
			break
		}
	}
	if target == nil {
		return mcp.NewToolResultError(fmt.Sprintf("issue #%d not found in %s", issueNum, repoName)), nil
	}

	result, err := agent.Dispatch(ctx, h.cfg, h.store, h.s3, repo.Path, target)
	if err != nil {
		return jsonResult(struct {
			Status store.RunStatus `json:"status"`
			Error  string          `json:"error"`
		}{Status: result.Status, Error: result.Error})
	}

	return jsonResult(struct {
		Status  store.RunStatus `json:"status"`
		PRUrl   string          `json:"pr_url,omitempty"`
		Elapsed float64         `json:"elapsed_seconds"`
	}{
		Status:  result.Status,
		PRUrl:   result.PRUrl,
		Elapsed: result.Elapsed.Seconds(),
	})
}

func (h *handlers) getAgentStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	runID, err := getIDArg(args, "run_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	run, err := h.store.GetRun(ctx, runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get run: %v", err)), nil
	}
	if run == nil {
		return mcp.NewToolResultError("run not found"), nil
	}

	return jsonResult(run)
}

func (h *handlers) listRuns(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	limit := limitArg(args, "limit", 20)

	runs, err := h.store.ListRuns(ctx, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list runs: %v", err)), nil
	}

	return jsonResult(runs)
}

// --- Helpers ---

func getIDArg(args map[string]any, key string) (int64, error) {
	v, ok := args[key].(float64)
	if !ok {
		return 0, fmt.Errorf("%s parameter is required", key)
	}
	if v < 0 || v != math.Trunc(v) {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return int64(v), nil
}

func limitArg(args map[string]any, key string, def int) int {
	v, ok := args[key].(float64)
	if !ok {
		return def
	}
	if v < 0 {
		return 0
	}
	if v > 1000 {
		return 1000
	}
	return int(v)
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
