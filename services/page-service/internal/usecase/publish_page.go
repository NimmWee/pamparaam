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

type PublishPageInput struct {
	PageID         string
	BaseRevisionNo int64
	WorkspaceID    string
	ActorUserID    string
	ActorRoles     []string
	Authenticated  bool
}

type PublishPage struct {
	store        ports.Store
	replayWindow ports.ReplayWindowStore
	authorizer   *PageActionAuthorizer
	now          func() time.Time
	nextID       func() string
}

func NewPublishPage(store ports.Store, replayWindow ports.ReplayWindowStore, authorizer *PageActionAuthorizer, now func() time.Time, nextID func() string) *PublishPage {
	ensureMetricsRegistered()
	return &PublishPage{store: store, replayWindow: replayWindow, authorizer: authorizer, now: now, nextID: nextID}
}

func (u *PublishPage) Execute(ctx context.Context, input PublishPageInput) (domain.PagePublishPayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionPagePublish, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.PagePublishPayload{}, err
	}
	page, err := u.store.GetPage(ctx, input.PageID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.PagePublishPayload{}, ErrPageNotFound
	}
	if err != nil {
		return domain.PagePublishPayload{}, err
	}
	if err := ensurePageMutable(page); err != nil {
		return domain.PagePublishPayload{}, err
	}

	if page.CurrentDraftRevisionNo != input.BaseRevisionNo {
		revision, err := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
		if err != nil {
			return domain.PagePublishPayload{}, err
		}
		return domain.PagePublishPayload{}, &RebaseRequiredError{Payload: domain.ConflictPayload{
			Reason:           "stale_revision",
			LatestRevisionNo: revision.RevisionNo,
			LatestRevisionID: revision.ID,
			ServerDocument:   revision.Document,
		}}
	}

	draftRevision, err := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
	if err != nil {
		return domain.PagePublishPayload{}, err
	}
	now := u.now().UTC()
	publishedRevision := domain.PageRevision{
		ID:             u.nextID(),
		PageID:         input.PageID,
		RevisionNo:     domain.NextRevisionNo(page),
		RevisionKind:   domain.RevisionViewPublished,
		BaseRevisionID: draftRevision.ID,
		Document:       draftRevision.Document.CanonicalSnapshot(),
		ExtractedTitle: draftRevision.ExtractedTitle,
		CreatedBy:      input.ActorUserID,
		CreatedVia:     domain.RevisionSourcePublish,
		CreatedAt:      now,
	}

	updatedPage := page
	updatedPage.Status = domain.PageStatusPublished
	updatedPage.Title = draftRevision.ExtractedTitle
	updatedPage.UpdatedBy = input.ActorUserID
	updatedPage.UpdatedAt = now
	updatedPage.CurrentPublishedRevisionID = publishedRevision.ID
	updatedPage.CurrentPublishedRevisionNo = publishedRevision.RevisionNo

	refs := publishedRevision.Document.ExtractReferences(input.PageID, publishedRevision.ID, now, u.nextID)
	publishedEvent, err := buildPublishedEvent(u.nextID, updatedPage, publishedRevision, now)
	if err != nil {
		return domain.PagePublishPayload{}, err
	}
	refEvents, err := buildReferenceEvents(u.nextID, updatedPage, publishedRevision, refs, now)
	if err != nil {
		return domain.PagePublishPayload{}, err
	}

	err = u.store.Execute(ctx, func(pages ports.PageWriter, projections ports.ProjectionWriter, outbox ports.OutboxWriter) error {
		if err := pages.SavePublishedRevision(ctx, input.BaseRevisionNo, updatedPage, publishedRevision); err != nil {
			return err
		}
		if err := projections.ReplaceEmbeddedTableRefs(ctx, publishedRevision.ID, refs.EmbeddedTables); err != nil {
			return err
		}
		if err := projections.ReplaceAttachmentRefs(ctx, publishedRevision.ID, refs.Attachments); err != nil {
			return err
		}
		if err := projections.ReplacePageLinks(ctx, publishedRevision.ID, refs.PageLinks); err != nil {
			return err
		}
		return outbox.Add(ctx, append([]domain.OutboxRecord{publishedEvent}, refEvents...))
	})
	if errors.Is(err, memory.ErrStaleRevision) || errors.Is(err, postgres.ErrStaleRevision) {
		revision, revisionErr := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
		if revisionErr != nil {
			return domain.PagePublishPayload{}, err
		}
		return domain.PagePublishPayload{}, &RebaseRequiredError{Payload: domain.ConflictPayload{
			Reason:           "stale_revision",
			LatestRevisionNo: revision.RevisionNo,
			LatestRevisionID: revision.ID,
			ServerDocument:   revision.Document,
		}}
	}
	if err != nil {
		return domain.PagePublishPayload{}, err
	}
	_ = recordReplayWindow(ctx, u.replayWindow, domain.ReplayWindowEntry{
		PageID:      input.PageID,
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		RevisionID:  publishedRevision.ID,
		RevisionNo:  publishedRevision.RevisionNo,
		Kind:        domain.ReplayEventPublish,
		CreatedAt:   now,
	})

	publishTotal.Inc()
	return domain.PagePublishPayload{
		PageID:              input.PageID,
		PublishedRevisionNo: publishedRevision.RevisionNo,
		PublishedRevisionID: publishedRevision.ID,
		PublishedAt:         now,
	}, nil
}
