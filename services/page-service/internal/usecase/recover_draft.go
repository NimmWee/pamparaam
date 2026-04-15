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

type RecoverDraftInput struct {
	PageID        string
	WorkspaceID   string
	ActorUserID   string
	AccessToken   string
	ActorRoles    []string
	Authenticated bool
}

type RecoverDraft struct {
	store         ports.Store
	embedResolver ports.EmbedResolver
	fileMetadata  ports.FileMetadataResolver
	authorizer    *PageActionAuthorizer
}

func NewRecoverDraft(store ports.Store, embedResolver ports.EmbedResolver, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer) *RecoverDraft {
	return &RecoverDraft{store: store, embedResolver: embedResolver, fileMetadata: fileMetadata, authorizer: authorizer}
}

func (u *RecoverDraft) Execute(ctx context.Context, input RecoverDraftInput) (domain.DraftRecoveryPayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionPageView, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.DraftRecoveryPayload{}, err
	}
	if _, err := u.store.GetPage(ctx, input.PageID); errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.DraftRecoveryPayload{}, ErrPageNotFound
	} else if err != nil {
		return domain.DraftRecoveryPayload{}, err
	}

	revision, err := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
	if err != nil {
		return domain.DraftRecoveryPayload{}, err
	}
	revision.Document, err = newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, revision.Document, input.PageID, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return domain.DraftRecoveryPayload{}, err
	}
	refs, err := u.store.ListEmbeddedTableRefs(ctx, revision.ID)
	if err != nil {
		return domain.DraftRecoveryPayload{}, err
	}
	descriptors, err := resolveDocumentEmbeds(ctx, u.embedResolver, actorContext{WorkspaceID: input.WorkspaceID, ActorUserID: input.ActorUserID, AccessToken: input.AccessToken}, input.PageID, revision.Document, refs, true)
	if err != nil {
		return domain.DraftRecoveryPayload{}, err
	}
	return domain.BuildDraftRecoveryPayload(input.PageID, revision, descriptors), nil
}
