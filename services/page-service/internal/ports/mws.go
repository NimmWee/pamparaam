package ports

import (
	"context"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

type ResolveEmbedInput struct {
	PageID        string
	WorkspaceID   string
	ActorUserID   string
	AccessToken   string
	AllowDegraded bool
	Reference     domain.EmbeddedTableReference
	StoredTitle   string
}

type EmbedResolver interface {
	Resolve(ctx context.Context, input ResolveEmbedInput) (domain.TableEmbedDescriptor, error)
}
