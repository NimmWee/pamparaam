package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/memory"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/adapters/postgres"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

type ResumeEditorSyncInput struct {
	PageID              string
	LastKnownRevisionNo int64
	PendingPatchIDs     []string
	ResumeToken         string
	WorkspaceID         string
	ActorUserID         string
	AccessToken         string
	ActorRoles          []string
	Authenticated       bool
}

type ResumeEditorSync struct {
	store         ports.Store
	replayWindow  ports.ReplayWindowStore
	embedResolver ports.EmbedResolver
	fileMetadata  ports.FileMetadataResolver
	authorizer    *PageActionAuthorizer
}

func NewResumeEditorSync(store ports.Store, replayWindow ports.ReplayWindowStore, embedResolver ports.EmbedResolver, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer) *ResumeEditorSync {
	return &ResumeEditorSync{
		store:         store,
		replayWindow:  replayWindow,
		embedResolver: embedResolver,
		fileMetadata:  fileMetadata,
		authorizer:    authorizer,
	}
}

func (u *ResumeEditorSync) Execute(ctx context.Context, input ResumeEditorSyncInput) (domain.EditorSyncResumePayload, error) {
	if err := u.authorizer.Authorize(ctx, authz.ActionPageView, AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		PageID:        input.PageID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}); err != nil {
		return domain.EditorSyncResumePayload{}, err
	}

	if _, err := u.store.GetPage(ctx, input.PageID); errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.EditorSyncResumePayload{}, ErrPageNotFound
	} else if err != nil {
		return domain.EditorSyncResumePayload{}, err
	}

	revision, err := u.store.GetRevision(ctx, input.PageID, domain.RevisionViewDraft)
	if errors.Is(err, memory.ErrNotFound) || errors.Is(err, postgres.ErrNotFound) {
		return domain.EditorSyncResumePayload{}, ErrPageNotFound
	}
	if err != nil {
		return domain.EditorSyncResumePayload{}, err
	}

	replayEntries, err := u.listReplayEntries(ctx, input.PageID, input.LastKnownRevisionNo)
	if err != nil {
		return domain.EditorSyncResumePayload{}, err
	}
	replayedPatchIDs, missingPatchIDs := u.classifyPendingPatchIDs(ctx, input.PageID, input.PendingPatchIDs, replayEntries)
	resumeToken := buildResumeToken(input.PageID, revision, replayEntries)
	if u.canResumeWithoutReload(input, revision, replayEntries, replayedPatchIDs) {
		return domain.EditorSyncResumePayload{
			PageID:               input.PageID,
			Mode:                 "resume",
			CurrentRevisionNo:    revision.RevisionNo,
			CurrentRevisionID:    revision.ID,
			MissingPatchIDs:      missingPatchIDs,
			ReplayWindowPatchIDs: replayedPatchIDs,
			ResumeToken:          resumeToken,
		}, nil
	}

	revision.Document, err = newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, revision.Document, input.PageID, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return domain.EditorSyncResumePayload{}, err
	}
	refs, err := u.store.ListEmbeddedTableRefs(ctx, revision.ID)
	if err != nil {
		return domain.EditorSyncResumePayload{}, err
	}
	descriptors, err := resolveDocumentEmbeds(ctx, u.embedResolver, actorContext{
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		AccessToken: input.AccessToken,
	}, input.PageID, revision.Document, refs, true)
	if err != nil {
		return domain.EditorSyncResumePayload{}, err
	}
	document := revision.Document.HydrateEmbeds(descriptors)

	return domain.EditorSyncResumePayload{
		PageID:               input.PageID,
		Mode:                 "replace",
		CurrentRevisionNo:    revision.RevisionNo,
		CurrentRevisionID:    revision.ID,
		Document:             &document,
		MissingPatchIDs:      missingPatchIDs,
		ReplayWindowPatchIDs: replayedPatchIDs,
		ResumeToken:          resumeToken,
	}, nil
}

func (u *ResumeEditorSync) classifyPendingPatchIDs(ctx context.Context, pageID string, pendingPatchIDs []string, replayEntries []domain.ReplayWindowEntry) ([]string, []string) {
	replayed := make([]string, 0, len(pendingPatchIDs))
	missing := make([]string, 0, len(pendingPatchIDs))
	seen := make(map[string]struct{}, len(pendingPatchIDs))
	replayedSet := make(map[string]struct{}, len(replayEntries))
	for _, entry := range replayEntries {
		if patchID := strings.TrimSpace(entry.PatchID); patchID != "" {
			replayedSet[patchID] = struct{}{}
		}
	}

	for _, patchID := range pendingPatchIDs {
		patchID = strings.TrimSpace(patchID)
		if patchID == "" {
			continue
		}
		if _, ok := seen[patchID]; ok {
			continue
		}
		seen[patchID] = struct{}{}

		if _, ok := replayedSet[patchID]; ok {
			replayed = append(replayed, patchID)
			continue
		}
		if _, found, err := u.store.GetDraftIdempotency(ctx, pageID, collabIdempotencyKey(patchID)); err == nil && found {
			replayed = append(replayed, patchID)
			continue
		}
		missing = append(missing, patchID)
	}

	return replayed, missing
}

func (u *ResumeEditorSync) listReplayEntries(ctx context.Context, pageID string, fromRevisionNo int64) ([]domain.ReplayWindowEntry, error) {
	if u.replayWindow == nil {
		return nil, nil
	}
	return u.replayWindow.ListSinceRevision(ctx, pageID, fromRevisionNo)
}

func (u *ResumeEditorSync) canResumeWithoutReload(input ResumeEditorSyncInput, revision domain.PageRevision, replayEntries []domain.ReplayWindowEntry, replayedPatchIDs []string) bool {
	token, hasToken := parseResumeToken(strings.TrimSpace(input.ResumeToken))
	if input.LastKnownRevisionNo == revision.RevisionNo {
		if !hasToken {
			return true
		}
		return token.PageID == input.PageID && token.RevisionNo == input.LastKnownRevisionNo
	}
	if !hasToken || token.PageID != input.PageID || token.RevisionNo != input.LastKnownRevisionNo {
		return false
	}
	if len(replayEntries) == 0 {
		return false
	}
	replayedSet := make(map[string]struct{}, len(replayedPatchIDs))
	for _, patchID := range replayedPatchIDs {
		replayedSet[patchID] = struct{}{}
	}
	for _, entry := range replayEntries {
		if entry.Kind != domain.ReplayEventCollabPatch {
			return false
		}
		if _, ok := replayedSet[strings.TrimSpace(entry.PatchID)]; !ok {
			return false
		}
	}
	return true
}
