package security

import (
	"context"
	"testing"
	"time"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
)

func TestTokenServiceIssuesAndParsesTokens(t *testing.T) {
	t.Parallel()

	service, err := NewTokenService(TokenServiceConfig{
		Issuer:     "wiki-auth",
		Audience:   "wiki-api",
		AccessTTL:  5 * time.Minute,
		RefreshTTL: time.Hour,
		KeyID:      "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pair, sessionID, _, err := service.IssueTokens(context.Background(), domain.User{
		ID:          "user-1",
		Email:       "owner@example.com",
		DisplayName: "Owner",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID == "" {
		t.Fatal("expected refresh session id")
	}

	accessClaims, err := service.ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("unexpected access parse error: %v", err)
	}
	if accessClaims.Subject != "user-1" || accessClaims.Email != "owner@example.com" {
		t.Fatalf("unexpected access claims: %#v", accessClaims)
	}

	refreshClaims, err := service.ParseRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("unexpected refresh parse error: %v", err)
	}
	if refreshClaims.SessionID != sessionID {
		t.Fatalf("expected session id %q, got %q", sessionID, refreshClaims.SessionID)
	}
}
