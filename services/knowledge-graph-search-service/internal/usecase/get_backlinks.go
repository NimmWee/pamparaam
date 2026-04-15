package usecase

import (
	"context"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/ports"
)

type GetBacklinks struct {
	store  ports.Store
	filter *ResultFilter
}

func NewGetBacklinks(store ports.Store, filter *ResultFilter) *GetBacklinks {
	return &GetBacklinks{store: store, filter: filter}
}

type GetBacklinksInput struct {
	WorkspaceID   string
	PageID        string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

func (u *GetBacklinks) Execute(ctx context.Context, input GetBacklinksInput) (domain.BacklinksPayload, error) {
	subject := AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}
	if err := u.filter.Require(ctx, authz.ActionPageView, subject); err != nil {
		return domain.BacklinksPayload{}, err
	}
	payload, err := u.store.GetBacklinks(ctx, input.WorkspaceID, input.PageID)
	if err != nil {
		return domain.BacklinksPayload{}, err
	}
	return u.filter.FilterBacklinks(ctx, subject, payload)
}
