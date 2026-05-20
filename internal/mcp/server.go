package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/lazrossi/code-agent/internal/config"
	"github.com/lazrossi/code-agent/internal/rest"
	"github.com/lazrossi/code-agent/internal/storage"
	"github.com/lazrossi/code-agent/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ToolListRepos      = "list_repos"
	ToolListIssues     = "list_issues"
	ToolDispatchAgent  = "dispatch_agent"
	ToolGetAgentStatus = "get_agent_status"
	ToolListRuns       = "list_runs"
)

func BuildMCPServer(cfg *config.Config, st *store.Store, s3 *storage.Client) *server.MCPServer {
	s := server.NewMCPServer("code-agent", "1.0.0",
		server.WithToolCapabilities(false),
	)

	h := &handlers{
		cfg:   cfg,
		store: st,
		s3:    s3,
	}

	// Read-only tools
	s.AddTool(listReposTool(), h.listRepos)
	s.AddTool(listIssuesTool(), h.listIssues)
	s.AddTool(getAgentStatusTool(), h.getAgentStatus)
	s.AddTool(listRunsTool(), h.listRuns)

	// Write tools
	s.AddTool(dispatchAgentTool(), h.dispatchAgent)

	return s
}

// Serve runs the MCP server over stdio.
func Serve(ctx context.Context, cfg *config.Config, st *store.Store) error {
	s := BuildMCPServer(cfg, st, nil)
	return server.ServeStdio(s)
}

// ServeStreamableHTTP runs the MCP server over Streamable HTTP.
func ServeStreamableHTTP(ctx context.Context, addr, apiKey string, cfg *config.Config, st *store.Store) error {
	// Init S3 client (nil-safe if not configured or unavailable)
	var s3 *storage.Client
	if cfg.S3.Endpoint != "" {
		var err error
		s3, err = storage.New(cfg.S3)
		if err != nil {
			log.Printf("S3 storage unavailable: %v (traces and log archival disabled)", err)
		}
	}

	s := BuildMCPServer(cfg, st, s3)
	httpServer := server.NewStreamableHTTPServer(s)

	mux := http.NewServeMux()
	handler := http.Handler(httpServer)

	if apiKey != "" {
		handler = requireBearer(apiKey, handler)
	}

	restHandler := rest.NewHandler(cfg, st, s3)
	restHandler.RegisterRoutes(mux)

	mux.Handle("/mcp", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	log.Printf("code-agent MCP listening on %s/mcp", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func requireBearer(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Tool Definitions ---

func listReposTool() mcp.Tool {
	return mcp.NewTool(ToolListRepos,
		mcp.WithDescription("List configured repositories with their status and paths."),
		mcp.WithReadOnlyHintAnnotation(true),
	)
}

func listIssuesTool() mcp.Tool {
	return mcp.NewTool(ToolListIssues,
		mcp.WithDescription("List open GitHub issues across configured repos. Shows eligibility status for agent dispatch."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("repo",
			mcp.Description("Filter by repo name (optional)"),
		),
	)
}

func dispatchAgentTool() mcp.Tool {
	return mcp.NewTool(ToolDispatchAgent,
		mcp.WithDescription("Dispatch a Claude Code agent to resolve a specific GitHub issue. Creates a branch, runs the agent, and opens a PR if successful."),
		mcp.WithString("repo",
			mcp.Required(),
			mcp.Description("Repository name as configured"),
		),
		mcp.WithNumber("issue_number",
			mcp.Required(),
			mcp.Description("GitHub issue number to resolve"),
		),
	)
}

func getAgentStatusTool() mcp.Tool {
	return mcp.NewTool(ToolGetAgentStatus,
		mcp.WithDescription("Get the status of an agent run by its ID."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithNumber("run_id",
			mcp.Required(),
			mcp.Description("Agent run ID"),
		),
	)
}

func listRunsTool() mcp.Tool {
	return mcp.NewTool(ToolListRuns,
		mcp.WithDescription("List recent agent run history with status and PR links."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithNumber("limit",
			mcp.Description("Maximum results to return (default 20)"),
		),
	)
}
