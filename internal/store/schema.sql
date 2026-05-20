CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo TEXT NOT NULL,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    labels TEXT NOT NULL DEFAULT '[]',
    state TEXT NOT NULL DEFAULT 'open',
    first_seen DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now')),
    UNIQUE(repo, number)
);

CREATE TABLE IF NOT EXISTS agent_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo TEXT NOT NULL,
    issue_num INTEGER NOT NULL,
    branch TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',  -- running/success/failed/timeout/skipped/rejected
    pr_url TEXT,
    error_msg TEXT,
    agent_output TEXT,
    review_verdict TEXT,   -- approve/reject (NULL if not yet reviewed)
    review_comment TEXT,
    started_at DATETIME DEFAULT (datetime('now')),
    ended_at DATETIME,
    duration_s REAL
);

CREATE TABLE IF NOT EXISTS poll_state (
    repo TEXT PRIMARY KEY,
    last_poll_at DATETIME DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_issues_repo_state ON issues(repo, state);
CREATE INDEX IF NOT EXISTS idx_agent_runs_repo_issue ON agent_runs(repo, issue_num);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status);
