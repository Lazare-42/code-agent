package cmd

import (
	"github.com/lazrossi/code-agent/internal/poller"
	"github.com/spf13/cobra"
)

var pollDryRun bool

var pollCmd = &cobra.Command{
	Use:   "poll",
	Short: "One-shot poll: check issues and dispatch agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := openStore(cmd.Context())
		if err != nil {
			return err
		}
		defer st.Close()

		s3 := openS3()

		return poller.PollAll(cmd.Context(), cfg, st, s3, pollDryRun)
	},
}

func init() {
	pollCmd.Flags().BoolVar(&pollDryRun, "dry-run", false, "List eligible issues without dispatching agents")
	rootCmd.AddCommand(pollCmd)
}
