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

var ErrPageNotFound = errors.New("page not found")

type GetPageInput struct {
	PageID        string
	View          domain.RevisionView
	WorkspaceID   string
	ActorUserID   string
	AccessToken   string
	ActorRoles    []string
	Authenticated bool
}

type GetPage struct {
	store         ports.Store
	embedResolver ports.EmbedResolver
	fileMetadata  ports.FileMetadataResolver
	authorizer    *PageActionAuthorizer
}

func NewGetPage(store ports.Store, embedResolver ports.EmbedResolver, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer) *GetPage {
	return &GetPage{store: store, embedResolver: embedResolver, fileMetadata: fileMetadata, authorizer: authorizer}
}

func (u *GetPage) Execute(ctx context.Context, input GetPageInput) (domain.PageViewPayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionPageView, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.PageViewPayload{}, err
	}
	page, err := u.store.GetPage(ctx, input.PageID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.PageViewPayload{}, ErrPageNotFound
	}
	if err != nil {
		return domain.PageViewPayload{}, err
	}

	revision, err := u.store.GetRevision(ctx, input.PageID, input.View)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.PageViewPayload{}, ErrPageNotFound
	}
	if err != nil {
		return domain.PageViewPayload{}, err
	}

	refs, err := u.store.ListEmbeddedTableRefs(ctx, revision.ID)
	if err != nil {
		return domain.PageViewPayload{}, err
	}
	revision.Document, err = newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, revision.Document, input.PageID, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return domain.PageViewPayload{}, err
	}

	descriptors := make(map[string]domain.TableEmbedDescriptor, len(refs))
	for _, ref := range refs {
		degraded := domain.TableEmbedDescriptor{
			MWSTableID:    ref.MWSTableID,
			Title:         revision.Document.EmbedTitleByBlockID(ref.BlockID),
			DisplayConfig: ref.DisplayConfig,
			PreviewState:  domain.PreviewStateDegraded,
		}
		if u.embedResolver == nil {
			descriptors[ref.BlockID] = degraded
			continue
		}

		descriptor, err := u.embedResolver.Resolve(ctx, ports.ResolveEmbedInput{
			PageID:        input.PageID,
			WorkspaceID:   input.WorkspaceID,
			ActorUserID:   input.ActorUserID,
			AccessToken:   input.AccessToken,
			AllowDegraded: true,
			Reference:     ref,
			StoredTitle:   revision.Document.EmbedTitleByBlockID(ref.BlockID),
		})
		if err != nil {
			descriptor = degraded
		}
		descriptors[ref.BlockID] = descriptor
	}

	return domain.BuildPageView(page, revision, descriptors), nil
}
