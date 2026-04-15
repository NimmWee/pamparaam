package usecase

import (
	"context"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/ports"
)

type SearchPages struct {
	store  ports.Store
	filter *ResultFilter
}

func NewSearchPages(store ports.Store, filter *ResultFilter) *SearchPages {
	return &SearchPages{store: store, filter: filter}
}

type SearchPagesInput struct {
	WorkspaceID   string
	Query         string
	Sort          string
	ActorUserID   string
	ActorRoles    []string
	Authenticated bool
}

func (u *SearchPages) Execute(ctx context.Context, input SearchPagesInput) (domain.SearchPayload, error) {
	subject := AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}
	if err := u.filter.Require(ctx, authz.ActionSearchQuery, subject); err != nil {
		return domain.SearchPayload{}, err
	}
	results, err := u.store.Search(ctx, input.WorkspaceID, input.Query, input.Sort)
	if err != nil {
		return domain.SearchPayload{}, err
	}
	results, err = u.filter.FilterSearchResults(ctx, subject, results)
	if err != nil {
		return domain.SearchPayload{}, err
	}
	return domain.SearchPayload{Results: results}, nil
}
