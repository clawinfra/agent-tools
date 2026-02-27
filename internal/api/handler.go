// Package api provides the HTTP API handler for agent-tools.
package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/clawinfra/agent-tools/internal/registry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

// Handler is the HTTP API handler.
type Handler struct {
	reg *registry.Registry
	log *zap.Logger
	mux *chi.Mux
}

// NewHandler creates a new Handler and registers routes.
func NewHandler(reg *registry.Registry, log *zap.Logger) http.Handler {
	h := &Handler{reg: reg, log: log, mux: chi.NewRouter()}
	h.routes()
	return h
}

func (h *Handler) routes() {
	r := h.mux

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(zapMiddleware(h.log))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	r.Get("/healthz", h.healthz)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/tools", func(r chi.Router) {
			r.Get("/", h.listTools)
			r.Post("/", h.registerTool)
			r.Get("/search", h.searchTools)
			r.Get("/{id}", h.getTool)
			r.Delete("/{id}", h.deactivateTool)
		})

		r.Post("/invoke", h.invokeTool)

		r.Route("/providers", func(r chi.Router) {
			r.Get("/", h.listProviders)
			r.Post("/", h.registerProvider)
			r.Get("/{id}", h.getProvider)
		})
	})
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// healthz returns service health status.
func (h *Handler) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	})
}

// listTools handles GET /v1/tools.
func (h *Handler) listTools(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	result, err := h.reg.ListTools(r.Context(), page, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// registerTool handles POST /v1/tools.
func (h *Handler) registerTool(w http.ResponseWriter, r *http.Request) {
	var req registry.RegisterToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid JSON")
		return
	}

	// TODO: extract providerID from auth context; use anonymous for now
	req.ProviderID = providerIDFromRequest(r)

	tool, err := h.reg.RegisterTool(r.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, registry.ErrDuplicate):
			writeError(w, http.StatusConflict, "DUPLICATE_TOOL", err.Error())
		default:
			h.log.Error("register tool", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, tool)
}

// getTool handles GET /v1/tools/{id}.
func (h *Handler) getTool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tool, err := h.reg.GetTool(r.Context(), id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "TOOL_NOT_FOUND", "tool not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

// searchTools handles GET /v1/tools/search.
func (h *Handler) searchTools(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	maxPrice, _ := strconv.ParseFloat(q.Get("max_price_claw"), 64)

	result, err := h.reg.SearchTools(r.Context(), &registry.SearchQuery{
		Query:    q.Get("q"),
		Tag:      q.Get("tag"),
		Provider: q.Get("provider"),
		MaxPrice: maxPrice,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// deactivateTool handles DELETE /v1/tools/{id}.
func (h *Handler) deactivateTool(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	providerID := providerIDFromRequest(r)

	if err := h.reg.DeactivateTool(r.Context(), id, providerID); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "TOOL_NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listProviders handles GET /v1/providers.
func (h *Handler) listProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.reg.ListProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// registerProvider handles POST /v1/providers.
func (h *Handler) registerProvider(w http.ResponseWriter, r *http.Request) {
	var req registry.Provider
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid JSON")
		return
	}

	provider, err := h.reg.RegisterProvider(r.Context(), &req)
	if err != nil {
		h.log.Error("register provider", zap.Error(err))
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, provider)
}

// getProvider handles GET /v1/providers/{id}.
func (h *Handler) getProvider(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	provider, err := h.reg.GetProvider(r.Context(), id)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			writeError(w, http.StatusNotFound, "PROVIDER_NOT_FOUND", "provider not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, provider)
}

// invokeTool handles POST /v1/invoke.
// v0.1: direct invocation stub — returns 501 until invocation router is implemented.
func (h *Handler) invokeTool(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotImplemented, "NOT_IMPLEMENTED",
		"tool invocation is coming in v0.2 — see ARCHITECTURE.md#roadmap")
}

// providerIDFromRequest extracts the provider DID from the request.
// In v0.1, uses the Authorization header as a simple DID.
// TODO: replace with proper DID-signed JWT verification.
func providerIDFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "did:claw:agent:anonymous"
	}
	// Strip "Bearer " prefix
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	return auth
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var e apiError
	e.Error.Code = code
	e.Error.Message = message
	writeJSON(w, status, e)
}

func zapMiddleware(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.Info("http",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
			)
		})
	}
}
