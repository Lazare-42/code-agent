package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// RunStatus is the typed status of an agent run.
type RunStatus string

const (
	RunStatusRunning  RunStatus = "running"
	RunStatusSuccess  RunStatus = "success"
	RunStatusFailed   RunStatus = "failed"
	RunStatusTimeout  RunStatus = "timeout"
	RunStatusSkipped  RunStatus = "skipped"
	RunStatusRejected RunStatus = "rejected"
)

// ReviewVerdict is the typed outcome of a surveyor review.
type ReviewVerdict string

const (
	ReviewApprove ReviewVerdict = "approve"
	ReviewReject  ReviewVerdict = "reject"
)

// IssueState is the typed state of a tracked issue.
type IssueState string

const (
	IssueStateOpen   IssueState = "open"
	IssueStateClosed IssueState = "closed"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) InitSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	// Migrations: add task columns (idempotent — SQLite ignores duplicate column adds)
	s.db.ExecContext(ctx, `ALTER TABLE agent_runs ADD COLUMN task_title TEXT`)
	s.db.ExecContext(ctx, `ALTER TABLE agent_runs ADD COLUMN task_prompt TEXT`)
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Issue represents a tracked GitHub issue.
type Issue struct {
	ID        int        `json:"id"`
	Repo      string     `json:"repo"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body,omitempty"`
	Labels    []string   `json:"labels"`
	State     IssueState `json:"state"`
	FirstSeen time.Time  `json:"first_seen"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// AgentRun represents one agent dispatch attempt.
type AgentRun struct {
	ID            int            `json:"id"`
	Repo          string         `json:"repo"`
	IssueNum      int            `json:"issue_num"`
	Branch        string         `json:"branch"`
	Status        RunStatus      `json:"status"`
	PRUrl         string         `json:"pr_url,omitempty"`
	ErrorMsg      string         `json:"error_msg,omitempty"`
	AgentOutput   string         `json:"agent_output,omitempty"`
	ReviewVerdict *ReviewVerdict `json:"review_verdict,omitempty"`
	ReviewComment string         `json:"review_comment,omitempty"`
	TaskTitle     *string        `json:"task_title,omitempty"`
	TaskPrompt    *string        `json:"task_prompt,omitempty"`
	StartedAt     time.Time      `json:"started_at"`
	EndedAt       *time.Time     `json:"ended_at,omitempty"`
	DurationS     *float64       `json:"duration_s,omitempty"`
}

// RunResult carries the outcome of a finished agent run.
type RunResult struct {
	Status      RunStatus
	PRUrl       string
	ErrorMsg    string
	AgentOutput string
	DurationS   float64
}

// UpsertIssue inserts or updates a tracked issue.
func (s *Store) UpsertIssue(ctx context.Context, issue *Issue) error {
	labelsJSON, _ := json.Marshal(issue.Labels)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO issues (repo, number, title, body, labels, state, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(repo, number) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			labels = excluded.labels,
			state = excluded.state,
			updated_at = datetime('now')
	`, issue.Repo, issue.Number, issue.Title, issue.Body, string(labelsJSON), string(issue.State))
	return err
}

// GetIssue returns a specific issue by repo and number.
func (s *Store) GetIssue(ctx context.Context, repo string, number int) (*Issue, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, repo, number, title, body, labels, state, first_seen, updated_at
		FROM issues WHERE repo = ? AND number = ?
	`, repo, number)
	return scanIssue(row)
}

// ListOpenIssues returns all open issues for a repo.
func (s *Store) ListOpenIssues(ctx context.Context, repo string) ([]*Issue, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo, number, title, body, labels, state, first_seen, updated_at
		FROM issues WHERE repo = ? AND state = ?
		ORDER BY number ASC
	`, repo, string(IssueStateOpen))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIssues(rows)
}

// CreateRun inserts a new agent run record and returns its ID.
func (s *Store) CreateRun(ctx context.Context, run *AgentRun) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_runs (repo, issue_num, branch, status, task_title, task_prompt)
		VALUES (?, ?, ?, ?, ?, ?)
	`, run.Repo, run.IssueNum, run.Branch, string(run.Status), run.TaskTitle, run.TaskPrompt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishRun updates a run with its final result.
func (s *Store) FinishRun(ctx context.Context, id int64, result RunResult) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = ?, pr_url = ?, error_msg = ?, agent_output = ?,
		    ended_at = datetime('now'), duration_s = ?
		WHERE id = ?
	`, string(result.Status), result.PRUrl, result.ErrorMsg, result.AgentOutput, result.DurationS, id)
	return err
}

// ReviewRun records a surveyor review on a run. If rejected, also updates status.
func (s *Store) ReviewRun(ctx context.Context, id int64, verdict ReviewVerdict, comment string) error {
	status := RunStatusSuccess
	if verdict == ReviewReject {
		status = RunStatusRejected
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET review_verdict = ?, review_comment = ?, status = ?
		WHERE id = ?
	`, string(verdict), comment, string(status), id)
	return err
}

// GetRun returns a specific agent run.
func (s *Store) GetRun(ctx context.Context, id int64) (*AgentRun, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, repo, issue_num, branch, status, pr_url, error_msg, agent_output,
		       review_verdict, review_comment, task_title, task_prompt,
		       started_at, ended_at, duration_s
		FROM agent_runs WHERE id = ?
	`, id)
	return scanRun(row)
}

// ListRuns returns recent agent runs.
func (s *Store) ListRuns(ctx context.Context, limit int) ([]*AgentRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo, issue_num, branch, status, pr_url, error_msg, agent_output,
		       review_verdict, review_comment, task_title, task_prompt,
		       started_at, ended_at, duration_s
		FROM agent_runs ORDER BY started_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuns(rows)
}

// ListRunsForIssue returns all runs for a specific issue.
func (s *Store) ListRunsForIssue(ctx context.Context, repo string, issueNum int) ([]*AgentRun, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo, issue_num, branch, status, pr_url, error_msg, agent_output,
		       review_verdict, review_comment, task_title, task_prompt,
		       started_at, ended_at, duration_s
		FROM agent_runs WHERE repo = ? AND issue_num = ?
		ORDER BY started_at DESC
	`, repo, issueNum)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRuns(rows)
}

// HasSuccessfulRun checks if there's an approved or currently-running agent run for an issue.
// Rejected runs do NOT block retry.
func (s *Store) HasSuccessfulRun(ctx context.Context, repo string, issueNum int) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_runs
		WHERE repo = ? AND issue_num = ? AND status IN (?, ?)
	`, repo, issueNum, string(RunStatusSuccess), string(RunStatusRunning)).Scan(&count)
	return count > 0, err
}

// CountAttempts returns how many dispatch attempts have been made for an issue
// (anything that isn't "skipped").
func (s *Store) CountAttempts(ctx context.Context, repo string, issueNum int) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_runs
		WHERE repo = ? AND issue_num = ? AND status != ?
	`, repo, issueNum, string(RunStatusSkipped)).Scan(&count)
	return count, err
}

// UpdatePollState records the last poll time for a repo.
func (s *Store) UpdatePollState(ctx context.Context, repo string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO poll_state (repo, last_poll_at)
		VALUES (?, datetime('now'))
		ON CONFLICT(repo) DO UPDATE SET last_poll_at = datetime('now')
	`, repo)
	return err
}

// GetPollState returns the last poll time for a repo.
func (s *Store) GetPollState(ctx context.Context, repo string) (*time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT last_poll_at FROM poll_state WHERE repo = ?
	`, repo).Scan(&t)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Stats holds aggregate statistics for the dashboard.
type Stats struct {
	TotalIssues int            `json:"total_issues"`
	OpenIssues  int            `json:"open_issues"`
	TotalRuns   int            `json:"total_runs"`
	RunsByStatus map[string]int `json:"runs_by_status"`
	RepoCount   int            `json:"repo_count"`
}

// GetStats returns aggregate statistics.
func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{RunsByStatus: make(map[string]int)}

	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues`).Scan(&stats.TotalIssues)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE state = 'open'`).Scan(&stats.OpenIssues)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_runs`).Scan(&stats.TotalRuns)
	s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT repo) FROM issues`).Scan(&stats.RepoCount)

	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM agent_runs GROUP BY status`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err == nil {
				stats.RunsByStatus[status] = count
			}
		}
	}

	return stats, nil
}

// ListAllIssues returns issues across all repos with optional repo filter and pagination.
func (s *Store) ListAllIssues(ctx context.Context, repo string, limit, offset int) ([]*Issue, int, error) {
	var total int
	var rows *sql.Rows
	var err error

	if repo != "" {
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE repo = ?`, repo).Scan(&total)
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, repo, number, title, body, labels, state, first_seen, updated_at
			FROM issues WHERE repo = ?
			ORDER BY updated_at DESC LIMIT ? OFFSET ?
		`, repo, limit, offset)
	} else {
		s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues`).Scan(&total)
		rows, err = s.db.QueryContext(ctx, `
			SELECT id, repo, number, title, body, labels, state, first_seen, updated_at
			FROM issues ORDER BY updated_at DESC LIMIT ? OFFSET ?
		`, limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	issues, err := scanIssues(rows)
	return issues, total, err
}

// ListRunsPaginated returns runs with pagination and total count.
func (s *Store) ListRunsPaginated(ctx context.Context, limit, offset int) ([]*AgentRun, int, error) {
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_runs`).Scan(&total)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo, issue_num, branch, status, pr_url, error_msg, agent_output,
		       review_verdict, review_comment, task_title, task_prompt,
		       started_at, ended_at, duration_s
		FROM agent_runs ORDER BY started_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	runs, err := scanRuns(rows)
	return runs, total, err
}

// ListTasksPaginated returns standalone task runs (task_title IS NOT NULL) with pagination.
func (s *Store) ListTasksPaginated(ctx context.Context, limit, offset int) ([]*AgentRun, int, error) {
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_runs WHERE task_title IS NOT NULL`).Scan(&total)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, repo, issue_num, branch, status, pr_url, error_msg, agent_output,
		       review_verdict, review_comment, task_title, task_prompt,
		       started_at, ended_at, duration_s
		FROM agent_runs WHERE task_title IS NOT NULL
		ORDER BY started_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	runs, err := scanRuns(rows)
	return runs, total, err
}

// RecoverOrphanedRuns marks all "running" runs as failed on startup.
func (s *Store) RecoverOrphanedRuns(ctx context.Context) (int, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = 'failed', error_msg = 'orphaned: process restarted', ended_at = datetime('now')
		WHERE status = 'running'
	`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// HasRunningTask checks if there's already a running task with the same repo and title.
func (s *Store) HasRunningTask(ctx context.Context, repo, title string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM agent_runs
		WHERE repo = ? AND task_title = ? AND status = 'running'
	`, repo, title).Scan(&count)
	return count > 0, err
}

// --- scanners ---

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (*AgentRun, error) {
	var r AgentRun
	var status string
	var prURL, errMsg, output, reviewVerdict, reviewComment, taskTitle, taskPrompt sql.NullString
	var endedAt sql.NullTime
	var durationS sql.NullFloat64
	err := row.Scan(&r.ID, &r.Repo, &r.IssueNum, &r.Branch, &status,
		&prURL, &errMsg, &output, &reviewVerdict, &reviewComment,
		&taskTitle, &taskPrompt,
		&r.StartedAt, &endedAt, &durationS)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Status = RunStatus(status)
	if prURL.Valid {
		r.PRUrl = prURL.String
	}
	if errMsg.Valid {
		r.ErrorMsg = errMsg.String
	}
	if output.Valid {
		r.AgentOutput = output.String
	}
	if reviewVerdict.Valid {
		v := ReviewVerdict(reviewVerdict.String)
		r.ReviewVerdict = &v
	}
	if reviewComment.Valid {
		r.ReviewComment = reviewComment.String
	}
	if taskTitle.Valid {
		r.TaskTitle = &taskTitle.String
	}
	if taskPrompt.Valid {
		r.TaskPrompt = &taskPrompt.String
	}
	if endedAt.Valid {
		r.EndedAt = &endedAt.Time
	}
	if durationS.Valid {
		r.DurationS = &durationS.Float64
	}
	return &r, nil
}

func scanRuns(rows *sql.Rows) ([]*AgentRun, error) {
	var runs []*AgentRun
	for rows.Next() {
		var r AgentRun
		var status string
		var prURL, errMsg, output, reviewVerdict, reviewComment, taskTitle, taskPrompt sql.NullString
		var endedAt sql.NullTime
		var durationS sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.Repo, &r.IssueNum, &r.Branch, &status,
			&prURL, &errMsg, &output, &reviewVerdict, &reviewComment,
			&taskTitle, &taskPrompt,
			&r.StartedAt, &endedAt, &durationS); err != nil {
			return nil, err
		}
		r.Status = RunStatus(status)
		if prURL.Valid {
			r.PRUrl = prURL.String
		}
		if errMsg.Valid {
			r.ErrorMsg = errMsg.String
		}
		if output.Valid {
			r.AgentOutput = output.String
		}
		if reviewVerdict.Valid {
			v := ReviewVerdict(reviewVerdict.String)
			r.ReviewVerdict = &v
		}
		if reviewComment.Valid {
			r.ReviewComment = reviewComment.String
		}
		if taskTitle.Valid {
			r.TaskTitle = &taskTitle.String
		}
		if taskPrompt.Valid {
			r.TaskPrompt = &taskPrompt.String
		}
		if endedAt.Valid {
			r.EndedAt = &endedAt.Time
		}
		if durationS.Valid {
			r.DurationS = &durationS.Float64
		}
		runs = append(runs, &r)
	}
	return runs, rows.Err()
}

func scanIssue(row rowScanner) (*Issue, error) {
	var i Issue
	var labelsJSON, state string
	err := row.Scan(&i.ID, &i.Repo, &i.Number, &i.Title, &i.Body, &labelsJSON, &state, &i.FirstSeen, &i.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i.State = IssueState(state)
	_ = json.Unmarshal([]byte(labelsJSON), &i.Labels)
	return &i, nil
}

func scanIssues(rows *sql.Rows) ([]*Issue, error) {
	var issues []*Issue
	for rows.Next() {
		var i Issue
		var labelsJSON, state string
		if err := rows.Scan(&i.ID, &i.Repo, &i.Number, &i.Title, &i.Body, &labelsJSON, &state, &i.FirstSeen, &i.UpdatedAt); err != nil {
			return nil, err
		}
		i.State = IssueState(state)
		_ = json.Unmarshal([]byte(labelsJSON), &i.Labels)
		issues = append(issues, &i)
	}
	return issues, rows.Err()
}
