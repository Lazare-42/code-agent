package cmd

import (
	"context"
	"os/signal"
	"syscall"

	intmcp "github.com/lazrossi/code-agent/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run MCP server over stdio",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		st, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer st.Close()

		return intmcp.Serve(ctx, cfg, st)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
