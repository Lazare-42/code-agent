package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/lazrossi/code-agent/internal/agent"
	gh "github.com/lazrossi/code-agent/internal/github"
	"github.com/lazrossi/code-agent/internal/store"
)

type listResponse struct {
	Data any            `json:"data"`
	Meta paginationMeta `json:"meta"`
}

type detailResponse struct {
	Data any `json:"data"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type paginationMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, detailResponse{Data: stats})
}

// pathToName returns the config name for a repo path, or the path's base dir as fallback.
func (h *Handler) pathToName(path string) string {
	for name, repo := range h.cfg.Repos {
		if repo.Path == path {
			return name
		}
	}
	return path
}

// nameToPath returns the filesystem path for a config name.
func (h *Handler) nameToPath(name string) string {
	if repo, ok := h.cfg.Repos[name]; ok {
		return repo.Path
	}
	return name
}

func (h *Handler) listRepos(w http.ResponseWriter, r *http.Request) {
	type repoInfo struct {
		Name       string  `json:"name"`
		Path       string  `json:"path"`
		Remote     string  `json:"remote,omitempty"`
		Enabled    bool    `json:"enabled"`
		LastPollAt *string `json:"last_poll_at,omitempty"`
	}

	var repos []repoInfo
	for name, repo := range h.cfg.Repos {
		ri := repoInfo{
			Name:    name,
			Path:    repo.Path,
			Enabled: repo.Enabled,
		}
		if nwo, err := gh.NWOFromPath(repo.Path); err == nil && nwo != "" {
			ri.Remote = nwo
		}
		// Poll state is stored by path, not config name
		if t, err := h.store.GetPollState(r.Context(), repo.Path); err == nil && t != nil {
			s := t.Format("2006-01-02T15:04:05Z")
			ri.LastPollAt = &s
		}
		repos = append(repos, ri)
	}
	if repos == nil {
		repos = []repoInfo{}
	}
	writeJSON(w, http.StatusOK, listResponse{
		Data: repos,
		Meta: paginationMeta{Total: len(repos), Limit: len(repos), Offset: 0},
	})
}

type issueResponse struct {
	ID        int        `json:"id"`
	Repo      string     `json:"repo"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body,omitempty"`
	Labels    []string   `json:"labels"`
	State     string     `json:"state"`
	URL       string     `json:"url,omitempty"`
	RepoURL   string     `json:"repo_url,omitempty"`
	FirstSeen string     `json:"first_seen"`
	UpdatedAt string     `json:"updated_at"`
}

func (h *Handler) issueToResponse(issue *store.Issue) issueResponse {
	name := h.pathToName(issue.Repo)
	nwo, _ := gh.NWOFromPath(issue.Repo)
	var issueURL, repoURL string
	if nwo != "" {
		issueURL = fmt.Sprintf("https://github.com/%s/issues/%d", nwo, issue.Number)
		repoURL = fmt.Sprintf("https://github.com/%s", nwo)
	}
	return issueResponse{
		ID:        issue.ID,
		Repo:      name,
		Number:    issue.Number,
		Title:     issue.Title,
		Body:      issue.Body,
		Labels:    issue.Labels,
		State:     string(issue.State),
		URL:       issueURL,
		RepoURL:   repoURL,
		FirstSeen: issue.FirstSeen.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: issue.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (h *Handler) listIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	repoFilter := q.Get("repo")
	limit := queryInt(q, "limit", 50)
	offset := queryInt(q, "offset", 0)

	// If filtering by config name, translate to path for the store query
	storeRepo := repoFilter
	if repoFilter != "" {
		storeRepo = h.nameToPath(repoFilter)
	}

	issues, total, err := h.store.ListAllIssues(r.Context(), storeRepo, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// De-duplicate issues by (NWO, number) — multiple local paths may
	// point to the same GitHub remote.
	type dedupKey struct{ nwo string; num int }
	seen := make(map[dedupKey]bool)
	var results []issueResponse
	for _, issue := range issues {
		nwo, _ := gh.NWOFromPath(issue.Repo)
		key := dedupKey{nwo, issue.Number}
		if nwo != "" && seen[key] {
			total-- // adjust count for deduped items
			continue
		}
		seen[key] = true
		results = append(results, h.issueToResponse(issue))
	}
	if results == nil {
		results = []issueResponse{}
	}
	writeJSON(w, http.StatusOK, listResponse{
		Data: results,
		Meta: paginationMeta{Total: total, Limit: limit, Offset: offset},
	})
}

func (h *Handler) getIssue(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repo")
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid issue number")
		return
	}
	// Translate config name to path for store lookup
	storePath := h.nameToPath(repo)
	issue, err := h.store.GetIssue(r.Context(), storePath, number)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if issue == nil {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	writeJSON(w, http.StatusOK, detailResponse{Data: h.issueToResponse(issue)})
}

func (h *Handler) createIssue(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo   string   `json:"repo"`
		Title  string   `json:"title"`
		Body   string   `json:"body"`
		Labels []string `json:"labels"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Repo == "" || body.Title == "" {
		writeError(w, http.StatusBadRequest, "repo and title are required")
		return
	}

	// Find the NWO for this repo
	repoConf, ok := h.cfg.Repos[body.Repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found in config", body.Repo))
		return
	}
	nwo, err := gh.NWOFromPath(repoConf.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	number, url, err := gh.CreateIssue(nwo, body.Title, body.Body, body.Labels)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Track the issue in the store
	_ = h.store.UpsertIssue(r.Context(), &store.Issue{
		Repo:   body.Repo,
		Number: number,
		Title:  body.Title,
		Body:   body.Body,
		Labels: body.Labels,
		State:  store.IssueStateOpen,
	})

	writeJSON(w, http.StatusCreated, detailResponse{Data: struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}{Number: number, URL: url}})
}

func (h *Handler) draftIssue(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo        string `json:"repo"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Repo == "" || body.Description == "" {
		writeError(w, http.StatusBadRequest, "repo and description are required")
		return
	}

	if _, ok := h.cfg.Repos[body.Repo]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found in config", body.Repo))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	draft, err := agent.DraftIssue(ctx, h.cfg, body.Repo, body.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detailResponse{Data: draft})
}

type runResponse struct {
	ID            int      `json:"id"`
	Repo          string   `json:"repo"`
	IssueNum      int      `json:"issue_num"`
	Branch        string   `json:"branch"`
	Status        string   `json:"status"`
	PRUrl         string   `json:"pr_url,omitempty"`
	ErrorMsg      string   `json:"error_msg,omitempty"`
	ReviewVerdict *string  `json:"review_verdict,omitempty"`
	ReviewComment string   `json:"review_comment,omitempty"`
	TaskTitle     *string  `json:"task_title,omitempty"`
	TaskPrompt    *string  `json:"task_prompt,omitempty"`
	StartedAt     string   `json:"started_at"`
	EndedAt       *string  `json:"ended_at,omitempty"`
	DurationS     *float64 `json:"duration_s,omitempty"`
}

func (h *Handler) runToResponse(run *store.AgentRun) runResponse {
	rr := runResponse{
		ID:            run.ID,
		Repo:          h.pathToName(run.Repo),
		IssueNum:      run.IssueNum,
		Branch:        run.Branch,
		Status:        string(run.Status),
		PRUrl:         run.PRUrl,
		ErrorMsg:      run.ErrorMsg,
		ReviewComment: run.ReviewComment,
		TaskTitle:     run.TaskTitle,
		TaskPrompt:    run.TaskPrompt,
		StartedAt:     run.StartedAt.Format("2006-01-02T15:04:05Z"),
		DurationS:     run.DurationS,
	}
	if run.ReviewVerdict != nil {
		v := string(*run.ReviewVerdict)
		rr.ReviewVerdict = &v
	}
	if run.EndedAt != nil {
		s := run.EndedAt.Format("2006-01-02T15:04:05Z")
		rr.EndedAt = &s
	}
	return rr
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := queryInt(q, "limit", 20)
	offset := queryInt(q, "offset", 0)

	runs, total, err := h.store.ListRunsPaginated(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	results := make([]runResponse, 0, len(runs))
	for _, run := range runs {
		results = append(results, h.runToResponse(run))
	}
	writeJSON(w, http.StatusOK, listResponse{
		Data: results,
		Meta: paginationMeta{Total: total, Limit: limit, Offset: offset},
	})
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}
	run, err := h.store.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	writeJSON(w, http.StatusOK, detailResponse{Data: h.runToResponse(run)})
}

func (h *Handler) getRunOutput(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}
	run, err := h.store.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	// Try log file first (live or recent runs)
	logPath := agent.LogPath(id)
	if data, err := os.ReadFile(logPath); err == nil {
		writeJSON(w, http.StatusOK, detailResponse{Data: struct {
			Output string `json:"output"`
			Live   bool   `json:"live"`
		}{Output: string(data), Live: run.Status == store.RunStatusRunning}})
		return
	}

	// Fall back to S3
	if h.s3 != nil {
		if s3data, err := h.s3.GetRunLog(r.Context(), id); err == nil {
			writeJSON(w, http.StatusOK, detailResponse{Data: struct {
				Output string `json:"output"`
				Live   bool   `json:"live"`
			}{Output: string(s3data), Live: false}})
			return
		}
	}

	// Fall back to DB
	writeJSON(w, http.StatusOK, detailResponse{Data: struct {
		Output string `json:"output"`
		Live   bool   `json:"live"`
	}{Output: run.AgentOutput, Live: false}})
}

func (h *Handler) getRunTrace(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}
	if h.s3 == nil {
		writeError(w, http.StatusNotFound, "trace storage not configured")
		return
	}
	events, err := h.s3.GetRunTrace(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}
	writeJSON(w, http.StatusOK, detailResponse{Data: events})
}

func (h *Handler) dispatchAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo        string `json:"repo"`
		IssueNumber int    `json:"issue_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Repo == "" || body.IssueNumber == 0 {
		writeError(w, http.StatusBadRequest, "repo and issue_number are required")
		return
	}

	repoConf, ok := h.cfg.Repos[body.Repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found in config", body.Repo))
		return
	}
	if !repoConf.Enabled {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q is disabled", body.Repo))
		return
	}

	ghRepoName, err := gh.RepoNameFromPath(repoConf.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	issues, err := gh.ListIssues(h.cfg.GitHub.Org, ghRepoName, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var target *gh.Issue
	for i := range issues {
		if issues[i].Number == body.IssueNumber {
			target = &issues[i]
			break
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("issue #%d not found in %s", body.IssueNumber, body.Repo))
		return
	}

	// Dispatch asynchronously
	go func() {
		ctx := context.Background()
		agent.Dispatch(ctx, h.cfg, h.store, h.s3, repoConf.Path, target)
	}()

	writeJSON(w, http.StatusAccepted, detailResponse{Data: struct {
		Status string `json:"status"`
	}{Status: "dispatched"}})
}

func (h *Handler) triggerRebase(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo string `json:"repo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Repo == "" {
		writeError(w, http.StatusBadRequest, "repo is required")
		return
	}

	repoConf, ok := h.cfg.Repos[body.Repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found in config", body.Repo))
		return
	}

	go func() {
		ctx := context.Background()
		agent.RebaseStale(ctx, h.cfg, repoConf.Path, false)
	}()

	writeJSON(w, http.StatusAccepted, detailResponse{Data: struct {
		Status string `json:"status"`
	}{Status: "rebase_started"}})
}

// --- PR handlers ---

type prSummary struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Repo           string `json:"repo"`
	Author         string `json:"author"`
	URL            string `json:"url"`
	ReviewDecision string `json:"review_decision"`
	CreatedAt      string `json:"created_at"`
}

func (h *Handler) listPRs(w http.ResponseWriter, r *http.Request) {
	type repoJob struct {
		name string
		nwo  string
	}

	var jobs []repoJob
	seen := make(map[string]string) // nwo → first config name (dedup)
	for name, repo := range h.cfg.Repos {
		if !repo.Enabled {
			continue
		}
		nwo, err := gh.NWOFromPath(repo.Path)
		if err != nil {
			continue
		}
		if _, exists := seen[nwo]; exists {
			continue
		}
		seen[nwo] = name
		jobs = append(jobs, repoJob{name: name, nwo: nwo})
	}

	var mu sync.Mutex
	var allPRs []prSummary
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j repoJob) {
			defer wg.Done()
			prs, err := gh.ListOpenPRs(j.nwo)
			if err != nil {
				return
			}
			mu.Lock()
			for _, pr := range prs {
				allPRs = append(allPRs, prSummary{
					Number:    pr.Number,
					Title:     pr.Title,
					Repo:      j.name,
					Author:    pr.Author.Login,
					URL:       pr.URL,
					CreatedAt: pr.CreatedAt.Format("2006-01-02T15:04:05Z"),
				})
			}
			mu.Unlock()
		}(job)
	}
	wg.Wait()

	if allPRs == nil {
		allPRs = []prSummary{}
	}
	writeJSON(w, http.StatusOK, listResponse{
		Data: allPRs,
		Meta: paginationMeta{Total: len(allPRs), Limit: len(allPRs), Offset: 0},
	})
}

type prDetailResponse struct {
	Number         int              `json:"number"`
	Title          string           `json:"title"`
	Body           string           `json:"body"`
	HeadRef        string           `json:"head_ref"`
	BaseRef        string           `json:"base_ref"`
	URL            string           `json:"url"`
	State          string           `json:"state"`
	Additions      int              `json:"additions"`
	Deletions      int              `json:"deletions"`
	ReviewDecision string           `json:"review_decision"`
	Author         string           `json:"author"`
	CreatedAt      string           `json:"created_at"`
	Diff           string           `json:"diff"`
	Comments       []commentResponse `json:"comments"`
	Files          []fileResponse   `json:"files"`
}

type commentResponse struct {
	ID          int    `json:"id"`
	Path        string `json:"path"`
	Line        *int   `json:"line"`
	Body        string `json:"body"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
	InReplyToID *int   `json:"in_reply_to_id"`
}

type fileResponse struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

func (h *Handler) getPRDetail(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repo")
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR number")
		return
	}

	repoConf, ok := h.cfg.Repos[repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found", repo))
		return
	}
	nwo, err := gh.NWOFromPath(repoConf.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	detail, err := gh.GetPRDetail(nwo, number)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	comments, _ := gh.ListPRReviewComments(nwo, number)
	diff, _ := gh.GetPRDiff(nwo, number)

	var commentResps []commentResponse
	for _, c := range comments {
		commentResps = append(commentResps, commentResponse{
			ID:          c.ID,
			Path:        c.Path,
			Line:        c.Line,
			Body:        c.Body,
			Author:      c.User.Login,
			CreatedAt:   c.CreatedAt.Format("2006-01-02T15:04:05Z"),
			InReplyToID: c.InReplyToID,
		})
	}
	if commentResps == nil {
		commentResps = []commentResponse{}
	}

	var fileResps []fileResponse
	for _, f := range detail.Files {
		fileResps = append(fileResps, fileResponse{
			Path:      f.Path,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}
	if fileResps == nil {
		fileResps = []fileResponse{}
	}

	resp := prDetailResponse{
		Number:         detail.Number,
		Title:          detail.Title,
		Body:           detail.Body,
		HeadRef:        detail.HeadRef,
		BaseRef:        detail.BaseRef,
		URL:            detail.URL,
		State:          detail.State,
		Additions:      detail.Additions,
		Deletions:      detail.Deletions,
		ReviewDecision: detail.ReviewDecision,
		Author:         detail.Author.Login,
		CreatedAt:      detail.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Diff:           diff,
		Comments:       commentResps,
		Files:          fileResps,
	}

	writeJSON(w, http.StatusOK, detailResponse{Data: resp})
}

func (h *Handler) reviewPR(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repo")
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR number")
		return
	}

	var body struct {
		Event string `json:"event"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Event == "" {
		writeError(w, http.StatusBadRequest, "event is required")
		return
	}

	repoConf, ok := h.cfg.Repos[repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found", repo))
		return
	}
	nwo, err := gh.NWOFromPath(repoConf.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := gh.SubmitPRReview(nwo, number, body.Event, body.Body); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, detailResponse{Data: struct {
		Status string `json:"status"`
	}{Status: "review_submitted"}})
}

func (h *Handler) resolvePRComments(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repo")
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR number")
		return
	}

	repoConf, ok := h.cfg.Repos[repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found", repo))
		return
	}
	nwo, err := gh.NWOFromPath(repoConf.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	comments, err := gh.ListPRReviewComments(nwo, number)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go func() {
		ctx := context.Background()
		agent.ResolveComments(ctx, h.cfg, h.store, h.s3, repoConf.Path, number, nwo, comments)
	}()

	writeJSON(w, http.StatusAccepted, detailResponse{Data: struct {
		Status string `json:"status"`
	}{Status: "resolve_dispatched"}})
}

func (h *Handler) dispatchAgentOnPR(w http.ResponseWriter, r *http.Request) {
	repo := r.PathValue("repo")
	number, err := strconv.Atoi(r.PathValue("number"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR number")
		return
	}

	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	repoConf, ok := h.cfg.Repos[repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found", repo))
		return
	}
	nwo, err := gh.NWOFromPath(repoConf.Path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go func() {
		ctx := context.Background()
		agent.RunOnPR(ctx, h.cfg, h.store, h.s3, repoConf.Path, number, nwo, body.Prompt)
	}()

	writeJSON(w, http.StatusAccepted, detailResponse{Data: struct {
		Status string `json:"status"`
	}{Status: "agent_dispatched"}})
}

// --- Task handlers ---

func (h *Handler) listTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := queryInt(q, "limit", 20)
	offset := queryInt(q, "offset", 0)

	runs, total, err := h.store.ListTasksPaginated(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	results := make([]runResponse, 0, len(runs))
	for _, run := range runs {
		results = append(results, h.runToResponse(run))
	}
	writeJSON(w, http.StatusOK, listResponse{
		Data: results,
		Meta: paginationMeta{Total: total, Limit: limit, Offset: offset},
	})
}

func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Repo   string `json:"repo"`
		Title  string `json:"title"`
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Repo == "" || body.Title == "" || body.Prompt == "" {
		writeError(w, http.StatusBadRequest, "repo, title, and prompt are required")
		return
	}

	repoConf, ok := h.cfg.Repos[body.Repo]
	if !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q not found in config", body.Repo))
		return
	}
	if !repoConf.Enabled {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("repo %q is disabled", body.Repo))
		return
	}

	if has, _ := h.store.HasRunningTask(r.Context(), repoConf.Path, body.Title); has {
		writeError(w, http.StatusConflict, "a task with this title is already running")
		return
	}

	go func() {
		ctx := context.Background()
		agent.RunTask(ctx, h.cfg, h.store, h.s3, repoConf.Path, body.Title, body.Prompt)
	}()

	writeJSON(w, http.StatusAccepted, detailResponse{Data: struct {
		Status string `json:"status"`
	}{Status: "task_dispatched"}})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

func queryInt(q interface{ Get(string) string }, key string, def int) int {
	s := q.Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
