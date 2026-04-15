package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/memory"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type RestoreRevisionInput struct {
	PageID        string
	RevisionID    string
	WorkspaceID   string
	ActorUserID   string
	AccessToken   string
	ActorRoles    []string
	Authenticated bool
}

type RestoreRevision struct {
	store         ports.Store
	replayWindow  ports.ReplayWindowStore
	embedResolver ports.EmbedResolver
	fileMetadata  ports.FileMetadataResolver
	authorizer    *PageActionAuthorizer
	now           func() time.Time
	nextID        func() string
}

func NewRestoreRevision(store ports.Store, replayWindow ports.ReplayWindowStore, embedResolver ports.EmbedResolver, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer, now func() time.Time, nextID func() string) *RestoreRevision {
	return &RestoreRevision{store: store, replayWindow: replayWindow, embedResolver: embedResolver, fileMetadata: fileMetadata, authorizer: authorizer, now: now, nextID: nextID}
}

func (u *RestoreRevision) Execute(ctx context.Context, input RestoreRevisionInput) (domain.DraftSavePayload, error) {
	subject := AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionPageRestore, subject); err != nil {
		return domain.DraftSavePayload{}, err
	}
	page, err := u.store.GetPage(ctx, input.PageID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.DraftSavePayload{}, ErrPageNotFound
	}
	if err != nil {
		return domain.DraftSavePayload{}, err
	}
	if err := ensurePageMutable(page); err != nil {
		return domain.DraftSavePayload{}, err
	}

	targetRevision, err := u.store.GetRevisionByID(ctx, input.PageID, input.RevisionID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.DraftSavePayload{}, ErrPageNotFound
	}
	if err != nil {
		return domain.DraftSavePayload{}, err
	}

	now := u.now().UTC()
	document, err := newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, targetRevision.Document.CanonicalSnapshot(), input.PageID, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return domain.DraftSavePayload{}, err
	}
	if err := u.authorizer.AuthorizeEmbedUsage(ctx, document, subject); err != nil {
		return domain.DraftSavePayload{}, err
	}
	restoredRevision := domain.PageRevision{
		ID:                     u.nextID(),
		PageID:                 input.PageID,
		RevisionNo:             domain.NextRevisionNo(page),
		RevisionKind:           domain.RevisionViewDraft,
		BaseRevisionID:         page.CurrentDraftRevisionID,
		RestoredFromRevisionID: targetRevision.ID,
		Document:               document,
		ExtractedTitle:         targetRevision.ExtractedTitle,
		CreatedBy:              input.ActorUserID,
		CreatedVia:             domain.RevisionSourceRestore,
		CreatedAt:              now,
	}

	updatedPage := page
	updatedPage.Title = targetRevision.ExtractedTitle
	updatedPage.UpdatedBy = input.ActorUserID
	updatedPage.UpdatedAt = now
	updatedPage.CurrentDraftRevisionID = restoredRevision.ID
	updatedPage.CurrentDraftRevisionNo = restoredRevision.RevisionNo

	refs := restoredRevision.Document.ExtractReferences(input.PageID, restoredRevision.ID, now, u.nextID)
	descriptors, err := resolveDocumentEmbeds(ctx, u.embedResolver, actorContext{WorkspaceID: input.WorkspaceID, ActorUserID: input.ActorUserID, AccessToken: input.AccessToken}, input.PageID, restoredRevision.Document, refs.EmbeddedTables, true)
	if err != nil {
		return domain.DraftSavePayload{}, err
	}
	draftEvent, err := buildDraftSavedEvent(u.nextID, updatedPage, restoredRevision, refs, now)
	if err != nil {
		return domain.DraftSavePayload{}, err
	}
	refEvents, err := buildReferenceEvents(u.nextID, updatedPage, restoredRevision, refs, now)
	if err != nil {
		return domain.DraftSavePayload{}, err
	}

	err = u.store.Execute(ctx, func(pages ports.PageWriter, projections ports.ProjectionWriter, outbox ports.OutboxWriter) error {
		if err := pages.SaveDraftRevision(ctx, page.CurrentDraftRevisionNo, updatedPage, restoredRevision, nil); err != nil {
			return err
		}
		if err := projections.ReplaceEmbeddedTableRefs(ctx, restoredRevision.ID, refs.EmbeddedTables); err != nil {
			return err
		}
		if err := projections.ReplaceAttachmentRefs(ctx, restoredRevision.ID, refs.Attachments); err != nil {
			return err
		}
		if err := projections.ReplacePageLinks(ctx, restoredRevision.ID, refs.PageLinks); err != nil {
			return err
		}
		records := append([]domain.OutboxRecord{draftEvent}, refEvents...)
		return outbox.Add(ctx, records)
	})
	if err != nil {
		return domain.DraftSavePayload{}, err
	}
	_ = recordReplayWindow(ctx, u.replayWindow, domain.ReplayWindowEntry{
		PageID:      input.PageID,
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		RevisionID:  restoredRevision.ID,
		RevisionNo:  restoredRevision.RevisionNo,
		Kind:        domain.ReplayEventRestore,
		CreatedAt:   now,
	})

	return domain.BuildDraftSavePayload(input.PageID, restoredRevision, descriptors), nil
}
