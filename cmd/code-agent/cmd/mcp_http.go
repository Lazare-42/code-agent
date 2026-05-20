package cmd

import (
	"context"
	"os/signal"
	"syscall"

	intmcp "github.com/lazrossi/code-agent/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	httpAddr   string
	httpAPIKey string
)

var mcpHTTPCmd = &cobra.Command{
	Use:   "mcp-http",
	Short: "Run MCP server over Streamable HTTP",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		st, err := openStore(ctx)
		if err != nil {
			return err
		}
		defer st.Close()

		addr := httpAddr
		if addr == "" {
			addr = cfg.HTTP.Addr
		}
		if addr == "" {
			addr = "127.0.0.1:8091"
		}

		apiKey := httpAPIKey
		if apiKey == "" {
			apiKey = cfg.HTTP.APIKey
		}

		return intmcp.ServeStreamableHTTP(ctx, addr, apiKey, cfg, st)
	},
}

func init() {
	mcpHTTPCmd.Flags().StringVar(&httpAddr, "addr", "", "Listen address (default from config or 127.0.0.1:8091)")
	mcpHTTPCmd.Flags().StringVar(&httpAPIKey, "api-key", "", "API key for Bearer auth (optional)")
	rootCmd.AddCommand(mcpHTTPCmd)
}
