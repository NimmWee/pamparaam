package usecase

import (
	"encoding/json"
	"time"

	"github.com/mtc/wiki-editor-backend/services/page-service/internal/domain"
)

func buildReferenceEvents(nextID func() string, page domain.Page, revision domain.PageRevision, refs domain.DocumentReferences, occurredAt time.Time) ([]domain.OutboxRecord, error) {
	records := make([]domain.OutboxRecord, 0, 2)

	links := make([]map[string]any, 0, len(refs.PageLinks))
	for _, link := range refs.PageLinks {
		links = append(links, map[string]any{
			"target_page_id": link.TargetPageID,
			"block_id":       link.BlockID,
			"link_kind":      link.LinkKind,
		})
	}
	linksPayload, err := json.Marshal(map[string]any{
		"page_id":      page.ID,
		"workspace_id": page.WorkspaceID,
		"revision_id":  revision.ID,
		"links":        links,
	})
	if err != nil {
		return nil, err
	}
	records = append(records, domain.OutboxRecord{
		ID:            nextID(),
		AggregateType: "page",
		AggregateID:   page.ID,
		EventType:     "page.links.extracted",
		Payload:       linksPayload,
		Status:        domain.OutboxStatusPending,
		CreatedAt:     occurredAt,
		AvailableAt:   occurredAt,
	})

	attachmentIDs := make([]string, 0, len(refs.Attachments))
	for _, attachment := range refs.Attachments {
		attachmentIDs = append(attachmentIDs, attachment.FileID)
	}
	attachmentsPayload, err := json.Marshal(map[string]any{
		"page_id":        page.ID,
		"workspace_id":   page.WorkspaceID,
		"revision_id":    revision.ID,
		"attachment_ids": attachmentIDs,
	})
	if err != nil {
		return nil, err
	}
	records = append(records, domain.OutboxRecord{
		ID:            nextID(),
		AggregateType: "page",
		AggregateID:   page.ID,
		EventType:     "page.attachments.changed",
		Payload:       attachmentsPayload,
		Status:        domain.OutboxStatusPending,
		CreatedAt:     occurredAt,
		AvailableAt:   occurredAt,
	})

	return records, nil
}
