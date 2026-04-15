package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/memory"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type AutosaveDraftInput struct {
	PageID         string
	BaseRevisionNo int64
	Document       domain.Document
	IdempotencyKey string
	WorkspaceID    string
	ActorUserID    string
	AccessToken    string
	ActorRoles     []string
	Authenticated  bool
}

type AutosaveDraft struct {
	store         ports.Store
	replayWindow  ports.ReplayWindowStore
	embedResolver ports.EmbedResolver
	fileMetadata  ports.FileMetadataResolver
	authorizer    *PageActionAuthorizer
	now           func() time.Time
	nextID        func() string
}

func NewAutosaveDraft(store ports.Store, replayWindow ports.ReplayWindowStore, embedResolver ports.EmbedResolver, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer, now func() time.Time, nextID func() string) *AutosaveDraft {
	ensureMetricsRegistered()
	return &AutosaveDraft{store: store, replayWindow: replayWindow, embedResolver: embedResolver, fileMetadata: fileMetadata, authorizer: authorizer, now: now, nextID: nextID}
}

func (u *AutosaveDraft) Execute(ctx context.Context, input AutosaveDraftInput) (domain.DraftSavePayload, error) {
	started := time.Now()
	defer func() {
		// Default to success when no panic/error path overrides it.
	}()

	if strings.TrimSpace(input.PageID) == "" || input.BaseRevisionNo <= 0 {
		autosaveDuration.WithLabelValues("validation_error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, ErrValidation
	}
	subject := AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionPageEdit, subject); err != nil {
		autosaveDuration.WithLabelValues("forbidden").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}
	if err := u.authorizer.AuthorizeEmbedUsage(ctx, input.Document, subject); err != nil {
		autosaveDuration.WithLabelValues("forbidden").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}

	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		record, found, err := u.store.GetDraftIdempotency(ctx, input.PageID, key)
		if err == nil && found {
			revision, err := u.store.GetRevisionByID(ctx, input.PageID, record.RevisionID)
			if err == nil {
				refs, _ := u.store.ListEmbeddedTableRefs(ctx, revision.ID)
				descriptors, _ := resolveDocumentEmbeds(ctx, u.embedResolver, actorContext{WorkspaceID: input.WorkspaceID, ActorUserID: input.ActorUserID, AccessToken: input.AccessToken}, input.PageID, revision.Document, refs, true)
				autosaveDuration.WithLabelValues("idempotent").Observe(time.Since(started).Seconds())
				return domain.BuildDraftSavePayload(input.PageID, revision, descriptors), nil
			}
		}
	}

	page, err := u.store.GetPage(ctx, input.PageID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		autosaveDuration.WithLabelValues("not_found").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, ErrPageNotFound
	}
	if err != nil {
		autosaveDuration.WithLabelValues("error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}
	if err := ensurePageMutable(page); err != nil {
		autosaveDuration.WithLabelValues("validation_error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}

	if page.CurrentDraftRevisionNo != input.BaseRevisionNo {
		payload, err := u.buildConflictPayload(ctx, input.PageID)
		if err != nil {
			autosaveDuration.WithLabelValues("conflict").Observe(time.Since(started).Seconds())
			autosaveConflicts.Inc()
			return domain.DraftSavePayload{}, err
		}
		autosaveDuration.WithLabelValues("conflict").Observe(time.Since(started).Seconds())
		autosaveConflicts.Inc()
		return domain.DraftSavePayload{}, &RebaseRequiredError{Payload: payload}
	}

	baseRevision, err := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
	if err != nil {
		autosaveDuration.WithLabelValues("error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}

	now := u.now().UTC()
	document := input.Document.CanonicalSnapshot()
	document, err = newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, document, input.PageID, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		autosaveDuration.WithLabelValues("validation_error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}
	acceptedRevision := domain.PageRevision{
		ID:             u.nextID(),
		PageID:         input.PageID,
		RevisionNo:     domain.NextRevisionNo(page),
		RevisionKind:   domain.RevisionViewDraft,
		BaseRevisionID: baseRevision.ID,
		Document:       document,
		ExtractedTitle: page.Title,
		CreatedBy:      input.ActorUserID,
		CreatedVia:     domain.RevisionSourceAutosave,
		CreatedAt:      now,
	}
	updatedPage := page
	updatedPage.CurrentDraftRevisionID = acceptedRevision.ID
	updatedPage.CurrentDraftRevisionNo = acceptedRevision.RevisionNo
	updatedPage.UpdatedBy = input.ActorUserID
	updatedPage.UpdatedAt = now

	refs := document.ExtractReferences(input.PageID, acceptedRevision.ID, now, u.nextID)
	descriptors, err := resolveDocumentEmbeds(ctx, u.embedResolver, actorContext{WorkspaceID: input.WorkspaceID, ActorUserID: input.ActorUserID, AccessToken: input.AccessToken}, input.PageID, document, refs.EmbeddedTables, u.embedResolver == nil)
	if err != nil {
		autosaveDuration.WithLabelValues("embed_unavailable").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}

	draftEvent, err := buildDraftSavedEvent(u.nextID, updatedPage, acceptedRevision, refs, now)
	if err != nil {
		autosaveDuration.WithLabelValues("error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}
	refEvents, err := buildReferenceEvents(u.nextID, updatedPage, acceptedRevision, refs, now)
	if err != nil {
		autosaveDuration.WithLabelValues("error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}

	idempotency := (*domain.DraftIdempotencyRecord)(nil)
	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		idempotency = &domain.DraftIdempotencyRecord{
			PageID:         input.PageID,
			IdempotencyKey: key,
			RevisionID:     acceptedRevision.ID,
			RevisionNo:     acceptedRevision.RevisionNo,
			CreatedAt:      now,
		}
	}

	err = u.store.Execute(ctx, func(pages ports.PageWriter, projections ports.ProjectionWriter, outbox ports.OutboxWriter) error {
		if err := pages.SaveDraftRevision(ctx, input.BaseRevisionNo, updatedPage, acceptedRevision, idempotency); err != nil {
			return err
		}
		if err := projections.ReplaceEmbeddedTableRefs(ctx, acceptedRevision.ID, refs.EmbeddedTables); err != nil {
			return err
		}
		if err := projections.ReplaceAttachmentRefs(ctx, acceptedRevision.ID, refs.Attachments); err != nil {
			return err
		}
		if err := projections.ReplacePageLinks(ctx, acceptedRevision.ID, refs.PageLinks); err != nil {
			return err
		}
		records := append([]domain.OutboxRecord{draftEvent}, refEvents...)
		return outbox.Add(ctx, records)
	})
	if errors.Is(err, memory.ErrStaleRevision) || errors.Is(err, postgres.ErrStaleRevision) {
		payload, payloadErr := u.buildConflictPayload(ctx, input.PageID)
		if payloadErr != nil {
			autosaveDuration.WithLabelValues("conflict").Observe(time.Since(started).Seconds())
			autosaveConflicts.Inc()
			return domain.DraftSavePayload{}, err
		}
		autosaveDuration.WithLabelValues("conflict").Observe(time.Since(started).Seconds())
		autosaveConflicts.Inc()
		return domain.DraftSavePayload{}, &RebaseRequiredError{Payload: payload}
	}
	if err != nil {
		autosaveDuration.WithLabelValues("error").Observe(time.Since(started).Seconds())
		return domain.DraftSavePayload{}, err
	}
	_ = recordReplayWindow(ctx, u.replayWindow, domain.ReplayWindowEntry{
		PageID:      input.PageID,
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		RevisionID:  acceptedRevision.ID,
		RevisionNo:  acceptedRevision.RevisionNo,
		Kind:        domain.ReplayEventAutosave,
		CreatedAt:   now,
	})

	autosaveDuration.WithLabelValues("success").Observe(time.Since(started).Seconds())
	return domain.BuildDraftSavePayload(input.PageID, acceptedRevision, descriptors), nil
}

func (u *AutosaveDraft) buildConflictPayload(ctx context.Context, pageID string) (domain.ConflictPayload, error) {
	revision, err := u.store.GetRevision(ctx, pageID, domain.RevisionViewDraft)
	if err != nil {
		return domain.ConflictPayload{}, err
	}
	return domain.ConflictPayload{
		Reason:           "stale_revision",
		LatestRevisionNo: revision.RevisionNo,
		LatestRevisionID: revision.ID,
		ServerDocument:   revision.Document,
	}, nil
}
