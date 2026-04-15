package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/usecase"
)

type Handler struct {
	searchPages  *usecase.SearchPages
	getBacklinks *usecase.GetBacklinks
}

func NewHandler(searchPages *usecase.SearchPages, getBacklinks *usecase.GetBacklinks) *Handler {
	return &Handler{searchPages: searchPages, getBacklinks: getBacklinks}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Get("/search", h.handleSearch)
	router.Get("/pages/{pageID}/backlinks", h.handleBacklinks)
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	response, err := h.searchPages.Execute(r.Context(), usecase.SearchPagesInput{
		WorkspaceID:   firstNonEmpty(strings.TrimSpace(r.URL.Query().Get("workspace_id")), strings.TrimSpace(r.Header.Get("X-Workspace-Id"))),
		Query:         strings.TrimSpace(r.URL.Query().Get("q")),
		Sort:          strings.TrimSpace(r.URL.Query().Get("sort")),
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: strings.EqualFold(r.Header.Get("X-Auth-Authenticated"), "true"),
	})
	if err != nil {
		writeUsecaseError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, response)
}

func (h *Handler) handleBacklinks(w http.ResponseWriter, r *http.Request) {
	response, err := h.getBacklinks.Execute(r.Context(), usecase.GetBacklinksInput{
		WorkspaceID:   strings.TrimSpace(r.Header.Get("X-Workspace-Id")),
		PageID:        chi.URLParam(r, "pageID"),
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: strings.EqualFold(r.Header.Get("X-Auth-Authenticated"), "true"),
	})
	if err != nil {
		writeUsecaseError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, response)
}

func writeUsecaseError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	switch {
	case errors.Is(err, usecase.ErrForbidden):
		status = http.StatusForbidden
		code = "forbidden"
	}
	writeError(w, r, status, code, err.Error())
}

func parseRoles(header string) []string {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	return strings.Split(header, ",")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, r *http.Request, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", transport.RequestIDFromContext(r.Context()))
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	writeJSON(w, r, statusCode, map[string]any{
		"error":      map[string]string{"code": code, "message": message},
		"request_id": transport.RequestIDFromContext(r.Context()),
	})
}
