package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/usecase"
)

type archivePageRequest struct {
	BaseRevisionNo int64 `json:"base_revision_no"`
}

func (h *Handler) handleArchivePage(w http.ResponseWriter, r *http.Request) {
	var request archivePageRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	response, err := h.archivePage.Execute(r.Context(), usecase.ArchivePageInput{
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
		case errors.Is(err, usecase.ErrPageNotFound):
			writeError(w, r, http.StatusNotFound, "page_not_found", err.Error())
		case errors.Is(err, usecase.ErrPageArchived), errors.Is(err, usecase.ErrValidation):
			writeError(w, r, http.StatusConflict, "page_state_conflict", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	writeJSON(w, r, http.StatusOK, response)
}
