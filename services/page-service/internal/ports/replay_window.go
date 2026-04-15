package ports

import (
	"context"
	"time"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

type ReplayWindowStore interface {
	Append(ctx context.Context, entry domain.ReplayWindowEntry, ttl time.Duration) error
	ListSinceRevision(ctx context.Context, pageID string, fromRevisionNo int64) ([]domain.ReplayWindowEntry, error)
}
