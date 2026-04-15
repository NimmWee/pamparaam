package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/memory"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type CommitCollaborativeRevisionInput struct {
	PageID         string
	BaseRevisionNo int64
	PatchID        string
	Document       domain.Document
	ActorUserID    string
	WorkspaceID    string
	ActorRoles     []string
	Authenticated  bool
}

type CommitCollaborativeRevisionPayload struct {
	AcceptedRevisionID string
	AcceptedRevisionNo int64
	DocumentHash       string
}

type CommitCollaborativeRevision struct {
	store         ports.Store
	replayWindow  ports.ReplayWindowStore
	embedResolver ports.EmbedResolver
	fileMetadata  ports.FileMetadataResolver
	authorizer    *PageActionAuthorizer
	now           func() time.Time
	nextID        func() string
}

func NewCommitCollaborativeRevision(store ports.Store, replayWindow ports.ReplayWindowStore, embedResolver ports.EmbedResolver, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer, now func() time.Time, nextID func() string) *CommitCollaborativeRevision {
	return &CommitCollaborativeRevision{store: store, replayWindow: replayWindow, embedResolver: embedResolver, fileMetadata: fileMetadata, authorizer: authorizer, now: now, nextID: nextID}
}

func (u *CommitCollaborativeRevision) Execute(ctx context.Context, input CommitCollaborativeRevisionInput) (CommitCollaborativeRevisionPayload, error) {
	if strings.TrimSpace(input.PageID) == "" || input.BaseRevisionNo <= 0 {
		return CommitCollaborativeRevisionPayload{}, ErrValidation
	}
	subject := AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionPageEdit, subject); err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}
	if err := u.authorizer.AuthorizeEmbedUsage(ctx, input.Document, subject); err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}

	if key := strings.TrimSpace(input.PatchID); key != "" {
		record, found, err := u.store.GetDraftIdempotency(ctx, input.PageID, collabIdempotencyKey(key))
		if err == nil && found {
			revision, err := u.store.GetRevisionByID(ctx, input.PageID, record.RevisionID)
			if err == nil {
				return CommitCollaborativeRevisionPayload{
					AcceptedRevisionID: revision.ID,
					AcceptedRevisionNo: revision.RevisionNo,
					DocumentHash:       hashDocument(revision.Document),
				}, nil
			}
		}
	}

	page, err := u.store.GetPage(ctx, input.PageID)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return CommitCollaborativeRevisionPayload{}, ErrPageNotFound
	}
	if err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}
	if err := ensurePageMutable(page); err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}
	if page.CurrentDraftRevisionNo != input.BaseRevisionNo {
		return CommitCollaborativeRevisionPayload{}, &RebaseRequiredError{Payload: domain.ConflictPayload{
			Reason:           "stale_revision",
			LatestRevisionNo: page.CurrentDraftRevisionNo,
			LatestRevisionID: page.CurrentDraftRevisionID,
		}}
	}

	baseRevision, err := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
	if err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}

	now := u.now().UTC()
	document := input.Document.CanonicalSnapshot()
	document, err = newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, document, input.PageID, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return CommitCollaborativeRevisionPayload{}, err
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
		CreatedVia:     domain.RevisionSourceCollabPatch,
		CreatedAt:      now,
	}

	updatedPage := page
	updatedPage.CurrentDraftRevisionID = acceptedRevision.ID
	updatedPage.CurrentDraftRevisionNo = acceptedRevision.RevisionNo
	updatedPage.UpdatedBy = input.ActorUserID
	updatedPage.UpdatedAt = now

	refs := document.ExtractReferences(input.PageID, acceptedRevision.ID, now, u.nextID)
	draftEvent, err := buildDraftSavedEvent(u.nextID, updatedPage, acceptedRevision, refs, now)
	if err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}
	refEvents, err := buildReferenceEvents(u.nextID, updatedPage, acceptedRevision, refs, now)
	if err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}

	idempotency := (*domain.DraftIdempotencyRecord)(nil)
	if key := strings.TrimSpace(input.PatchID); key != "" {
		idempotency = &domain.DraftIdempotencyRecord{
			PageID:         input.PageID,
			IdempotencyKey: collabIdempotencyKey(key),
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
		return CommitCollaborativeRevisionPayload{}, &RebaseRequiredError{Payload: domain.ConflictPayload{
			Reason:           "stale_revision",
			LatestRevisionNo: page.CurrentDraftRevisionNo,
			LatestRevisionID: page.CurrentDraftRevisionID,
		}}
	}
	if err != nil {
		return CommitCollaborativeRevisionPayload{}, err
	}
	_ = recordReplayWindow(ctx, u.replayWindow, domain.ReplayWindowEntry{
		PageID:      input.PageID,
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		RevisionID:  acceptedRevision.ID,
		RevisionNo:  acceptedRevision.RevisionNo,
		Kind:        domain.ReplayEventCollabPatch,
		PatchID:     strings.TrimSpace(input.PatchID),
		CreatedAt:   now,
	})

	return CommitCollaborativeRevisionPayload{
		AcceptedRevisionID: acceptedRevision.ID,
		AcceptedRevisionNo: acceptedRevision.RevisionNo,
		DocumentHash:       hashDocument(document),
	}, nil
}

func hashDocument(document domain.Document) string {
	payload, err := json.Marshal(document)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func collabIdempotencyKey(patchID string) string {
	return "collab:" + patchID
}
