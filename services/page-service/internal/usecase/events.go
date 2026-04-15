package usecase

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

func buildDraftSavedEvent(nextID func() string, page domain.Page, revision domain.PageRevision, refs domain.DocumentReferences, occurredAt time.Time) (domain.OutboxRecord, error) {
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

	payload, err := json.Marshal(map[string]any{
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
		return domain.OutboxRecord{}, err
	}
	return domain.OutboxRecord{
		ID:            nextID(),
		AggregateType: "page",
		AggregateID:   page.ID,
		EventType:     "page.draft.saved",
		Payload:       payload,
		Status:        domain.OutboxStatusPending,
		CreatedAt:     occurredAt,
		AvailableAt:   occurredAt,
	}, nil
}

func buildPublishedEvent(nextID func() string, page domain.Page, revision domain.PageRevision, occurredAt time.Time) (domain.OutboxRecord, error) {
	refs := revision.Document.ExtractReferences(page.ID, revision.ID, occurredAt, nextID)
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
	payload, err := json.Marshal(map[string]any{
		"page_id":               page.ID,
		"workspace_id":          page.WorkspaceID,
		"published_revision_no": revision.RevisionNo,
		"published_revision_id": revision.ID,
		"title":                 revision.ExtractedTitle,
		"canonical_links":       canonicalLinks,
		"embedded_tables":       embeddedTables,
		"embedded_titles":       embeddedTitles,
		"attachment_ids":        attachmentIDs,
		"searchable_text":       flattenSearchableText(revision.Document),
		"updated_at":            revision.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return domain.OutboxRecord{}, err
	}
	return domain.OutboxRecord{
		ID:            nextID(),
		AggregateType: "page",
		AggregateID:   page.ID,
		EventType:     "page.published",
		Payload:       payload,
		Status:        domain.OutboxStatusPending,
		CreatedAt:     occurredAt,
		AvailableAt:   occurredAt,
	}, nil
}

func buildArchivedEvent(nextID func() string, page domain.Page, occurredAt time.Time) (domain.OutboxRecord, error) {
	payload, err := json.Marshal(map[string]any{
		"page_id":             page.ID,
		"workspace_id":        page.WorkspaceID,
		"status":              page.Status,
		"current_revision_no": page.CurrentDraftRevisionNo,
		"current_revision_id": page.CurrentDraftRevisionID,
		"updated_at":          page.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return domain.OutboxRecord{}, err
	}
	return domain.OutboxRecord{
		ID:            nextID(),
		AggregateType: "page",
		AggregateID:   page.ID,
		EventType:     "page.archived",
		Payload:       payload,
		Status:        domain.OutboxStatusPending,
		CreatedAt:     occurredAt,
		AvailableAt:   occurredAt,
	}, nil
}

func flattenSearchableText(document domain.Document) string {
	parts := make([]string, 0, len(document.Blocks))
	for _, block := range document.Blocks {
		if block.Text != "" {
			parts = append(parts, block.Text)
		}
		if block.Link != nil && block.Link.Title != "" {
			parts = append(parts, block.Link.Title)
		}
		if block.Attachment != nil && block.Attachment.Filename != "" {
			parts = append(parts, block.Attachment.Filename)
		}
		if block.Embed != nil && block.Embed.Title != "" {
			parts = append(parts, block.Embed.Title)
		}
	}
	return strings.Join(parts, " ")
}
