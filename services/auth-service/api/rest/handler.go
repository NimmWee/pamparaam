package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/ports"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/usecase"
)

type Handler struct {
	authenticator *usecase.Authenticator
	currentUser   *usecase.CurrentUserReader
	tokenService  ports.TokenService
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func NewHandler(authenticator *usecase.Authenticator, currentUser *usecase.CurrentUserReader, tokenService ports.TokenService) *Handler {
	return &Handler{
		authenticator: authenticator,
		currentUser:   currentUser,
		tokenService:  tokenService,
	}
}

func (h *Handler) RegisterRoutes(router chi.Router) {
	router.Route("/auth", func(r chi.Router) {
		r.Post("/login", h.handleLogin)
		r.Post("/refresh", h.handleRefresh)
		r.Get("/me", h.handleCurrentUser)
	})
	router.Get("/.well-known/jwks.json", h.handleJWKS)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	if strings.TrimSpace(request.Email) == "" || request.Password == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "email and password are required")
		return
	}

	result, err := h.authenticator.Login(r.Context(), usecase.LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCredentials) {
			writeError(w, r, http.StatusUnauthorized, "invalid_credentials", err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"access_token":       result.Tokens.AccessToken,
		"refresh_token":      result.Tokens.RefreshToken,
		"token_type":         result.Tokens.TokenType,
		"expires_in_seconds": result.Tokens.ExpiresInSeconds,
		"user":               userPayload(result.User),
	})
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var request RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	if strings.TrimSpace(request.RefreshToken) == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "refresh_token is required")
		return
	}

	result, err := h.authenticator.Refresh(r.Context(), request.RefreshToken)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidRefreshToken) {
			writeError(w, r, http.StatusUnauthorized, "invalid_refresh_token", err.Error())
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"access_token":       result.Tokens.AccessToken,
		"refresh_token":      result.Tokens.RefreshToken,
		"token_type":         result.Tokens.TokenType,
		"expires_in_seconds": result.Tokens.ExpiresInSeconds,
		"user":               userPayload(result.User),
	})
}

func (h *Handler) handleCurrentUser(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, r, http.StatusUnauthorized, "missing_token", "authorization header is required")
		return
	}

	claims, err := h.tokenService.ParseAccessToken(token)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "invalid_token", err.Error())
		return
	}

	user, memberships, err := h.currentUser.Execute(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	responseMemberships := make([]map[string]any, 0, len(memberships))
	for _, membership := range memberships {
		responseMemberships = append(responseMemberships, map[string]any{
			"workspace_id": membership.WorkspaceID,
			"role":         string(membership.Role),
		})
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"user":        userPayload(user),
		"memberships": responseMemberships,
	})
}

func (h *Handler) handleJWKS(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, h.tokenService.JWKS())
}

func userPayload(user domain.User) map[string]any {
	return map[string]any{
		"id":           user.ID,
		"email":        user.Email,
		"display_name": user.DisplayName,
	}
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

func writeJSON(w http.ResponseWriter, r *http.Request, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", transport.RequestIDFromContext(r.Context()))
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	writeJSON(w, r, statusCode, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
		"request_id": transport.RequestIDFromContext(r.Context()),
	})
}
