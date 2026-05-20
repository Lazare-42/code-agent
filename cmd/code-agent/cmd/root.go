package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/lazrossi/code-agent/internal/config"
	"github.com/lazrossi/code-agent/internal/storage"
	"github.com/lazrossi/code-agent/internal/store"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "code-agent",
	Short: "GitHub issue agent dispatcher — polls issues, runs Claude Code, opens PRs",
}

func Execute() error {
	return rootCmd.Execute()
}

func ExecuteContext(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $CODE_AGENT_HOME/config.toml)")
}

func initConfig() {
	if cfgFile == "" {
		cfgFile = filepath.Join(config.Home(), "config.toml")
	}

	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		cfg = config.Default()
	}
}

// openStore opens the SQLite store, creating the parent directory and schema as needed.
// It also recovers any orphaned runs left from a previous crash.
func openStore(ctx context.Context) (*store.Store, error) {
	dbPath := cfg.DBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	st, err := store.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := st.InitSchema(ctx); err != nil {
		st.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	if n, err := st.RecoverOrphanedRuns(ctx); err != nil {
		log.Printf("orphan recovery error: %v", err)
	} else if n > 0 {
		log.Printf("recovered %d orphaned runs", n)
	}

	return st, nil
}

// openS3 creates an S3 storage client from config. Returns nil if not configured or unavailable.
func openS3() *storage.Client {
	if cfg.S3.Endpoint == "" {
		return nil
	}
	s3, err := storage.New(cfg.S3)
	if err != nil {
		log.Printf("S3 storage unavailable: %v (traces and log archival disabled)", err)
		return nil
	}
	return s3
}
