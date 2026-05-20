package cmd

import (
	"log"
	"time"

	"github.com/lazrossi/code-agent/internal/poller"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Long-running polling daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		st, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer st.Close()

		s3 := openS3()

		interval := cfg.GitHub.PollInterval.Duration
		log.Printf("starting poll daemon (interval: %s)", interval)

		// Run immediately on start
		if err := poller.PollAll(ctx, cfg, st, s3, false); err != nil {
			log.Printf("poll error: %v", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("shutting down")
				return nil
			case <-ticker.C:
				if err := poller.PollAll(ctx, cfg, st, s3, false); err != nil {
					log.Printf("poll error: %v", err)
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
