package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
)

type authorizationRepoStub struct {
	memberships []domain.Membership
	grants      []domain.PageGrant
}

func (s authorizationRepoStub) Ping(context.Context) error { return nil }

func (s authorizationRepoStub) GetUserByEmail(context.Context, string) (domain.User, error) {
	return domain.User{}, nil
}

func (s authorizationRepoStub) GetUserByID(context.Context, string) (domain.User, error) {
	return domain.User{}, nil
}

func (s authorizationRepoStub) ListMemberships(context.Context, string) ([]domain.Membership, error) {
	return s.memberships, nil
}

func (s authorizationRepoStub) ListPageGrants(context.Context, string, string, string) ([]domain.PageGrant, error) {
	return s.grants, nil
}

func (s authorizationRepoStub) CreateRefreshSession(context.Context, domain.RefreshSession) error {
	return nil
}

func (s authorizationRepoStub) GetRefreshSession(context.Context, string) (domain.RefreshSession, error) {
	return domain.RefreshSession{}, nil
}

func (s authorizationRepoStub) RevokeRefreshSession(context.Context, string, string, time.Time) error {
	return nil
}

func TestAuthorizerWorkspaceMembership(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(authorizationRepoStub{
		memberships: []domain.Membership{{WorkspaceID: "w-1", UserID: "u-1", Role: domain.RoleAdmin}},
	})

	decision, err := authorizer.Execute(context.Background(), AuthorizeInput{
		SubjectUserID: "u-1",
		WorkspaceID:   "w-1",
		Action:        domain.ActionPageRestore,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected admin to be authorized")
	}
}

func TestAuthorizerPageGrantFallback(t *testing.T) {
	t.Parallel()

	authorizer := NewAuthorizer(authorizationRepoStub{
		memberships: []domain.Membership{{WorkspaceID: "w-1", UserID: "u-1", Role: domain.RoleViewer}},
		grants:      []domain.PageGrant{{WorkspaceID: "w-1", SubjectUserID: "u-1", PageID: "p-1", Permission: domain.PagePermissionEdit}},
	})

	decision, err := authorizer.Execute(context.Background(), AuthorizeInput{
		SubjectUserID: "u-1",
		WorkspaceID:   "w-1",
		PageID:        "p-1",
		Action:        domain.ActionPageEdit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected page grant to allow edit")
	}
}
