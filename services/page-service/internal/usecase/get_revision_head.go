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

type RevisionHeadPayload struct {
	PageID      string
	WorkspaceID string
	RevisionID  string
	RevisionNo  int64
	Document    domain.Document
}

type GetRevisionHead struct {
	store        ports.Store
	fileMetadata ports.FileMetadataResolver
	authorizer   *PageActionAuthorizer
}

func NewGetRevisionHead(store ports.Store, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer) *GetRevisionHead {
	return &GetRevisionHead{store: store, fileMetadata: fileMetadata, authorizer: authorizer}
}

func (u *GetRevisionHead) Execute(ctx context.Context, pageID, workspaceID, actorUserID string, roles []string, authenticated bool) (RevisionHeadPayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionPageView, AuthorizationSubject{
		ActorUserID:   actorUserID,
		WorkspaceID:   workspaceID,
		PageID:        pageID,
		Roles:         roles,
		Authenticated: authenticated,
	}); err != nil {
		return RevisionHeadPayload{}, err
	}
	page, err := u.store.GetPage(ctx, pageID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return RevisionHeadPayload{}, ErrPageNotFound
	}
	if err != nil {
		return RevisionHeadPayload{}, err
	}

	revision, err := u.store.GetRevision(ctx, pageID, domain.RevisionViewDraft)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return RevisionHeadPayload{}, ErrPageNotFound
	}
	if err != nil {
		return RevisionHeadPayload{}, err
	}
	revision.Document, err = newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, revision.Document, pageID, workspaceID, actorUserID)
	if err != nil {
		return RevisionHeadPayload{}, err
	}

	return RevisionHeadPayload{
		PageID:      page.ID,
		WorkspaceID: page.WorkspaceID,
		RevisionID:  revision.ID,
		RevisionNo:  revision.RevisionNo,
		Document:    revision.Document,
	}, nil
}
