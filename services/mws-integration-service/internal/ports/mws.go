package ports

import (
	"context"

	"github.com/mtc/wiki-editor-backend/services/mws-integration-service/internal/domain"
)

type MWSClient interface {
	ValidateAccess(ctx context.Context, accessToken string, tableID string) error
	FetchSchema(ctx context.Context, accessToken string, tableID string) (map[string]any, error)
	FetchPreview(ctx context.Context, accessToken string, tableID string) ([]map[string]any, error)
}

type Cache interface {
	Get(ctx context.Context, tableID string) (domain.CachedTablePreview, bool, error)
	Put(ctx context.Context, value domain.CachedTablePreview) error
}
