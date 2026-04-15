package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mtc/wiki-editor-backend/pkg/authn"
)

type validatorStub struct {
	identity authn.Identity
	err      error
}

func (v validatorStub) ValidateAccessToken(context.Context, string) (authn.Identity, error) {
	return v.identity, v.err
}

func TestRequireAuthInjectsIdentity(t *testing.T) {
	t.Parallel()

	middleware := NewAuthMiddleware(validatorStub{
		identity: authn.Identity{UserID: "u-1", Email: "owner@example.com"},
	}, nil)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/pages?workspace_id=w-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()

	middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := authn.IdentityFromContext(r.Context())
		if !ok {
			t.Fatal("expected identity in context")
		}
		if identity.UserID != "u-1" || identity.WorkspaceID != "w-1" {
			t.Fatalf("unexpected identity: %#v", identity)
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
}

func TestRequireAuthRejectsInvalidToken(t *testing.T) {
	t.Parallel()

	middleware := NewAuthMiddleware(validatorStub{err: errors.New("bad token")}, nil)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/pages", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()

	middleware.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}
