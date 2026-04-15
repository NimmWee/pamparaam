package usecase

import (
	"context"

	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/auth-service/internal/ports"
)

type AuthorizeInput struct {
	SubjectUserID string
	WorkspaceID   string
	PageID        string
	Action        domain.Action
}

type Authorizer struct {
	repository ports.Repository
}

func NewAuthorizer(repository ports.Repository) *Authorizer {
	return &Authorizer{repository: repository}
}

func (a *Authorizer) Execute(ctx context.Context, input AuthorizeInput) (domain.AuthorizationDecision, error) {
	memberships, err := a.repository.ListMemberships(ctx, input.SubjectUserID)
	if err != nil {
		return domain.AuthorizationDecision{}, err
	}

	role := domain.RoleUnknown
	for _, membership := range memberships {
		if membership.WorkspaceID == input.WorkspaceID {
			role = membership.Role
			break
		}
	}

	var grants []domain.PageGrant
	if input.PageID != "" {
		grants, err = a.repository.ListPageGrants(ctx, input.SubjectUserID, input.WorkspaceID, input.PageID)
		if err != nil {
			return domain.AuthorizationDecision{}, err
		}
	}

	return domain.EvaluateAuthorization(role, grants, input.Action), nil
}
