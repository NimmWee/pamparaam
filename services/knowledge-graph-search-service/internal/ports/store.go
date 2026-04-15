package ports

import (
	"context"

	"github.com/mtc/wiki-editor-backend/services/knowledge-graph-search-service/internal/domain"
)

type Store interface {
	UpsertPage(ctx context.Context, projection domain.PageProjection, targetPageIDs []string) error
	ReplacePageLinks(ctx context.Context, workspaceID, sourcePageID string, targetPageIDs []string) error
	Search(ctx context.Context, workspaceID, query, sort string) ([]domain.SearchResult, error)
	GetBacklinks(ctx context.Context, workspaceID, pageID string) (domain.BacklinksPayload, error)
	Ping(ctx context.Context) error
}
