package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/usecase"
)

type resumeEditorSyncRequest struct {
	PageID              string   `json:"page_id"`
	LastKnownRevisionNo int64    `json:"last_known_revision_no"`
	PendingPatchIDs     []string `json:"pending_patch_ids"`
	ResumeToken         string   `json:"resume_token"`
}

func (h *Handler) handleResumeEditorSync(w http.ResponseWriter, r *http.Request) {
	var request resumeEditorSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}

	response, err := h.resumeEditorSync.Execute(r.Context(), usecase.ResumeEditorSyncInput{
		PageID:              request.PageID,
		LastKnownRevisionNo: request.LastKnownRevisionNo,
		PendingPatchIDs:     request.PendingPatchIDs,
		ResumeToken:         request.ResumeToken,
		WorkspaceID:         strings.TrimSpace(r.Header.Get("X-Workspace-Id")),
		ActorUserID:         strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		AccessToken:         bearerToken(r.Header.Get("Authorization")),
		ActorRoles:          parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated:       isAuthenticated(r),
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
