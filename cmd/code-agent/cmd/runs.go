package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var runsLimit int

var runsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List recent agent runs",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := openStore(cmd.Context())
		if err != nil {
			return err
		}
		defer st.Close()

		runs, err := st.ListRuns(cmd.Context(), runsLimit)
		if err != nil {
			return fmt.Errorf("list runs: %w", err)
		}

		if len(runs) == 0 {
			fmt.Println("No agent runs found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tREPO\tISSUE\tSTATUS\tBRANCH\tPR\tDURATION\tSTARTED")
		for _, r := range runs {
			dur := ""
			if r.DurationS != nil {
				dur = fmt.Sprintf("%.1fs", *r.DurationS)
			}
			pr := r.PRUrl
			if pr == "" {
				pr = "-"
			}
			repo := filepath.Base(r.Repo)
			fmt.Fprintf(w, "%d\t%s\t#%d\t%s\t%s\t%s\t%s\t%s\n",
				r.ID, repo, r.IssueNum, r.Status, r.Branch, pr, dur, r.StartedAt.Format("2006-01-02 15:04"))
		}
		return w.Flush()
	},
}

func init() {
	runsCmd.Flags().IntVar(&runsLimit, "limit", 20, "Maximum runs to show")
	rootCmd.AddCommand(runsCmd)
}
