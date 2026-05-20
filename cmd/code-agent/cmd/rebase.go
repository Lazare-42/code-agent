package cmd

import (
	"log"

	"github.com/lazrossi/code-agent/internal/agent"
	"github.com/spf13/cobra"
)

var rebaseDryRun bool

var rebaseCmd = &cobra.Command{
	Use:   "rebase",
	Short: "Rebase stale agent PRs onto their base branch",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		repos := cfg.EnabledRepos()
		if len(repos) == 0 {
			log.Println("no enabled repos configured")
			return nil
		}

		for name, repo := range repos {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			log.Printf("checking repo: %s (%s)", name, repo.Path)
			if err := agent.RebaseStale(ctx, cfg, repo.Path, rebaseDryRun); err != nil {
				log.Printf("  %s: rebase error: %v", name, err)
			}
		}
		return nil
	},
}

func init() {
	rebaseCmd.Flags().BoolVar(&rebaseDryRun, "dry-run", false, "Show stale PRs without rebasing")
	rootCmd.AddCommand(rebaseCmd)
}
