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

type ArchivePageInput struct {
	PageID         string
	BaseRevisionNo int64
	WorkspaceID    string
	ActorUserID    string
	ActorRoles     []string
	Authenticated  bool
}

type ArchivePage struct {
	store      ports.Store
	authorizer *PageActionAuthorizer
	now        func() time.Time
	nextID     func() string
}

func NewArchivePage(store ports.Store, authorizer *PageActionAuthorizer, now func() time.Time, nextID func() string) *ArchivePage {
	return &ArchivePage{store: store, authorizer: authorizer, now: now, nextID: nextID}
}

func (u *ArchivePage) Execute(ctx context.Context, input ArchivePageInput) (domain.PageArchivePayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionPageArchive, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.PageArchivePayload{}, err
	}

	page, err := u.store.GetPage(ctx, input.PageID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.PageArchivePayload{}, ErrPageNotFound
	}
	if err != nil {
		return domain.PageArchivePayload{}, err
	}
	if page.CurrentDraftRevisionNo != input.BaseRevisionNo {
		revision, revisionErr := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
		if revisionErr != nil {
			return domain.PageArchivePayload{}, err
		}
		return domain.PageArchivePayload{}, &RebaseRequiredError{Payload: domain.ConflictPayload{
			Reason:           "stale_revision",
			LatestRevisionNo: revision.RevisionNo,
			LatestRevisionID: revision.ID,
			ServerDocument:   revision.Document,
		}}
	}
	if page.Status == domain.PageStatusArchived {
		return domain.PageArchivePayload{
			PageID:                 page.ID,
			Status:                 page.Status,
			CurrentDraftRevisionNo: page.CurrentDraftRevisionNo,
			CurrentDraftRevisionID: page.CurrentDraftRevisionID,
			ArchivedAt:             page.UpdatedAt,
		}, nil
	}

	now := u.now().UTC()
	page.Status = domain.PageStatusArchived
	page.UpdatedBy = input.ActorUserID
	page.UpdatedAt = now

	archivedEvent, err := buildArchivedEvent(u.nextID, page, now)
	if err != nil {
		return domain.PageArchivePayload{}, err
	}

	err = u.store.Execute(ctx, func(pages ports.PageWriter, _ ports.ProjectionWriter, outbox ports.OutboxWriter) error {
		if err := pages.ArchivePage(ctx, input.BaseRevisionNo, page); err != nil {
			return err
		}
		return outbox.Add(ctx, []domain.OutboxRecord{archivedEvent})
	})
	if errors.Is(err, memory.ErrStaleRevision) || errors.Is(err, postgres.ErrStaleRevision) {
		revision, revisionErr := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
		if revisionErr != nil {
			return domain.PageArchivePayload{}, err
		}
		return domain.PageArchivePayload{}, &RebaseRequiredError{Payload: domain.ConflictPayload{
			Reason:           "stale_revision",
			LatestRevisionNo: revision.RevisionNo,
			LatestRevisionID: revision.ID,
			ServerDocument:   revision.Document,
		}}
	}
	if err != nil {
		return domain.PageArchivePayload{}, err
	}

	return domain.PageArchivePayload{
		PageID:                 page.ID,
		Status:                 page.Status,
		CurrentDraftRevisionNo: page.CurrentDraftRevisionNo,
		CurrentDraftRevisionID: page.CurrentDraftRevisionID,
		ArchivedAt:             now,
	}, nil
}
