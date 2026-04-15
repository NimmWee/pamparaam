package rest

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/usecase"
)

func (h *Handler) handleGetEditorMetadata(w http.ResponseWriter, r *http.Request) {
	response, err := h.getEditorMetadata.Execute(r.Context(), usecase.GetEditorMetadataInput{
		WorkspaceID:   firstNonEmpty(strings.TrimSpace(r.URL.Query().Get("workspace_id")), strings.TrimSpace(r.Header.Get("X-Workspace-Id"))),
		PageID:        strings.TrimSpace(r.URL.Query().Get("page_id")),
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: isAuthenticated(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			writeError(w, r, http.StatusForbidden, "forbidden", err.Error())
		default:
			writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	writeJSON(w, r, http.StatusOK, response)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
