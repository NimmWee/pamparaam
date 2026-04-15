package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/file-service/internal/usecase"
)

type Handler struct {
	startUpload    *usecase.StartUpload
	completeUpload *usecase.CompleteUpload
	getFile        *usecase.GetFile
	deleteFile     *usecase.DeleteFile
}

func NewHandler(startUpload *usecase.StartUpload, completeUpload *usecase.CompleteUpload, getFile *usecase.GetFile, deleteFile *usecase.DeleteFile) *Handler {
	return &Handler{startUpload: startUpload, completeUpload: completeUpload, getFile: getFile, deleteFile: deleteFile}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Post("/files/uploads", h.handleStartUpload)
	router.Post("/files/uploads/{uploadID}/complete", h.handleCompleteUpload)
	router.Get("/files/{fileID}", h.handleGetFile)
	router.Delete("/files/{fileID}", h.handleDeleteFile)
}

func (h *Handler) handleStartUpload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		WorkspaceID string `json:"workspace_id"`
		PageID      string `json:"page_id"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
		Checksum    string `json:"checksum"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	response, err := h.startUpload.Execute(r.Context(), usecase.StartUploadInput{
		WorkspaceID:   request.WorkspaceID,
		PageID:        request.PageID,
		Filename:      request.Filename,
		ContentType:   request.ContentType,
		SizeBytes:     request.SizeBytes,
		Checksum:      request.Checksum,
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: strings.EqualFold(r.Header.Get("X-Auth-Authenticated"), "true"),
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			status = http.StatusForbidden
			code = "forbidden"
		default:
			status = http.StatusBadRequest
			code = "validation_error"
		}
		writeError(w, r, status, code, err.Error())
		return
	}
	writeJSON(w, r, http.StatusCreated, response)
}

func (h *Handler) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	var request struct {
		PageID   string `json:"page_id"`
		Checksum string `json:"checksum"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	response, err := h.completeUpload.Execute(r.Context(), usecase.CompleteUploadInput{
		UploadID:      chi.URLParam(r, "uploadID"),
		PageID:        request.PageID,
		Checksum:      request.Checksum,
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: strings.EqualFold(r.Header.Get("X-Auth-Authenticated"), "true"),
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			status = http.StatusForbidden
			code = "forbidden"
		case errors.Is(err, domain.ErrNotFound):
			status = http.StatusNotFound
			code = "not_found"
		default:
			status = http.StatusBadRequest
			code = "validation_error"
		}
		writeError(w, r, status, code, err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, response)
}

func (h *Handler) handleGetFile(w http.ResponseWriter, r *http.Request) {
	response, err := h.getFile.Execute(r.Context(), usecase.GetFileInput{
		FileID:        chi.URLParam(r, "fileID"),
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: strings.EqualFold(r.Header.Get("X-Auth-Authenticated"), "true"),
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			status = http.StatusForbidden
			code = "forbidden"
		case errors.Is(err, domain.ErrNotFound):
			status = http.StatusNotFound
			code = "not_found"
		}
		writeError(w, r, status, code, err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, response)
}

func (h *Handler) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	response, err := h.deleteFile.Execute(r.Context(), usecase.DeleteFileInput{
		FileID:        chi.URLParam(r, "fileID"),
		ActorUserID:   strings.TrimSpace(r.Header.Get("X-Auth-User-Id")),
		ActorRoles:    parseRoles(r.Header.Get("X-Auth-Roles")),
		Authenticated: strings.EqualFold(r.Header.Get("X-Auth-Authenticated"), "true"),
	})
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal_error"
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			status = http.StatusForbidden
			code = "forbidden"
		case errors.Is(err, domain.ErrNotFound):
			status = http.StatusNotFound
			code = "not_found"
		}
		writeError(w, r, status, code, err.Error())
		return
	}
	writeJSON(w, r, http.StatusOK, response)
}

func parseRoles(header string) []string {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	return strings.Split(header, ",")
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
