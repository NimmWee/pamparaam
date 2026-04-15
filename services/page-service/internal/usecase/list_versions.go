package usecase

import (
	"context"
	"errors"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/memory"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type ListVersions struct {
	store      ports.Store
	authorizer *PageActionAuthorizer
}

type ListVersionsInput struct {
	PageID        string
	WorkspaceID   string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

func NewListVersions(store ports.Store, authorizer *PageActionAuthorizer) *ListVersions {
	return &ListVersions{store: store, authorizer: authorizer}
}

func (u *ListVersions) Execute(ctx context.Context, input ListVersionsInput) (domain.PageVersionListPayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionPageView, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.PageVersionListPayload{}, err
	}

	if _, err := u.store.GetPage(ctx, input.PageID); errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.PageVersionListPayload{}, ErrPageNotFound
	} else if err != nil {
		return domain.PageVersionListPayload{}, err
	}

	revisions, err := u.store.ListRevisions(ctx, input.PageID)
	if err != nil {
		return domain.PageVersionListPayload{}, err
	}
	return domain.BuildVersionListPayload(revisions), nil
}
