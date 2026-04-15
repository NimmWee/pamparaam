package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/usecase"
)

type Handler struct {
	createPage        *usecase.CreatePage
	getPage           *usecase.GetPage
	archivePage       *usecase.ArchivePage
	autosaveDraft     *usecase.AutosaveDraft
	recoverDraft      *usecase.RecoverDraft
	publishPage       *usecase.PublishPage
	listVersions      *usecase.ListVersions
	restoreRevision   *usecase.RestoreRevision
	getEditorMetadata *usecase.GetEditorMetadata
	resumeEditorSync  *usecase.ResumeEditorSync
}

type createPageRequest struct {
	WorkspaceID     string          `json:"workspace_id"`
	Title           string          `json:"title"`
	Slug            string          `json:"slug"`
	InitialDocument domain.Document `json:"initial_document"`
}

func NewHandler(
	createPage *usecase.CreatePage,
	getPage *usecase.GetPage,
	archivePage *usecase.ArchivePage,
	autosaveDraft *usecase.AutosaveDraft,
	recoverDraft *usecase.RecoverDraft,
	publishPage *usecase.PublishPage,
	listVersions *usecase.ListVersions,
	restoreRevision *usecase.RestoreRevision,
	getEditorMetadata *usecase.GetEditorMetadata,
	resumeEditorSync *usecase.ResumeEditorSync,
) *Handler {
	return &Handler{
		createPage:        createPage,
		getPage:           getPage,
		archivePage:       archivePage,
		autosaveDraft:     autosaveDraft,
		recoverDraft:      recoverDraft,
		publishPage:       publishPage,
		listVersions:      listVersions,
		restoreRevision:   restoreRevision,
		getEditorMetadata: getEditorMetadata,
		resumeEditorSync:  resumeEditorSync,
	}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/pages", h.handleCreatePage)
	router.Get("/pages/{pageID}", h.handleGetPage)
	router.Post("/pages/{pageID}/archive", h.handleArchivePage)
	router.Patch("/pages/{pageID}/draft", h.handleAutosaveDraft)
	router.Get("/pages/{pageID}/draft/recover", h.handleRecoverDraft)
	router.Post("/pages/{pageID}/publish", h.handlePublishPage)
	router.Get("/pages/{pageID}/versions", h.handleListVersions)
	router.Post("/pages/{pageID}/versions/{revisionID}/restore", h.handleRestoreRevision)
	router.Get("/editor/metadata", h.handleGetEditorMetadata)
	router.Post("/editor/sync", h.handleResumeEditorSync)
}

func (h *Handler) handleCreatePage(w http.ResponseWriter, r *http.Request) {
	var request createPageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	page, err := h.createPage.Execute(r.Context(), usecase.CreatePageInput{
		WorkspaceID:     request.WorkspaceID,
		Title:           request.Title,
		Slug:            request.Slug,
		InitialDocument: request.InitialDocument,
		ActorUserID:     strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		AccessToken:     bearerToken(r.Header.Get("Authorization")),
		ActorRoles:      parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated:   isAuthenticated(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			writeError(w, r, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, usecase.ErrValidation):
			writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
		case errors.Is(err, usecase.ErrEmbedUnavailable):
			writeError(w, r, http.StatusBadGateway, "embed_unavailable", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	writeJSON(w, r, http.StatusCreated, page)
}

func (h *Handler) handleGetPage(w http.ResponseWriter, r *http.Request) {
	view := domain.RevisionView(r.URL.Query().Get("view"))
	if view == "" {
		view = domain.RevisionViewDraft
	}
	if view != domain.RevisionViewDraft && view != domain.RevisionViewPublished {
		writeError(w, r, http.StatusBadRequest, "validation_error", "view must be draft or published")
		return
	}

	page, err := h.getPage.Execute(r.Context(), usecase.GetPageInput{
		PageID:        chi.URLParam(r, "pageID"),
		View:          view,
		WorkspaceID:   strings.TrimSpace(r.Header.Get("X-Workspace-Id")),
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		AccessToken:   bearerToken(r.Header.Get("Authorization")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: isAuthenticated(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			writeError(w, r, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, usecase.ErrPageNotFound):
			writeError(w, r, http.StatusNotFound, "page_not_found", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	writeJSON(w, r, http.StatusOK, page)
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func parseRoles(header string) []string {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	return strings.Split(header, ",")
}

func isAuthenticated(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("X-Auth-Authenticated"), "true")
}

func writeJSON(w http.ResponseWriter, r *http.Request, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", transport.RequestIDFromContext(r.Context()))
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	writeJSON(w, r, statusCode, map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
		"request_id": transport.RequestIDFromContext(r.Context()),
	})
}
