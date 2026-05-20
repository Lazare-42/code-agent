package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration for code-agent.
type Config struct {
	GitHub   GitHubConfig          `toml:"github"`
	Agent    AgentConfig           `toml:"agent"`
	Database DatabaseConfig        `toml:"database"`
	HTTP     HTTPConfig            `toml:"http"`
	S3       S3Config              `toml:"s3"`
	Repos    map[string]RepoConfig `toml:"repos"`
}

// S3Config controls optional MinIO/S3 artifact storage.
type S3Config struct {
	Endpoint  string `toml:"endpoint"`
	AccessKey string `toml:"access_key"`
	SecretKey string `toml:"secret_key"`
	Bucket    string `toml:"bucket"`
	UseSSL    bool   `toml:"use_ssl"`
}

// GitHubConfig controls which issues get picked up.
type GitHubConfig struct {
	Org          string   `toml:"org"`
	PollInterval Duration `toml:"poll_interval"`
	RequireLabel string   `toml:"require_label"`
	SkipLabels   []string `toml:"skip_labels"`
}

// AgentConfig controls the claude -p invocation.
type AgentConfig struct {
	Model       string   `toml:"model"`
	MaxTurns    int      `toml:"max_turns"`
	Timeout     Duration `toml:"timeout"`
	Concurrency int      `toml:"concurrency"`
	MaxAttempts int      `toml:"max_attempts"`
	MaxPRAge    Duration `toml:"max_pr_age"`
}

// DatabaseConfig controls the SQLite database location.
type DatabaseConfig struct {
	Path string `toml:"path"`
}

// HTTPConfig controls the optional HTTP MCP server.
type HTTPConfig struct {
	Addr   string `toml:"addr"`
	APIKey string `toml:"api_key"`
}

// RepoConfig maps a repo name to a local checkout path.
type RepoConfig struct {
	Path    string `toml:"path"`
	Enabled bool   `toml:"enabled"`
}

// Duration wraps time.Duration for TOML string unmarshalling ("30m", "15m").
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// Home returns the CODE_AGENT_HOME directory, defaulting to ~/.code-agent.
func Home() string {
	if h := os.Getenv("CODE_AGENT_HOME"); h != "" {
		return h
	}
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(userHome, ".code-agent")
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		GitHub: GitHubConfig{
			Org:          "Meeting-Baas",
			PollInterval: Duration{30 * time.Minute},
			SkipLabels:   []string{"wontfix", "duplicate", "invalid", "question"},
		},
		Agent: AgentConfig{
			Model:       "sonnet",
			MaxTurns:    30,
			Timeout:     Duration{15 * time.Minute},
			Concurrency: 1,
			MaxAttempts: 3,
			MaxPRAge:    Duration{30 * 24 * time.Hour},
		},
		HTTP: HTTPConfig{
			Addr: "127.0.0.1:8091",
		},
		S3: S3Config{
			Endpoint:  "localhost:9000",
			Bucket:    "code-agent",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
		},
		Repos: make(map[string]RepoConfig),
	}
}

// Load reads a TOML config file, falling back to defaults for missing fields.
func Load(path string) (*Config, error) {
	c := Default()
	if _, err := toml.DecodeFile(path, c); err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	if c.Repos == nil {
		c.Repos = make(map[string]RepoConfig)
	}
	return c, nil
}

// DBPath returns the resolved database file path.
func (c *Config) DBPath() string {
	if c.Database.Path != "" {
		return c.Database.Path
	}
	return filepath.Join(Home(), "code-agent.db")
}

// EnabledRepos returns only repos where Enabled is true.
func (c *Config) EnabledRepos() map[string]RepoConfig {
	enabled := make(map[string]RepoConfig)
	for name, repo := range c.Repos {
		if repo.Enabled {
			enabled[name] = repo
		}
	}
	return enabled
}

// DiscoverRepos scans a base directory for git repos with a remote matching the org.
func DiscoverRepos(baseDir, org string) map[string]RepoConfig {
	repos := make(map[string]RepoConfig)

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return repos
	}

	orgLower := strings.ToLower(org)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		repoPath := filepath.Join(baseDir, e.Name())
		gitDir := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitDir); err != nil {
			continue
		}

		out, err := exec.Command("git", "-C", repoPath, "remote", "-v").Output()
		if err != nil {
			continue
		}
		remotes := strings.ToLower(string(out))
		if !strings.Contains(remotes, orgLower) {
			continue
		}
		// Only include GitHub repos, skip GitLab and others
		if !strings.Contains(remotes, "github.com") {
			continue
		}
		repos[e.Name()] = RepoConfig{
			Path:    repoPath,
			Enabled: true,
		}
	}

	return repos
}
