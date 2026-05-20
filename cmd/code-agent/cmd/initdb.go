package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var initDBCmd = &cobra.Command{
	Use:   "init-db",
	Short: "Initialize the SQLite database schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := openStore(cmd.Context())
		if err != nil {
			return err
		}
		defer st.Close()

		log.Printf("Database initialized at %s", cfg.DBPath())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initDBCmd)
}
