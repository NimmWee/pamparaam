package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mtc/wiki-editor-backend/pkg/authz"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
	"github.com/mtc/wiki-editor-backend/services/page-service/internal/ports"
)

var (
	ErrValidation       = errors.New("validation error")
	ErrEmbedUnavailable = errors.New("embed unavailable")
	ErrPageArchived     = errors.New("page archived")
	slugSanitizer       = regexp.MustCompile(`[^a-z0-9]+`)
)

type CreatePageInput struct {
	WorkspaceID     string
	Title           string
	Slug            string
	InitialDocument domain.Document
	ActorUserID     string
	AccessToken     string
	ActorRoles      []string
	Authenticated   bool
}

type CreatePage struct {
	store         ports.Store
	embedResolver ports.EmbedResolver
	fileMetadata  ports.FileMetadataResolver
	authorizer    *PageActionAuthorizer
	now           func() time.Time
	nextID        func() string
}

func NewCreatePage(store ports.Store, embedResolver ports.EmbedResolver, fileMetadata ports.FileMetadataResolver, authorizer *PageActionAuthorizer, now func() time.Time, nextID func() string) *CreatePage {
	return &CreatePage{store: store, embedResolver: embedResolver, fileMetadata: fileMetadata, authorizer: authorizer, now: now, nextID: nextID}
}

func (u *CreatePage) Execute(ctx context.Context, input CreatePageInput) (domain.PageViewPayload, error) {
	if strings.TrimSpace(input.WorkspaceID) == "" || strings.TrimSpace(input.Title) == "" {
		return domain.PageViewPayload{}, fmt.Errorf("%w: workspace_id and title are required", ErrValidation)
	}
	subject := AuthorizationSubject{
		ActorUserID:   input.ActorUserID,
		WorkspaceID:   input.WorkspaceID,
		Roles:         input.ActorRoles,
		Authenticated: input.Authenticated,
	}
	if err := u.authorizer.Authorize(ctx, authz.ActionPageEdit, subject); err != nil {
		return domain.PageViewPayload{}, err
	}
	if err := u.authorizer.AuthorizeEmbedUsage(ctx, input.InitialDocument, subject); err != nil {
		return domain.PageViewPayload{}, err
	}
	if u.now == nil || u.nextID == nil {
		return domain.PageViewPayload{}, fmt.Errorf("%w: create page use case not configured", ErrValidation)
	}

	now := u.now().UTC()
	pageID := u.nextID()
	revisionID := u.nextID()
	document := input.InitialDocument.CanonicalSnapshot()
	if len(document.Blocks) == 0 {
		document.Blocks = []domain.DocumentBlock{}
	}
	var err error
	document, err = newAttachmentHydrator(u.fileMetadata).ValidateAndHydrate(ctx, document, pageID, input.WorkspaceID, input.ActorUserID)
	if err != nil {
		return domain.PageViewPayload{}, err
	}

	page := domain.Page{
		ID:                     pageID,
		WorkspaceID:            input.WorkspaceID,
		Slug:                   normalizeSlug(input.Slug, input.Title),
		Title:                  input.Title,
		Status:                 domain.PageStatusDraft,
		CreatedBy:              input.ActorUserID,
		UpdatedBy:              input.ActorUserID,
		CurrentDraftRevisionID: revisionID,
		CurrentDraftRevisionNo: 1,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	revision := domain.PageRevision{
		ID:             revisionID,
		PageID:         pageID,
		RevisionNo:     1,
		RevisionKind:   domain.RevisionViewDraft,
		Document:       document,
		ExtractedTitle: input.Title,
		CreatedBy:      input.ActorUserID,
		CreatedVia:     domain.RevisionSourceCreate,
		CreatedAt:      now,
	}

	refs := document.ExtractReferences(pageID, revisionID, now, u.nextID)
	descriptors, err := resolveDocumentEmbeds(ctx, u.embedResolver, actorContext{
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		AccessToken: input.AccessToken,
	}, pageID, document, refs.EmbeddedTables, u.embedResolver == nil)
	if err != nil {
		return domain.PageViewPayload{}, err
	}

	events, err := buildCreateEvents(u.nextID, page, revision, refs, now)
	if err != nil {
		return domain.PageViewPayload{}, err
	}
	refEvents, err := buildReferenceEvents(u.nextID, page, revision, refs, now)
	if err != nil {
		return domain.PageViewPayload{}, err
	}
	events = append(events, refEvents...)

	if err := u.store.Execute(ctx, func(pages ports.PageWriter, projections ports.ProjectionWriter, outbox ports.OutboxWriter) error {
		if err := pages.CreatePage(ctx, page, revision); err != nil {
			return err
		}
		if err := projections.ReplaceEmbeddedTableRefs(ctx, revision.ID, refs.EmbeddedTables); err != nil {
			return err
		}
		if err := projections.ReplaceAttachmentRefs(ctx, revision.ID, refs.Attachments); err != nil {
			return err
		}
		if err := projections.ReplacePageLinks(ctx, revision.ID, refs.PageLinks); err != nil {
			return err
		}
		return outbox.Add(ctx, events)
	}); err != nil {
		return domain.PageViewPayload{}, err
	}

	return domain.BuildPageView(page, revision, descriptors), nil
}

func (u *CreatePage) resolveEmbeds(ctx context.Context, input CreatePageInput, pageID string, document domain.Document, refs []domain.EmbeddedTableReference, allowDegraded bool) (map[string]domain.TableEmbedDescriptor, error) {
	descriptors := make(map[string]domain.TableEmbedDescriptor, len(refs))
	for _, ref := range refs {
		if u.embedResolver == nil {
			return nil, fmt.Errorf("%w: resolver unavailable", ErrEmbedUnavailable)
		}
		descriptor, err := u.embedResolver.Resolve(ctx, ports.ResolveEmbedInput{
			PageID:        pageID,
			WorkspaceID:   input.WorkspaceID,
			ActorUserID:   input.ActorUserID,
			AccessToken:   input.AccessToken,
			AllowDegraded: allowDegraded,
			Reference:     ref,
			StoredTitle:   document.EmbedTitleByBlockID(ref.BlockID),
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEmbedUnavailable, err)
		}
		descriptors[ref.BlockID] = descriptor
	}
	return descriptors, nil
}

func buildCreateEvents(nextID func() string, page domain.Page, revision domain.PageRevision, refs domain.DocumentReferences, occurredAt time.Time) ([]domain.OutboxRecord, error) {
	createdPayload, err := json.Marshal(map[string]any{
		"page_id":      page.ID,
		"workspace_id": page.WorkspaceID,
		"title":        page.Title,
		"revision_no":  revision.RevisionNo,
		"revision_id":  revision.ID,
		"status":       page.Status,
	})
	if err != nil {
		return nil, err
	}

	embeddedTables := make([]map[string]any, 0, len(refs.EmbeddedTables))
	for _, ref := range refs.EmbeddedTables {
		embeddedTables = append(embeddedTables, map[string]any{"mws_table_id": ref.MWSTableID, "block_id": ref.BlockID, "title": revision.Document.EmbedTitleByBlockID(ref.BlockID)})
	}
	attachmentIDs := make([]string, 0, len(refs.Attachments))
	for _, ref := range refs.Attachments {
		attachmentIDs = append(attachmentIDs, ref.FileID)
	}
	canonicalLinks := make([]map[string]any, 0, len(refs.PageLinks))
	for _, ref := range refs.PageLinks {
		targetTitle := ""
		for _, block := range revision.Document.Blocks {
			if block.ID == ref.BlockID && block.Link != nil {
				targetTitle = block.Link.Title
				break
			}
		}
		canonicalLinks = append(canonicalLinks, map[string]any{"target_page_id": ref.TargetPageID, "block_id": ref.BlockID, "target_title": targetTitle})
	}
	embeddedTitles := make([]string, 0, len(refs.EmbeddedTables))
	for _, ref := range refs.EmbeddedTables {
		if title := revision.Document.EmbedTitleByBlockID(ref.BlockID); title != "" {
			embeddedTitles = append(embeddedTitles, title)
		}
	}

	draftPayload, err := json.Marshal(map[string]any{
		"page_id":         page.ID,
		"workspace_id":    page.WorkspaceID,
		"revision_no":     revision.RevisionNo,
		"revision_id":     revision.ID,
		"title":           revision.ExtractedTitle,
		"saved_via":       revision.CreatedVia,
		"canonical_links": canonicalLinks,
		"embedded_tables": embeddedTables,
		"embedded_titles": embeddedTitles,
		"attachment_ids":  attachmentIDs,
		"searchable_text": flattenSearchableText(revision.Document),
		"updated_at":      revision.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, err
	}

	return []domain.OutboxRecord{
		{ID: nextID(), AggregateType: "page", AggregateID: page.ID, EventType: "page.created", Payload: createdPayload, Status: domain.OutboxStatusPending, CreatedAt: occurredAt, AvailableAt: occurredAt},
		{ID: nextID(), AggregateType: "page", AggregateID: page.ID, EventType: "page.draft.saved", Payload: draftPayload, Status: domain.OutboxStatusPending, CreatedAt: occurredAt, AvailableAt: occurredAt},
	}, nil
}

func normalizeSlug(rawSlug, title string) string {
	value := strings.TrimSpace(strings.ToLower(rawSlug))
	if value == "" {
		value = strings.TrimSpace(strings.ToLower(title))
	}
	value = slugSanitizer.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "page"
	}
	return value
}
