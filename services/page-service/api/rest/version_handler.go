package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/usecase"
)

type publishPageRequest struct {
	BaseRevisionNo int64 `json:"base_revision_no"`
}

func (h *Handler) handlePublishPage(w http.ResponseWriter, r *http.Request) {
	var request publishPageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	response, err := h.publishPage.Execute(r.Context(), usecase.PublishPageInput{
		PageID:         chi.URLParam(r, "pageID"),
		BaseRevisionNo: request.BaseRevisionNo,
		WorkspaceID:    strings.TrimSpace(r.Header.Get("X-Workspace-Id")),
		ActorUserID:    strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:     parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated:  isAuthenticated(r),
	})
	if err != nil {
		var rebase *usecase.RebaseRequiredError
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			writeError(w, r, http.StatusForbidden, "forbidden", err.Error())
		case errors.As(err, &rebase):
			writeJSON(w, r, http.StatusConflict, rebase.Payload)
		case errors.Is(err, usecase.ErrPageArchived):
			writeError(w, r, http.StatusConflict, "page_state_conflict", err.Error())
		case errors.Is(err, usecase.ErrPageNotFound):
			writeError(w, r, http.StatusNotFound, "page_not_found", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, r, http.StatusOK, response)
}

func (h *Handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	response, err := h.listVersions.Execute(r.Context(), usecase.ListVersionsInput{
		PageID:        chi.URLParam(r, "pageID"),
		WorkspaceID:   strings.TrimSpace(r.Header.Get("X-Workspace-Id")),
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: isAuthenticated(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			writeError(w, r, http.StatusForbidden, "forbidden", err.Error())
		case errors.Is(err, usecase.ErrPageArchived):
			writeError(w, r, http.StatusConflict, "page_state_conflict", err.Error())
		case errors.Is(err, usecase.ErrPageNotFound):
			writeError(w, r, http.StatusNotFound, "page_not_found", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, r, http.StatusOK, response)
}

func (h *Handler) handleRestoreRevision(w http.ResponseWriter, r *http.Request) {
	response, err := h.restoreRevision.Execute(r.Context(), usecase.RestoreRevisionInput{
		PageID:        chi.URLParam(r, "pageID"),
		RevisionID:    chi.URLParam(r, "revisionID"),
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
	writeJSON(w, r, http.StatusOK, response)
}
