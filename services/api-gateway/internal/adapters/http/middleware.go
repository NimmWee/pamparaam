package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mtc/wiki-editor-backend/pkg/authn"
	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/pkg/runtimeauthz"
	"github.com/mtc/wiki-editor-backend/pkg/transport"
)

type AccessTokenValidator interface {
	ValidateAccessToken(ctx context.Context, rawToken string) (authn.Identity, error)
}

type AuthMiddleware struct {
	validator AccessTokenValidator
	checker   *runtimeauthz.Client
}

func NewAuthMiddleware(validator AccessTokenValidator, checker *runtimeauthz.Client) *AuthMiddleware {
	return &AuthMiddleware{validator: validator, checker: checker}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeGatewayError(w, r, http.StatusUnauthorized, "missing_token", "authorization header is required")
			return
		}

		identity, err := m.validator.ValidateAccessToken(r.Context(), token)
		if err != nil {
			writeGatewayError(w, r, http.StatusUnauthorized, "invalid_token", err.Error())
			return
		}

		identity.WorkspaceID = resolveWorkspaceID(r)
		next.ServeHTTP(w, r.WithContext(authn.WithIdentity(r.Context(), identity)))
	})
}

func (m *AuthMiddleware) RequireAction(action authz.Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := authn.IdentityFromContext(r.Context())
			if !ok {
				writeGatewayError(w, r, http.StatusForbidden, "forbidden", "permission denied")
				return
			}

			if m.checker != nil && m.checker.Enabled() {
				decision, err := m.checker.Authorize(r.Context(), runtimeauthz.CheckInput{
					ActorUserID: identity.UserID,
					WorkspaceID: identity.WorkspaceID,
					PageID:      resolvePageID(r),
					Action:      action,
				})
				if err != nil {
					writeGatewayError(w, r, http.StatusServiceUnavailable, "authorization_unavailable", "authorization dependency unavailable")
					return
				}
				if !decision.Allowed {
					writeGatewayError(w, r, http.StatusForbidden, "forbidden", "permission denied")
					return
				}
			} else if !authz.Allowed(identity.Roles, action) {
				writeGatewayError(w, r, http.StatusForbidden, "forbidden", "permission denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func applyIdentityHeaders(r *http.Request) {
	identity, ok := authn.IdentityFromContext(r.Context())
	if !ok {
		return
	}

	r.Header.Set("X-Auth-User-Id", identity.UserID)
	r.Header.Set("X-Auth-Email", identity.Email)
	r.Header.Set("X-Auth-Display-Name", identity.DisplayName)
	r.Header.Set("X-Auth-Authenticated", "true")
	if identity.WorkspaceID != "" {
		r.Header.Set("X-Workspace-Id", identity.WorkspaceID)
	}
	r.Header.Set("X-Auth-Roles", strings.Join(identity.Roles, ","))
}

func resolveWorkspaceID(r *http.Request) string {
	if workspaceID := strings.TrimSpace(r.Header.Get("X-Workspace-Id")); workspaceID != "" {
		return workspaceID
	}
	return strings.TrimSpace(r.URL.Query().Get("workspace_id"))
}

func resolvePageID(r *http.Request) string {
	if pageID := strings.TrimSpace(chi.URLParam(r, "pageID")); pageID != "" {
		return pageID
	}
	return strings.TrimSpace(r.URL.Query().Get("page_id"))
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

func writeGatewayError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-Id", transport.RequestIDFromContext(r.Context()))
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
		"request_id": transport.RequestIDFromContext(r.Context()),
	})
}
