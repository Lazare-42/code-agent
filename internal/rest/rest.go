package rest

import (
	"net/http"

	"github.com/lazrossi/code-agent/internal/config"
	"github.com/lazrossi/code-agent/internal/storage"
	"github.com/lazrossi/code-agent/internal/store"
)

// Handler holds dependencies for REST endpoints.
type Handler struct {
	cfg   *config.Config
	store *store.Store
	s3    *storage.Client
}

// NewHandler creates a new REST handler. s3 may be nil.
func NewHandler(cfg *config.Config, st *store.Store, s3 *storage.Client) *Handler {
	return &Handler{cfg: cfg, store: st, s3: s3}
}

// RegisterRoutes registers all REST API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	api := http.NewServeMux()

	api.HandleFunc("GET /stats", h.getStats)
	api.HandleFunc("GET /repos", h.listRepos)
	api.HandleFunc("GET /issues", h.listIssues)
	api.HandleFunc("GET /issues/{repo}/{number}", h.getIssue)
	api.HandleFunc("POST /issues/draft", h.draftIssue)
	api.HandleFunc("POST /issues", h.createIssue)
	api.HandleFunc("GET /runs", h.listRuns)
	api.HandleFunc("GET /runs/{id}/output", h.getRunOutput)
	api.HandleFunc("GET /runs/{id}/trace", h.getRunTrace)
	api.HandleFunc("GET /runs/{id}", h.getRun)
	api.HandleFunc("POST /dispatch", h.dispatchAgent)
	api.HandleFunc("POST /rebase", h.triggerRebase)
	api.HandleFunc("GET /prs", h.listPRs)
	api.HandleFunc("GET /prs/{repo}/{number}", h.getPRDetail)
	api.HandleFunc("POST /prs/{repo}/{number}/review", h.reviewPR)
	api.HandleFunc("POST /prs/{repo}/{number}/resolve", h.resolvePRComments)
	api.HandleFunc("POST /prs/{repo}/{number}/agent", h.dispatchAgentOnPR)
	api.HandleFunc("GET /tasks", h.listTasks)
	api.HandleFunc("POST /tasks", h.createTask)

	mux.Handle("/api/", corsMiddleware(http.StripPrefix("/api", api)))
}
