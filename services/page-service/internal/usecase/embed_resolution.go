package usecase

import (
	"context"
	"fmt"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type actorContext struct {
	WorkspaceID string
	ActorUserID string
	AccessToken string
}

func resolveDocumentEmbeds(ctx context.Context, resolver ports.EmbedResolver, actor actorContext, pageID string, document domain.Document, refs []domain.EmbeddedTableReference, allowDegraded bool) (map[string]domain.TableEmbedDescriptor, error) {
	descriptors := make(map[string]domain.TableEmbedDescriptor, len(refs))
	for _, ref := range refs {
		if resolver == nil {
			if allowDegraded {
				descriptors[ref.BlockID] = domain.TableEmbedDescriptor{
					MWSTableID:    ref.MWSTableID,
					Title:         document.EmbedTitleByBlockID(ref.BlockID),
					DisplayConfig: ref.DisplayConfig,
					PreviewState:  domain.PreviewStateDegraded,
				}
				continue
			}
			return nil, fmt.Errorf("%w: resolver unavailable", ErrEmbedUnavailable)
		}

		descriptor, err := resolver.Resolve(ctx, ports.ResolveEmbedInput{
			PageID:        pageID,
			WorkspaceID:   actor.WorkspaceID,
			ActorUserID:   actor.ActorUserID,
			AccessToken:   actor.AccessToken,
			AllowDegraded: allowDegraded,
			Reference:     ref,
			StoredTitle:   document.EmbedTitleByBlockID(ref.BlockID),
		})
		if err != nil {
			if allowDegraded {
				descriptors[ref.BlockID] = domain.TableEmbedDescriptor{
					MWSTableID:    ref.MWSTableID,
					Title:         document.EmbedTitleByBlockID(ref.BlockID),
					DisplayConfig: ref.DisplayConfig,
					PreviewState:  domain.PreviewStateDegraded,
				}
				continue
			}
			return nil, fmt.Errorf("%w: %v", ErrEmbedUnavailable, err)
		}
		descriptors[ref.BlockID] = descriptor
	}
	return descriptors, nil
}
